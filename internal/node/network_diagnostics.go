package node

import (
	"context"
	"strings"
	"wings-windows/internal/config"
	"wings-windows/internal/identity"
	"wings-windows/internal/networkpolicy"
)

type NetworkDiagnosticReport struct {
	ServerUUID string                            `json:"server_uuid"`
	Passed     bool                              `json:"passed"`
	Error      string                            `json:"error,omitempty"`
	Identity   *identity.DiagnosticReport        `json:"identity,omitempty"`
	Policy     *networkpolicy.VerificationReport `json:"policy,omitempty"`
}

// CheckNetworkPolicy explicitly and temporarily blocks a provisioned account.
// Identity preflight must pass before any WFP session is opened.
func CheckNetworkPolicy(ctx context.Context, c config.Config, id string) NetworkDiagnosticReport {
	report := NetworkDiagnosticReport{ServerUUID: strings.ToLower(id)}
	profile, ok := c.Profiles[report.ServerUUID]
	if !ok || !config.UUID.MatchString(id) {
		report.Error = "a configured full server UUID is required"
		return report
	}
	if profile.NetworkPolicy.Mode != "block_connections" {
		report.Error = "profile must explicitly select block_connections"
		return report
	}
	if err := profile.NetworkPolicy.Validate(profile.RunAs); err != nil {
		report.Error = err.Error()
		return report
	}
	preflight := CheckIdentity(ctx, c, id)
	report.Identity = &preflight
	if !preflight.Passed {
		report.Error = "identity preflight failed; no network policy was installed"
		return report
	}
	result := networkpolicy.Verify(ctx, profile.NetworkPolicy, profile.RunAs)
	report.Policy = &result
	report.Passed = result.Passed
	return report
}
