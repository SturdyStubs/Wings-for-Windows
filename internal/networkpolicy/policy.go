package networkpolicy

import (
	"fmt"
	"wings-windows/internal/identity"
)

type Policy struct {
	Mode string `json:"mode,omitempty"`
}

type VerificationReport struct {
	Passed          bool                       `json:"passed"`
	CleanupVerified bool                       `json:"cleanup_verified"`
	TrafficTested   bool                       `json:"traffic_tested"`
	GameLaunched    bool                       `json:"game_launched"`
	RuleIDs         []string                   `json:"rule_ids"`
	SublayerID      string                     `json:"sublayer_id,omitempty"`
	Checks          []identity.DiagnosticCheck `json:"checks"`
}

func (p Policy) Validate(ref *identity.Reference) error {
	switch p.Mode {
	case "", "unmanaged":
		return nil
	case "block_connections":
		if ref == nil {
			return fmt.Errorf("block_connections requires a dedicated run_as identity")
		}
		return ref.Validate()
	default:
		return fmt.Errorf("unsupported network policy mode %q", p.Mode)
	}
}
