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
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"wings-windows/internal/node"
)

type tarTestEntry struct {
	name string
	kind byte
	data []byte
	link string
	pax  map[string]string
}

func testTar(t *testing.T, entries ...tarTestEntry) []byte {
	t.Helper()
	var b bytes.Buffer
	tw := tar.NewWriter(&b)
	for _, entry := range entries {
		h := &tar.Header{Name: entry.name, Typeflag: entry.kind, Size: int64(len(entry.data)), Linkname: entry.link, Mode: 07777, Uid: 123, Gid: 456, Format: tar.FormatPAX, PAXRecords: entry.pax}
		if entry.kind == tar.TypeXGlobalHeader {
			h = &tar.Header{Typeflag: tar.TypeXGlobalHeader, PAXRecords: map[string]string{"comment": "fixture"}}
		}
		if entry.kind == tar.TypeDir || entry.kind == tar.TypeSymlink || entry.kind == tar.TypeLink {
			h.Size = 0
		}
		if e := tw.WriteHeader(h); e != nil {
			t.Fatal(e)
		}
		if h.Size > 0 {
			if _, e := tw.Write(entry.data); e != nil {
				t.Fatal(e)
			}
		}
	}
	if e := tw.Close(); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func testGzip(t *testing.T, tarBytes []byte) []byte {
	t.Helper()
	var b bytes.Buffer
	z := gzip.NewWriter(&b)
	if _, e := z.Write(tarBytes); e != nil {
		t.Fatal(e)
	}
	if e := z.Close(); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func extractRequest(t *testing.T, a *API, root, file string) *httptest.ResponseRecorder {
	t.Helper()
	b, e := json.Marshal(map[string]string{"root": root, "file": file})
	if e != nil {
		t.Fatal(e)
	}
	return request(t, a, "POST", "/api/servers/"+serverID+"/files/decompress", string(b), secret)
}
func extractStages(t *testing.T, a *API) int {
	t.Helper()
	entries, e := os.ReadDir(a.Manager.Get(serverID).Root)
	if e != nil {
		t.Fatal(e)
	}
	n := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".wings-extract-") {
			n++
		}
	}
	return n
}

