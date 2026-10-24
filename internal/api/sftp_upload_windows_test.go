package api

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"wings-windows/internal/config"
	"wings-windows/internal/node"
	"wings-windows/internal/sftpserver"
)

func uploadAccess() sftpserver.Access {
	a := sftpAccess()
	a.Permissions = append(a.Permissions, "file.create")
	return a
}
func uploadsOn(a *API) { a.Manager.Config.SFTP = &config.SFTP{AllowUploads: true} }
func noSFTPStages(t *testing.T, a *API) {
	t.Helper()
	entries, e := os.ReadDir(a.Manager.Get(serverID).Root)
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".wings-sftp-") {
			t.Fatal("unpublished stage retained", entry.Name())
		}
	}
}
func TestSFTPUploadPublishesOnlyOnClose(t *testing.T) {
	a, _ := setup(t)
	uploadsOn(a)
	b := SFTPBackend{a}
	access := uploadAccess()
	f, e := b.Upload(context.Background(), access, "binary")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	data := bytes.Repeat([]byte{0, 255, 17}, 3<<20)
	for off := len(data) - (64 << 10); off >= 0; off -= 64 << 10 {
		if n, e := f.WriteAt(data[off:off+(64<<10)], int64(off)); e != nil || n != 64<<10 {
			t.Fatal(n, e)
		}
	}
	if _, e := os.Stat(filepath.Join(a.Manager.Get(serverID).Root, "binary")); !os.IsNotExist(e) {
		t.Fatal("published before CLOSE", e)
	}
	if e := f.Close(); e != nil {
		t.Fatal(e)
	}
	if e := f.Close(); e != nil {
		t.Fatal("idempotent CLOSE", e)
	}
	got, e := os.ReadFile(filepath.Join(a.Manager.Get(serverID).Root, "binary"))
	if e != nil || !bytes.Equal(got, data) {
		t.Fatal(e, "binary integrity")
	}
	if _, e = f.WriteAt([]byte("x"), 0); e == nil {
		t.Fatal("closed writer accepted")
	}
	if overwrite, e := b.Upload(context.Background(), access, "binary"); e == nil {
		overwrite.Close()
		t.Fatal("existing destination accepted")
	}
	noSFTPStages(t, a)
}
func TestSFTPUploadEmptyAndOverlappingWrites(t *testing.T) {
	for _, kind := range []string{"empty", "overlap", "gap-filled"} {
		t.Run(kind, func(t *testing.T) {
			a, _ := setup(t)
			uploadsOn(a)
			b := SFTPBackend{a}
			f, e := b.Upload(context.Background(), uploadAccess(), "new")
			if e != nil {
				t.Fatal(e)
			}
			defer f.Close()
			want := []byte{}
			if kind == "overlap" {
				f.WriteAt([]byte("aaaa"), 0)
				f.WriteAt([]byte("bb"), 1)
				want = []byte("abba")
			}
			if kind == "gap-filled" {
				f.WriteAt([]byte("tail"), 4)
				f.WriteAt([]byte("head"), 0)
				want = []byte("headtail")
			}
			if e = f.Close(); e != nil {
				t.Fatal(e)
			}
			got, e := os.ReadFile(filepath.Join(a.Manager.Get(serverID).Root, "new"))
			if e != nil || !bytes.Equal(got, want) {
				t.Fatal(e, string(got))
			}
			noSFTPStages(t, a)
		})
	}
}
func TestSFTPUploadRefusalAndWholeTransferAbort(t *testing.T) {
	for _, kind := range []string{"negative", "packet", "hole", "offset-overflow", "total-overflow", "fragmentation", "transport", "cancel", "collision", "revocation", "shutdown", "policy", "suspension"} {
		t.Run(kind, func(t *testing.T) {
			changed := false
			a, _ := setupWithSettings(t, func(s *node.Settings) {
				if changed {
					if kind == "suspension" {
						s.Suspended = true
					} else {
						s.Egg.FileDenylist = []string{"new"}
					}
				}
			})
			uploadsOn(a)
			b := SFTPBackend{a}
			access := uploadAccess()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			f, e := b.Upload(ctx, access, "new")
			if e != nil {
				t.Fatal(e)
			}
			defer f.Close()
			if _, e = f.WriteAt([]byte("abc"), 0); e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "negative":
				_, e = f.WriteAt([]byte("x"), -1)
			case "packet":
				_, e = f.WriteAt(make([]byte, (64<<10)+1), 0)
			case "hole":
				_, e = f.WriteAt([]byte("x"), 10)
			case "offset-overflow":
				_, e = f.WriteAt([]byte("x"), maxUploadBytes)
			case "total-overflow":
				chunk := make([]byte, 64<<10)
				for n := int64(0); n < maxUploadBytes-int64(len(chunk)); n += int64(len(chunk)) {
					if _, e = f.WriteAt(chunk, n); e != nil {
						t.Fatal(e)
					}
				}
				// The initial three bytes also count toward the accepted payload budget.
				_, e = f.WriteAt(chunk, maxUploadBytes-int64(len(chunk)))
			case "fragmentation":
				for n := int64(0); n <= maxSFTPUploadRanges; n++ {
					_, e = f.WriteAt([]byte("x"), 10+2*n)
					if e != nil {
						break
					}
				}
			case "transport":
				f.TransferError(errors.New("disconnected"))
			case "cancel":
				cancel()
			case "collision":
				putDownload(t, a, "new", []byte("other"))
			case "revocation":
				a.revoked.Store("user:"+serverID+":"+userID, time.Now())
			case "shutdown":
				a.Close()
			case "policy", "suspension":
				changed = true
				if e := a.Manager.Get(serverID).Sync(context.Background()); e != nil {
					t.Fatal(e)
				}
			}
			if kind == "negative" || kind == "packet" || kind == "offset-overflow" || kind == "total-overflow" || kind == "fragmentation" {
				if e == nil {
					t.Fatal("bad write accepted")
				}
			}
			if e = f.Close(); e == nil {
				t.Fatal("failed/incomplete upload published")
			}
			result, e := os.ReadFile(filepath.Join(a.Manager.Get(serverID).Root, "new"))
			if kind == "collision" {
				if e != nil || string(result) != "other" {
					t.Fatal("collision altered", e)
				}
			} else if !os.IsNotExist(e) {
				t.Fatal("unpublished output appeared", e)
			}
			noSFTPStages(t, a)
		})
	}
}
func TestSFTPUploadDefaultsPermissionsAndPaths(t *testing.T) {
	for _, kind := range []string{"disabled", "read-only", "permission", "traversal", "stream", "reserved", "missing-parent", "denied"} {
		t.Run(kind, func(t *testing.T) {
			a, _ := setupWithSettings(t, func(s *node.Settings) {
				if kind == "denied" {
					s.Egg.FileDenylist = []string{"new"}
				}
			})
			access := uploadAccess()
			name := "new"
			if kind != "disabled" {
				uploadsOn(a)
			}
			if kind == "read-only" {
				a.Manager.Config.SFTP.AllowUploads = false
			}
			if kind == "permission" {
				access.Permissions = []string{"file.sftp", "file.update"}
			}
			if kind == "traversal" {
				name = "../new"
			}
			if kind == "stream" {
				name = "new:stream"
			}
			if kind == "reserved" {
				name = "NUL"
			}
			if kind == "missing-parent" {
				name = "absent/new"
			}
			if f, e := (SFTPBackend{a}).Upload(context.Background(), access, name); e == nil {
				f.Close()
				t.Fatal("invalid upload accepted")
			}
			noSFTPStages(t, a)
		})
	}
}
func TestSFTPUploadSharedCapacityAndCancellationCleanup(t *testing.T) {
	a, _ := setup(t)
	uploadsOn(a)
	b := SFTPBackend{a}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f, e := b.Upload(ctx, uploadAccess(), "first")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	f.WriteAt([]byte("x"), 0)
	if other, e := b.Upload(ctx, uploadAccess(), "second"); e == nil {
		other.Close()
		t.Fatal("per-server capacity bypassed")
	}
	cancel()
	deadline := time.Now().Add(3 * time.Second)
	for {
		u := f.(*sftpUpload)
		u.mu.Lock()
		closed := u.closed
		u.mu.Unlock()
		if closed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("cancelled upload did not close")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Close waits for native cleanup if automatic cancellation is still finishing.
	if e = f.Close(); e == nil {
		t.Fatal("cancelled close succeeded")
	}
	noSFTPStages(t, a)
	for i := 0; i < cap(a.uploadSlots); i++ {
		a.uploadSlots <- struct{}{}
	}
	if other, e := b.Upload(context.Background(), uploadAccess(), "third"); e == nil {
		other.Close()
		t.Fatal("HTTP/SFTP shared capacity ignored")
	}
	for i := 0; i < cap(a.uploadSlots); i++ {
		<-a.uploadSlots
	}
	next, e := b.Upload(context.Background(), uploadAccess(), "fourth")
	if e != nil {
		t.Fatal("capacity not released", e)
	}
	if e = next.Close(); e != nil {
		t.Fatal(e)
	}
}

type sftpUploadAuthFixture struct {
	SFTPBackend
	access sftpserver.Access
}

func (b sftpUploadAuthFixture) Authenticate(context.Context, sftpserver.AuthRequest) (sftpserver.Access, error) {
	return b.access, nil
}
func TestSFTPUploadEncryptedProtocolAndDisconnect(t *testing.T) {
	for _, kind := range []string{"create-close", "write-only", "disconnect", "no-create-permission", "read-only", "append", "existing-update"} {
		t.Run(kind, func(t *testing.T) {
			a, _ := setup(t)
			uploadsOn(a)
			access := uploadAccess()
			if kind == "no-create-permission" {
				access.Permissions = []string{"file.sftp", "file.read", "file.read-content", "file.update"}
			}
			if kind == "read-only" {
				a.Manager.Config.SFTP.AllowUploads = false
			}
			_, key, e := ed25519.GenerateKey(rand.Reader)
			if e != nil {
				t.Fatal(e)
			}
			signer, e := ssh.NewSignerFromKey(key)
			if e != nil {
				t.Fatal(e)
			}
			listener, e := net.Listen("tcp", "127.0.0.1:0")
			if e != nil {
				t.Fatal(e)
			}
			gateway := sftpserver.New(listener, signer, sftpUploadAuthFixture{SFTPBackend{a}, access})
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				if e := gateway.Close(ctx); e != nil {
					t.Error(e)
				}
			}()
			conn, e := ssh.Dial("tcp", listener.Addr().String(), &ssh.ClientConfig{User: "test.12345678", Auth: []ssh.AuthMethod{ssh.Password("fixture")}, HostKeyCallback: ssh.FixedHostKey(signer.PublicKey()), Timeout: 3 * time.Second})
			if e != nil {
				t.Fatal(e)
			}
			defer conn.Close()
			client, e := sftp.NewClient(conn)
			if e != nil {
				t.Fatal(e)
			}
			defer client.Close()
			flags := os.O_RDWR | os.O_CREATE | os.O_TRUNC
			if kind == "write-only" {
				flags = os.O_WRONLY | os.O_CREATE | os.O_EXCL
			}
			if kind == "append" {
				flags |= os.O_APPEND
			}
			if kind == "existing-update" {
				putDownload(t, a, "new", []byte("original"))
				flags = os.O_RDWR
			}
			f, e := client.OpenFile("new", flags)
			if kind == "no-create-permission" || kind == "read-only" || kind == "append" || kind == "existing-update" {
				if e == nil {
					f.Close()
					t.Fatal("forbidden OPEN accepted")
				}
				noSFTPStages(t, a)
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			if _, e = f.WriteAt([]byte("tail"), 4); e != nil {
				t.Fatal(e)
			}
			if _, e = f.WriteAt([]byte("head"), 0); e != nil {
				t.Fatal(e)
			}
			dest := filepath.Join(a.Manager.Get(serverID).Root, "new")
			if _, e = os.Stat(dest); !os.IsNotExist(e) {
				t.Fatal("visible before CLOSE", e)
			}
			if kind == "disconnect" {
				conn.Close()
				deadline := time.Now().Add(3 * time.Second)
				for {
					_, active := a.sftpUploadLocks.Load(serverID)
					if !active {
						break
					}
					if time.Now().After(deadline) {
						t.Fatal("disconnect did not release upload")
					}
					time.Sleep(10 * time.Millisecond)
				}
				if _, e = os.Stat(dest); !os.IsNotExist(e) {
					t.Fatal("disconnect published upload", e)
				}
			} else {
				if e = f.Close(); e != nil {
					t.Fatal(e)
				}
				got, e := os.ReadFile(dest)
				if e != nil || string(got) != "headtail" {
					t.Fatal(e, string(got))
				}
			}
			noSFTPStages(t, a)
		})
	}
}
func TestSFTPUploadExactFileLimit(t *testing.T) {
	a, _ := setup(t)
	uploadsOn(a)
	f, e := (SFTPBackend{a}).Upload(context.Background(), uploadAccess(), "limit")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	chunk := make([]byte, 64<<10)
	for off := int64(0); off < maxUploadBytes; off += int64(len(chunk)) {
		if n, e := f.WriteAt(chunk, off); e != nil || n != len(chunk) {
			t.Fatal(off, n, e)
		}
	}
	if e = f.Close(); e != nil {
		t.Fatal(e)
	}
	info, e := os.Stat(filepath.Join(a.Manager.Get(serverID).Root, "limit"))
	if e != nil || info.Size() != maxUploadBytes {
		t.Fatal(info, e)
	}
	noSFTPStages(t, a)
}

func TestSFTPUploadParentAndLinkGuards(t *testing.T) {
	for _, kind := range []string{"junction", "denied-parent", "hardlink"} {
		t.Run(kind, func(t *testing.T) {
			a, _ := setupWithSettings(t, func(s *node.Settings) {
				if kind == "denied-parent" {
					s.Egg.FileDenylist = []string{"hidden/"}
				}
			})
			uploadsOn(a)
			b := SFTPBackend{a}
			name := "new"
			outside := t.TempDir()
			sentinel := filepath.Join(outside, "data")
			if e := os.WriteFile(sentinel, []byte("sentinel"), 0600); e != nil {
				t.Fatal(e)
			}
			if kind == "junction" {
				if e := createTestLink(outside, filepath.Join(a.Manager.Get(serverID).Root, "linked")); e != nil {
					t.Fatal(e)
				}
				name = "linked/new"
			}
			if kind == "denied-parent" {
				putDownload(t, a, "hidden/data", []byte("hidden"))
				name = "hidden/new"
			}
			if kind == "hardlink" {
				if e := os.Link(sentinel, filepath.Join(a.Manager.Get(serverID).Root, "new")); e != nil {
					t.Fatal(e)
				}
			}
			if f, e := b.Upload(context.Background(), uploadAccess(), name); e == nil {
				f.Close()
				t.Fatal("unsafe upload accepted")
			}
			got, e := os.ReadFile(sentinel)
			if e != nil || string(got) != "sentinel" {
				t.Fatal("external data changed", e)
			}
			noSFTPStages(t, a)
		})
	}
}
