package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractZIPRawStoredCommentAndReadOnly(t *testing.T) {
	a, _ := setup(t)
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	if e := z.SetComment("ZIP comment"); e != nil {
		t.Fatal(e)
	}
	data := []byte("stored without a data descriptor")
	h := &zip.FileHeader{Name: "read-only", Method: zip.Store, CRC32: crc32.ChecksumIEEE(data), CompressedSize64: uint64(len(data)), UncompressedSize64: uint64(len(data))}
	h.SetMode(0444)
	w, e := z.CreateRaw(h)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = w.Write(data); e != nil {
		t.Fatal(e)
	}
	if e = z.Close(); e != nil {
		t.Fatal(e)
	}
	putDownload(t, a, "bundle", b.Bytes())
	if r := extractRequest(t, a, "/", "bundle"); r.Code != 204 {
		t.Fatal(r.Code, r.Body)
	}
	got, e := os.ReadFile(filepath.Join(a.Manager.Get(serverID).Root, "read-only"))
	if e != nil || !bytes.Equal(got, data) {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(a.Manager.Get(serverID).Root, "read-only"), []byte("updated"), 0600); e != nil {
		t.Fatal("metadata was applied", e)
	}
}

func TestExtractZIPEmptyAndPrefixedRefused(t *testing.T) {
	for _, kind := range []string{"empty", "prefix"} {
		t.Run(kind, func(t *testing.T) {
			a, _ := setup(t)
			b := testZIP(t)
			if kind == "prefix" {
				b = append([]byte("MZ fake executable"), testZIP(t, zipTestEntry{name: "payload"})...)
			}
			putDownload(t, a, "bundle", b)
			if r := extractRequest(t, a, "/", "bundle"); r.Code < 400 {
				t.Fatal(r.Code, r.Body)
			}
		})
	}
}

type zipTestEntry struct {
	name   string
	data   []byte
	mode   os.FileMode
	method uint16
}

