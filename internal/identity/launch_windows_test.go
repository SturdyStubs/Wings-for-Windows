package identity

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func launchFixture(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "game.exe")
	if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	return dir, path
}

func TestLaunchGuardPinsFilesAndAncestorsUntilClose(t *testing.T) {
	dir, path := launchFixture(t)
	guard, err := PinLaunchPaths(context.Background(), []string{path, dir, path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { guard.Close() })
	audit := pathAudit{ctx: context.Background(), limit: 10}
	if err := audit.walk(PathRule{Path: path}, noACLCheck); err != nil {
		t.Fatal("snapshot audit failed under startup guard", err)
	}
	// Returning from the audit must not reopen the replacement window.
	if err := os.WriteFile(path, []byte("replacement"), 0600); !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
		t.Fatal("launch file remained writable", err)
	}
	for _, pinned := range []string{path, dir} {
		if err := os.Rename(pinned, pinned+".moved"); !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			t.Fatal("launch component remained replaceable", pinned, err)
		}
	}
	// Existing read handles and unrelated child-file activity are compatible.
	if _, err := os.ReadFile(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "game.log"), []byte("output"), 0600); err != nil {
		t.Fatal("guard blocked ordinary child file creation", err)
	}
	name, _ := windows.UTF16PtrFromString(dir)
	h, err := windows.CreateFile(name, windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err == nil {
		windows.CloseHandle(h)
		t.Fatal("pinned ancestor accepted a directory writer")
	}
	if !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
		t.Fatal(err)
	}
	if err := guard.Close(); err != nil {
		t.Fatal(err)
	}
	if err := guard.Close(); err != nil {
		t.Fatal("close must be idempotent", err)
	}
	if err := os.WriteFile(path, []byte("updated after startup"), 0600); err != nil {
		t.Fatal("guard leaked file handle", err)
	}
	if err := os.Rename(dir, dir+".moved"); err != nil {
		t.Fatal("guard leaked ancestor handle", err)
	}
	if err := os.Rename(dir+".moved", dir); err != nil {
		t.Fatal(err)
	}
}

func TestLaunchGuardRejectsExistingWriterAndRollsBack(t *testing.T) {
	_, path := launchFixture(t)
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	guard, err := PinLaunchPaths(nil, []string{path})
	f.Close()
	if guard != nil || !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
		t.Fatal("active writer accepted", err)
	}
	// Failure after acquiring a complete first path must release everything.
	guard, err = PinLaunchPaths(nil, []string{path, path + ".missing"})
	if guard != nil || err == nil {
		t.Fatal("missing path accepted", err)
	}
	if err := os.Rename(path, path+".moved"); err != nil {
		t.Fatal("failed acquisition leaked handle", err)
	}
}

func TestLaunchGuardRejectsLinksAndAmbiguousPaths(t *testing.T) {
	dir, path := launchFixture(t)
	if err := os.Link(path, path+".alias"); err != nil {
		t.Fatal(err)
	}
	if guard, err := PinLaunchPaths(nil, []string{path}); guard != nil || err == nil || !strings.Contains(err.Error(), "hardlink") {
		t.Fatal("hardlink accepted", err)
	}
	junction := filepath.Join(t.TempDir(), "junction")
	if output, err := exec.Command("cmd.exe", "/c", "mklink", "/J", junction, dir).CombinedOutput(); err != nil {
		t.Fatalf("junction: %v %s", err, output)
	}
	for _, path := range []string{junction, filepath.Join(junction, "game.exe"), `relative\game.exe`, `C:\game.exe:stream`} {
		if guard, err := PinLaunchPaths(nil, []string{path}); guard != nil || err == nil {
			t.Fatal("unsupported path accepted", path, err)
		}
	}
}

func TestLaunchGuardCancellationAndHandleBound(t *testing.T) {
	dir, path := launchFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if guard, err := PinLaunchPaths(ctx, []string{path}); guard != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled acquisition accepted", err)
	}
	if guard, err := PinLaunchPaths(nil, nil); guard != nil || err == nil {
		t.Fatal("empty guard accepted", err)
	}
	paths := []string{path}
	for i := 0; i < maxLaunchHandles; i++ {
		file := filepath.Join(dir, fmt.Sprintf("dependency-%04d.dll", i))
		if err := os.WriteFile(file, nil, 0600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, file)
	}
	if guard, err := PinLaunchPaths(nil, paths); guard != nil || err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatal("handle bound ignored", err)
	}
	if err := os.WriteFile(path, []byte("after bounded rollback"), 0600); err != nil {
		t.Fatal("bounded rollback leaked a handle", err)
	}
}
