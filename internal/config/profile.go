package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

const ProfileSchemaVersion = 1

// Requirements are operator-owned preflight declarations, never panel commands.
type Requirements struct {
	Architecture        string   `json:"architecture,omitempty"`
	MinimumWindowsBuild int      `json:"minimum_windows_build,omitempty"`
	RequiredFiles       []string `json:"required_files,omitempty"`
}

func (p *Profile) NormalizeRequirements() error {
	if p.SchemaVersion == 0 {
		p.SchemaVersion = ProfileSchemaVersion
	}
	if p.SchemaVersion != ProfileSchemaVersion {
		return fmt.Errorf("unsupported profile schema_version %d (supported: %d)", p.SchemaVersion, ProfileSchemaVersion)
	}
	r := p.Requirements
	if r.Architecture != "" && r.Architecture != "amd64" && r.Architecture != "386" {
		return errors.New("requirements.architecture must be amd64 or 386")
	}
	if r.MinimumWindowsBuild < 0 || r.MinimumWindowsBuild > 1_000_000 {
		return errors.New("requirements.minimum_windows_build must be 0..1000000")
	}
	if len(r.RequiredFiles) > 64 {
		return errors.New("requirements.required_files exceeds 64 files")
	}
	for _, path := range r.RequiredFiles {
		if len(path) > 32760 || strings.ContainsAny(path, "\x00\r\n") || !filepath.IsAbs(path) {
			return errors.New("requirements.required_files must contain absolute local file paths")
		}
		// Network paths can block filesystem checks independently of cancellation.
		if strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, `//`) {
			return errors.New("network/device paths are unsupported in required_files")
		}
	}
	return nil
}
