//go:build !windows

package networkpolicy

import (
	"context"
	"wings-windows/internal/identity"
)

func Verify(context.Context, Policy, *identity.Reference) VerificationReport {
	return VerificationReport{Checks: []identity.DiagnosticCheck{{Name: "install", Status: "failed", Error: "WFP verification requires Windows"}}}
}
