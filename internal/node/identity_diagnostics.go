package node

import (
	"context"
	"runtime"
	"strings"
	"wings-windows/internal/config"
	"wings-windows/internal/identity"
	"wings-windows/internal/provision"
)

// CheckIdentity is offline with respect to the panel. It never initializes a
// manager or server root, and it does not launch an executable.
func CheckIdentity(ctx context.Context, c config.Config, serverID string) identity.DiagnosticReport {
	if ctx == nil {
		ctx = context.Background()
	}
	report := identity.DiagnosticReport{ServerUUID: strings.ToLower(serverID), Version: config.Version, HostOS: runtime.GOOS, HostArchitecture: runtime.GOARCH, Checks: []identity.DiagnosticCheck{{Name: "profile", Status: "not_run"}, {Name: "profile_files", Status: "not_run"}, {Name: "protected_path_rules", Status: "not_run"}}}
	report.Checks = append(report.Checks, identity.PendingAuditChecks()...)
	fail := func(i int, message string) identity.DiagnosticReport {
		report.Checks[i].Status = "failed"
		report.Checks[i].Error = message
		return report
	}
	if !config.UUID.MatchString(serverID) {
		return fail(0, "a full server UUID is required")
	}
	profile, ok := c.Profiles[report.ServerUUID]
	if !ok {
		return fail(0, "server has no configured local profile")
	}
	if profile.RunAs == nil {
		return fail(0, "profile uses the shared daemon account; configure run_as before identity diagnostics")
	}
	if err := ctx.Err(); err != nil {
		return fail(0, err.Error())
	}
	if err := profile.RunAs.Validate(); err != nil {
		return fail(0, err.Error())
	}
	if !profile.RunAs.MatchesServer(serverID) {
		return fail(0, "credential reference belongs to a different server")
	}
	report.Checks[0].Status = "passed"
	if err := provision.Check(ctx, profile); err != nil {
		return fail(1, err.Error())
	}
	report.Checks[1].Status = "passed"
	rules, err := buildIdentityRules(c, report.ServerUUID, profile)
	if err != nil {
		return fail(2, err.Error())
	}
	report.Checks[2].Status = "passed"
	copy(report.Checks[3:], identity.AuditIdentity(ctx, *profile.RunAs, rules))
	report.Passed = true
	for _, check := range report.Checks {
		if check.Status != "passed" {
			report.Passed = false
		}
	}
	return report
}
