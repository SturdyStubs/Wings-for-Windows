package config

import (
	"errors"
	"path/filepath"
	"regexp"
	"strings"
)

type Installer struct {
	PersistState       bool         `json:"persist_state,omitempty"`
	Executable         string       `json:"executable"`
	Arguments          []string     `json:"arguments"`
	AllowedEnvironment []string     `json:"allowed_environment,omitempty"`
	TimeoutSeconds     int          `json:"timeout_seconds,omitempty"`
	MemoryMB           int          `json:"memory_mb,omitempty"`
	Requirements       Requirements `json:"requirements,omitempty"`
}

func (p *Profile) NormalizeInstaller() error {
	i := p.Installer
	if i == nil {
		return nil
	}
	if len(i.Executable) > 32760 || strings.ContainsAny(i.Executable, "\x00\r\n") || !filepath.IsAbs(i.Executable) || filepath.Clean(i.Executable) != i.Executable || strings.HasPrefix(i.Executable, `\\`) || !strings.EqualFold(filepath.Ext(i.Executable), ".exe") {
		return errors.New("installer.executable must be a clean absolute local .exe path")
	}
	if len(i.Arguments) > 256 {
		return errors.New("installer arguments exceed 256")
	}
	for _, arg := range i.Arguments {
		if len(arg) > 32760 || strings.ContainsRune(arg, 0) {
			return errors.New("invalid installer argument")
		}
	}
	if i.TimeoutSeconds == 0 {
		i.TimeoutSeconds = 600
	}
	if i.TimeoutSeconds < 1 || i.TimeoutSeconds > 7200 {
		return errors.New("installer timeout_seconds must be 1..7200")
	}
	if i.MemoryMB == 0 {
		i.MemoryMB = 256
	}
	if i.MemoryMB < 16 || i.MemoryMB > 8192 {
		return errors.New("installer memory_mb must be 16..8192")
	}
	if len(i.AllowedEnvironment) > 128 {
		return errors.New("installer environment exceeds 128 names")
	}
	for _, key := range i.AllowedEnvironment {
		if !regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`).MatchString(key) || strings.HasPrefix(key, "WINGS_") {
			return errors.New("invalid or reserved installer environment name")
		}
		switch key {
		case "PATH", "SYSTEMROOT", "WINDIR", "TEMP", "TMP", "COMSPEC":
			return errors.New("reserved installer environment name")
		}
	}
	v := p.InstallerProfile()
	return v.NormalizeRequirements()
}
func (p Profile) InstallerProfile() Profile {
	i := p.Installer
	return Profile{Executable: i.Executable, Arguments: append([]string(nil), i.Arguments...), AllowedEnvironment: append([]string(nil), i.AllowedEnvironment...), RunAs: p.RunAs, NetworkPolicy: p.NetworkPolicy, SchemaVersion: p.SchemaVersion, Requirements: i.Requirements, MemoryMB: i.MemoryMB, ReadyPattern: "^NEVER_MATCH_INSTALLER$", StopTimeoutSeconds: 1}
}
