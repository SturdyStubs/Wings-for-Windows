package node

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"wings-windows/internal/config"
	"wings-windows/internal/identity"
)

func (s *Server) identityAccessRules() ([]identity.PathRule, error) {
	if s.Profile.RunAs == nil {
		return nil, nil
	}
	combined := map[string]identity.PathRule{}
	add := func(path string, mask uint32, recursive bool) {
		if path == "" {
			return
		}
		// Preserve case variants: per-directory case-sensitive filesystems can
		// contain distinct files whose names differ only in case.
		key := filepath.Clean(path)
		rule := combined[key]
		rule.Path = path
		rule.Forbidden |= mask
		rule.Recursive = rule.Recursive || recursive
		combined[key] = rule
	}
	protect := func(path string, mask uint32) {
		if path != "" {
			add(path, mask, false)
			add(filepath.Dir(path), identity.ReplaceChildAccess, false)
		}
	}
	binary, err := os.Executable()
	if err != nil {
		return nil, err
	}
	protect(binary, identity.WriteAccess)
	protect(s.panel.Config.SourcePath, identity.ReadWriteAccess)
	protect(s.panel.Config.TLS.Key, identity.ReadWriteAccess)
	protectProfile := func(profile config.Profile) {
		protect(profile.Executable, identity.WriteAccess)
		for _, file := range profile.Requirements.RequiredFiles {
			protect(file, identity.WriteAccess)
		}
	}
	protectProfile(s.Profile)
	for id, profile := range s.panel.Config.Profiles {
		if !strings.EqualFold(id, s.ID()) {
			add(filepath.Join(s.panel.Config.DataDirectory, id), identity.ReadWriteAccess, true)
			protectProfile(profile)
		}
	}
	keys := make([]string, 0, len(combined))
	for key := range combined {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rules := make([]identity.PathRule, 0, len(keys))
	for _, key := range keys {
		rules = append(rules, combined[key])
	}
	return rules, nil
}