func testZIP(t *testing.T, entries ...zipTestEntry) []byte {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for _, entry := range entries {
		h := &zip.FileHeader{Name: entry.name, Method: entry.method}
		if entry.mode != 0 {
			h.SetMode(entry.mode)
		}
		w, e := z.CreateHeader(h)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = w.Write(entry.data); e != nil {
			t.Fatal(e)
		}
	}
	if e := z.Close(); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func TestExtractZIPFormatsAndContract(t *testing.T) {
	for _, method := range []uint16{zip.Store, zip.Deflate} {
		t.Run(fmt.Sprint(method), func(t *testing.T) {
			a, _ := setup(t)
			data := bytes.Repeat([]byte{0, 255, 42}, 3<<20)
			b := testZIP(t, zipTestEntry{name: "nested/Café.bin", data: data, method: method}, zipTestEntry{name: "empty/"}, zipTestEntry{name: "zero"})
			putDownload(t, a, "target/bundle.dat", b)
			w := extractRequest(t, a, "/target", "bundle.dat")
			if w.Code != 204 {
				t.Fatal(w.Code, w.Body)
			}
			got, e := os.ReadFile(filepath.Join(a.Manager.Get(serverID).Root, "target/nested/Café.bin"))
			if e != nil || !bytes.Equal(got, data) {
				t.Fatal(e, "integrity")
			}
			if w = extractRequest(t, a, "target", "bundle.dat"); w.Code != 409 {
				t.Fatal(w.Code, w.Body)
			}
			if e = os.WriteFile(filepath.Join(a.Manager.Get(serverID).Root, "target/nested/Café.bin"), []byte("editable"), 0600); e != nil {
				t.Fatal(e)
			}
			if extractStages(t, a) != 0 {
				t.Fatal("stage remains")
			}
		})
	}
}
func TestExtractZIPUnsafePathsAndModes(t *testing.T) {
	for _, name := range []string{"../escape", "/absolute", `back\slash`, "C:/drive", "a//b", "a/./b", "a/../b", "file:stream", "NUL", "CON.txt", "trailing.", "trailing ", "a//", "./", "\x00bad", strings.Repeat("a", 256)} {
		t.Run(fmt.Sprintf("%q", name), func(t *testing.T) {
			a, _ := setup(t)
			putDownload(t, a, "bundle", testZIP(t, zipTestEntry{name: "safe", data: []byte("staged")}, zipTestEntry{name: name}))
			w := extractRequest(t, a, "/", "bundle")
			if w.Code != 422 {
				t.Fatal(w.Code, w.Body)
			}
			if _, e := os.Stat(filepath.Join(a.Manager.Get(serverID).Root, "safe")); !os.IsNotExist(e) {
				t.Fatal("published early")
			}
			if extractStages(t, a) != 0 {
				t.Fatal("stage remains")
			}
		})
	}
	for _, mode := range []os.FileMode{os.ModeSymlink, os.ModeNamedPipe, os.ModeDevice, os.ModeSocket} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			a, _ := setup(t)
			putDownload(t, a, "bundle", testZIP(t, zipTestEntry{name: "special", mode: mode | 0600}))
			if w := extractRequest(t, a, "/", "bundle"); w.Code != 422 {
				t.Fatal(w.Code, w.Body)
			}
		})
	}
}
func TestExtractZIPConflicts(t *testing.T) {
	for _, names := range [][]string{{"same", "same"}, {"same", "SAME"}, {"A/one", "a/two"}, {"file", "file/child"}, {"dir/", "dir/"}} {
		t.Run(strings.Join(names, "-"), func(t *testing.T) {
			a, _ := setup(t)
			var entries []zipTestEntry
			for _, n := range names {
				entries = append(entries, zipTestEntry{name: n})
			}
			putDownload(t, a, "bundle", testZIP(t, entries...))
			if w := extractRequest(t, a, "/", "bundle"); w.Code != 422 {
				t.Fatal(w.Code, w.Body)
			}
			if extractStages(t, a) != 0 {
				t.Fatal("stage remains")
			}
		})
	}
}
func TestExtractZIPMalformedStructure(t *testing.T) {
	mutations := map[string]func([]byte, int, int) []byte{
		"crc":                func(b []byte, c, e int) []byte { b[30+len("safe")+16] ^= 1; return b },
		"truncated":          func(b []byte, c, e int) []byte { return b[:len(b)-1] },
		"trailing":           func(b []byte, c, e int) []byte { return append(b, 1) },
		"multi-disk":         func(b []byte, c, e int) []byte { b[e+4] = 1; return b },
		"zip64":              func(b []byte, c, e int) []byte { binary.LittleEndian.PutUint16(b[e+10:], 65535); return b },
		"entry-count":        func(b []byte, c, e int) []byte { b[e+8] = 3; b[e+10] = 3; return b },
		"directory-size":     func(b []byte, c, e int) []byte { binary.LittleEndian.PutUint32(b[e+12:], 2<<20); return b },
		"central-offset":     func(b []byte, c, e int) []byte { b[e+16]++; return b },
		"local-name":         func(b []byte, c, e int) []byte { b[30] = 'X'; return b },
		"encrypted":          func(b []byte, c, e int) []byte { b[6] |= 1; b[c+8] |= 1; return b },
		"unsupported-method": func(b []byte, c, e int) []byte { b[8] = 99; b[c+10] = 99; return b },
		"local-method":       func(b []byte, c, e int) []byte { b[8] = 8; return b },
		"overlapping":        func(b []byte, c, e int) []byte { binary.LittleEndian.PutUint32(b[c+20:], 0xffffff); return b },
		"descriptor":         func(b []byte, c, e int) []byte { b[c-1] ^= 1; return b },
		"non-utf8":           func(b []byte, c, e int) []byte { b[30] = 255; b[c+46] = 255; return b },
		"oversize-data": func(b []byte, c, e int) []byte {
			binary.LittleEndian.PutUint32(b[c+24:], uint32(maxArchiveInput+1))
			binary.LittleEndian.PutUint32(b[c-4:], uint32(maxArchiveInput+1))
			return b
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			a, _ := setup(t)
			b := testZIP(t, zipTestEntry{name: "safe", data: bytes.Repeat([]byte("data"), 20)})
			end := len(b) - 22
			central := int(binary.LittleEndian.Uint32(b[end+16:]))
			b = mutate(b, central, end)
			putDownload(t, a, "bundle", b)
			w := extractRequest(t, a, "/", "bundle")
			if w.Code < 400 {
				t.Fatal(w.Code, w.Body)
			}
			if _, e := os.Stat(filepath.Join(a.Manager.Get(serverID).Root, "safe")); !os.IsNotExist(e) {
				t.Fatal("published invalid data")
			}
			if extractStages(t, a) != 0 {
				t.Fatal("stage remains")
			}
		})
	}
}
func TestExtractZIPBounds(t *testing.T) {
	for _, count := range []int{256, 257} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			a, _ := setup(t)
			var entries []zipTestEntry
			for i := 0; i < count; i++ {
				entries = append(entries, zipTestEntry{name: fmt.Sprintf("f%03d", i)})
			}
			putDownload(t, a, "bundle", testZIP(t, entries...))
			want := 204
			if count > 256 {
				want = 413
			}
			if w := extractRequest(t, a, "/", "bundle"); w.Code != want {
				t.Fatal(w.Code, w.Body)
			}
		})
	}
	for _, size := range []int64{maxArchiveInput, maxArchiveInput + 1} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			a, _ := setup(t)
			var b bytes.Buffer
			z := zip.NewWriter(&b)
			w, e := z.CreateHeader(&zip.FileHeader{Name: "large", Method: zip.Deflate})
			if e != nil {
				t.Fatal(e)
			}
			if _, e = io.CopyN(w, zeroUploadReader{}, size); e != nil {
				t.Fatal(e)
			}
			z.Close()
			putDownload(t, a, "bundle", b.Bytes())
			want := 204
			if size > maxArchiveInput {
				want = 413
			}
			if r := extractRequest(t, a, "/", "bundle"); r.Code != want {
				t.Fatal(r.Code, r.Body)
			}
			if extractStages(t, a) != 0 {
				t.Fatal("stage remains")
			}
		})
	}
}
func TestExtractZIPCancellationAndPolicy(t *testing.T) {
	for _, kind := range []string{"cancel", "policy"} {
		t.Run(kind, func(t *testing.T) {
			a, _ := setup(t)
			putDownload(t, a, "bundle", testZIP(t, zipTestEntry{name: "data", data: bytes.Repeat([]byte("x"), 1<<20), method: zip.Deflate}))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			fired := false
			check := func() error {
				items, err := os.ReadDir(a.Manager.Get(serverID).Root)
				if err != nil {
					t.Fatal(err)
				}
				nonempty := false
				for _, item := range items {
					if strings.HasPrefix(item.Name(), ".wings-extract-") {
						info, err := item.Info()
						if err != nil {
							t.Fatal(err)
						}
						nonempty = nonempty || info.Size() >= 64<<10
					}
				}
				if nonempty && !fired {
					fired = true
					if kind == "cancel" {
						cancel()
					}
				}
				if ctx.Err() != nil {
					return &directoryFailure{408, "cancelled"}
				}
				if fired && kind == "policy" {
					return &directoryFailure{403, "changed policy"}
				}
				return nil
			}
			e := extractArchive(ctx, a.Manager.Get(serverID), "", "bundle", check)
			if e == nil || !fired {
				t.Fatal(e, fired)
			}
			if extractStages(t, a) != 0 {
				t.Fatal("stage remains")
			}
		})
	}
}
