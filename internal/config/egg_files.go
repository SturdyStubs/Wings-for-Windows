package config

import (
	"fmt"
	"strings"
)

// EggFileName is deliberately stricter than URL file paths: relative, canonical,
// slash-separated names without aliases, devices or NTFS alternate streams.
func EggFileName(name string) error {
	if len(name) == 0 || len(name) > 240 || strings.HasPrefix(name, "/") || strings.ContainsAny(name, "\\:*?\"<>|\x00\r\n") {
		return fmt.Errorf("invalid egg config filename")
	}
	for _, r := range name {
		if r < 32 {
			return fmt.Errorf("control character in egg config filename")
		}
	}
	for _, part := range strings.Split(name, "/") {
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if part == "" || part == "." || part == ".." || strings.TrimRight(part, " .") != part || base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || (len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9') {
			return fmt.Errorf("ambiguous egg config filename")
		}
	}
	return nil
}
func (p Profile) ValidateEggFiles() error {
	if len(p.EggConfigFiles) > 16 {
		return fmt.Errorf("egg_config_files exceeds 16 files")
	}
	if len(p.EggConfigFiles) > 0 && !p.UseEggProcessConfiguration {
		return fmt.Errorf("egg_config_files requires use_egg_process_configuration")
	}
	seen := map[string]bool{}
	for _, name := range p.EggConfigFiles {
		if err := EggFileName(name); err != nil {
			return err
		}
		key := strings.ToLower(name)
		if seen[key] {
			return fmt.Errorf("duplicate egg config filename")
		}
		seen[key] = true
	}
	return nil
}
