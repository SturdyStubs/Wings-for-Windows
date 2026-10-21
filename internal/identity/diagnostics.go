package identity

type DiagnosticCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type DiagnosticReport struct {
	ServerUUID          string            `json:"server_uuid"`
	Version             string            `json:"version"`
	HostOS              string            `json:"host_os"`
	HostArchitecture    string            `json:"host_architecture"`
	Passed              bool              `json:"passed"`
	GameLaunched        bool              `json:"game_launched"`
	PanelTested         bool              `json:"panel_tested"`
	NetworkPolicyTested bool              `json:"network_policy_tested"`
	Checks              []DiagnosticCheck `json:"checks"`
}

func PendingAuditChecks() []DiagnosticCheck {
	checks := make([]DiagnosticCheck, 0, 7)
	for _, name := range []string{"identity_reference", "credential_presence", "account_token", "daemon_access", "filesystem_access", "launch_access", "service_privileges"} {
		checks = append(checks, DiagnosticCheck{Name: name, Status: "not_run"})
	}
	return checks
}
