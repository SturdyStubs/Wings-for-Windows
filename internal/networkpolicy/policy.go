package networkpolicy

import (
	"fmt"
	"wings-windows/internal/identity"
)

type Policy struct {
	Mode string `json:"mode,omitempty"`
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
