package api

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"io"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"wings-windows/internal/node"
)

func compressRequest(t *testing.T, a *API, base string, names []string) *httptest.ResponseRecorder {
	t.Helper()
	b, e := json.Marshal(map[string]any{"root": base, "files": names})
	if e != nil {
		t.Fatal(e)
	}
	return request(t, a, "POST", "/api/servers/"+serverID+"/files/compress", string(b), secret)
}
func readTestArchive(t *testing.T, a *API, base string, w *httptest.ResponseRecorder) map[string][]byte {
	t.Helper()
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	var metadata fileMetadata
	if e := json.Unmarshal(w.Body.Bytes(), &metadata); e != nil {
		t.Fatal(e)
	}
	if !strings.HasPrefix(metadata.Name, "archive-") || !strings.HasSuffix(metadata.Name, ".tar.gz") || metadata.Mime != "application/tar+gzip" || !metadata.File || metadata.Directory || metadata.Symlink || metadata.Size <= 0 || metadata.Created == "" || metadata.Modified == "" {
		t.Fatal(metadata)
	}
	b, e := os.ReadFile(filepath.Join(a.Manager.Get(serverID).Root, filepath.FromSlash(base), metadata.Name))
	if e != nil || int64(len(b)) != metadata.Size {
		t.Fatal(e, metadata)
	}
	z, e := gzip.NewReader(bytes.NewReader(b))
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	tr := tar.NewReader(z)
	result := map[string][]byte{}
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		if strings.HasPrefix(h.Name, "/") || strings.Contains(h.Name, "..") || h.Uname != "" || h.Gname != "" || h.Uid != 0 || h.Gid != 0 {
			t.Fatal(h)
		}
		if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeDir {
			t.Fatal(h)
		}
		value, e := io.ReadAll(tr)
		if e != nil {
			t.Fatal(e)
		}
		result[h.Name] = value
	}
	return result
}
func archiveTemps(t *testing.T, a *API) int {
	t.Helper()
	count := 0
	e := filepath.WalkDir(a.Manager.Get(serverID).Root, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if strings.HasPrefix(d.Name(), ".wings-archive-") {
			count++
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	return count
}
func TestCompressPanelContract(t *testing.T) {
	a, _ := setup(t)
	data := []byte{0, 255, 7, 0, 1}
	putDownload(t, a, "config/Café.bin", data)
	putDownload(t, a, "config/nested/data.txt", []byte("hello"))
	putDownload(t, a, "config/nested/deeper/zero", nil)
	putDownload(t, a, "config/empty", nil)
	raw, e := os.ReadFile("testdata/upstream-compress-request.json")
	if e != nil {
		t.Fatal(e)
	}
	w := request(t, a, "POST", "/api/servers/"+serverID+"/files/compress", string(raw), secret)
	contents := readTestArchive(t, a, "config", w)
	if len(contents) != 6 || !bytes.Equal(contents["Café.bin"], data) || string(contents["nested/data.txt"]) != "hello" {
		t.Fatal(contents)
	}
	if _, ok := contents["nested/deeper/"]; !ok {
		t.Fatal(contents)
	}
	if archiveTemps(t, a) != 0 {
		t.Fatal("staging retained")
	}
}
func TestCompressRootNestedSelectionAndLarge(t *testing.T) {
	for _, base := range []string{"", "/", "/config/"} {
		t.Run(fmt.Sprintf("%q", base), func(t *testing.T) {
			a, _ := setup(t)
			data := bytes.Repeat([]byte{0, 255, 1}, 3<<20)
			prefix := strings.Trim(base, "/")
			putDownload(t, a, path.Join(prefix, "nested/large.bin"), data)
			w := compressRequest(t, a, base, []string{"nested/large.bin"})
			got := readTestArchive(t, a, prefix, w)
			if !bytes.Equal(got["nested/large.bin"], data) || len(got) != 1 {
				t.Fatal("large/nested binary mismatch")
			}
		})
	}
}
func TestCompressInputGuards(t *testing.T) {
	for _, tc := range []struct {
		name, base string
		files      []string
	}{
		{"empty", "/", nil}, {"root-selection", "/", []string{"/"}}, {"parent", "/", []string{"../private"}}, {"base-parent", "../", []string{"file"}}, {"ads", "/", []string{"file:stream"}}, {"device", "/", []string{"NUL.txt"}}, {"trailing-dot", "/", []string{"file."}}, {"backslash", "/", []string{`a\file`}}, {"overlap", "/", []string{"dir", "dir/file"}}, {"duplicate-case", "/", []string{"file", "FILE"}}, {"count", "/", make([]string, 101)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, _ := setup(t)
			w := compressRequest(t, a, tc.base, tc.files)
			if w.Code != 422 {
				t.Fatal(w.Code, w.Body)
			}
			uploadClean(t, a, "", nil)
		})
	}
}
func TestCompressPolicyAndResources(t *testing.T) {
	for _, kind := range []string{"missing", "denied-file", "denied-child", "denied-output", "denied-stage", "locked", "hardlink", "junction", "entry-limit", "input-limit"} {
		t.Run(kind, func(t *testing.T) {
			a, _ := setupWithSettings(t, func(s *node.Settings) {
				switch kind {
				case "denied-file", "denied-child":
					s.Egg.FileDenylist = []string{"*.secret"}
				case "denied-output":
					s.Egg.FileDenylist = []string{"archive-*.tar.gz"}
				case "denied-stage":
					s.Egg.FileDenylist = []string{".wings-archive-*"}
				}
			})
			names := []string{"data.bin"}
			want := 403
			putDownload(t, a, "data.bin", []byte("original"))
			switch kind {
			case "missing":
				names = []string{"absent"}
				want = 404
			case "denied-file":
				putDownload(t, a, "data.secret", []byte("private"))
				names = []string{"data.secret"}
			case "denied-child":
				putDownload(t, a, "folder/data.secret", []byte("private"))
				names = []string{"folder"}
			case "locked":
				f, e := os.OpenFile(filepath.Join(a.Manager.Get(serverID).Root, "data.bin"), os.O_WRONLY, 0)
				if e != nil {
					t.Fatal(e)
				}
				defer f.Close()
				want = 423
			case "hardlink":
				if e := os.Link(filepath.Join(a.Manager.Get(serverID).Root, "data.bin"), filepath.Join(a.Manager.Get(serverID).Root, "second")); e != nil {
					t.Fatal(e)
				}
			case "junction":
				names = []string{"linked"}
				if e := createTestLink(t.TempDir(), filepath.Join(a.Manager.Get(serverID).Root, "linked")); e != nil {
					t.Fatal(e)
				}
			case "entry-limit":
				for i := 0; i < maxArchiveEntries; i++ {
					putDownload(t, a, fmt.Sprintf("many/%03d", i), nil)
				}
				names = []string{"many"}
				want = 413
			case "input-limit":
				f, e := os.Create(filepath.Join(a.Manager.Get(serverID).Root, "large"))
				if e != nil {
					t.Fatal(e)
				}
				var n uint32
				if e = windows.DeviceIoControl(windows.Handle(f.Fd()), windows.FSCTL_SET_SPARSE, nil, 0, nil, 0, &n, nil); e != nil {
					f.Close()
					t.Fatal(e)
				}
				if e = f.Truncate(maxArchiveInput + 1); e != nil {
					f.Close()
					t.Fatal(e)
				}
				f.Close()
				names = []string{"large"}
				want = 413
			}
			w := compressRequest(t, a, "/", names)
			if w.Code != want {
				t.Fatal(w.Code, w.Body)
			}
			if archiveTemps(t, a) != 0 {
				t.Fatal("owned stage left")
			}
			b, e := os.ReadFile(filepath.Join(a.Manager.Get(serverID).Root, "data.bin"))
			if e != nil || string(b) != "original" {
				t.Fatal("source changed", e)
			}
			entries, _ := os.ReadDir(a.Manager.Get(serverID).Root)
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), "archive-") {
					t.Fatal("archive published on refusal")
				}
			}
		})
	}
}
func TestCompressCancellationAndPinning(t *testing.T) {
	for _, kind := range []string{"cancel", "shutdown", "pin"} {
		t.Run(kind, func(t *testing.T) {
			a, _ := setup(t)
			data := bytes.Repeat([]byte("hello"), 1<<17)
			putDownload(t, a, "data.bin", data)
			s := a.Manager.Get(serverID)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			fired := false
			check := func() error {
				if ctx.Err() != nil {
					return &directoryFailure{408, "cancelled"}
				}
				entries, e := os.ReadDir(s.Root)
				if e != nil {
					return e
				}
				staging := false
				for _, entry := range entries {
					if strings.HasPrefix(entry.Name(), ".wings-archive-") {
						info, e := entry.Info()
						if e != nil {
							return e
						}
						staging = info.Size() > 0
					}
				}
				if staging && !fired {
					fired = true
					switch kind {
					case "cancel":
						cancel()
						return &directoryFailure{408, "cancelled"}
					case "shutdown":
						a.Close()
						return &directoryFailure{503, "stopping"}
					case "pin":
						f, e := os.OpenFile(filepath.Join(s.Root, "data.bin"), os.O_WRONLY, 0)
						if e == nil {
							f.Close()
							t.Fatal("source writer admitted")
						}
						if e = os.Rename(filepath.Join(s.Root, "data.bin"), filepath.Join(s.Root, "renamed")); e == nil {
							t.Fatal("source rename admitted")
						}
					}
				}
				return nil
			}
			metadata, e := createArchive(ctx, s, ".", []string{"data.bin"}, check)
			if !fired {
				t.Fatal("guard did not run during staging")
			}
			if kind == "pin" {
				if e != nil || metadata.Name == "" {
					t.Fatal(e)
				}
			} else {
				var failure *directoryFailure
				if !errors.As(e, &failure) {
					t.Fatal(e)
				}
				want := 408
				if kind == "shutdown" {
					want = 503
				}
				if failure.status != want {
					t.Fatal(e)
				}
			}
			if archiveTemps(t, a) != 0 {
				t.Fatal("stage cleanup failed")
			}
			f, e := os.OpenFile(filepath.Join(s.Root, "data.bin"), os.O_WRONLY, 0)
			if e != nil {
				t.Fatal("source handle leaked", e)
			}
			f.Close()
		})
	}
}

