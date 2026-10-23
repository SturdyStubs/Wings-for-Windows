package api

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"wings-windows/internal/node"
)

func renameRequest(t *testing.T, a *API, root string, pairs ...renamePair) int {
	t.Helper()
	body, _ := json.Marshal(struct {
		Root  string       `json:"root"`
		Files []renamePair `json:"files"`
	}{root, pairs})
	w := request(t, a, "PUT", "/api/servers/"+serverID+"/files/rename", string(body), secret)
	if w.Code != 204 {
		t.Log(w.Body.String())
	}
	return w.Code
}

func TestRenamePanelContract(t *testing.T) {
	a, _ := setup(t)
	root := a.Manager.Get(serverID).Root
	if err := os.MkdirAll(filepath.Join(root, "config", "plugins"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.cfg", "b.cfg"} {
		if err := os.WriteFile(filepath.Join(root, "config", name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	body, err := os.ReadFile("testdata/upstream-rename.json")
	if err != nil {
		t.Fatal(err)
	}
	w := request(t, a, "PUT", "/api/servers/"+serverID+"/files/rename", string(body), secret)
	if w.Code != 204 || w.Body.Len() != 0 {
		t.Fatal(w.Code, w.Body)
	}
	for to, want := range map[string]string{"plugins/a.cfg": "a.cfg", "renamed.cfg": "b.cfg"} {
		got, err := os.ReadFile(filepath.Join(root, "config", filepath.FromSlash(to)))
		if err != nil || string(got) != want {
			t.Fatal(to, err, string(got))
		}
	}
	if got := renameRequest(t, a, "/config", renamePair{"missing.cfg", "also-missing.cfg"}); got != 204 {
		t.Fatal(got)
	}
}

func TestRenameRelativeMovesCaseAndNoop(t *testing.T) {
	a, _ := setup(t)
	root := a.Manager.Get(serverID).Root
	os.Mkdir(filepath.Join(root, "folder"), 0700)
	os.WriteFile(filepath.Join(root, "folder", "Old.cfg"), []byte("keep"), 0600)
	for _, item := range []struct {
		root string
		p    renamePair
	}{{"/folder", renamePair{"Old.cfg", "../New.cfg"}}, {"/", renamePair{"New.cfg", "new.cfg"}}, {"/", renamePair{"new.cfg", "new.cfg"}}} {
		if got := renameRequest(t, a, item.root, item.p); got != 204 {
			t.Fatal(item, got)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range entries {
		if e.Name() == "new.cfg" {
			found = true
		}
	}
	if !found {
		t.Fatal("case-only name not saved")
	}
}

func TestRenameDirectoryAndPreserveContents(t *testing.T) {
	a, _ := setup(t)
	root := a.Manager.Get(serverID).Root
	os.MkdirAll(filepath.Join(root, "old", "child"), 0700)
	os.WriteFile(filepath.Join(root, "old", "child", "Café.cfg"), []byte("unchanged"), 0600)
	os.Mkdir(filepath.Join(root, "destination"), 0700)
	if got := renameRequest(t, a, "/", renamePair{"old", "destination/new"}); got != 204 {
		t.Fatal(got)
	}
	if got, err := os.ReadFile(filepath.Join(root, "destination", "new", "child", "Café.cfg")); err != nil || string(got) != "unchanged" {
		t.Fatal(err, string(got))
	}
}

func TestRenamePreflightRejectsWholeBatch(t *testing.T) {
	for name, pairs := range map[string][]renamePair{
		"existing destination": {{"a", "moved"}, {"b", "existing"}}, "traversal": {{"a", "moved"}, {"b", "../outside"}},
		"device": {{"a", "moved"}, {"b", "CON.cfg"}}, "ADS": {{"a", "moved"}, {"b", "x:stream"}},
		"source overlap": {{"a", "moved"}, {"a", "other"}}, "destination collision": {{"a", "moved"}, {"b", "MOVED"}},
		"chain": {{"a", "b"}, {"b", "other"}}, "child move": {{"folder", "folder/child"}},
		"ancestor overlap": {{"folder", "other"}, {"folder/child", "child"}}, "missing destination parent": {{"a", "missing/moved"}},
	} {
		t.Run(name, func(t *testing.T) {
			a, _ := setup(t)
			root := a.Manager.Get(serverID).Root
			for _, n := range []string{"a", "b", "existing"} {
				os.WriteFile(filepath.Join(root, n), []byte(n), 0600)
			}
			os.MkdirAll(filepath.Join(root, "folder", "child"), 0700)
			code := renameRequest(t, a, "/", pairs...)
			if code != 422 && code != 400 {
				t.Fatal(code)
			}
			for _, n := range []string{"a", "b", "existing"} {
				if data, err := os.ReadFile(filepath.Join(root, n)); err != nil || string(data) != n {
					t.Fatal("preflight changed data", n, err)
				}
			}
		})
	}
}

func TestRenameDenylistDescendants(t *testing.T) {
	for name, pair := range map[string]renamePair{"source": {"private/a", "new"}, "destination": {"a", "private/new"}, "directory descendant": {"folder", "other"}, "new descendant location": {"clean", "target"}} {
		t.Run(name, func(t *testing.T) {
			a, _ := setupWithSettings(t, func(s *node.Settings) { s.Egg.FileDenylist = []string{"private/", "*.secret", "target/inside.cfg"} })
			root := a.Manager.Get(serverID).Root
			for _, n := range []string{"private", "folder", "clean"} {
				os.Mkdir(filepath.Join(root, n), 0700)
			}
			for _, n := range []string{"first", "a", "private/a", "folder/key.secret", "clean/inside.cfg"} {
				os.WriteFile(filepath.Join(root, filepath.FromSlash(n)), []byte("keep"), 0600)
			}
			if got := renameRequest(t, a, "/", renamePair{"first", "moved"}, pair); got != 403 {
				t.Fatal(got)
			}
			if _, err := os.Stat(filepath.Join(root, "first")); err != nil {
				t.Fatal("earlier entry moved before denial", err)
			}
		})
	}
}

func TestRenameLinksAndHardlinks(t *testing.T) {
	for _, kind := range []string{"source junction", "parent junction", "destination junction", "descendant junction", "file hardlink", "descendant hardlink"} {
		t.Run(kind, func(t *testing.T) {
			a, _ := setup(t)
			root := a.Manager.Get(serverID).Root
			outside := t.TempDir()
			os.WriteFile(filepath.Join(outside, "keep"), []byte("keep"), 0600)
			os.WriteFile(filepath.Join(root, "a"), []byte("a"), 0600)
			os.Mkdir(filepath.Join(root, "tree"), 0700)
			pair := renamePair{"a", "new"}
			link := func(name string) {
				t.Helper()
				if err := createTestLink(outside, filepath.Join(root, name)); err != nil {
					t.Fatal(err)
				}
			}
			switch kind {
			case "source junction":
				link("alias")
				pair = renamePair{"alias", "new"}
			case "parent junction":
				link("alias")
				pair = renamePair{"alias/keep", "new"}
			case "destination junction":
				link("alias")
				pair.To = "alias/new"
			case "descendant junction":
				link(filepath.Join("tree", "alias"))
				pair = renamePair{"tree", "new"}
			case "file hardlink":
				if err := os.Link(filepath.Join(root, "a"), filepath.Join(root, "alias")); err != nil {
					t.Fatal(err)
				}
			case "descendant hardlink":
				if err := os.Link(filepath.Join(root, "a"), filepath.Join(root, "tree", "alias")); err != nil {
					t.Fatal(err)
				}
				pair = renamePair{"tree", "new"}
			}
			if got := renameRequest(t, a, "/", pair); got != 403 && got != 422 {
				t.Fatal(got)
			}
			if got, err := os.ReadFile(filepath.Join(outside, "keep")); err != nil || string(got) != "keep" {
				t.Fatal(err)
			}
		})
	}
}

func TestRenameAuthMalformedAndBounds(t *testing.T) {
	a, _ := setup(t)
	route := "/api/servers/" + serverID + "/files/rename"
	for _, body := range []string{`null`, `[]`, `{}`, `{"files":[]}`, `{"files":[{"from":5,"to":"x"}]}`, `{"files":[{}]}`, `{"files":[{"from":"a","to":"b"}]}{}`, `{"files":`} {
		w := request(t, a, "PUT", route, body, secret)
		if w.Code != 400 && w.Code != 422 {
			t.Fatal(body, w.Code, w.Body)
		}
	}
	if got := renameRequest(t, a, "/", make([]renamePair, 101)...); got != 422 {
		t.Fatal(got)
	}
	if w := request(t, a, "PUT", route, `{"root":"`+strings.Repeat("a", 65536)+`"}`, secret); w.Code != 413 {
		t.Fatal(w.Code)
	}
	if w := request(t, a, "PUT", route, `{}`, ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w := request(t, a, "POST", route, `{}`, secret); w.Code != 501 {
		t.Fatal(w.Code)
	}
}

func TestRenameLockCancellationAndScanBound(t *testing.T) {
	a, _ := setup(t)
	s := a.Manager.Get(serverID)
	os.WriteFile(filepath.Join(s.Root, "a"), []byte("a"), 0600)
	lock := &sync.Mutex{}
	lock.Lock()
	a.renameLocks.Store(s.ID(), lock)
	if got := renameRequest(t, a, "/", renamePair{"a", "b"}); got != 409 {
		t.Fatal(got)
	}
	lock.Unlock()
	root, err := os.OpenRoot(s.Root)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := renamePlan{from: "a", to: "b"}
	left := 100
	if err := prepareRename(ctx, root, s, &p, &left); err == nil {
		t.Fatal("cancelled preparation accepted")
	}
	os.Mkdir(filepath.Join(s.Root, "tree"), 0700)
	os.WriteFile(filepath.Join(s.Root, "tree", "a"), []byte("a"), 0600)
	dir, err := root.OpenRoot("tree")
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	left = 1
	if err := scanRenameTree(context.Background(), dir, s, "tree", "new", 0, &left); err == nil {
		t.Fatal("scan bound ignored")
	}
}

func TestRenameNativeCollisionAndSourcePin(t *testing.T) {
	a, _ := setup(t)
	root := a.Manager.Get(serverID).Root
	os.WriteFile(filepath.Join(root, "a"), []byte("original"), 0600)
	r, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	parent, err := r.Open(".")
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	source, err := openRenameSource(parent, "a")
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	if err := os.Rename(filepath.Join(root, "a"), filepath.Join(root, "replaced")); err == nil {
		t.Fatal("ordinary rename replaced pinned source")
	}
	os.WriteFile(filepath.Join(root, "b"), []byte("competitor"), 0600)
	if err := renameNoReplace(source, parent, "b"); err == nil || !os.IsExist(err) {
		t.Fatal("late collision not refused", err)
	}
	source.Close()
	for n, want := range map[string]string{"a": "original", "b": "competitor"} {
		if data, err := os.ReadFile(filepath.Join(root, n)); err != nil || string(data) != want {
			t.Fatal(n, err)
		}
	}
	source, err = openRenameSource(parent, "a")
	if err != nil {
		t.Fatal(err)
	}
	if err := renameNoReplace(source, parent, "c"); err != nil {
		t.Fatal("single-character destination", err)
	}
	source.Close()
	if data, err := os.ReadFile(filepath.Join(root, "c")); err != nil || string(data) != "original" {
		t.Fatal(err)
	}
}

func TestRenamePathContract(t *testing.T) {
	for _, item := range []struct{ root, value, want string }{{"/config", "../a.cfg", "a.cfg"}, {"/config/", "./plugins/a.cfg", "config/plugins/a.cfg"}, {"/", "a/../b.cfg", "b.cfg"}, {"/config", "/a.cfg", "config/a.cfg"}} {
		t.Run(item.value, func(t *testing.T) {
			got, err := renamePath(item.root, item.value)
			if err != nil || got != item.want {
				t.Fatal(got, err)
			}
		})
	}
	for _, item := range []struct{ root, value string }{{"/", "../escape"}, {"/config", "../../escape"}, {"/config", ".."}, {"/", "."}, {"/bad./", "a"}, {"/", "a//b"}, {"/", "COM¹.cfg"}, {"/", `dir\file`}, {"/", "a:stream"}, {"/", strings.Repeat("a", 256)}, {"/", strings.Repeat("d/", 33) + "a"}} {
		if _, err := renamePath(item.root, item.value); err == nil {
			t.Fatal("accepted invalid rename", item)
		}
	}
}
