package identity

import (
	"context"
	"fmt"
	"golang.org/x/sys/windows"
	"strings"
	"unsafe"
)

// AuditIdentity attempts one batch logon and the launch-time access checks,
// without creating a process, contacting the panel or changing host policy.
func AuditIdentity(ctx context.Context, ref Reference, rules []PathRule) []DiagnosticCheck {
	if ctx == nil {
		ctx = context.Background()
	}
	checks := PendingAuditChecks()
	var token windows.Token
	defer func() {
		if token != 0 {
			token.Close()
		}
	}()
	stages := []func() error{
		ref.Validate,
		func() error {
			metadata, err := CredentialStatus(strings.TrimPrefix(ref.CredentialTarget, "WingsWindows/"))
			if err != nil {
				return err
			}
			if !metadata.Present {
				return fmt.Errorf("game credential is missing from the current Windows account")
			}
			return nil
		},
		func() error { var err error; token, err = Open(ref); return err },
		func() error { return CheckDaemonAccess(token) },
		func() error { return CheckAccessContext(ctx, token, rules) },
		checkServicePrivileges,
	}
	for i, stage := range stages {
		err := ctx.Err()
		if err == nil {
			err = stage()
		}
		if err == nil {
			err = ctx.Err()
		}
		if err != nil {
			checks[i].Status = "failed"
			checks[i].Error = err.Error()
			break
		}
		checks[i].Status = "passed"
	}
	return checks
}

func checkServicePrivileges() error {
	var size uint32
	token := windows.GetCurrentProcessToken()
	windows.GetTokenInformation(token, windows.TokenPrivileges, nil, 0, &size)
	if size == 0 {
		return fmt.Errorf("cannot query daemon token privileges")
	}
	buffer := make([]byte, size)
	if err := windows.GetTokenInformation(token, windows.TokenPrivileges, &buffer[0], size, &size); err != nil {
		return fmt.Errorf("query daemon token privileges: %w", err)
	}
	privileges := (*windows.Tokenprivileges)(unsafe.Pointer(&buffer[0])).AllPrivileges()
	var missing []string
	for _, name := range []string{"SeAssignPrimaryTokenPrivilege", "SeIncreaseQuotaPrivilege"} {
		ptr, _ := windows.UTF16PtrFromString(name)
		var expected windows.LUID
		if err := windows.LookupPrivilegeValue(nil, ptr, &expected); err != nil {
			return fmt.Errorf("resolve service privilege: %w", err)
		}
		found := false
		for _, privilege := range privileges {
			if privilege.Luid == expected && privilege.Attributes&windows.SE_PRIVILEGE_REMOVED == 0 {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, name)
		}
	}
	if len(missing) != 0 {
		return fmt.Errorf("conservative service readiness check: daemon token lacks %s; no rights were granted", strings.Join(missing, ", "))
	}
	return nil
}
