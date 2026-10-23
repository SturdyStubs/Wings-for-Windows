package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"golang.org/x/sys/windows"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"wings-windows/internal/node"
)

func copyRequest(t *testing.T, a *API, name string) int {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"location": name})
	w := request(t, a, "POST", "/api/servers/"+serverID+"/files/copy", string(body), secret)
	if w.Code != 204 {
		t.Log(w.Body.String())
	}
	return w.Code
}
func assertNoCopyStages(t *testing.T, root string) {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".wings-copy-") {
			t.Fatal("owned copy stage remains", e.Name())
		}
	}
}

func TestCopyPanelContract(t *testing.T) {
	a, _ := setup(t)
	root := a.Manager.Get(serverID).Root
	os.Mkdir(filepath.Join(root, "config"), 0700)
	os.WriteFile(filepath.Join(root, "config", "source.txt"), []byte("source bytes"), 0600)
	body, err := os.ReadFile("testdata/upstream-copy.json")
	if err != nil {
		t.Fatal(err)
	}
	w := request(t, a, "POST", "/api/servers/"+serverID+"/files/copy", string(body), secret)
	if w.Code != 204 || w.Body.Len() != 0 {
		t.Fatal(w.Code, w.Body)
	}
	for _, name := range []string{"source.txt", "source copy.txt"} {
		got, err := os.ReadFile(filepath.Join(root, "config", name))
		if err != nil || string(got) != "source bytes" {
			t.Fatal(name, err)
		}
	}
	assertNoCopyStages(t, filepath.Join(root, "config"))
}

