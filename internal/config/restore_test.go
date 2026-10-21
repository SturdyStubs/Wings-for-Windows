package config

import (
	"path/filepath"
	"testing"
)

func TestRestoreRunningRequiresEnabledDurableBudget(t *testing.T) {
	for _, mode := range []string{"no-storage", "no-budget", "disabled", "volatile", "valid"} {
		t.Run(mode, func(t *testing.T) {
			c := valid(t)
			c.ConsoleLogs = &ConsoleLogs{Directory: filepath.Join(t.TempDir(), "console")}
			p := c.Profiles[id]
			p.RestoreRunning = true
			p.CrashRecovery = &CrashRecovery{Enabled: true, PersistHistory: true}
			switch mode {
			case "no-storage":
				c.ConsoleLogs = nil
			case "no-budget":
				p.CrashRecovery = nil
			case "disabled":
				p.CrashRecovery.Enabled = false
			case "volatile":
				p.CrashRecovery.PersistHistory = false
			}
			c.Profiles[id] = p
			if err := c.Validate(); (err == nil) != (mode == "valid") {
				t.Fatal(mode, err)
			}
		})
	}
}
