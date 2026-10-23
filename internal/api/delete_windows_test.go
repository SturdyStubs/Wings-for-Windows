package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"wings-windows/internal/node"
)

func deleteRequest(t *testing.T, a *API, root string, names ...string) int {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"root": root, "files": names})
	w := request(t, a, "POST", "/api/servers/"+serverID+"/files/delete", string(b), secret)
	if w.Code != 204 {
		t.Log(w.Body.String())
	}
	return w.Code
}
func putDeleteFixture(t *testing.T, root, name string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("preserve-test-data"), 0600); err != nil {
		t.Fatal(err)
	}
}
func assertDeleteExists(t *testing.T, root, name string, want bool) {
	t.Helper()
	_, e := os.Lstat(filepath.Join(root, filepath.FromSlash(name)))
	if want && e != nil || !want && !os.IsNotExist(e) {
		t.Fatalf("%s existence wanted %v, error %v", name, want, e)
	}
}

func TestDeletePanelContract(t *testing.T) {
	a, _ := setup(t)
	root := a.Manager.Get(serverID).Root
	for _, n := range []string{"config/a.cfg", "config/plugins/nested/one.cfg", "config/keep.cfg"} {
		putDeleteFixture(t, root, n)
	}
	b, e := os.ReadFile("testdata/upstream-delete.json")
	if e != nil {
		t.Fatal(e)
	}
	w := request(t, a, "POST", "/api/servers/"+serverID+"/files/delete", string(b), secret)
	if w.Code != 204 || w.Body.Len() != 0 {
		t.Fatal(w.Code, w.Body)
	}
	assertDeleteExists(t, root, "config/a.cfg", false)
	assertDeleteExists(t, root, "config/plugins", false)
	assertDeleteExists(t, root, "config/keep.cfg", true)
	if got := deleteRequest(t, a, "/config", "a.cfg", "plugins"); got != 204 {
		t.Fatal(got)
	}
}

func TestDeleteUnicodeEmptyAndCase(t *testing.T) {
	a, _ := setup(t)
	root := a.Manager.Get(serverID).Root
	putDeleteFixture(t, root, "Café/世界.cfg")
	if e := os.Mkdir(filepath.Join(root, "empty"), 0700); e != nil {
		t.Fatal(e)
	}
	if got := deleteRequest(t, a, "/", "CAFÉ/世界.cfg", "empty"); got != 204 {
		t.Fatal(got)
	}
	assertDeleteExists(t, root, "Café", true)
	assertDeleteExists(t, root, "empty", false)
}

func TestDeleteValidation(t *testing.T) {
	a, _ := setup(t)
	root := a.Manager.Get(serverID).Root
	putDeleteFixture(t, root, "keep")
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"empty files", `{"files":[]}`, 422}, {"null files", `{"files":null}`, 422}, {"null body", `null`, 422}, {"array body", `[]`, 400}, {"malformed", `{`, 400},
		{"server root", `{"root":"/","files":["/"]}`, 422}, {"dot", `{"files":["."]}`, 422}, {"traversal", `{"files":["keep","../escape"]}`, 422},
		{"internal traversal", `{"files":["a/../keep"]}`, 422}, {"invalid root", `{"root":"../","files":["keep"]}`, 422},
		{"device", `{"files":["NUL"]}`, 422}, {"ads", `{"files":["keep:stream"]}`, 422}, {"backslash", `{"files":["a\\keep"]}`, 422},
		{"trailing dot", `{"files":["keep."]}`, 422}, {"duplicate case", `{"files":["keep","KEEP"]}`, 422}, {"overlap", `{"files":["tree","tree/a"]}`, 422},
		{"trailing object", `{"files":["keep"]} {}`, 400}, {"wrong element type", `{"files":[false]}`, 400}, {"oversized", `{"files":["` + strings.Repeat("x", 65536) + `"]}`, 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := request(t, a, "POST", "/api/servers/"+serverID+"/files/delete", tc.body, secret)
			if w.Code != tc.status {
				t.Fatalf("status %d expected %d", w.Code, tc.status)
			}
			assertDeleteExists(t, root, "keep", true)
		})
	}
	if request(t, a, "POST", "/api/servers/"+serverID+"/files/delete", `{"files":["keep"]}`, "").Code != 401 {
		t.Fatal("anonymous mutation")
	}
	if request(t, a, "DELETE", "/api/servers/"+serverID+"/files/delete", `{"files":["keep"]}`, secret).Code != 501 {
		t.Fatal("method")
	}
	assertDeleteExists(t, root, "keep", true)
	files := make([]string, 101)
	for i := range files {
		files[i] = fmt.Sprint(i)
	}
	b, _ := json.Marshal(map[string]any{"files": files})
	if request(t, a, "POST", "/api/servers/"+serverID+"/files/delete", string(b), secret).Code != 422 {
		t.Fatal("batch bound")
	}
}

