package identity

import (
	"errors"
	"golang.org/x/sys/windows"
	"runtime"
	"strings"
	"testing"
	"unsafe"
)

func restrictingToken(t *testing.T) windows.Token {
	t.Helper()
	var original windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY|windows.TOKEN_DUPLICATE, &original); err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	sid, err := windows.StringToSid("S-1-5-21-1-2-3-1801")
	if err != nil {
		t.Fatal(err)
	}
	group := windows.SIDAndAttributes{Sid: sid}
	var token windows.Token
	if ok, _, err := restrictToken.Call(uintptr(original), 1, 0, 0, 0, 0, 1, uintptr(unsafe.Pointer(&group)), uintptr(unsafe.Pointer(&token))); ok == 0 {
		t.Fatal(err)
	}
	t.Cleanup(func() { token.Close() })
	return token
}

func TestDaemonAuditAndActualProcessAccessDenial(t *testing.T) {
	var original windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY|windows.TOKEN_DUPLICATE, &original); err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	if err := CheckDaemonAccess(original); err == nil || !strings.Contains(err.Error(), "daemon process") {
		t.Fatal("daemon's own access was not detected", err)
	}
	game := restrictingToken(t)
	if err := CheckDaemonAccess(game); err != nil {
		t.Fatal("restricted token should not access daemon objects", err)
	}
	var impersonation windows.Token
	if err := windows.DuplicateTokenEx(game, windows.TOKEN_QUERY|windows.TOKEN_IMPERSONATE, nil, windows.SecurityImpersonation, windows.TokenImpersonation, &impersonation); err != nil {
		t.Fatal(err)
	}
	defer impersonation.Close()
	// Keep impersonation confined to one OS thread and revert before assertions.
	runtime.LockOSThread()
	if err := windows.SetThreadToken(nil, impersonation); err != nil {
		runtime.UnlockOSThread()
		t.Fatal(err)
	}
	process, processErr := windows.OpenProcess(windows.PROCESS_VM_READ, false, windows.GetCurrentProcessId())
	var copied windows.Token
	tokenErr := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_DUPLICATE, &copied)
	revertErr := windows.RevertToSelf()
	if revertErr != nil {
		t.Fatal("could not revert test thread", revertErr)
	}
	runtime.UnlockOSThread()
	if process != 0 {
		windows.CloseHandle(process)
	}
	if copied != 0 {
		copied.Close()
	}
	if !errors.Is(processErr, windows.ERROR_ACCESS_DENIED) {
		t.Fatal("actual daemon memory access was not denied", processErr)
	}
	if !errors.Is(tokenErr, windows.ERROR_ACCESS_DENIED) {
		t.Fatal("actual daemon token duplication was not denied", tokenErr)
	}
}

func TestKernelObjectRightsAndConservativeGenericMapping(t *testing.T) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	var original, token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY|windows.TOKEN_DUPLICATE, &original); err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	if err := windows.DuplicateTokenEx(original, windows.TOKEN_QUERY, nil, windows.SecurityImpersonation, windows.TokenImpersonation, &token); err != nil {
		t.Fatal(err)
	}
	defer token.Close()
	for _, test := range []struct {
		name, rights   string
		all, forbidden uint32
		reject         bool
	}{
		{"process memory read", "0x10", 0x1fffff, processDangerous, true},
		{"process memory write", "0x20", 0x1fffff, processDangerous, true},
		{"process query limited", "0x1000", 0x1fffff, processDangerous, false},
		{"process generic read conservative", "GR", 0x1fffff, processDangerous, true},
		{"token duplication", "0x2", windows.TOKEN_ALL_ACCESS, tokenDangerous, true},
		{"token query only", "0x8", windows.TOKEN_ALL_ACCESS, tokenDangerous, false},
		{"token generic execute conservative", "GX", windows.TOKEN_ALL_ACCESS, tokenDangerous, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			sd, err := windows.SecurityDescriptorFromString("O:SYG:SYD:P(A;;" + test.rights + ";;;" + user.User.Sid.String() + ")")
			if err != nil {
				t.Fatal(err)
			}
			err = rejectKernelAccess(token, sd, test.all, test.forbidden, "fixture")
			if (err != nil) != test.reject {
				t.Fatalf("unexpected kernel grant: %v", err)
			}
		})
	}
}
