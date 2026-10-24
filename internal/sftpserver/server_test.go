package sftpserver

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

type fakeBackend struct {
	root           string
	denied         atomic.Bool
	noRead, noList bool
	calls          atomic.Int32
}

func (b *fakeBackend) Authenticate(_ context.Context, r AuthRequest) (Access, error) {
	b.calls.Add(1)
	if r.Pass != "correct" && r.Type != "public_key" {
		return Access{}, os.ErrPermission
	}
	permissions := []string{"file.sftp"}
	if !b.noRead {
		permissions = append(permissions, "file.read-content")
	}
	if !b.noList {
		permissions = append(permissions, "file.read")
	}
	return Access{Server: "server", User: "user", Permissions: permissions, Issued: time.Now()}, nil
}
func (b *fakeBackend) Allowed(Access) bool { return !b.denied.Load() }
func (b *fakeBackend) Read(_ context.Context, _ Access, _ string) (ReadHandle, error) {
	return os.Open(filepath.Join(b.root, "data"))
}
func (b *fakeBackend) List(context.Context, Access, string) ([]os.FileInfo, error) {
	f, e := os.Stat(filepath.Join(b.root, "data"))
	return []os.FileInfo{f}, e
}
func (b *fakeBackend) Stat(context.Context, Access, string) (os.FileInfo, error) {
	return os.Stat(filepath.Join(b.root, "data"))
}
func fixture(t *testing.T) (*Server, *fakeBackend, *ssh.ClientConfig) {
	t.Helper()
	root := t.TempDir()
	if e := os.WriteFile(filepath.Join(root, "data"), bytes.Repeat([]byte{0, 255, 42}, 1<<18), 0600); e != nil {
		t.Fatal(e)
	}
	_, key, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	signer, e := ssh.NewSignerFromKey(key)
	if e != nil {
		t.Fatal(e)
	}
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	b := &fakeBackend{root: root}
	s := New(l, signer, b)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if e := s.Close(ctx); e != nil {
			t.Error(e)
		}
	})
	cfg := &ssh.ClientConfig{User: "user.12345678", Auth: []ssh.AuthMethod{ssh.Password("correct")}, HostKeyCallback: ssh.FixedHostKey(signer.PublicKey()), Timeout: 3 * time.Second}
	return s, b, cfg
}
func connect(t *testing.T, s *Server, cfg *ssh.ClientConfig) (*ssh.Client, *sftp.Client) {
	t.Helper()
	c, e := ssh.Dial("tcp", s.Addr().String(), cfg)
	if e != nil {
		t.Fatal(e)
	}
	f, e := sftp.NewClient(c)
	if e != nil {
		c.Close()
		t.Fatal(e)
	}
	t.Cleanup(func() { f.Close(); c.Close() })
	return c, f
}

