package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestConsoleLogDirectoryCannotContainLoadedConfig(t *testing.T) {
	c := valid(t)
	directory := filepath.Join(t.TempDir(), "logs")
	os.MkdirAll(directory, 0700)
	c.ConsoleLogs = &ConsoleLogs{Directory: directory}
	data, _ := json.Marshal(c)
	path := filepath.Join(directory, "node.json")
	os.WriteFile(path, data, 0600)
	if _, err := Load(path); err == nil {
		t.Fatal("config file inside console log tree accepted")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(data) {
		t.Fatal("validation modified configuration")
	}
}

func TestConsoleLogConfiguration(t *testing.T) {
	for _, name := range []string{"defaults", "small file", "large file", "negative count", "large count", "relative", "network", "alias", "device", "inside data", "contains data", "contains executable", "contains TLS"} {
		t.Run(name, func(t *testing.T) {
			c := valid(t)
			c.ConsoleLogs = &ConsoleLogs{Directory: filepath.Join(t.TempDir(), "console")}
			switch name {
			case "small file":
				c.ConsoleLogs.MaxFileBytes = 1024
			case "large file":
				c.ConsoleLogs.MaxFileBytes = 65 << 20
			case "negative count":
				c.ConsoleLogs.MaxFiles = -1
			case "large count":
				c.ConsoleLogs.MaxFiles = 9
			case "relative":
				c.ConsoleLogs.Directory = "logs"
			case "network":
				c.ConsoleLogs.Directory = `\\host\logs`
			case "alias":
				c.ConsoleLogs.Directory += `\..\alias`
			case "device":
				c.ConsoleLogs.Directory += `\CON`
			case "inside data":
				c.ConsoleLogs.Directory = filepath.Join(c.DataDirectory, "logs")
			case "contains data":
				c.ConsoleLogs.Directory = filepath.Dir(c.DataDirectory)
			case "contains executable":
				p := c.Profiles[id]
				p.Executable = filepath.Join(c.ConsoleLogs.Directory, "game.exe")
				c.Profiles[id] = p
			case "contains TLS":
				c.TLS.Cert = filepath.Join(c.ConsoleLogs.Directory, "tls.crt")
				c.TLS.Key = filepath.Join(c.ConsoleLogs.Directory, "tls.key")
			}
			err := c.Validate()
			if (err == nil) != (name == "defaults") {
				t.Fatalf("unexpected validation: %v", err)
			}
			if name == "defaults" && (c.ConsoleLogs.MaxFileBytes != 5<<20 || c.ConsoleLogs.MaxFiles != 4) {
				t.Fatal("defaults missing")
			}
		})
	}
}
