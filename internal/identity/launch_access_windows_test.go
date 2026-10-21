package identity

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func launchAccessFixture(t *testing.T) (windows.Token, LaunchAccess, func(string, string)) {
	t.Helper()
	var original windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY|windows.TOKEN_DUPLICATE, &original); err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	user, err := original.GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	// A restricting SID tests both AccessCheck passes without creating/logging
	// on a second Windows account. ACL changes apply only to temporary fixtures.
	sid, err := windows.StringToSid("S-1-5-21-1-2-3-1741")
	if err != nil {
		t.Fatal(err)
	}
	group := windows.SIDAndAttributes{Sid: sid}
	var token windows.Token
	if ok, _, err := restrictToken.Call(uintptr(original), 1, 0, 0, 0, 0, 1, uintptr(unsafe.Pointer(&group)), uintptr(unsafe.Pointer(&token))); ok == 0 {
		t.Fatal(err)
	}
	t.Cleanup(func() { token.Close() })
	setAccess := func(path, entries string) {
		t.Helper()
		sd, err := windows.SecurityDescriptorFromString("D:P(A;;FA;;;" + user.User.Sid.String() + ")" + strings.ReplaceAll(entries, "GAME", sid.String()))
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
	base := t.TempDir()
	plan := LaunchAccess{Executable: filepath.Join(base, "game.exe"), Directory: filepath.Join(base, "data"), TempDirectory: filepath.Join(base, "data", ".tmp"), RequiredFiles: []string{filepath.Join(base, "runtime.dll")}}
	if err := os.MkdirAll(plan.TempDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{plan.Executable, plan.RequiredFiles[0]} {
		if err := os.WriteFile(file, []byte("harmless access fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	setAccess(plan.Executable, "(A;;FRFX;;;GAME)")
	setAccess(plan.RequiredFiles[0], "(A;;FR;;;GAME)")
	for _, dir := range []string{plan.Directory, plan.TempDirectory} {
		setAccess(dir, "(A;OICI;FRFWFX;;;GAME)")
	}
	return token, plan, setAccess
}

func TestLaunchAccessUsesRestrictedIdentityGrants(t *testing.T) {
	for _, test := range []struct {
		name, target, entries, want string
	}{
		{"allowed", "", "", ""},
		{"no executable execute", "executable", "(A;;FR;;;GAME)", "executable read/execute"},
		{"explicit execute deny", "executable", "(D;;FX;;;GAME)(A;;FRFX;;;GAME)", "executable read/execute"},
		{"no dependency read", "dependency", "", "declared dependency read"},
		{"no root file creation", "root", "(D;;0x2;;;GAME)(A;;FRFWFX;;;GAME)", "working directory"},
		{"read only temporary directory", "temporary", "(A;;FRFX;;;GAME)", "temporary directory"},
	} {
		t.Run(test.name, func(t *testing.T) {
			token, plan, set := launchAccessFixture(t)
			paths := map[string]string{"executable": plan.Executable, "dependency": plan.RequiredFiles[0], "root": plan.Directory, "temporary": plan.TempDirectory}
			if test.target != "" {
				set(paths[test.target], test.entries)
			}
			err := CheckLaunchAccessContext(context.Background(), token, plan)
			if test.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), test.want) || !strings.Contains(err.Error(), "missing mask") {
				t.Fatal("incorrect launch access result", err)
			}
		})
	}
}

func TestLaunchAccessRejectsMissingWrongTypeAndCancelledInputs(t *testing.T) {
	token, plan, _ := launchAccessFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := CheckLaunchAccessContext(ctx, token, plan); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		edit func(*LaunchAccess)
	}{
		{"missing temporary directory", func(p *LaunchAccess) { p.TempDirectory += ".missing" }},
		{"file used as directory", func(p *LaunchAccess) { p.Directory = p.Executable }},
		{"directory used as executable", func(p *LaunchAccess) { p.Executable = p.Directory }},
		{"missing executable", func(p *LaunchAccess) { p.Executable += ".missing" }},
		{"empty directory", func(p *LaunchAccess) { p.Directory = "" }},
		{"too many dependencies", func(p *LaunchAccess) { p.RequiredFiles = make([]string, 257) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			changed := plan
			test.edit(&changed)
			if err := CheckLaunchAccessContext(nil, token, changed); err == nil {
				t.Fatal("invalid launch layout accepted")
			}
		})
	}
	if _, err := os.Stat(plan.TempDirectory + ".missing"); !os.IsNotExist(err) {
		t.Fatal("read-only check created missing data", err)
	}
}

func TestLaunchAccessMatchesRealRestrictedFileAndTempOperations(t *testing.T) {
	token, plan, set := launchAccessFixture(t)
	if err := CheckLaunchAccessContext(nil, token, plan); err != nil {
		t.Fatal(err)
	}
	readErr, createErr := restrictedLaunchOperations(t, token, plan, "allowed.tmp")
	if readErr != nil || createErr != nil {
		t.Fatalf("permitted operations failed: image=%v temporary=%v", readErr, createErr)
	}
	set(plan.TempDirectory, "(A;;FRFX;;;GAME)")
	if err := CheckLaunchAccessContext(nil, token, plan); err == nil || !strings.Contains(err.Error(), "temporary directory") {
		t.Fatal("read-only temporary directory accepted", err)
	}
	readErr, createErr = restrictedLaunchOperations(t, token, plan, "denied.tmp")
	if readErr != nil || !errors.Is(createErr, windows.ERROR_ACCESS_DENIED) {
		t.Fatalf("real access disagreed with denied check: image=%v temporary=%v", readErr, createErr)
	}
}

func restrictedLaunchOperations(t *testing.T, token windows.Token, plan LaunchAccess, childName string) (error, error) {
	t.Helper()
	var impersonation windows.Token
	if err := windows.DuplicateTokenEx(token, windows.TOKEN_QUERY|windows.TOKEN_IMPERSONATE, nil, windows.SecurityImpersonation, windows.TokenImpersonation, &impersonation); err != nil {
		t.Fatal(err)
	}
	defer impersonation.Close()
	// Always revert before assertions, and keep token-bearing work on one OS
	// thread. Only harmless fixture operations run under impersonation.
	runtime.LockOSThread()
	if err := windows.SetThreadToken(nil, impersonation); err != nil {
		runtime.UnlockOSThread()
		t.Fatal(err)
	}
	file, _ := windows.UTF16PtrFromString(plan.Executable)
	h, readErr := windows.CreateFile(file, windows.GENERIC_READ|windows.GENERIC_EXECUTE, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
	if readErr == nil {
		windows.CloseHandle(h)
	}
	child, _ := windows.UTF16PtrFromString(filepath.Join(plan.TempDirectory, childName))
	h, createErr := windows.CreateFile(child, windows.GENERIC_WRITE, windows.FILE_SHARE_READ, nil, windows.CREATE_NEW, 0, 0)
	if createErr == nil {
		windows.CloseHandle(h)
	}
	revertErr := windows.RevertToSelf()
	if revertErr != nil {
		// An impersonating thread must never return to Go's thread pool.
		t.Fatal("failed to revert fixture impersonation", revertErr)
	}
	runtime.UnlockOSThread()
	return readErr, createErr
}
