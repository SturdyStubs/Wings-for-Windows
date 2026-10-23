package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"wings-windows/internal/node"
)

// Contract independently taken from Wings 747bc43 postServerCreateDirectory,
// Filesystem.CreateDirectory tests, and panel 2.0.0-beta.1 FilesClient.
func TestPanelCreateDirectoryContract(t *testing.T) {
	a, _ := setup(t)
	body, err := os.ReadFile("testdata/upstream-create-directory.json")
	if err != nil {
		t.Fatal(err)
	}
	route := "/api/servers/" + serverID + "/files/create-directory"
	for i := 0; i < 2; i++ {
		w := request(t, a, "POST", route, string(body), secret)
		if w.Code != 204 || w.Body.Len() != 0 {
			t.Fatalf("create/replay %d: %d %s", i, w.Code, w.Body)
		}
	}
	root := a.Manager.Get(serverID).Root
	if info, err := os.Stat(filepath.Join(root, "foo", "bar", "baz", "test")); err != nil || !info.IsDir() {
		t.Fatal("missing nested directory", err)
	}
	if w := request(t, a, "POST", "/api/servers/"+serverID+"/files/write?file=/foo/bar/baz/test/config.cfg", "nested config", secret); w.Code != 204 {
		t.Fatal(w.Code, w.Body)
	}
	if w := request(t, a, "GET", "/api/servers/"+serverID+"/files/list-directory?directory=/foo/bar/baz/test", "", secret); w.Code != 200 || !strings.Contains(w.Body.String(), "config.cfg") {
		t.Fatal(w.Code, w.Body)
	}
}

func TestDirectoryInputsRejectBeforeCreatingParents(t *testing.T) {
	for name, body := range map[string]string{
		"empty": `{"name":"","path":"new"}`, "missing": `{"path":"new"}`, "root": `{"name":"/","path":"new"}`,
		"traversal": `{"name":"../escape","path":"new"}`, "parent traversal": `{"name":"child","path":"new/../escape"}`,
		"backslash": `{"name":"dir\\child","path":"new"}`, "drive": `{"name":"C:/child","path":"new"}`,
		"ADS": `{"name":"dir:stream","path":"new"}`, "device": `{"name":"COM1.cfg","path":"new"}`, "superscript device": `{"name":"COM¹.cfg","path":"new"}`,
		"trailing dot": `{"name":"dir.","path":"new"}`, "trailing space": `{"name":"dir ","path":"new"}`, "double slash": `{"name":"a//b","path":"new"}`,
		"wildcard": `{"name":"dir*","path":"new"}`, "control": `{"name":"dir\u0001","path":"new"}`, "quote": `{"name":"dir\"","path":"new"}`,
		"null": `null`, "name type": `{"name":5}`, "array": `[]`, "null name": `{"name":null}`, "trailing object": `{"name":"x"}{}`, "malformed": `{"name":`,
		"component bound": `{"path":"new","name":"` + strings.Repeat("x", 256) + `"}`,
		"UTF16 bound":     `{"path":"new","name":"` + strings.Repeat("😀", 128) + `"}`,
		"depth":           `{"name":"` + strings.Repeat("a/", 32) + `b"}`, "path bound": `{"name":"` + strings.Repeat("a", 4097) + `"}`,
	} {
		t.Run(name, func(t *testing.T) {
			a, _ := setup(t)
			w := request(t, a, "POST", "/api/servers/"+serverID+"/files/create-directory", body, secret)
			if w.Code != 422 && w.Code != 400 {
				t.Fatal(w.Code, w.Body)
			}
			entries, err := os.ReadDir(a.Manager.Get(serverID).Root)
			if err != nil || len(entries) != 0 {
				t.Fatal("rejected request mutated root", entries, err)
			}
		})
	}
}

func TestDirectoryAuthorizationMethodsAndBounds(t *testing.T) {
	a, _ := setup(t)
	route := "/api/servers/" + serverID + "/files/create-directory"
	for _, token := range []string{"", "wrong"} {
		if w := request(t, a, "POST", route, `{"name":"new"}`, token); w.Code != 401 {
			t.Fatal(w.Code)
		}
	}
	for _, method := range []string{"GET", "PUT", "DELETE"} {
		if w := request(t, a, method, route, `{"name":"new"}`, secret); w.Code != 501 {
			t.Fatal(w.Code)
		}
	}
	if w := request(t, a, "POST", route, `{"name":"`+strings.Repeat("x", 65536)+`"}`, secret); w.Code != 413 {
		t.Fatal(w.Code)
	}
	if w := request(t, a, "POST", strings.Replace(route, serverID, userID, 1), `{"name":"new"}`, secret); w.Code != 404 {
		t.Fatal(w.Code)
	}
}

