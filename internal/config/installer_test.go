package config

import (
	"path/filepath"
	"testing"
)

func TestInstallerApprovalAndDefaults(t *testing.T) {
	for _, mode := range []string{"valid", "relative", "unc", "timeout", "memory", "reserved-env", "too-many-args", "nul-arg", "nul-exe", "dependency"} {
		t.Run(mode, func(t *testing.T) {
			c := valid(t)
			p := c.Profiles[id]
			p.Installer = &Installer{Executable: filepath.Join(t.TempDir(), "setup.exe"), Arguments: []string{"install"}}
			switch mode {
			case "relative":
				p.Installer.Executable = "setup.exe"
			case "unc":
				p.Installer.Executable = `\\host\share\setup.exe`
			case "timeout":
				p.Installer.TimeoutSeconds = 7201
			case "memory":
				p.Installer.MemoryMB = 1
			case "reserved-env":
				p.Installer.AllowedEnvironment = []string{"WINGS_NATIVE_TOKEN"}
			case "too-many-args":
				p.Installer.Arguments = make([]string, 257)
			case "nul-arg":
				p.Installer.Arguments = []string{"a\x00b"}
			case "nul-exe":
				p.Installer.Executable = filepath.Join(t.TempDir(), "a\x00.exe")
			case "dependency":
				p.Installer.Requirements.RequiredFiles = []string{"relative.dll"}
			}
			c.Profiles[id] = p
			err := c.Validate()
			if (err == nil) != (mode == "valid") {
				t.Fatal(mode, err)
			}
			if err == nil && (p.Installer.TimeoutSeconds != 600 || p.Installer.MemoryMB != 256) {
				t.Fatal("defaults missing")
			}
		})
	}
}