func TestExtractPanelContract(t *testing.T) {
	a, _ := setup(t)
	data := []byte{0, 255, 1, 0, 7}
	archive := testGzip(t, testTar(t, tarTestEntry{name: "nested/Café.bin", data: data, kind: tar.TypeReg}, tarTestEntry{name: "empty-dir/", kind: tar.TypeDir}, tarTestEntry{name: "zero", kind: tar.TypeReg}))
	putDownload(t, a, "target/bundle.tar.gz", archive)
	raw, e := os.ReadFile("testdata/upstream-extract-request.json")
	if e != nil {
		t.Fatal(e)
	}
	w := request(t, a, "POST", "/api/servers/"+serverID+"/files/decompress", string(raw), secret)
	if w.Code != 204 {
		t.Fatal(w.Code, w.Body)
	}
	b, e := os.ReadFile(filepath.Join(a.Manager.Get(serverID).Root, "target", "nested", "Café.bin"))
	if e != nil || !bytes.Equal(b, data) {
		t.Fatal(e, b)
	}
	for _, name := range []string{"empty-dir", "nested"} {
		info, e := os.Stat(filepath.Join(a.Manager.Get(serverID).Root, "target", name))
		if e != nil || !info.IsDir() {
			t.Fatal(name, e)
		}
	}
	if e := os.WriteFile(filepath.Join(a.Manager.Get(serverID).Root, "target", "nested", "Café.bin"), []byte("editable"), 0600); e != nil {
		t.Fatal("archive mode was applied or handle leaked", e)
	}
	if w = extractRequest(t, a, "target", "bundle.tar.gz"); w.Code != 409 {
		t.Fatal("repeat must preserve destinations", w.Code, w.Body)
	}
	if extractStages(t, a) != 0 {
		t.Fatal("temporary data remains")
	}
}
func TestExtractCreatedArchiveRoundTrip(t *testing.T) {
	a, _ := setup(t)
	data := bytes.Repeat([]byte{0, 255, 7}, 3<<20)
	putDownload(t, a, "source/tree/big.bin", data)
	putDownload(t, a, "source/tree/empty", nil)
	w := compressRequest(t, a, "source", []string{"tree"})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	var m fileMetadata
	if e := json.Unmarshal(w.Body.Bytes(), &m); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(filepath.Join(a.Manager.Get(serverID).Root, "source", m.Name))
	if e != nil {
		t.Fatal(e)
	}
	putDownload(t, a, "target/bundle", b)
	w = extractRequest(t, a, "target", "bundle")
	if w.Code != 204 {
		t.Fatal(w.Code, w.Body)
	}
	b, e = os.ReadFile(filepath.Join(a.Manager.Get(serverID).Root, "target", "tree", "big.bin"))
	if e != nil || !bytes.Equal(b, data) {
		t.Fatal("round trip", e)
	}
}
func TestExtractUnsafePathsAndTypes(t *testing.T) {
	for _, name := range []string{"../escape", "/escape", `C:\escape`, `a\b`, "file:stream", "NUL.txt", "CON", "COM¹.txt", "bad.", "bad ", "a/../b", "a//b", "./b", "a/./b", strings.Repeat("x", 256)} {
		t.Run(fmt.Sprintf("%q", name), func(t *testing.T) {
			a, _ := setup(t)
			b := testGzip(t, testTar(t, tarTestEntry{name: "good", kind: tar.TypeReg, data: []byte("never publish")}, tarTestEntry{name: name, kind: tar.TypeReg, data: []byte("bad")}))
			putDownload(t, a, "bundle", b)
			w := extractRequest(t, a, "/", "bundle")
			if w.Code != 422 {
				t.Fatal(w.Code, w.Body)
			}
			uploadClean(t, a, "", map[string][]byte{"bundle": b})
		})
	}
	for _, kind := range []byte{tar.TypeSymlink, tar.TypeLink, tar.TypeFifo, tar.TypeChar, tar.TypeBlock, tar.TypeXGlobalHeader} {
		t.Run(fmt.Sprint(kind), func(t *testing.T) {
			a, _ := setup(t)
			b := testGzip(t, testTar(t, tarTestEntry{name: "good", kind: tar.TypeReg, data: []byte("no publish")}, tarTestEntry{name: "unsafe", kind: kind, link: "../outside"}))
			putDownload(t, a, "bundle", b)
			w := extractRequest(t, a, "/", "bundle")
			if w.Code != 422 {
				t.Fatal(w.Code, w.Body)
			}
			uploadClean(t, a, "", map[string][]byte{"bundle": b})
		})
	}
}
func TestExtractAliasesAndConflicts(t *testing.T) {
	for _, entries := range [][]tarTestEntry{
		{{name: "a", kind: tar.TypeReg}, {name: "a", kind: tar.TypeReg}},
		{{name: "a", kind: tar.TypeReg}, {name: "A", kind: tar.TypeReg}},
		{{name: "a/one", kind: tar.TypeReg}, {name: "A/two", kind: tar.TypeReg}},
		{{name: "a", kind: tar.TypeReg}, {name: "a/b", kind: tar.TypeReg}},
		{{name: "a/", kind: tar.TypeDir}, {name: "a/", kind: tar.TypeDir}},
	} {
		t.Run(fmt.Sprint(entries), func(t *testing.T) {
			a, _ := setup(t)
			b := testGzip(t, testTar(t, entries...))
			putDownload(t, a, "bundle", b)
			w := extractRequest(t, a, "/", "bundle")
			if w.Code != 422 {
				t.Fatal(w.Code, w.Body)
			}
			uploadClean(t, a, "", map[string][]byte{"bundle": b})
		})
	}
}
func TestExtractFramingAndIntegrity(t *testing.T) {
	for _, kind := range []string{"checksum", "truncated-gzip", "truncated-tar", "missing-terminator", "second-member", "compressed-trailer", "tar-trailer", "empty", "plain-tar"} {
		t.Run(kind, func(t *testing.T) {
			a, _ := setup(t)
			raw := testTar(t, tarTestEntry{name: "good", kind: tar.TypeReg, data: []byte("must stay unpublished")})
			b := testGzip(t, raw)
			switch kind {
			case "checksum":
				b[len(b)-8] ^= 1
			case "truncated-gzip":
				b = b[:len(b)-5]
			case "truncated-tar":
				b = testGzip(t, raw[:520])
			case "missing-terminator":
				b = testGzip(t, raw[:len(raw)-1024])
			case "second-member":
				b = append(b, testGzip(t, raw)...)
			case "compressed-trailer":
				b = append(b, []byte("junk")...)
			case "tar-trailer":
				b = testGzip(t, append(raw, []byte("junk")...))
			case "empty":
				b = testGzip(t, testTar(t))
			case "plain-tar":
				b = raw
			}
			putDownload(t, a, "bundle", b)
			w := extractRequest(t, a, "/", "bundle")
			if w.Code != 400 && w.Code != 422 {
				t.Fatal(w.Code, w.Body)
			}
			uploadClean(t, a, "", map[string][]byte{"bundle": b})
		})
	}
}
func TestExtractExistingTargetsAndPolicy(t *testing.T) {
	for _, kind := range []string{"file", "directory", "junction", "denied-source", "denied-child", "denied-stage", "locked-source"} {
		t.Run(kind, func(t *testing.T) {
			a, _ := setupWithSettings(t, func(s *node.Settings) {
				switch kind {
				case "denied-source":
					s.Egg.FileDenylist = []string{"bundle"}
				case "denied-child":
					s.Egg.FileDenylist = []string{"*.secret"}
				case "denied-stage":
					s.Egg.FileDenylist = []string{".wings-extract-*"}
				}
			})
			name := "new/data"
			want := 409
			if kind == "denied-child" {
				name = "new/data.secret"
			}
			b := testGzip(t, testTar(t, tarTestEntry{name: name, kind: tar.TypeReg, data: []byte("new")}))
			putDownload(t, a, "bundle", b)
			switch kind {
			case "file":
				putDownload(t, a, "new", []byte("old"))
			case "directory":
				putDownload(t, a, "new/retained", []byte("old"))
			case "junction":
				if e := createTestLink(t.TempDir(), filepath.Join(a.Manager.Get(serverID).Root, "new")); e != nil {
					t.Fatal(e)
				}
			case "denied-source", "denied-child", "denied-stage":
				want = 403
			case "locked-source":
				want = 423
				f, e := os.OpenFile(filepath.Join(a.Manager.Get(serverID).Root, "bundle"), os.O_WRONLY, 0)
				if e != nil {
					t.Fatal(e)
				}
				defer f.Close()
			}
			w := extractRequest(t, a, "/", "bundle")
			if w.Code != want {
				t.Fatal(w.Code, w.Body)
			}
			if extractStages(t, a) != 0 {
				t.Fatal("stage remains")
			}
			if kind == "directory" {
				p, e := os.ReadFile(filepath.Join(a.Manager.Get(serverID).Root, "new", "retained"))
				if e != nil || string(p) != "old" {
					t.Fatal(e)
				}
			}
		})
	}
}
func TestExtractPathCountAndExpandedBounds(t *testing.T) {
	for _, kind := range []string{"entries", "implicit", "expanded", "decoded-padding"} {
		t.Run(kind, func(t *testing.T) {
			a, _ := setup(t)
			var entries []tarTestEntry
			switch kind {
			case "entries":
				for i := 0; i < 257; i++ {
					entries = append(entries, tarTestEntry{name: fmt.Sprint(i), kind: tar.TypeReg})
				}
			case "implicit":
				for i := 0; i < 129; i++ {
					entries = append(entries, tarTestEntry{name: fmt.Sprintf("dir%d/file", i), kind: tar.TypeReg})
				}
			}
			var b []byte
			if kind == "expanded" {
				var body bytes.Buffer
				z := gzip.NewWriter(&body)
				tw := tar.NewWriter(z)
				size := maxArchiveInput + 1
				if e := tw.WriteHeader(&tar.Header{Name: "huge", Size: size, Mode: 0600}); e != nil {
					t.Fatal(e)
				}
				if _, e := io.CopyN(tw, zeroUploadReader{}, size); e != nil {
					t.Fatal(e)
				}
				tw.Close()
				z.Close()
				b = body.Bytes()
			} else if kind == "decoded-padding" {
				var body bytes.Buffer
				z := gzip.NewWriter(&body)
				z.Write(testTar(t, tarTestEntry{name: "empty", kind: tar.TypeReg}))
				io.CopyN(z, zeroUploadReader{}, maxExtractStream)
				z.Close()
				b = body.Bytes()
			} else {
				b = testGzip(t, testTar(t, entries...))
			}
			putDownload(t, a, "bundle", b)
			w := extractRequest(t, a, "/", "bundle")
			if w.Code != 413 {
				t.Fatal(w.Code, w.Body)
			}
			uploadClean(t, a, "", map[string][]byte{"bundle": b})
		})
	}
}
func TestExtractCancellationPolicyAndLateCollision(t *testing.T) {
	for _, kind := range []string{"cancel", "policy", "collision", "shutdown"} {
		t.Run(kind, func(t *testing.T) {
			var denied atomic.Bool
			a, _ := setupWithSettings(t, func(s *node.Settings) {
				if denied.Load() {
					s.Egg.FileDenylist = []string{"data"}
				}
			})
			b := testGzip(t, testTar(t, tarTestEntry{name: "data", kind: tar.TypeReg, data: bytes.Repeat([]byte("x"), 256<<10)}))
			putDownload(t, a, "bundle", b)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			fired := false
			check := func() error {
				if ctx.Err() != nil {
					return &directoryFailure{408, "cancelled"}
				}
				staged := false
				list, e := os.ReadDir(a.Manager.Get(serverID).Root)
				if e != nil {
					return e
				}
				for _, item := range list {
					if strings.HasPrefix(item.Name(), ".wings-extract-") {
						info, e := item.Info()
						if e != nil {
							return e
						}
						if info.Size() > 0 {
							staged = true
						}
					}
				}
				if !fired && staged {
					fired = true
					switch kind {
					case "cancel":
						cancel()
						return &directoryFailure{408, "cancelled"}
					case "policy":
						denied.Store(true)
						return a.Manager.Get(serverID).Sync(context.Background())
					case "collision":
						putDownload(t, a, "data", []byte("late"))
					case "shutdown":
						a.Close()
						return &directoryFailure{503, "stopping"}
					}
				}
				return nil
			}
			e := extractArchive(ctx, a.Manager.Get(serverID), "", "bundle", check)
			var failure *directoryFailure
			if !fired || !errors.As(e, &failure) {
				t.Fatal(fired, e)
			}
			want := 408
			if kind == "policy" {
				want = 403
			}
			if kind == "collision" {
				want = 409
			}
			if kind == "shutdown" {
				want = 503
			}
			if failure.status != want {
				t.Fatal(e)
			}
			if extractStages(t, a) != 0 {
				t.Fatal("stage remains")
			}
			if kind == "collision" {
				data, e := os.ReadFile(filepath.Join(a.Manager.Get(serverID).Root, "data"))
				if e != nil || string(data) != "late" {
					t.Fatal(e)
				}
			}
		})
	}
}

