package api

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"wings-windows/internal/config"
	"wings-windows/internal/node"
	"wings-windows/internal/sftpserver"
)

func managementOn(a *API) { a.Manager.Config.SFTP = &config.SFTP{AllowFileManagement: true} }
func managementAccess() sftpserver.Access {
	a := uploadAccess()
	a.Permissions = append(a.Permissions, "file.update", "file.delete")
	return a
}
func TestSFTPCommandsLifecycle(t *testing.T) {
	a, _ := setup(t)
	managementOn(a)
	b := SFTPBackend{a}
	access := managementAccess()
	ctx := context.Background()
	if e := b.Command(ctx, access, "Mkdir", "first", ""); e != nil {
		t.Fatal(e)
	}
	if e := b.Command(ctx, access, "Mkdir", "first", ""); e == nil {
		t.Fatal("duplicate directory accepted")
	}
	putDownload(t, a, "first/file", []byte("content"))
	if e := b.Command(ctx, access, "Rename", "first/file", "first/renamed"); e != nil {
		t.Fatal(e)
	}
	if e := b.Command(ctx, access, "Rename", "first", "second"); e != nil {
		t.Fatal(e)
	}
	if got, e := os.ReadFile(filepath.Join(a.Manager.Get(serverID).Root, "second/renamed")); e != nil || string(got) != "content" {
		t.Fatal(e, string(got))
	}
	if e := b.Command(ctx, access, "Rmdir", "second", ""); e == nil {
		t.Fatal("nonempty directory removed")
	}
	if e := b.Command(ctx, access, "Remove", "second", ""); e == nil {
		t.Fatal("REMOVE accepted directory")
	}
	if e := b.Command(ctx, access, "Rmdir", "second/renamed", ""); e == nil {
		t.Fatal("RMDIR accepted file")
	}
	if e := b.Command(ctx, access, "Remove", "second/renamed", ""); e != nil {
		t.Fatal(e)
	}
	if e := b.Command(ctx, access, "Rmdir", "second", ""); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(filepath.Join(a.Manager.Get(serverID).Root, "second")); !os.IsNotExist(e) {
		t.Fatal(e)
	}
}
func TestSFTPCommandsGates(t *testing.T) {
	for _, method := range []string{"Mkdir", "Rename", "Remove", "Rmdir"} {
		for _, kind := range []string{"disabled", "missing-permission", "revoked", "cancelled", "suspended", "shutdown"} {
			t.Run(method+"/"+kind, func(t *testing.T) {
				a, _ := setupWithSettings(t, func(s *node.Settings) { s.Suspended = kind == "suspended" })
				managementOn(a)
				access := managementAccess()
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				putDownload(t, a, "source", []byte("preserve"))
				switch kind {
				case "disabled":
					a.Manager.Config.SFTP.AllowFileManagement = false
				case "missing-permission":
					access.Permissions = []string{"file.sftp"}
				case "revoked":
					a.revoked.Store("user:"+serverID+":"+userID, time.Now())
				case "cancelled":
					cancel()
				case "shutdown":
					a.Close()
				}
				if e := (SFTPBackend{a}).Command(ctx, access, method, "source", "destination"); e == nil {
					t.Fatal("gate bypassed")
				}
				got, e := os.ReadFile(filepath.Join(a.Manager.Get(serverID).Root, "source"))
				if e != nil || string(got) != "preserve" {
					t.Fatal("source changed", e)
				}
			})
		}
	}
}
func TestSFTPCommandsPathsPolicyAndCollisions(t *testing.T) {
	for _, kind := range []string{"root", "traversal", "stream", "reserved", "missing-parent", "collision", "destination-policy", "source-policy", "child-policy", "child-destination-policy", "junction", "hardlink", "missing-source", "unsupported", "case-only", "noop"} {
		t.Run(kind, func(t *testing.T) {
			a, _ := setupWithSettings(t, func(s *node.Settings) {
				switch kind {
				case "source-policy":
					s.Egg.FileDenylist = []string{"source"}
				case "destination-policy":
					s.Egg.FileDenylist = []string{"destination"}
				case "child-policy":
					s.Egg.FileDenylist = []string{"source/child"}
				case "child-destination-policy":
					s.Egg.FileDenylist = []string{"destination/child"}
				}
			})
			managementOn(a)
			access := managementAccess()
			b := SFTPBackend{a}
			method, name, target := "Rename", "source", "destination"
			putDownload(t, a, "source", []byte("keep"))
			switch kind {
			case "root":
				name = "/"
			case "traversal":
				target = "../destination"
			case "stream":
				target = "destination:stream"
			case "reserved":
				target = "NUL"
			case "missing-parent":
				target = "absent/destination"
			case "collision":
				putDownload(t, a, "destination", []byte("other"))
			case "child-policy", "child-destination-policy":
				os.Remove(filepath.Join(a.Manager.Get(serverID).Root, "source"))
				putDownload(t, a, "source/child", []byte("keep"))
			case "junction":
				outside := t.TempDir()
				if e := createTestLink(outside, filepath.Join(a.Manager.Get(serverID).Root, "linked")); e != nil {
					t.Fatal(e)
				}
				name = "linked"
			case "hardlink":
				if e := os.Link(filepath.Join(a.Manager.Get(serverID).Root, "source"), filepath.Join(a.Manager.Get(serverID).Root, "alias")); e != nil {
					t.Fatal(e)
				}
			case "missing-source":
				name = "absent"
			case "unsupported":
				method = "PosixRename"
			case "case-only":
				target = "SOURCE"
			case "noop":
				target = name
			}
			e := b.Command(context.Background(), access, method, name, target)
			if kind == "case-only" || kind == "noop" {
				if e != nil {
					t.Fatal(e)
				}
				return
			}
			if e == nil {
				t.Fatal("unsafe command accepted")
			}
			if kind == "collision" {
				got, e := os.ReadFile(filepath.Join(a.Manager.Get(serverID).Root, "destination"))
				if e != nil || string(got) != "other" {
					t.Fatal("collision changed", e)
				}
			}
		})
	}
}
func TestSFTPCommandsBoundsAndSharedLocks(t *testing.T) {
	for _, kind := range []string{"slots", "rename-lock", "delete-lock", "tree-bound", "locked-file"} {
		t.Run(kind, func(t *testing.T) {
			a, _ := setup(t)
			managementOn(a)
			b := SFTPBackend{a}
			name, target, method := "source", "destination", "Rename"
			putDownload(t, a, "source", []byte("keep"))
			switch kind {
			case "slots":
				for i := 0; i < cap(a.sftpCommandSlots); i++ {
					a.sftpCommandSlots <- struct{}{}
				}
				defer func() {
					for i := 0; i < cap(a.sftpCommandSlots); i++ {
						<-a.sftpCommandSlots
					}
				}()
			case "rename-lock", "delete-lock":
				locks := &a.renameLocks
				if kind == "delete-lock" {
					locks = &a.deleteLocks
					method = "Remove"
				}
				lock := &sync.Mutex{}
				lock.Lock()
				defer lock.Unlock()
				locks.Store(serverID, lock)
			case "tree-bound":
				os.Remove(filepath.Join(a.Manager.Get(serverID).Root, "source"))
				for i := 0; i < 1001; i++ {
					putDownload(t, a, fmt.Sprintf("source/f-%04d", i), nil)
				}
			case "locked-file":
				f, e := os.Open(filepath.Join(a.Manager.Get(serverID).Root, "source"))
				if e != nil {
					t.Fatal(e)
				}
				defer f.Close()
			}
			if e := b.Command(context.Background(), managementAccess(), method, name, target); e == nil {
				t.Fatal("bound/lock bypassed")
			}
		})
	}
}
func TestSFTPCommandsEncryptedProtocol(t *testing.T) {
	for _, kind := range []string{"enabled", "disabled", "no-create", "no-update", "no-delete"} {
		t.Run(kind, func(t *testing.T) {
			a, _ := setup(t)
			managementOn(a)
			access := managementAccess()
			if kind == "disabled" {
				a.Manager.Config.SFTP.AllowFileManagement = false
			}
			if strings.HasPrefix(kind, "no-") {
				access.Permissions = []string{"file.sftp", "file.read", "file.read-content"}
				for _, p := range []string{"file.create", "file.update", "file.delete"} {
					if p != "file."+strings.TrimPrefix(kind, "no-") {
						access.Permissions = append(access.Permissions, p)
					}
				}
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
			server := sftpserver.New(listener, signer, sftpUploadAuthFixture{SFTPBackend{a}, access})
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				if e := server.Close(ctx); e != nil {
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
			putDownload(t, a, "file", []byte("keep"))
			e = client.Mkdir("directory")
			if (e == nil) != (kind != "disabled" && kind != "no-create") {
				t.Fatal("mkdir permission", e)
			}
			e = client.Rename("file", "renamed")
			if (e == nil) != (kind != "disabled" && kind != "no-update") {
				t.Fatal("rename permission", e)
			}
			file := "renamed"
			if kind == "disabled" || kind == "no-update" {
				file = "file"
			}
			e = client.Remove(file)
			if (e == nil) != (kind != "disabled" && kind != "no-delete") {
				t.Fatal("delete permission", e)
			}
			if kind == "enabled" {
				if e = client.RemoveDirectory("directory"); e != nil {
					t.Fatal(e)
				}
			}
			if e = client.Chmod("file", 0600); e == nil {
				t.Fatal("attributes accepted")
			}
		})
	}
}

func TestSFTPCommandsPolicySnapshotChangesOnSync(t *testing.T) {
	denied := false
	a, _ := setupWithSettings(t, func(s *node.Settings) {
		if denied {
			s.Egg.FileDenylist = []string{"source/child"}
		}
	})
	server := a.Manager.Get(serverID)
	before := server.FilePolicySnapshot()
	denied = true
	if e := server.Sync(context.Background()); e != nil {
		t.Fatal(e)
	}
	if server.FilePolicySnapshot() == before || !server.FileDenied("source/child", false) {
		t.Fatal("policy sync not detected")
	}
}