func TestDirectoryDenylistPreflightsWholePath(t *testing.T) {
	a, _ := setupWithSettings(t, func(s *node.Settings) {
		s.Egg.FileDenylist = []string{"private/", "*.secret", "blocked/", "!public.secret/"}
	})
	for _, body := range []string{`{"name":"child","path":"PRIVATE"}`, `{"name":"allowed/blocked/child"}`, `{"name":"allowed/data.secret"}`} {
		w := request(t, a, "POST", "/api/servers/"+serverID+"/files/create-directory", body, secret)
		if w.Code != 403 {
			t.Fatal(w.Code, w.Body)
		}
	}
	if entries, _ := os.ReadDir(a.Manager.Get(serverID).Root); len(entries) != 0 {
		t.Fatal("denylist left earlier parents", entries)
	}
	if w := request(t, a, "POST", "/api/servers/"+serverID+"/files/create-directory", `{"name":"public.secret"}`, secret); w.Code != 204 {
		t.Fatal(w.Code, w.Body)
	}
}

func TestDirectoryExistingFilePreserved(t *testing.T) {
	a, _ := setup(t)
	root := a.Manager.Get(serverID).Root
	if err := os.WriteFile(filepath.Join(root, "config.cfg"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	w := request(t, a, "POST", "/api/servers/"+serverID+"/files/create-directory", `{"path":"config.cfg","name":"child"}`, secret)
	if w.Code != 400 {
		t.Fatal(w.Code, w.Body)
	}
	if body, _ := os.ReadFile(filepath.Join(root, "config.cfg")); string(body) != "keep" {
		t.Fatal("existing file changed")
	}
}

func TestDirectoryJunctionsRefusedWithoutMutation(t *testing.T) {
	for _, target := range []string{"outside", "denied", "inside"} {
		t.Run(target, func(t *testing.T) {
			a, _ := setupWithSettings(t, func(s *node.Settings) { s.Egg.FileDenylist = []string{"private/"} })
			root := a.Manager.Get(serverID).Root
			destination := t.TempDir()
			if target != "outside" {
				destination = filepath.Join(root, "safe")
				if target == "denied" {
					destination = filepath.Join(root, "private")
				}
				if err := os.Mkdir(destination, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if err := createTestLink(destination, filepath.Join(root, "alias")); err != nil {
				t.Fatal(err)
			}
			w := request(t, a, "POST", "/api/servers/"+serverID+"/files/create-directory", `{"path":"alias","name":"new/child"}`, secret)
			if w.Code != 403 {
				t.Fatal(w.Code, w.Body)
			}
			if _, err := os.Stat(filepath.Join(destination, "new")); !os.IsNotExist(err) {
				t.Fatal("created through junction", err)
			}
		})
	}
}

func TestDirectoryUnicodeAndCancellation(t *testing.T) {
	a, _ := setup(t)
	s := a.Manager.Get(serverID)
	route := "/api/servers/" + serverID + "/files/create-directory"
	if w := request(t, a, "POST", route, `{"path":"/","name":"Café/世界"}`, secret); w.Code != 204 {
		t.Fatal(w.Code, w.Body)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := httptest.NewRequest(http.MethodPost, route, strings.NewReader(`{"name":"cancelled/child"}`)).WithContext(ctx)
	r.Header.Set("Authorization", "Bearer "+secret)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 408 {
		t.Fatal(w.Code, w.Body)
	}
	if _, err := os.Stat(filepath.Join(s.Root, "cancelled")); !os.IsNotExist(err) {
		t.Fatal("cancelled operation created directory", err)
	}
}

func TestDirectoryLongPathAndHandleRelease(t *testing.T) {
	a, _ := setup(t)
	name := strings.Repeat("a", 100) + "/" + strings.Repeat("b", 100) + "/" + strings.Repeat("c", 100)
	w := request(t, a, "POST", "/api/servers/"+serverID+"/files/create-directory", `{"name":"`+name+`"}`, secret)
	if w.Code != 204 {
		t.Fatal(w.Code, w.Body)
	}
	root := a.Manager.Get(serverID).Root
	if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); err != nil || !info.IsDir() {
		t.Fatal("long path missing", err)
	}
	if err := os.Rename(filepath.Join(root, strings.Repeat("a", 100)), filepath.Join(root, "renamed")); err != nil {
		t.Fatal("request retained directory handles", err)
	}
}

func TestDirectoryPolicyRefresh(t *testing.T) {
	enabled := &atomic.Bool{}
	a, _ := setupWithSettings(t, func(s *node.Settings) {
		if enabled.Load() {
			s.Egg.FileDenylist = []string{"blocked/"}
		}
	})
	enabled.Store(true)
	if w := request(t, a, "POST", "/api/servers/"+serverID+"/sync", `{}`, secret); w.Code != 204 {
		t.Fatal(w.Code, w.Body)
	}
	if w := request(t, a, "POST", "/api/servers/"+serverID+"/files/create-directory", `{"name":"blocked/new"}`, secret); w.Code != 403 {
		t.Fatal(w.Code, w.Body)
	}
	if _, err := os.Stat(filepath.Join(a.Manager.Get(serverID).Root, "blocked")); !os.IsNotExist(err) {
		t.Fatal("synced denylist not enforced", err)
	}
}
