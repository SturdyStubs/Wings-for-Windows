package config

import "fmt"

type LogFile struct {
	Path      string `json:"path"`
	Readiness bool   `json:"readiness,omitempty"`
}

func (p Profile) ValidateLogFile() error {
	if p.LogFile == nil {
		return nil
	}
	if err := EggFileName(p.LogFile.Path); err != nil {
		return fmt.Errorf("log_file.path must be a canonical relative file name: %w", err)
	}
	return nil
}
