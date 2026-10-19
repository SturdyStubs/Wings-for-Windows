package identity

import (
	"fmt"
	"golang.org/x/sys/windows"
	"unsafe"
)

type genericMapping struct{ Read, Write, Execute, All uint32 }

// CheckAccess rejects any forbidden access present in the token's DACL grant.
// This is a snapshot, not a filesystem race or network sandbox.
func CheckAccess(token windows.Token, rules []PathRule) error {
	if len(rules) == 0 {
		return fmt.Errorf("run_as requires protected path checks")
	}
	var impersonation windows.Token
	if err := windows.DuplicateTokenEx(token, windows.TOKEN_QUERY, nil, windows.SecurityImpersonation, windows.TokenImpersonation, &impersonation); err != nil {
		return fmt.Errorf("duplicate token for ACL checks: %w", err)
	}
	defer impersonation.Close()
	for _, rule := range rules {
		sd, err := windows.GetNamedSecurityInfo(rule.Path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.GROUP_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
		if err != nil {
			return fmt.Errorf("read protected path ACL %q: %w", rule.Path, err)
		}
		granted, err := grantedAccess(impersonation, sd)
		if err != nil {
			return fmt.Errorf("check protected path ACL %q: %w", rule.Path, err)
		}
		if granted&rule.Forbidden != 0 {
			return fmt.Errorf("game identity has forbidden access to protected path %q", rule.Path)
		}
	}
	return nil
}

func grantedAccess(token windows.Token, sd *windows.SECURITY_DESCRIPTOR) (uint32, error) {
	mapping := genericMapping{windows.FILE_GENERIC_READ, windows.FILE_GENERIC_WRITE, windows.FILE_GENERIC_EXECUTE, 0x1f01ff}
	// Privilege sets are small here: restricted tokens retain at most traverse.
	privileges := make([]byte, 4096)
	size := uint32(len(privileges))
	var granted, status uint32
	ok, _, checkErr := advapi.NewProc("AccessCheck").Call(uintptr(unsafe.Pointer(sd)), uintptr(token), windows.MAXIMUM_ALLOWED, uintptr(unsafe.Pointer(&mapping)), uintptr(unsafe.Pointer(&privileges[0])), uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&granted)), uintptr(unsafe.Pointer(&status)))
	if ok == 0 {
		return 0, checkErr
	}
	if status == 0 {
		return 0, nil
	}
	return granted, nil
}