func TestSFTPEncryptedReadOnlyProtocol(t *testing.T) {
	s, b, cfg := fixture(t)
	_, c := connect(t, s, cfg)
	items, e := c.ReadDir("/")
	if e != nil || len(items) != 1 || items[0].Name() != "data" {
		t.Fatal(items, e)
	}
	f, e := c.Open("data")
	if e != nil {
		t.Fatal(e)
	}
	got, e := io.ReadAll(f)
	f.Close()
	want, _ := os.ReadFile(filepath.Join(b.root, "data"))
	if e != nil || !bytes.Equal(got, want) {
		t.Fatal(e, "content mismatch")
	}
	if _, e = c.Stat("data"); e != nil {
		t.Fatal(e)
	}
	if _, e = c.Getwd(); e != nil {
		t.Fatal(e)
	}
}
func TestSFTPRefusesEveryWriteFamily(t *testing.T) {
	for _, kind := range []string{"create", "read-write", "truncate", "mkdir", "remove", "rmdir", "rename", "posix-rename", "chmod", "chown", "symlink", "hardlink"} {
		t.Run(kind, func(t *testing.T) {
			s, b, cfg := fixture(t)
			_, c := connect(t, s, cfg)
			var e error
			switch kind {
			case "create":
				var f *sftp.File
				f, e = c.Create("new")
				if f != nil {
					f.Close()
				}
			case "read-write":
				var f *sftp.File
				f, e = c.OpenFile("data", os.O_RDWR)
				if f != nil {
					f.Close()
				}
			case "truncate":
				e = c.Truncate("data", 1)
			case "mkdir":
				e = c.Mkdir("new")
			case "remove":
				e = c.Remove("data")
			case "rmdir":
				e = c.RemoveDirectory("dir")
			case "rename":
				e = c.Rename("data", "new")
			case "posix-rename":
				e = c.PosixRename("data", "new")
			case "chmod":
				e = c.Chmod("data", 0777)
			case "chown":
				e = c.Chown("data", 1, 1)
			case "symlink":
				e = c.Symlink("data", "new")
			case "hardlink":
				e = c.Link("data", "new")
			}
			if e == nil {
				t.Fatal("mutation accepted")
			}
			info, e := os.Stat(filepath.Join(b.root, "data"))
			if e != nil || info.Size() != 3<<18 {
				t.Fatal(e, "data changed")
			}
		})
	}
}
func TestSFTPNoShellExecOrForwarding(t *testing.T) {
	s, _, cfg := fixture(t)
	c, e := ssh.Dial("tcp", s.Addr().String(), cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	session, e := c.NewSession()
	if e != nil {
		t.Fatal(e)
	}
	defer session.Close()
	if e = session.Shell(); e == nil {
		t.Fatal("shell accepted")
	}
	if e = session.Run("echo forbidden"); e == nil {
		t.Fatal("exec accepted")
	}
	if _, _, e = c.OpenChannel("direct-tcpip", nil); e == nil {
		t.Fatal("forwarding accepted")
	}
}
func TestSFTPAuthenticationFailureAndHostPin(t *testing.T) {
	for _, kind := range []string{"password", "hostkey"} {
		t.Run(kind, func(t *testing.T) {
			s, _, cfg := fixture(t)
			if kind == "password" {
				cfg.Auth = []ssh.AuthMethod{ssh.Password("wrong")}
			} else {
				_, k, _ := ed25519.GenerateKey(rand.Reader)
				signer, _ := ssh.NewSignerFromKey(k)
				cfg.HostKeyCallback = ssh.FixedHostKey(signer.PublicKey())
			}
			if c, e := ssh.Dial("tcp", s.Addr().String(), cfg); e == nil {
				c.Close()
				t.Fatal("authentication accepted")
			}
		})
	}
}
func TestSFTPPerOperationReadPermissions(t *testing.T) {
	for _, kind := range []string{"no-read", "no-list"} {
		t.Run(kind, func(t *testing.T) {
			s, b, cfg := fixture(t)
			if kind == "no-read" {
				b.noRead = true
			} else {
				b.noList = true
			}
			_, c := connect(t, s, cfg)
			if kind == "no-read" {
				if f, e := c.Open("data"); e == nil {
					f.Close()
					t.Fatal("read accepted")
				}
				if _, e := c.ReadDir("/"); e != nil {
					t.Fatal(e)
				}
			} else {
				if _, e := c.ReadDir("/"); e == nil {
					t.Fatal("list accepted")
				}
				if f, e := c.Open("data"); e != nil {
					t.Fatal(e)
				} else {
					f.Close()
				}
			}
		})
	}
}
func TestSFTPRevokesOpenReadsAndClosesTransport(t *testing.T) {
	s, b, cfg := fixture(t)
	_, c := connect(t, s, cfg)
	f, e := c.Open("data")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	b.denied.Store(true)
	if _, e = f.ReadAt(make([]byte, 32), 0); e == nil {
		t.Fatal("revoked read accepted")
	}
	time.Sleep(1100 * time.Millisecond)
	if _, e := c.Stat("data"); e == nil {
		t.Fatal("revoked session survived")
	}
}
func TestSFTPReadHandleLimitAndRelease(t *testing.T) {
	s, _, cfg := fixture(t)
	_, c := connect(t, s, cfg)
	var files []*sftp.File
	for i := 0; i < 16; i++ {
		f, e := c.Open("data")
		if e != nil {
			t.Fatal(i, e)
		}
		files = append(files, f)
	}
	if f, e := c.Open("data"); e == nil {
		f.Close()
		t.Fatal("handle bound ignored")
	}
	files[0].Close()
	f, e := c.Open("data")
	if e != nil {
		t.Fatal(e)
	}
	f.Close()
	for _, f := range files[1:] {
		f.Close()
	}
}
func TestSFTPCloseDrainsAndRefusesNewSessions(t *testing.T) {
	s, _, cfg := fixture(t)
	_, c := connect(t, s, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if e := s.Close(ctx); e != nil {
		t.Fatal(e)
	}
	if _, e := c.Stat("data"); e == nil {
		t.Fatal("closed connection still usable")
	}
	if conn, e := net.DialTimeout("tcp", s.Addr().String(), time.Second); e == nil {
		conn.Close()
		t.Fatal("closed listener accepted")
	}
}

func TestSFTPPublicKeyAndSingleSession(t *testing.T) {
	s, _, cfg := fixture(t)
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	c, f := connect(t, s, cfg)
	if _, err = f.Stat("data"); err != nil {
		t.Fatal(err)
	}
	if extra, err := c.NewSession(); err == nil {
		extra.Close()
		t.Fatal("second session accepted")
	}
}
func TestSFTPPendingConnectionLimit(t *testing.T) {
	s, _, cfg := fixture(t)
	var pending []net.Conn
	for i := 0; i < 4; i++ {
		conn, err := net.DialTimeout("tcp", s.Addr().String(), time.Second)
		if err != nil {
			t.Fatal(err)
		}
		pending = append(pending, conn)
		defer conn.Close()
		// Reading the banner confirms admission before trying the next connection.
		conn.SetReadDeadline(time.Now().Add(time.Second))
		buf := make([]byte, 256)
		n, err := conn.Read(buf)
		if err != nil || !bytes.HasPrefix(buf[:n], []byte("SSH-2.0-")) {
			t.Fatal(n, err)
		}
	}
	if c, err := ssh.Dial("tcp", s.Addr().String(), cfg); err == nil {
		c.Close()
		t.Fatal("fifth connection admitted")
	}
	pending[0].Close()
	deadline := time.Now().Add(2 * time.Second)
	for {
		s.mu.Lock()
		count := len(s.connections)
		s.mu.Unlock()
		if count == 3 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("connection slot not released")
		}
		time.Sleep(10 * time.Millisecond)
	}
	c, f := connect(t, s, cfg)
	if _, err := f.Stat("data"); err != nil {
		t.Fatal(err)
	}
	c.Close()
}
func TestSFTPDirectoryHandlesShareReadBudget(t *testing.T) {
	s, b, _ := fixture(t)
	h := &handler{backend: b, access: Access{Permissions: []string{"*"}}, ctx: context.Background(), handles: make(chan struct{}, 16)}
	var handles []sftp.ListerAt
	for i := 0; i < 16; i++ {
		l, err := h.Filelist(&sftp.Request{Method: "List", Filepath: "/"})
		if err != nil {
			t.Fatal(i, err)
		}
		handles = append(handles, l)
	}
	if _, err := h.Fileread(&sftp.Request{Filepath: "data"}); err == nil {
		t.Fatal("mixed handle limit bypassed")
	}
	handles[0].(io.Closer).Close()
	handles[0].(io.Closer).Close()
	r, err := h.Fileread(&sftp.Request{Filepath: "data"})
	if err != nil {
		t.Fatal(err)
	}
	r.(io.Closer).Close()
	for _, l := range handles[1:] {
		l.(io.Closer).Close()
	}
	_ = s
}
