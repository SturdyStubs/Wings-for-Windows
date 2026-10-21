package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

type ConsoleLogs struct {
	Directory    string `json:"directory"`
	MaxFileBytes int64  `json:"max_file_bytes,omitempty"`
	MaxFiles     int    `json:"max_files,omitempty"`
}

func (p *ConsoleLogs) Normalize(data string) error {
	if p.MaxFileBytes == 0 {
		p.MaxFileBytes = 5 << 20
	}
	if p.MaxFiles == 0 {
		p.MaxFiles = 4
	}
	if p.MaxFileBytes < 1<<20 || p.MaxFileBytes > 64<<20 || p.MaxFiles < 1 || p.MaxFiles > 8 {
		return fmt.Errorf("console_logs requires max_file_bytes 1..64 MiB and max_files 1..8")
	}
	if !filepath.IsAbs(p.Directory) || filepath.Clean(p.Directory) != p.Directory || len(filepath.VolumeName(p.Directory)) != 2 || strings.HasPrefix(p.Directory, `\\`) {
		return fmt.Errorf("console_logs.directory must be a canonical absolute local directory")
	}
	for _, part := range strings.Split(p.Directory[3:], `\`) {
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if part == "" || strings.TrimRight(part, " .") != part || strings.ContainsAny(part, ":*?\"<>|\x00\r\n") || base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || (len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9') {
			return fmt.Errorf("ambiguous console log directory")
		}
	}
	// A shared log tree must never be exposed through a server's file API.
	for _, pair := range [][2]string{{data, p.Directory}, {p.Directory, data}} {
		rel, err := filepath.Rel(pair[0], pair[1])
		if err == nil && rel != ".." && !strings.HasPrefix(rel, `..\`) {
			return fmt.Errorf("console_logs.directory and data_directory must not overlap")
		}
	}
	return nil
}

func (p *ConsoleLogs) Protect(paths ...string) error {
	for _, path := range paths {
		if path == "" {
			continue
		}
		relative, err := filepath.Rel(p.Directory, path)
		if err == nil && relative != ".." && !strings.HasPrefix(relative, `..\`) {
			return fmt.Errorf("console log directory must not contain configuration, TLS or executable/dependency files")
		}
	}
	return nil
}
