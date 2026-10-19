package identity

import (
	"context"
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"
)

func noACLCheck(string, *windows.SECURITY_DESCRIPTOR, uint32) error { return nil }

func TestAuditRejectsHardlinksAndJunctions(t *testing.T) {
	directory := t.TempDir()
	file := filepath.Join(directory, "original.cfg")
	if err := os.WriteFile(file, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(file, filepath.Join(directory, "alias.cfg")); err != nil {
		t.Fatal(err)
	}
	audit := pathAudit{ctx: context.Background(), limit: 100}
	if err := audit.walk(PathRule{Path: file}, noACLCheck); err == nil || !strings.Contains(err.Error(), "hardlink") {
		t.Fatal("hardlink accepted", err)
	}
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "secret.cfg"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	junction := filepath.Join(directory, "junction")
	if output, err := exec.Command("cmd.exe", "/c", "mklink", "/J", junction, target).CombinedOutput(); err != nil {
		t.Fatalf("junction: %v %s", err, output)
	}
	for _, path := range []string{junction, filepath.Join(junction, "secret.cfg")} {
		if err := audit.walk(PathRule{Path: path, Recursive: true}, noACLCheck); err == nil || !strings.Contains(err.Error(), "reparse") {
			t.Fatal("junction accepted", err)
		}
	}
}

func TestAuditAncestorsBoundsAndCancellation(t *testing.T) {
	directory := t.TempDir()
	file := filepath.Join(directory, "child.cfg")
	if err := os.WriteFile(file, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	seen := map[string]uint32{}
	audit := pathAudit{ctx: context.Background(), limit: 10}
	if err := audit.walk(PathRule{Path: file, Forbidden: ReadWriteAccess}, func(path string, _ *windows.SECURITY_DESCRIPTOR, mask uint32) error { seen[path] = mask; return nil }); err != nil {
		t.Fatal(err)
	}
	parts, err := localComponents(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range parts[:len(parts)-1] {
		if seen[part] != AncestorAccess {
			t.Fatal("ancestor was not audited", part)
		}
	}
	if seen[file] != ReadWriteAccess {
		t.Fatal("leaf was not audited")
	}
	audit = pathAudit{ctx: context.Background(), limit: 1}
	if err := audit.walk(PathRule{Path: directory, Recursive: true}, noACLCheck); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatal("entry bound ignored", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	audit = pathAudit{ctx: ctx, limit: 10}
	if err := audit.walk(PathRule{Path: directory, Recursive: true}, noACLCheck); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation ignored", err)
	}
}

func TestAuditRejectsAmbiguousPaths(t *testing.T) {
	for _, path := range []string{`C:\data\..\secret`, `C:\data\file:stream`, `C:\data\folder.\file`, `C:\data\NUL.cfg`, `C:\data\com1`, `\\?\C:\data`, `\\server\share\file`, `relative\file`} {
		if _, err := localComponents(path); err == nil {
			t.Fatalf("accepted ambiguous path %q", path)
		}
	}
	if _, err := localComponents(`C:\Data\Game.CFG`); err != nil {
		t.Fatal("ordinary mixed case path rejected", err)
	}
}

func TestAuditAllowsExistingWriterAndPinsRenameDuringCheck(t *testing.T) {
	path := filepath.Join(t.TempDir(), "active.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	audit := pathAudit{ctx: context.Background(), limit: 10}
	err = audit.walk(PathRule{Path: path}, func(current string, _ *windows.SECURITY_DESCRIPTOR, _ uint32) error {
		if current == path {
			if err := os.Rename(path, path+".moved"); err == nil {
				t.Fatal("checked file was replaceable while pinned")
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal("existing writer blocked read-only audit", err)
	}
}

func TestRecursiveAuditFindsExposedChildBehindDeniedRoot(t *testing.T) {
	// A restricting SID supplies a second access-check pass without creating or
	// logging on any user. The daemon's original token retains fixture access.
	sid, err := windows.StringToSid("S-1-5-21-1-2-3-1701")
	if err != nil {
		t.Fatal(err)
	}
	var original windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY|windows.TOKEN_DUPLICATE, &original); err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	group := windows.SIDAndAttributes{Sid: sid}
	var restricted windows.Token
	if ok, _, err := restrictToken.Call(uintptr(original), 1, 0, 0, 0, 0, 1, uintptr(unsafe.Pointer(&group)), uintptr(unsafe.Pointer(&restricted))); ok == 0 {
		t.Fatal(err)
	}
	defer restricted.Close()
	root := t.TempDir()
	child := filepath.Join(root, "known-secret.cfg")
	if err := os.WriteFile(child, []byte("not a real secret"), 0600); err != nil {
		t.Fatal(err)
	}
	user, err := original.GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	setDACL := func(path string, extra string) {
		t.Helper()
		sd, err := windows.SecurityDescriptorFromString("D:P(A;;FA;;;" + user.User.Sid.String() + ")" + extra)
		if err != nil {
			t.Fatal(err)
		}
		dacl, _, err := sd.DACL()
		if err != nil {
			t.Fatal(err)
		}
		if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil); err != nil {
			t.Fatal(err)
		}
	}
	setDACL(root, "")
	setDACL(child, "(A;;FR;;;"+sid.String()+")")
	// Root-only checking passes even though a known child is exposed.
	if err := CheckAccess(restricted, []PathRule{{Path: root, Forbidden: ReadWriteAccess}}); err != nil {
		t.Fatal("expected denied root to pass", err)
	}
	err = CheckAccess(restricted, []PathRule{{Path: root, Forbidden: ReadWriteAccess, Recursive: true}})
	if err == nil || !strings.Contains(err.Error(), "known-secret.cfg") {
		t.Fatal("exposed child escaped recursive audit", err)
	}
}
