package identity

import (
	"fmt"
	"golang.org/x/sys/windows"
)

const processDangerous = windows.PROCESS_VM_READ | windows.PROCESS_VM_WRITE | windows.PROCESS_VM_OPERATION | windows.PROCESS_CREATE_THREAD | windows.PROCESS_CREATE_PROCESS | windows.PROCESS_DUP_HANDLE | windows.PROCESS_TERMINATE | windows.PROCESS_SET_INFORMATION | windows.PROCESS_SET_QUOTA | windows.PROCESS_SUSPEND_RESUME | windows.WRITE_DAC | windows.WRITE_OWNER | windows.DELETE
const tokenDangerous = windows.TOKEN_ASSIGN_PRIMARY | windows.TOKEN_DUPLICATE | windows.TOKEN_IMPERSONATE | windows.TOKEN_ADJUST_PRIVILEGES | windows.TOKEN_ADJUST_GROUPS | windows.TOKEN_ADJUST_DEFAULT | windows.TOKEN_ADJUST_SESSIONID | windows.WRITE_DAC | windows.WRITE_OWNER | windows.DELETE

// CheckDaemonAccess rejects dangerous DACL grants to the current process and
// its primary token. It does not change either security descriptor.
func CheckDaemonAccess(game windows.Token) error {
	var impersonation windows.Token
	if err := windows.DuplicateTokenEx(game, windows.TOKEN_QUERY, nil, windows.SecurityImpersonation, windows.TokenImpersonation, &impersonation); err != nil {
		return fmt.Errorf("duplicate game token for daemon audit: %w", err)
	}
	defer impersonation.Close()
	sd, err := windows.GetSecurityInfo(windows.CurrentProcess(), windows.SE_KERNEL_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.GROUP_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return fmt.Errorf("read daemon process security: %w", err)
	}
	if err := rejectKernelAccess(impersonation, sd, 0x1fffff, processDangerous, "daemon process"); err != nil {
		return err
	}
	var daemon windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.READ_CONTROL|windows.TOKEN_QUERY, &daemon); err != nil {
		return fmt.Errorf("open daemon primary token for audit: %w", err)
	}
	defer daemon.Close()
	sd, err = windows.GetSecurityInfo(windows.Handle(daemon), windows.SE_KERNEL_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.GROUP_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return fmt.Errorf("read daemon token security: %w", err)
	}
	return rejectKernelAccess(impersonation, sd, windows.TOKEN_ALL_ACCESS, tokenDangerous, "daemon primary token")
}

func rejectKernelAccess(game windows.Token, sd *windows.SECURITY_DESCRIPTOR, all, forbidden uint32, label string) error {
	granted, err := grantedAccessWithMapping(game, sd, genericMapping{all, all, all, all})
	if err != nil {
		return fmt.Errorf("check %s access: %w", label, err)
	}
	if granted&forbidden != 0 {
		return fmt.Errorf("game identity has forbidden access to %s", label)
	}
	return nil
}
