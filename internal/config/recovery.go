package config

import "fmt"

// CrashRecovery applies to unexpected nonzero game exits, never daemon boot.
type CrashRecovery struct {
	PersistHistory bool `json:"persist_history,omitempty"`
	Enabled        bool `json:"enabled"`
	MaxRestarts    int  `json:"max_restarts,omitempty"`
	WindowSeconds  int  `json:"window_seconds,omitempty"`
	DelaySeconds   int  `json:"delay_seconds,omitempty"`
}

func (p *CrashRecovery) Normalize() error {
	if p.MaxRestarts == 0 {
		p.MaxRestarts = 3
	}
	if p.WindowSeconds == 0 {
		p.WindowSeconds = 300
	}
	if p.DelaySeconds == 0 {
		p.DelaySeconds = 5
	}
	if p.MaxRestarts < 1 || p.MaxRestarts > 10 || p.WindowSeconds < 30 || p.WindowSeconds > 3600 || p.DelaySeconds < 1 || p.DelaySeconds > 300 || p.DelaySeconds >= p.WindowSeconds {
		return fmt.Errorf("crash_recovery requires max_restarts 1..10, window_seconds 30..3600 and delay_seconds 1..300 shorter than the window")
	}
	return nil
}
