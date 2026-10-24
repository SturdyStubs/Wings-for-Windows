package diskusage

import (
	"context"
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"testing"
)

func TestDiskScanBinaryEmptyHiddenAndRefresh(t *testing.T) {
	root := t.TempDir()
	if e := os.Mkdir(filepath.Join(root, "nested"), 0700); e != nil {
		t.Fatal(e)
	}
	for name, data := range map[string][]byte{"binary": {0, 255, 1}, "nested/empty": {}, ".denied-hidden": {1, 2}} {
		if e := os.WriteFile(filepath.Join(root, name), data, 0600); e != nil {
			t.Fatal(e)
		}
	}
	r, e := Scan(context.Background(), root)
	if e != nil || r != (Result{Bytes: 5, Files: 3, Directories: 1}) {
		t.Fatal(r, e)
	}
	if e := os.WriteFile(filepath.Join(root, "binary"), make([]byte, 100), 0600); e != nil {
		t.Fatal(e)
	}
	r, e = Scan(context.Background(), root)
	if e != nil || r.Bytes != 102 {
		t.Fatal(r, e)
	}
	if e := os.Remove(filepath.Join(root, "binary")); e != nil {
		t.Fatal("handle leaked", e)
	}
}

func TestDiskScanBounds(t *testing.T) {
	for _, kind := range []string{"exact-entry", "entry-overflow", "exact-depth", "depth-overflow", "cancel", "missing", "relative"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			if e := os.Mkdir(filepath.Join(root, "child"), 0700); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(filepath.Join(root, "child/file"), []byte("value"), 0600); e != nil {
				t.Fatal(e)
			}
			entries, depth := 2, 1
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			want := false
			switch kind {
			case "entry-overflow":
				entries = 1
				want = true
			case "depth-overflow":
				depth = 0
				want = true
			case "cancel":
				cancel()
				want = true
			case "missing":
				root = filepath.Join(root, "missing")
				want = true
			case "relative":
				root = "relative"
				want = true
			}
			r, e := scan(ctx, root, entries, depth)
			if (e != nil) != want {
				t.Fatal(r, e)
			}
			if want && r != (Result{}) {
				t.Fatal("partial result", r)
			}
		})
	}
}

func TestDiskScanHardlinkAndJunctionRefused(t *testing.T) {
	for _, kind := range []string{"hardlink", "junction", "root-junction", "ancestor-junction"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			outside := t.TempDir()
			if e := os.WriteFile(filepath.Join(outside, "sentinel"), []byte("outside"), 0600); e != nil {
				t.Fatal(e)
			}
			if kind == "hardlink" {
				if e := os.Link(filepath.Join(outside, "sentinel"), filepath.Join(root, "linked")); e != nil {
					t.Fatal(e)
				}
			} else {
				link := filepath.Join(root, "junction")
				if e := createTestLink(outside, link); e != nil {
					t.Fatal("junction fixture failed", e)
				}
				if kind == "root-junction" {
					root = link
				}
				if kind == "ancestor-junction" {
					if e := os.Mkdir(filepath.Join(outside, "child"), 0700); e != nil {
						t.Fatal(e)
					}
					root = filepath.Join(link, "child")
				}
			}
			r, e := Scan(context.Background(), root)
			if e == nil || r != (Result{}) {
				t.Fatal(r, e)
			}
			b, e := os.ReadFile(filepath.Join(outside, "sentinel"))
			if e != nil || string(b) != "outside" {
				t.Fatal(e)
			}
		})
	}
}

func TestDiskScanSparseLogicalSizeAndWriter(t *testing.T) {
	root := t.TempDir()
	name := filepath.Join(root, "sparse")
	f, e := os.Create(name)
	if e != nil {
		t.Fatal(e)
	}
	var n uint32
	if e = windows.DeviceIoControl(windows.Handle(f.Fd()), windows.FSCTL_SET_SPARSE, nil, 0, nil, 0, &n, nil); e != nil {
		f.Close()
		t.Fatal(e)
	}
	if e = f.Truncate(1 << 33); e != nil {
		f.Close()
		t.Fatal(e)
	}
	r, e := Scan(context.Background(), root)
	if e != nil || r.Bytes != 1<<33 {
		f.Close()
		t.Fatal(r, e)
	}
	f.Close()
	// Attribute-only opens do not conflict with ordinary data sharing locks.
	p, _ := windows.UTF16PtrFromString(name)
	h, e := windows.CreateFile(p, windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING, 0, 0)
	if e != nil {
		t.Fatal(e)
	}
	r, e = Scan(context.Background(), root)
	windows.CloseHandle(h)
	// FILE_READ_ATTRIBUTES is compatible even with exclusive data sharing on Windows.
	if e != nil || r.Bytes != 1<<33 {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = Scan(ctx, root)
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestDiskDirectoryPinAllowsWritesAndReleases(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if e := os.Mkdir(child, 0700); e != nil {
		t.Fatal(e)
	}
	parent, e := os.Open(root)
	if e != nil {
		t.Fatal(e)
	}
	defer parent.Close()
	f, e := openChild(parent, "child", true)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Rename(child, filepath.Join(root, "moved")); e == nil {
		f.Close()
		t.Fatal("active directory was movable")
	}
	if e = os.WriteFile(filepath.Join(child, "data"), []byte("ordinary writer"), 0600); e != nil {
		f.Close()
		t.Fatal(e)
	}
	f.Close()
	if e = os.Rename(child, filepath.Join(root, "moved")); e != nil {
		t.Fatal("directory handle leaked", e)
	}
}