func TestCopyNames(t *testing.T) {
	for name, want := range map[string]string{"file.txt": "file copy.txt", "archive.tar.gz": "archive copy.tar.gz", "plain": "plain copy", ".env": " copy.env", "Café.cfg": "Café copy.cfg"} {
		t.Run(name, func(t *testing.T) {
			if got := copyName(name, 0); got != want {
				t.Fatal(got)
			}
			a, _ := setup(t)
			root := a.Manager.Get(serverID).Root
			os.WriteFile(filepath.Join(root, name), []byte("keep"), 0600)
			for i := 0; i < 2; i++ {
				if got := copyRequest(t, a, name); got != 204 {
					t.Fatal(got)
				}
			}
			for i := 0; i < 2; i++ {
				if data, err := os.ReadFile(filepath.Join(root, copyName(name, i))); err != nil || string(data) != "keep" {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestCopyRejectsPathsAndBodies(t *testing.T) {
	a, _ := setup(t)
	route := "/api/servers/" + serverID + "/files/copy"
	for _, name := range []string{"", "/", "../escape", "a/../file", "C:/file", "file:stream", `a\file`, "CON.txt", "COM¹.cfg", "file.", "file ", "a//b", strings.Repeat("a", 256)} {
		if got := copyRequest(t, a, name); got != 422 {
			t.Fatal(name, got)
		}
	}
	for _, body := range []string{`null`, `[]`, `{}`, `{"location":4}`, `{"location":"a"}{}`, `{"location":`} {
		w := request(t, a, "POST", route, body, secret)
		if w.Code != 400 && w.Code != 422 {
			t.Fatal(w.Code, w.Body)
		}
	}
	if w := request(t, a, "POST", route, `{"location":"`+strings.Repeat("a", 65536)+`"}`, secret); w.Code != 413 {
		t.Fatal(w.Code)
	}
	if w := request(t, a, "POST", route, `{}`, ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w := request(t, a, "PUT", route, `{}`, secret); w.Code != 501 {
		t.Fatal(w.Code)
	}
}

func TestCopyDenylist(t *testing.T) {
	for _, kind := range []string{"source", "destination", "parent", "stage"} {
		t.Run(kind, func(t *testing.T) {
			patterns := map[string][]string{"source": {"source.txt"}, "destination": {"source copy*"}, "parent": {"config/"}, "stage": {".wings-copy-*"}}[kind]
			a, _ := setupWithSettings(t, func(s *node.Settings) { s.Egg.FileDenylist = patterns })
			root := a.Manager.Get(serverID).Root
			name := "source.txt"
			if kind == "parent" {
				os.Mkdir(filepath.Join(root, "config"), 0700)
				name = "config/source.txt"
			}
			os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte("unchanged"), 0600)
			if got := copyRequest(t, a, name); got != 403 {
				t.Fatal(got)
			}
			entries, _ := os.ReadDir(filepath.Dir(filepath.Join(root, filepath.FromSlash(name))))
			if len(entries) != 1 {
				t.Fatal("denied copy changed parent", entries)
			}
		})
	}
}

func TestCopyLinksAndTypes(t *testing.T) {
	for _, kind := range []string{"missing", "directory", "source junction", "parent junction", "hardlink"} {
		t.Run(kind, func(t *testing.T) {
			a, _ := setup(t)
			root := a.Manager.Get(serverID).Root
			outside := t.TempDir()
			os.WriteFile(filepath.Join(outside, "source.txt"), []byte("outside"), 0600)
			os.Mkdir(filepath.Join(root, "dir"), 0700)
			name := "missing"
			expected := 404
			switch kind {
			case "directory":
				name = "dir"
			case "source junction":
				if err := createTestLink(outside, filepath.Join(root, "alias")); err != nil {
					t.Fatal(err)
				}
				name = "alias"
				expected = 403
			case "parent junction":
				if err := createTestLink(outside, filepath.Join(root, "alias")); err != nil {
					t.Fatal(err)
				}
				name = "alias/source.txt"
				expected = 403
			case "hardlink":
				os.WriteFile(filepath.Join(root, "source.txt"), []byte("keep"), 0600)
				if err := os.Link(filepath.Join(root, "source.txt"), filepath.Join(root, "alias")); err != nil {
					t.Fatal(err)
				}
				name = "alias"
				expected = 403
			}
			if got := copyRequest(t, a, name); got != expected {
				t.Fatal(got)
			}
			if files, _ := os.ReadDir(outside); len(files) != 1 {
				t.Fatal("outside changed")
			}
			assertNoCopyStages(t, root)
		})
	}
}

func TestCopyBinaryEmptyAndSuffixExhaustion(t *testing.T) {
	a, _ := setup(t)
	root := a.Manager.Get(serverID).Root
	data := bytes.Repeat([]byte{0, 255, 1}, 3<<20)
	os.WriteFile(filepath.Join(root, "binary.bin"), data, 0600)
	if got := copyRequest(t, a, "binary.bin"); got != 204 {
		t.Fatal(got)
	}
	if got, err := os.ReadFile(filepath.Join(root, "binary copy.bin")); err != nil || !bytes.Equal(got, data) {
		t.Fatal("binary copy differs", err)
	}
	os.WriteFile(filepath.Join(root, "empty"), nil, 0600)
	if got := copyRequest(t, a, "empty"); got != 204 {
		t.Fatal(got)
	}
	for i := 0; i <= 50; i++ {
		os.WriteFile(filepath.Join(root, copyName("occupied", i)), []byte("keep"), 0600)
	}
	os.WriteFile(filepath.Join(root, "occupied"), []byte("source"), 0600)
	if got := copyRequest(t, a, "occupied"); got != 409 {
		t.Fatal(got)
	}
	assertNoCopyStages(t, root)
}

func TestCopySizeAndLockedSource(t *testing.T) {
	a, _ := setup(t)
	root := a.Manager.Get(serverID).Root
	f, err := os.Create(filepath.Join(root, "large"))
	if err != nil {
		t.Fatal(err)
	}
	var result uint32
	if err = windows.DeviceIoControl(windows.Handle(f.Fd()), windows.FSCTL_SET_SPARSE, nil, 0, nil, 0, &result, nil); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err = f.Truncate(maxCopyBytes + 1); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()
	if got := copyRequest(t, a, "large"); got != 413 {
		t.Fatal(got)
	}
	os.WriteFile(filepath.Join(root, "locked"), []byte("keep"), 0600)
	writer, err := os.OpenFile(filepath.Join(root, "locked"), os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	if got := copyRequest(t, a, "locked"); got != 422 {
		t.Fatal(got)
	}
	assertNoCopyStages(t, root)
}

type cancelOnStage struct {
	context.Context
	cancel context.CancelFunc
	root   string
}

func (c cancelOnStage) Err() error {
	entries, _ := os.ReadDir(c.root)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".wings-copy-") {
			c.cancel()
		}
	}
	return c.Context.Err()
}
func TestCopyCancelAfterStageAndLock(t *testing.T) {
	a, _ := setup(t)
	s := a.Manager.Get(serverID)
	os.WriteFile(filepath.Join(s.Root, "source"), []byte("keep"), 0600)
	lock := &sync.Mutex{}
	lock.Lock()
	a.copyLocks.Store(s.ID(), lock)
	if got := copyRequest(t, a, "source"); got != 409 {
		t.Fatal(got)
	}
	lock.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := httptest.NewRequest("POST", "/api/servers/"+serverID+"/files/copy", strings.NewReader(`{"location":"source"}`)).WithContext(cancelOnStage{ctx, cancel, s.Root})
	r.Header.Set("Authorization", "Bearer "+secret)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 408 {
		t.Fatal(w.Code, w.Body)
	}
	assertNoCopyStages(t, s.Root)
	if _, err := os.Stat(filepath.Join(s.Root, "source copy")); !os.IsNotExist(err) {
		t.Fatal("cancelled copy published", err)
	}
	if got, err := os.ReadFile(filepath.Join(s.Root, "source")); err != nil || string(got) != "keep" {
		t.Fatal(err)
	}
}

func TestCopyOwnedStageAndSourcePin(t *testing.T) {
	a, _ := setup(t)
	root := a.Manager.Get(serverID).Root
	os.WriteFile(filepath.Join(root, "source"), []byte("keep"), 0600)
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
	source, err := openCopyHandle(parent, "source", false)
	if err != nil {
		t.Fatal(err)
	}
	if writer, err := os.OpenFile(filepath.Join(root, "source"), os.O_WRONLY, 0); err == nil {
		writer.Close()
		t.Fatal("writer bypassed source pin")
	}
	source.Close()
	stage, err := openCopyHandle(parent, ".wings-copy-test.tmp", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = stage.Write([]byte("partial")); err != nil {
		stage.Close()
		t.Fatal(err)
	}
	if err = os.Rename(filepath.Join(root, ".wings-copy-test.tmp"), filepath.Join(root, "replaced")); err == nil {
		stage.Close()
		t.Fatal("stage replacement allowed")
	}
	if err = discardCopyStage(stage); err != nil {
		stage.Close()
		t.Fatal(err)
	}
	stage.Close()
	assertNoCopyStages(t, root)
	if got, err := os.ReadFile(filepath.Join(root, "source")); err != nil || string(got) != "keep" {
		t.Fatal(err)
	}
}

type failingCopyWriter struct{}

func (failingCopyWriter) Write([]byte) (int, error) { return 0, windows.ERROR_DISK_FULL }
func TestCopyStreamFailures(t *testing.T) {
	if err := copyBytes(context.Background(), strings.NewReader("long"), io.Discard, 3); err == nil {
		t.Fatal("growth accepted")
	}
	if err := copyBytes(context.Background(), strings.NewReader("a"), io.Discard, 3); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatal(err)
	}
	err := copyBytes(context.Background(), strings.NewReader("a"), failingCopyWriter{}, 1)
	if !copyDiskFull(err) {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	copyFailure(w, err)
	if w.Code != 507 {
		t.Fatal(w.Code)
	}
}

func TestCopyOccupiedDirectoryAndCaseAlias(t *testing.T) {
	a, _ := setup(t)
	root := a.Manager.Get(serverID).Root
	os.WriteFile(filepath.Join(root, "source.txt"), []byte("source"), 0600)
	os.Mkdir(filepath.Join(root, "source copy.txt"), 0700)
	os.WriteFile(filepath.Join(root, "SOURCE COPY 1.TXT"), []byte("competitor"), 0600)
	if got := copyRequest(t, a, "source.txt"); got != 204 {
		t.Fatal(got)
	}
	if data, err := os.ReadFile(filepath.Join(root, "source copy 2.txt")); err != nil || string(data) != "source" {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(root, "SOURCE COPY 1.TXT")); err != nil || string(data) != "competitor" {
		t.Fatal("existing case alias overwritten", err)
	}
}

func TestCopyNativePublication(t *testing.T) {
	root := t.TempDir()
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
	stage, err := openCopyHandle(parent, ".wings-copy-stage.tmp", true)
	if err != nil {
		t.Fatal(err)
	}
	defer stage.Close()
	if _, err = stage.Write([]byte("ready")); err != nil {
		t.Fatal(err)
	}
	if err = stage.Sync(); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "copy.txt"), []byte("competitor"), 0600)
	if err = renameNoReplace(stage, parent, "copy.txt"); !os.IsExist(err) {
		t.Fatal("late destination not protected", err)
	}
	if err = renameNoReplace(stage, parent, "copy 1.txt"); err != nil {
		t.Fatal(err)
	}
	stage.Close()
	for name, want := range map[string]string{"copy.txt": "competitor", "copy 1.txt": "ready"} {
		if data, err := os.ReadFile(filepath.Join(root, name)); err != nil || string(data) != want {
			t.Fatal(name, err)
		}
	}
	assertNoCopyStages(t, root)
}