func TestCompressPolicyRefreshDuringStaging(t *testing.T) {
	var denied atomic.Bool
	a, _ := setupWithSettings(t, func(s *node.Settings) {
		if denied.Load() {
			s.Egg.FileDenylist = []string{"data.bin"}
		}
	})
	putDownload(t, a, "data.bin", bytes.Repeat([]byte("x"), 256<<10))
	fired := false
	check := func() error {
		entries, e := os.ReadDir(a.Manager.Get(serverID).Root)
		if e != nil {
			return e
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".wings-archive-") {
				info, e := entry.Info()
				if e != nil {
					return e
				}
				if info.Size() > 0 && !fired {
					fired = true
					denied.Store(true)
					return a.Manager.Get(serverID).Sync(context.Background())
				}
			}
		}
		return nil
	}
	_, e := createArchive(context.Background(), a.Manager.Get(serverID), ".", []string{"data.bin"}, check)
	var failure *directoryFailure
	if !fired || !errors.As(e, &failure) || failure.status != 403 {
		t.Fatal("refreshed policy not enforced", fired, e)
	}
	if archiveTemps(t, a) != 0 {
		t.Fatal("temporary archive remains")
	}
}

func TestCompressAggregateBoundary(t *testing.T) {
	for _, extra := range []int64{0, 1} {
		t.Run(fmt.Sprint(extra), func(t *testing.T) {
			a, _ := setup(t)
			for name, size := range map[string]int64{"a": 60 << 20, "b": (40 << 20) + extra} {
				f, e := os.Create(filepath.Join(a.Manager.Get(serverID).Root, name))
				if e != nil {
					t.Fatal(e)
				}
				var n uint32
				if e = windows.DeviceIoControl(windows.Handle(f.Fd()), windows.FSCTL_SET_SPARSE, nil, 0, nil, 0, &n, nil); e == nil {
					e = f.Truncate(size)
				}
				f.Close()
				if e != nil {
					t.Fatal(e)
				}
			}
			w := compressRequest(t, a, "/", []string{"a", "b"})
			if extra == 1 {
				if w.Code != 413 {
					t.Fatal(w.Code, w.Body)
				}
			} else {
				if w.Code != 200 {
					t.Fatal(w.Code, w.Body)
				}
				var info fileMetadata
				if e := json.Unmarshal(w.Body.Bytes(), &info); e != nil {
					t.Fatal(e)
				}
				f, e := os.Open(filepath.Join(a.Manager.Get(serverID).Root, info.Name))
				if e != nil {
					t.Fatal(e)
				}
				defer f.Close()
				z, e := gzip.NewReader(f)
				if e != nil {
					t.Fatal(e)
				}
				defer z.Close()
				tr := tar.NewReader(z)
				total := int64(0)
				for {
					_, e := tr.Next()
					if e == io.EOF {
						break
					}
					if e != nil {
						t.Fatal(e)
					}
					n, e := io.Copy(io.Discard, tr)
					if e != nil {
						t.Fatal(e)
					}
					total += n
				}
				if total != maxArchiveInput {
					t.Fatal(total)
				}
			}
			if archiveTemps(t, a) != 0 {
				t.Fatal("staging retained")
			}
		})
	}
}
func TestCompressAdmissionAndOutputBound(t *testing.T) {
	a, _ := setup(t)
	putDownload(t, a, "file", nil)
	a.archiveLocks.Store(serverID, &sync.Mutex{})
	v, _ := a.archiveLocks.Load(serverID)
	lock := v.(*sync.Mutex)
	lock.Lock()
	if w := compressRequest(t, a, "/", []string{"file"}); w.Code != 409 {
		t.Fatal(w.Code)
	}
	lock.Unlock()
	for i := 0; i < cap(a.archiveSlots); i++ {
		a.archiveSlots <- struct{}{}
	}
	if w := compressRequest(t, a, "/", []string{"file"}); w.Code != 503 {
		t.Fatal(w.Code)
	}
	for i := 0; i < cap(a.archiveSlots); i++ {
		<-a.archiveSlots
	}
	var b bytes.Buffer
	w := &archiveOutput{writer: &b, remaining: 3, check: func() error { return nil }}
	if n, e := w.Write([]byte("abc")); e != nil || n != 3 {
		t.Fatal(n, e)
	}
	if n, e := w.Write([]byte("x")); n != 0 || e == nil || b.String() != "abc" {
		t.Fatal(n, e, b.String())
	}
	a.Close()
	if w := compressRequest(t, a, "/", []string{"file"}); w.Code != 503 {
		t.Fatal(w.Code)
	}
}

func TestCompressRequestCancellation(t *testing.T) {
	a, _ := setup(t)
	putDownload(t, a, "file", nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := httptest.NewRequest("POST", "/api/servers/"+serverID+"/files/compress", strings.NewReader(`{"root":"/","files":["file"]}`)).WithContext(ctx)
	r.Header.Set("Authorization", "Bearer "+secret)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 408 {
		t.Fatal(w.Code, w.Body)
	}
	ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	_, e := createArchive(ctx, a.Manager.Get(serverID), ".", []string{"file"}, func() error { return ctx.Err() })
	if !errors.Is(e, context.DeadlineExceeded) {
		t.Fatal(e)
	}
}
