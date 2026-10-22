package logtail

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func appendLog(t *testing.T, path, text string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString(text)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
}
func openReader(t *testing.T, root, name string) *Reader {
	t.Helper()
	r, err := Open(context.Background(), root, name, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	return r
}
func poll(t *testing.T, r *Reader) []string {
	t.Helper()
	lines, err := r.Poll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return lines
}
func TestStaleHistoryAndPartialLineAreSkipped(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "server.log")
	appendLog(t, path, "READY\nold partial ")
	r := openReader(t, root, "server.log")
	appendLog(t, path, "READY\nfresh\n")
	if lines := poll(t, r); !slices.Equal(lines, []string{"fresh"}) {
		t.Fatal(lines)
	}
	if lines := poll(t, r); len(lines) != 0 {
		t.Fatal("history repeated", lines)
	}
}
func TestMissingFileAndSplitUTF8CRLF(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "server.log")
	r := openReader(t, root, "server.log")
	if len(poll(t, r)) != 0 {
		t.Fatal("missing file emitted")
	}
	appendLog(t, path, "\xef\xbb\xbfReady \xe9")
	if len(poll(t, r)) != 0 {
		t.Fatal("partial line emitted")
	}
	appendLog(t, path, "\x9b\xaa\r\nnext\n")
	if lines := poll(t, r); !slices.Equal(lines, []string{"Ready 雪", "next"}) {
		t.Fatal(lines)
	}
}
func TestRotationTruncationAndPartialReset(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "server.log")
	r := openReader(t, root, "server.log")
	appendLog(t, path, "a long previous line\npartial")
	if lines := poll(t, r); !slices.Equal(lines, []string{"a long previous line"}) {
		t.Fatal(lines)
	}
	if err := os.Rename(path, path+".old"); err != nil {
		t.Fatal(err)
	}
	if len(poll(t, r)) != 0 {
		t.Fatal("missing rotated path emitted")
	}
	appendLog(t, path, "replacement has a long line\n")
	if lines := poll(t, r); !slices.Equal(lines, []string{"replacement has a long line"}) {
		t.Fatal(lines)
	}
	if err := os.WriteFile(path, []byte("new\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if lines := poll(t, r); !slices.Equal(lines, []string{"new"}) {
		t.Fatal(lines)
	}
}
func TestReadAndLineBudgetsPreserveBacklog(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "server.log")
	r := openReader(t, root, "server.log")
	appendLog(t, path, strings.Repeat("x\n", MaxLines+30))
	first := poll(t, r)
	second := poll(t, r)
	if len(first) != MaxLines || len(second) != 30 {
		t.Fatal(len(first), len(second))
	}
	appendLog(t, path, strings.Repeat(strings.Repeat("a", MaxLine)+"\n", 8))
	total := 0
	for i := 0; i < 5; i++ {
		lines := poll(t, r)
		for _, line := range lines {
			if len(line) != MaxLine {
				t.Fatal("line split")
			}
		}
		total += len(lines)
	}
	if total != 8 {
		t.Fatal("read budget lost lines", total)
	}
}
func TestUnsafePathsAndProtectedFiles(t *testing.T) {
	for _, mode := range []string{"parent-missing", "directory", "hardlink", "protected", "traversal", "junction-parent"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			name := "server.log"
			path := filepath.Join(root, name)
			var protected []string
			switch mode {
			case "parent-missing":
				name = "missing/server.log"
			case "directory":
				os.Mkdir(path, 0700)
			case "hardlink":
				os.WriteFile(path, []byte("secret"), 0600)
				if err := os.Link(path, path+".other"); err != nil {
					t.Fatal(err)
				}
			case "protected":
				os.WriteFile(path, []byte("secret"), 0600)
				protected = []string{path}
			case "traversal":
				name = "../secret.log"
			case "junction-parent":
				outside := t.TempDir()
				parent := filepath.Join(root, "linked")
				if b, err := exec.Command("cmd", "/c", "mklink", "/J", parent, outside).CombinedOutput(); err != nil {
					t.Fatal(err, string(b))
				}
				name = "linked/server.log"
			}
			r, err := Open(context.Background(), root, name, protected)
			if r != nil {
				r.Close()
			}
			if err == nil {
				t.Fatal("unsafe path accepted")
			}
		})
	}
}
func TestRuntimeTextAndLinkFailuresDiscardPoll(t *testing.T) {
	for _, mode := range []string{"long-line", "invalid-utf8", "nul", "new-hardlink"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "server.log")
			r := openReader(t, root, "server.log")
			data := "valid\n"
			switch mode {
			case "long-line":
				data += strings.Repeat("x", MaxLine+5)
			case "invalid-utf8":
				data += "\xff\n"
			case "nul":
				data += "a\x00b\n"
			case "new-hardlink":
				data += "secret\n"
			}
			appendLog(t, path, data)
			if mode == "new-hardlink" {
				if err := os.Link(path, path+".other"); err != nil {
					t.Fatal(err)
				}
			}
			lines, err := r.Poll(context.Background())
			if err == nil || len(lines) != 0 {
				t.Fatal("unsafe data returned", lines, err)
			}
		})
	}
}
func TestDirectoryPinCancellationAndClose(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	os.Mkdir(root, 0700)
	r := openReader(t, root, "server.log")
	if err := os.Rename(root, root+"-moved"); err == nil {
		t.Fatal("pinned parent replaced")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Poll(ctx); err == nil {
		t.Fatal("canceled poll succeeded")
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Poll(context.Background()); err == nil {
		t.Fatal("closed reader worked")
	}
	if err := os.Rename(root, root+"-moved"); err != nil {
		t.Fatal("pin leaked", err)
	}
}
