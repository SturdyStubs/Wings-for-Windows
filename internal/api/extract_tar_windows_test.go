package api

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractPlainTarRoundTrip(t *testing.T) {
	a, _ := setup(t)
	data := bytes.Repeat([]byte{0, 255, 17}, 3<<20)
	putDownload(t, a, "target/archive.unrecognized", testTar(t, tarTestEntry{name: "nested/Caf\u00e9.bin", kind: tar.TypeReg, data: data}, tarTestEntry{name: "empty/", kind: tar.TypeDir}, tarTestEntry{name: "zero", kind: tar.TypeReg}))
	if w := extractRequest(t, a, "/target", "archive.unrecognized"); w.Code != 204 {
		t.Fatal(w.Code, w.Body)
	}
	got, e := os.ReadFile(filepath.Join(a.Manager.Get(serverID).Root, "target/nested/Caf\u00e9.bin"))
	if e != nil || !bytes.Equal(got, data) {
		t.Fatal(e, "integrity")
	}
	if w := extractRequest(t, a, "/target", "archive.unrecognized"); w.Code != 409 {
		t.Fatal(w.Code, w.Body)
	}
	if extractStages(t, a) != 0 {
		t.Fatal("stage remains")
	}
}

func TestExtractPlainTarMalformedOrUnsafe(t *testing.T) {
	for _, kind := range []string{"header-checksum", "truncated-body", "missing-footer", "nonzero-trailer", "concatenated", "empty", "traversal", "link", "case-alias", "file-conflict"} {
		t.Run(kind, func(t *testing.T) {
			a, _ := setup(t)
			entries := []tarTestEntry{{name: "safe", kind: tar.TypeReg, data: []byte("staged")}}
			switch kind {
			case "traversal":
				entries = append(entries, tarTestEntry{name: "../outside", kind: tar.TypeReg})
			case "link":
				entries = append(entries, tarTestEntry{name: "linked", kind: tar.TypeSymlink, link: "safe"})
			case "case-alias":
				entries = append(entries, tarTestEntry{name: "SAFE", kind: tar.TypeReg})
			case "file-conflict":
				entries = append(entries, tarTestEntry{name: "safe/child", kind: tar.TypeReg})
			}
			b := testTar(t, entries...)
			switch kind {
			case "header-checksum":
				b[0] ^= 1
			case "truncated-body":
				b = b[:514]
			case "missing-footer":
				b = b[:len(b)-1024]
			case "nonzero-trailer":
				b = append(b, 1)
			case "concatenated":
				b = append(b, testTar(t, tarTestEntry{name: "second", kind: tar.TypeReg})...)
			case "empty":
				b = testTar(t)
			}
			putDownload(t, a, "bundle", b)
			w := extractRequest(t, a, "/", "bundle")
			if w.Code < 400 {
				t.Fatal(w.Code, w.Body)
			}
			if _, e := os.Stat(filepath.Join(a.Manager.Get(serverID).Root, "safe")); !os.IsNotExist(e) {
				t.Fatal("published before validation", e)
			}
			if extractStages(t, a) != 0 {
				t.Fatal("stage remains")
			}
		})
	}
}

func TestExtractPlainTarSizeAndEntryBounds(t *testing.T) {
	for _, kind := range []string{"exact-size", "data-overflow", "entries", "implicit"} {
		t.Run(kind, func(t *testing.T) {
			a, _ := setup(t)
			var b bytes.Buffer
			tw := tar.NewWriter(&b)
			if kind == "exact-size" || kind == "data-overflow" {
				size := maxArchiveInput
				if kind == "data-overflow" {
					size++
				}
				if e := tw.WriteHeader(&tar.Header{Name: "large", Size: size, Mode: 0600}); e != nil {
					t.Fatal(e)
				}
				if _, e := io.CopyN(tw, zeroUploadReader{}, size); e != nil {
					t.Fatal(e)
				}
			} else {
				for i := 0; i < 257; i++ {
					name := fmt.Sprintf("f%03d", i)
					if kind == "implicit" {
						name = "parent/" + name
					}
					if e := tw.WriteHeader(&tar.Header{Name: name, Mode: 0600}); e != nil {
						t.Fatal(e)
					}
				}
			}
			if e := tw.Close(); e != nil {
				t.Fatal(e)
			}
			putDownload(t, a, "bundle", b.Bytes())
			want := 413
			if kind == "exact-size" {
				want = 204
			}
			if w := extractRequest(t, a, "/", "bundle"); w.Code != want {
				t.Fatal(w.Code, w.Body)
			}
			if extractStages(t, a) != 0 {
				t.Fatal("stage remains")
			}
		})
	}
}

func TestExtractPlainTarNonemptyCancellation(t *testing.T) {
	a, _ := setup(t)
	putDownload(t, a, "bundle", testTar(t, tarTestEntry{name: "data", kind: tar.TypeReg, data: bytes.Repeat([]byte("x"), 1<<20)}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fired := false
	check := func() error {
		entries, e := os.ReadDir(a.Manager.Get(serverID).Root)
		if e != nil {
			t.Fatal(e)
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".wings-extract-") {
				info, e := entry.Info()
				if e != nil {
					t.Fatal(e)
				}
				if info.Size() >= 64<<10 {
					fired = true
					cancel()
				}
			}
		}
		if ctx.Err() != nil {
			return &directoryFailure{408, "cancelled"}
		}
		return nil
	}
	if e := extractArchive(ctx, a.Manager.Get(serverID), "", "bundle", check); e == nil || !fired {
		t.Fatal(e, fired)
	}
	if extractStages(t, a) != 0 {
		t.Fatal("stage remains")
	}
}
