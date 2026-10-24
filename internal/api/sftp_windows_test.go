package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"wings-windows/internal/node"
	"wings-windows/internal/sftpserver"
)

func sftpAccess() sftpserver.Access {
	return sftpserver.Access{Server: serverID, User: userID, Permissions: []string{"file.sftp", "file.read", "file.read-content"}, Issued: time.Now()}
}
func TestSFTPBackendConfinedReadStatList(t *testing.T) {
	a, _ := setup(t)
	b := SFTPBackend{a}
	access := sftpAccess()
	putDownload(t, a, "nested/data", []byte{0, 255, 1, 2})
	f, e := b.Read(context.Background(), access, "/nested/data")
	if e != nil {
		t.Fatal(e)
	}
	got := make([]byte, 4)
	if n, e := f.ReadAt(got, 0); e != nil || n != 4 || got[1] != 255 {
		t.Fatal(n, e)
	}
	if _, e = f.ReadAt(got, 4); e != io.EOF {
		t.Fatal(e)
	}
	f.Close()
	for _, name := range []string{"/", ".", "nested", "nested/data"} {
		if _, e := b.Stat(context.Background(), access, name); e != nil {
			t.Fatal(name, e)
		}
	}
	items, e := b.List(context.Background(), access, "nested")
	if e != nil || len(items) != 1 || items[0].Name() != "data" {
		t.Fatal(items, e)
	}
}
func TestSFTPBackendUnsafePaths(t *testing.T) {
	for _, name := range []string{"../escape", `C:\outside`, `\\host\share`, "file:stream", "NUL", "trailing.", "a//b", "a/./b", "a/../b", strings.Repeat("a", 256)} {
		t.Run(name, func(t *testing.T) {
			a, _ := setup(t)
			b := SFTPBackend{a}
			access := sftpAccess()
			if f, e := b.Read(context.Background(), access, name); e == nil {
				f.Close()
				t.Fatal("read accepted")
			}
			if _, e := b.Stat(context.Background(), access, name); e == nil {
				t.Fatal("stat accepted")
			}
			if _, e := b.List(context.Background(), access, name); e == nil {
				t.Fatal("list accepted")
			}
		})
	}
}
func TestSFTPBackendLinksAndPolicy(t *testing.T) {
	for _, kind := range []string{"hardlink", "junction", "denied-source", "denied-parent"} {
		t.Run(kind, func(t *testing.T) {
			a, _ := setupWithSettings(t, func(s *node.Settings) {
				if kind == "denied-source" {
					s.Egg.FileDenylist = []string{"data"}
				}
				if kind == "denied-parent" {
					s.Egg.FileDenylist = []string{"hidden/"}
				}
			})
			b := SFTPBackend{a}
			putDownload(t, a, "data", []byte("original"))
			name := "data"
			switch kind {
			case "hardlink":
				if e := os.Link(filepath.Join(a.Manager.Get(serverID).Root, "data"), filepath.Join(a.Manager.Get(serverID).Root, "linked")); e != nil {
					t.Fatal(e)
				}
				name = "linked"
			case "junction":
				outside := t.TempDir()
				if e := os.WriteFile(filepath.Join(outside, "data"), []byte("outside"), 0600); e != nil {
					t.Fatal(e)
				}
				if e := createTestLink(outside, filepath.Join(a.Manager.Get(serverID).Root, "linked")); e != nil {
					t.Fatal(e)
				}
				name = "linked/data"
			case "denied-parent":
				putDownload(t, a, "hidden/data", []byte("hidden"))
				name = "hidden/data"
			}
			if f, e := b.Read(context.Background(), sftpAccess(), name); e == nil {
				f.Close()
				t.Fatal("restricted read accepted")
			}
			if _, e := b.Stat(context.Background(), sftpAccess(), name); e == nil {
				t.Fatal("restricted stat accepted")
			}
			items, e := b.List(context.Background(), sftpAccess(), "/")
			if e != nil {
				t.Fatal(e)
			}
			for _, entry := range items {
				if entry.Name() == strings.Split(name, "/")[0] {
					t.Fatal("restricted name listed")
				}
			}
		})
	}
}
func TestSFTPBackendOpenReadAndListingRecheck(t *testing.T) {
	denied := false
	a, _ := setupWithSettings(t, func(s *node.Settings) {
		if denied {
			s.Egg.FileDenylist = []string{"data"}
		}
	})
	b := SFTPBackend{a}
	putDownload(t, a, "data", []byte("bytes"))
	access := sftpAccess()
	f, e := b.Read(context.Background(), access, "data")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	items, e := b.List(context.Background(), access, "/")
	if e != nil || len(items) != 1 {
		t.Fatal(e)
	}
	denied = true
	if e = a.Manager.Get(serverID).Sync(context.Background()); e != nil {
		t.Fatal(e)
	}
	if _, e = f.ReadAt(make([]byte, 1), 0); e == nil {
		t.Fatal("policy update ignored")
	}
	dynamic, ok := items[0].(interface{ Allowed() bool })
	if !ok || dynamic.Allowed() {
		t.Fatal("listing policy update ignored")
	}
}
func TestSFTPBackendRevocationSuspensionAndShutdown(t *testing.T) {
	for _, kind := range []string{"revocation", "suspension", "shutdown"} {
		t.Run(kind, func(t *testing.T) {
			suspended := false
			a, _ := setupWithSettings(t, func(s *node.Settings) { s.Suspended = suspended })
			b := SFTPBackend{a}
			putDownload(t, a, "data", []byte("bytes"))
			access := sftpAccess()
			f, e := b.Read(context.Background(), access, "data")
			if e != nil {
				t.Fatal(e)
			}
			defer f.Close()
			switch kind {
			case "revocation":
				w := request(t, a, "POST", "/api/deauthorize-user", `{"user":"`+userID+`","servers":["`+serverID+`"]}`, secret)
				if w.Code != 204 {
					t.Fatal(w.Code, w.Body)
				}
			case "suspension":
				suspended = true
				if e = a.Manager.Get(serverID).Sync(context.Background()); e != nil {
					t.Fatal(e)
				}
			case "shutdown":
				a.Close()
			}
			if b.Allowed(access) {
				t.Fatal("session still allowed")
			}
			if _, e = f.ReadAt(make([]byte, 1), 0); e == nil {
				t.Fatal("read accepted")
			}
		})
	}
}
func TestSFTPBackendAuthContractAndScope(t *testing.T) {
	for _, kind := range []string{"valid", "wrong-server", "bad-user", "no-sftp", "http-error", "invalid-json", "too-many-permissions", "unknown-server"} {
		t.Run(kind, func(t *testing.T) {
			a, _ := setup(t)
			calls := 0
			panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "POST" || r.URL.Path != "/api/remote/sftp/auth" || r.Header.Get("Authorization") != "Bearer node."+secret {
					t.Error("auth contract")
				}
				var input sftpserver.AuthRequest
				if e := json.NewDecoder(r.Body).Decode(&input); e != nil || input.Type != "public_key" || input.User != "operator.11111111" || input.Pass != "public-test-key" {
					t.Error("request fields")
				}
				if kind == "http-error" {
					w.WriteHeader(403)
					return
				}
				if kind == "invalid-json" {
					w.Write([]byte("invalid"))
					return
				}
				reply := map[string]any{"server": serverID, "user": userID, "permissions": []string{"file.sftp", "file.read"}}
				switch kind {
				case "wrong-server":
					reply["server"] = userID
				case "bad-user":
					reply["user"] = "bad"
				case "no-sftp":
					reply["permissions"] = []string{"file.read"}
				case "too-many-permissions":
					reply["permissions"] = make([]string, 129)
				}
				json.NewEncoder(w).Encode(reply)
			}))
			defer panel.Close()
			a.Manager.Panel.Config.PanelURL = panel.URL
			b := SFTPBackend{a}
			username := "operator.11111111"
			if kind == "unknown-server" {
				username = "operator.99999999"
			}
			v, e := b.Authenticate(context.Background(), sftpserver.AuthRequest{Type: "public_key", User: username, Pass: "public-test-key", IP: "127.0.0.1"})
			if kind == "valid" {
				if e != nil || v.Server != serverID || v.User != userID || !v.Can("file.read") {
					t.Fatal(v, e)
				}
			} else if e == nil {
				t.Fatal("auth accepted")
			}
			if kind == "unknown-server" && calls != 0 {
				t.Fatal("unknown profile reached panel")
			}
		})
	}
}
func TestSFTPBackendListingCapAndReadLock(t *testing.T) {
	a, _ := setup(t)
	b := SFTPBackend{a}
	putDownload(t, a, "data", []byte("bytes"))
	f, e := b.Read(context.Background(), sftpAccess(), "data")
	if e != nil {
		t.Fatal(e)
	}
	if writer, e := os.OpenFile(filepath.Join(a.Manager.Get(serverID).Root, "data"), os.O_WRONLY, 0); e == nil {
		writer.Close()
		t.Fatal("reader did not pin source")
	}
	f.Close()
	for i := 0; i < 1000; i++ {
		putDownload(t, a, filepath.Join("many", fmtSFTPName(i)), nil)
	}
	if items, e := b.List(context.Background(), sftpAccess(), "many"); e != nil || len(items) != 1000 {
		t.Fatal(len(items), e)
	}
	putDownload(t, a, "many/overflow", nil)
	if _, e := b.List(context.Background(), sftpAccess(), "many"); e == nil {
		t.Fatal("listing bound ignored")
	}
}
func fmtSFTPName(i int) string { return fmt.Sprintf("f%04d", i) }
