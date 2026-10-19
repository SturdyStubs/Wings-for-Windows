//go:build !windows

package identity

import "context"

func AuditIdentity(context.Context, Reference, []PathRule) []DiagnosticCheck {
	checks := PendingAuditChecks()
	checks[0].Status = "failed"
	checks[0].Error = "identity diagnostics require Windows"
	return checks
}
