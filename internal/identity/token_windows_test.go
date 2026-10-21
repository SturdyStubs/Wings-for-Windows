package identity

import (
	"context"
	"crypto/rand"
	"fmt"
	"golang.org/x/sys/windows"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"
	"wings-windows/internal/testutil"
)

func TestTokenPinAndDaemonAccountRejection(t *testing.T) {
	token := windows.GetCurrentProcessToken()
	user, err := token.GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	if err := validateToken(token, "S-1-5-21-1-2-3-1001"); err == nil || !strings.Contains(err.Error(), "pinned") {
		t.Fatal("SID mismatch accepted", err)
	}
	if err := validateToken(token, user.User.Sid.String()); err == nil || !strings.Contains(err.Error(), "differ") {
		t.Fatal("daemon account accepted", err)
	}
}

func TestCredentialHelperRoundTripWithoutAccountLogon(t *testing.T) {
	var guid [16]byte
	rand.Read(guid[:])
	id := fmt.Sprintf("%x-%x-%x-%x-%x", guid[:4], guid[4:6], guid[6:8], guid[8:10], guid[10:])
	target := "WingsWindows/" + id
	name, _ := windows.UTF16PtrFromString(target)
	t.Cleanup(func() {
		if ok, _, err := advapi.NewProc("CredDeleteW").Call(uintptr(unsafe.Pointer(name)), 1, 0); ok == 0 && err != windows.ERROR_NOT_FOUND {
			t.Errorf("temporary test credential cleanup failed: %v", err)
		}
	})
	path, err := filepath.Abs("../../scripts/Store-GameCredential.ps1")
	if err != nil {
		t.Fatal(err)
	}
	// A unique temporary generic credential, never a real account/password.
	command := "& '" + strings.ReplaceAll(path, "'", "''") + "' -ServerUUID '" + id + "' -Credential ([PSCredential]::new('.\u005cWingsTestNeverLogOn', (ConvertTo-SecureString 'TEST-only-password-123' -AsPlainText -Force)))"
	if output, err := testutil.PowerShell(context.Background(), "-NoProfile", "-NonInteractive", "-Command", command).CombinedOutput(); err != nil {
		t.Fatalf("credential helper: %v %s", err, output)
	}
	user, password, err := readCredential(target)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(password)
	if windows.UTF16PtrToString(user) != "WingsTestNeverLogOn" || windows.UTF16ToString(password) != "TEST-only-password-123" {
		t.Fatal("credential UTF-16 round trip failed")
	}
	// No LogonUser call, account creation or machine policy changes occur.
}

func TestMissingCredentialFailsClosed(t *testing.T) {
	var guid [16]byte
	rand.Read(guid[:])
	ref := Reference{SID: "S-1-5-21-1-2-3-1001", CredentialTarget: fmt.Sprintf("WingsWindows/%x-%x-%x-%x-%x", guid[:4], guid[4:6], guid[6:8], guid[8:10], guid[10:])}
	token, err := Open(ref)
	if err == nil || token != 0 {
		if token != 0 {
			token.Close()
		}
		t.Fatal("missing credential did not fail closed")
	}
	if !strings.Contains(err.Error(), "read game credential") {
		t.Fatal(err)
	}
}

func TestRestrictedTokenRemovesPrivileges(t *testing.T) {
	var original windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_DUPLICATE|windows.TOKEN_QUERY|windows.TOKEN_ASSIGN_PRIMARY, &original); err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	token, err := Restrict(original)
	if err != nil {
		t.Fatal(err)
	}
	defer token.Close()
	if err := VerifyRestricted(token); err != nil {
		t.Fatal(err)
	}
}

// VerifyRestricted is test-only and queries actual Windows token privileges.
func VerifyRestricted(token windows.Token) error {
	var length uint32
	windows.GetTokenInformation(token, windows.TokenPrivileges, nil, 0, &length)
	if length == 0 {
		return fmt.Errorf("missing privilege information")
	}
	buffer := make([]byte, length)
	if err := windows.GetTokenInformation(token, windows.TokenPrivileges, &buffer[0], length, &length); err != nil {
		return err
	}
	privileges := (*windows.Tokenprivileges)(unsafe.Pointer(&buffer[0]))
	name, _ := windows.UTF16PtrFromString("SeChangeNotifyPrivilege")
	var allowed windows.LUID
	if err := windows.LookupPrivilegeValue(nil, name, &allowed); err != nil {
		return err
	}
	for _, p := range privileges.AllPrivileges() {
		if p.Luid != allowed {
			return fmt.Errorf("unexpected retained privilege: %+v", p.Luid)
		}
	}
	return nil
}