func TestExtractExactBoundsAndOversizeSource(t *testing.T) {
	for _, kind := range []string{"bytes", "entries", "compressed"} {
		t.Run(kind, func(t *testing.T) {
			a, _ := setup(t)
			var b []byte
			if kind == "bytes" {
				var body bytes.Buffer
				z := gzip.NewWriter(&body)
				tw := tar.NewWriter(z)
				if e := tw.WriteHeader(&tar.Header{Name: "large", Size: maxArchiveInput, Mode: 0600}); e != nil {
					t.Fatal(e)
				}
				if _, e := io.CopyN(tw, zeroUploadReader{}, maxArchiveInput); e != nil {
					t.Fatal(e)
				}
				tw.Close()
				z.Close()
				b = body.Bytes()
			} else if kind == "entries" {
				var entries []tarTestEntry
				for i := 0; i < 256; i++ {
					entries = append(entries, tarTestEntry{name: fmt.Sprintf("f%03d", i), kind: tar.TypeReg})
				}
				b = testGzip(t, testTar(t, entries...))
			}
			if kind == "compressed" {
				f, e := os.Create(filepath.Join(a.Manager.Get(serverID).Root, "bundle"))
				if e != nil {
					t.Fatal(e)
				}
				var n uint32
				if e = windows.DeviceIoControl(windows.Handle(f.Fd()), windows.FSCTL_SET_SPARSE, nil, 0, nil, 0, &n, nil); e == nil {
					e = f.Truncate(maxArchiveOutput + 1)
				}
				f.Close()
				if e != nil {
					t.Fatal(e)
				}
			} else {
				putDownload(t, a, "bundle", b)
			}
			w := extractRequest(t, a, "/", "bundle")
			want := 204
			if kind == "compressed" {
				want = 413
			}
			if w.Code != want {
				t.Fatal(w.Code, w.Body)
			}
			if kind == "bytes" {
				info, e := os.Stat(filepath.Join(a.Manager.Get(serverID).Root, "large"))
				if e != nil || info.Size() != maxArchiveInput {
					t.Fatal(info, e)
				}
			}
			if kind == "entries" {
				list, e := os.ReadDir(a.Manager.Get(serverID).Root)
				if e != nil || len(list) != 257 {
					t.Fatal(len(list), e)
				}
			}
			if extractStages(t, a) != 0 {
				t.Fatal("stage remains")
			}
		})
	}
}