func TestDeletePolicyPreflight(t *testing.T) {
	for _, name := range []string{"private/a.cfg", "tree/key.secret", "tree"} {
		t.Run(name, func(t *testing.T) {
			a, _ := setupWithSettings(t, func(s *node.Settings) { s.Egg.FileDenylist = []string{"private/", "*.secret"} })
			root := a.Manager.Get(serverID).Root
			for _, n := range []string{"first", "private/a.cfg", "tree/key.secret", "tree/public"} {
				putDeleteFixture(t, root, n)
			}
			if got := deleteRequest(t, a, "/", "first", name); got != 403 {
				t.Fatal(got)
			}
			for _, n := range []string{"first", "private/a.cfg", "tree/key.secret", "tree/public"} {
				assertDeleteExists(t, root, n, true)
			}
			if e := os.Rename(filepath.Join(root, "tree"), filepath.Join(root, "other")); e != nil {
				t.Fatal("preflight leaked handle", e)
			}
		})
	}
}

func TestDeleteLinks(t *testing.T) {
	for _, kind := range []string{"source junction", "parent junction", "descendant junction", "hardlink", "descendant hardlink"} {
		t.Run(kind, func(t *testing.T) {
			a, _ := setup(t)
			root := a.Manager.Get(serverID).Root
			outside := t.TempDir()
			putDeleteFixture(t, outside, "keep")
			putDeleteFixture(t, root, "first")
			putDeleteFixture(t, root, "tree/data")
			target := "alias"
			switch kind {
			case "source junction":
				if e := createTestLink(outside, filepath.Join(root, "alias")); e != nil {
					t.Fatal(e)
				}
			case "parent junction":
				if e := createTestLink(outside, filepath.Join(root, "alias")); e != nil {
					t.Fatal(e)
				}
				target = "alias/keep"
			case "descendant junction":
				if e := createTestLink(outside, filepath.Join(root, "tree", "link")); e != nil {
					t.Fatal(e)
				}
				target = "tree"
			case "hardlink":
				if e := os.Link(filepath.Join(outside, "keep"), filepath.Join(root, "alias")); e != nil {
					t.Fatal(e)
				}
			case "descendant hardlink":
				if e := os.Link(filepath.Join(outside, "keep"), filepath.Join(root, "tree", "alias")); e != nil {
					t.Fatal(e)
				}
				target = "tree"
			}
			if got := deleteRequest(t, a, "/", "first", target); got != 403 {
				t.Fatal(got)
			}
			assertDeleteExists(t, root, "first", true)
			assertDeleteExists(t, outside, "keep", true)
		})
	}
}

func TestDeleteBudget(t *testing.T) {
	for _, n := range []int{maxDeleteEntries - 1, maxDeleteEntries} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			a, _ := setup(t)
			root := a.Manager.Get(serverID).Root
			for i := 0; i < n; i++ {
				putDeleteFixture(t, root, fmt.Sprintf("tree/f%04d", i))
			}
			want := 204
			if n == maxDeleteEntries {
				want = 413
			}
			if got := deleteRequest(t, a, "/", "tree"); got != want {
				t.Fatal(got)
			}
			if want == 413 {
				assertDeleteExists(t, root, "tree/f0000", true)
			} else {
				assertDeleteExists(t, root, "tree", false)
			}
		})
	}
}

func TestDeleteLockedAndReadOnly(t *testing.T) {
	a, _ := setup(t)
	root := a.Manager.Get(serverID).Root
	putDeleteFixture(t, root, "first")
	putDeleteFixture(t, root, "locked")
	f, e := os.Open(filepath.Join(root, "locked"))
	if e != nil {
		t.Fatal(e)
	}
	if got := deleteRequest(t, a, "/", "first", "locked"); got != 423 {
		f.Close()
		t.Fatal(got)
	}
	f.Close()
	assertDeleteExists(t, root, "first", true)
	if e = os.Chmod(filepath.Join(root, "locked"), 0400); e != nil {
		t.Fatal(e)
	}
	defer os.Chmod(filepath.Join(root, "locked"), 0600)
	if got := deleteRequest(t, a, "/", "locked"); got == 204 {
		t.Fatal("read-only file removed")
	}
	assertDeleteExists(t, root, "locked", true)
}

func TestDeleteLateChildPreserved(t *testing.T) {
	a, _ := setup(t)
	s := a.Manager.Get(serverID)
	putDeleteFixture(t, s.Root, "tree/original")
	root, e := os.OpenRoot(s.Root)
	if e != nil {
		t.Fatal(e)
	}
	defer root.Close()
	parent, e := root.Open(".")
	if e != nil {
		t.Fatal(e)
	}
	defer parent.Close()
	remaining := maxDeleteEntries
	p, e := pinDeleteEntry(context.Background(), root, s, parent, "tree", "tree", 0, &remaining)
	if e != nil {
		t.Fatal(e)
	}
	defer p.close()
	putDeleteFixture(t, s.Root, "tree/late")
	e = applyDeletion(context.Background(), root, s, p)
	if status, _ := deleteErrorStatus(e); status != 409 {
		t.Fatal("late child not refused", e)
	}
	assertDeleteExists(t, s.Root, "tree/late", true)
	assertDeleteExists(t, s.Root, "tree/original", false)
	assertDeleteExists(t, s.Root, "tree", true)
}

type deleteAfterContext struct {
	context.Context
	after string
}

func (c deleteAfterContext) Err() error {
	if _, e := os.Stat(c.after); os.IsNotExist(e) {
		return context.Canceled
	}
	return nil
}
func TestDeleteCancellationAndLock(t *testing.T) {
	a, _ := setup(t)
	s := a.Manager.Get(serverID)
	for _, n := range []string{"first", "second"} {
		putDeleteFixture(t, s.Root, n)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := httptest.NewRequest("POST", "/api/servers/"+serverID+"/files/delete", strings.NewReader(`{"files":["first"]}`)).WithContext(ctx)
	r.Header.Set("Authorization", "Bearer "+secret)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 408 {
		t.Fatal(w.Code)
	}
	lock := &sync.Mutex{}
	lock.Lock()
	a.deleteLocks.Store(serverID, lock)
	if got := deleteRequest(t, a, "/", "first"); got != 409 {
		t.Fatal(got)
	}
	lock.Unlock()
	r = httptest.NewRequest("POST", "/api/servers/"+serverID+"/files/delete", strings.NewReader(`{"files":["first","second"]}`)).WithContext(deleteAfterContext{context.Background(), filepath.Join(s.Root, "first")})
	r.Header.Set("Authorization", "Bearer "+secret)
	w = httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 408 || !strings.Contains(w.Body.String(), "earlier deletions") {
		t.Fatal(w.Code, w.Body)
	}
	assertDeleteExists(t, s.Root, "first", false)
	assertDeleteExists(t, s.Root, "second", true)
}

func TestDeleteDepthPreflight(t *testing.T) {
	a, _ := setup(t)
	root := a.Manager.Get(serverID).Root
	putDeleteFixture(t, root, "first")
	putDeleteFixture(t, root, "tree/"+strings.Repeat("n/", 32)+"file")
	if got := deleteRequest(t, a, "/", "first", "tree"); got != 422 {
		t.Fatal(got)
	}
	assertDeleteExists(t, root, "first", true)
}

func TestDeletePinnedObjectAndPolicyRefresh(t *testing.T) {
	denied := false
	a, _ := setupWithSettings(t, func(s *node.Settings) {
		if denied {
			s.Egg.FileDenylist = []string{"target"}
		}
	})
	s := a.Manager.Get(serverID)
	putDeleteFixture(t, s.Root, "target")
	root, e := os.OpenRoot(s.Root)
	if e != nil {
		t.Fatal(e)
	}
	defer root.Close()
	parent, e := root.Open(".")
	if e != nil {
		t.Fatal(e)
	}
	defer parent.Close()
	remaining := maxDeleteEntries
	p, e := pinDeleteEntry(context.Background(), root, s, parent, "target", "target", 0, &remaining)
	if e != nil {
		t.Fatal(e)
	}
	defer p.close()
	if e = os.Rename(filepath.Join(s.Root, "target"), filepath.Join(s.Root, "replacement")); e == nil {
		t.Fatal("pinned target could be replaced")
	}
	denied = true
	if e = s.Sync(context.Background()); e != nil {
		t.Fatal(e)
	}
	e = applyDeletion(context.Background(), root, s, p)
	problem, ok := e.(*directoryFailure)
	if !ok || problem.status != 403 {
		t.Fatal("changed denylist was ignored", e)
	}
	assertDeleteExists(t, s.Root, "target", true)
}