func TestExtractPublicationGuardsAndPartialOutcome(t *testing.T) {
	for _, kind := range []string{"pin", "late-child", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			a, _ := setup(t)
			data := []byte("new")
			putDownload(t, a, "bundle", testGzip(t, testTar(t, tarTestEntry{name: "new/data", kind: tar.TypeReg, data: data})))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			fired := false
			check := func() error {
				if ctx.Err() != nil {
					return &directoryFailure{408, "cancelled"}
				}
				if _, e := os.Stat(filepath.Join(a.Manager.Get(serverID).Root, "new")); e == nil && !fired {
					fired = true
					switch kind {
					case "pin":
						if e = os.Rename(filepath.Join(a.Manager.Get(serverID).Root, "new"), filepath.Join(a.Manager.Get(serverID).Root, "moved")); e == nil {
							t.Fatal("published parent was movable during extraction")
						}
					case "late-child":
						putDownload(t, a, "new/data", []byte("late"))
					case "cancel":
						cancel()
						return &directoryFailure{408, "cancelled"}
					}
				}
				return nil
			}
			e := extractArchive(ctx, a.Manager.Get(serverID), "", "bundle", check)
			if !fired {
				t.Fatal("publication guard did not run")
			}
			switch kind {
			case "pin":
				if e != nil {
					t.Fatal(e)
				}
			case "late-child":
				if !os.IsExist(e) {
					t.Fatal(e)
				}
			case "cancel":
				var failure *directoryFailure
				if !errors.As(e, &failure) || failure.status != 408 {
					t.Fatal(e)
				}
			}
			if extractStages(t, a) != 0 {
				t.Fatal("stage remains")
			}
			if _, e = os.Stat(filepath.Join(a.Manager.Get(serverID).Root, "new")); e != nil {
				t.Fatal("published directory should be retained", e)
			}
			if kind == "late-child" {
				b, e := os.ReadFile(filepath.Join(a.Manager.Get(serverID).Root, "new", "data"))
				if e != nil || string(b) != "late" {
					t.Fatal(e)
				}
			}
		})
	}
}
func TestExtractAdmissionAndCancellation(t *testing.T) {
	a, _ := setup(t)
	b := testGzip(t, testTar(t, tarTestEntry{name: "data", kind: tar.TypeReg}))
	putDownload(t, a, "bundle", b)
	lock := &sync.Mutex{}
	lock.Lock()
	a.archiveLocks.Store(serverID, lock)
	if w := extractRequest(t, a, "/", "bundle"); w.Code != 409 {
		t.Fatal(w.Code)
	}
	lock.Unlock()
	for i := 0; i < cap(a.archiveSlots); i++ {
		a.archiveSlots <- struct{}{}
	}
	if w := extractRequest(t, a, "/", "bundle"); w.Code != 503 {
		t.Fatal(w.Code)
	}
	for i := 0; i < cap(a.archiveSlots); i++ {
		<-a.archiveSlots
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	r := httptest.NewRequest("POST", "/api/servers/"+serverID+"/files/decompress", strings.NewReader(`{"root":"/","file":"bundle"}`)).WithContext(ctx)
	r.Header.Set("Authorization", "Bearer "+secret)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 408 {
		t.Fatal(w.Code, w.Body)
	}
	a.Close()
	if w := extractRequest(t, a, "/", "bundle"); w.Code != 503 {
		t.Fatal(w.Code, w.Body)
	}
}
