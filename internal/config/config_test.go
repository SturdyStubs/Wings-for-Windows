package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const id = "11111111-1111-4111-8111-111111111111"

func valid(t *testing.T) Config {
	return Config{Listen: "127.0.0.1:8080", PanelURL: "http://127.0.0.1:8081", NodeUUID: id, TokenID: "node", Token: strings.Repeat("s", 32), DataDirectory: t.TempDir(), AllowInsecureLoopback: true, Profiles: map[string]Profile{id: {Executable: filepath.Join(t.TempDir(), "game.exe"), ReadyPattern: "READY", StopCommand: "stop"}}}
}
func TestConfigurationBoundaries(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{{"public HTTP", func(c *Config) { c.Listen = "0.0.0.0:8080" }}, {"remote panel HTTP", func(c *Config) { c.PanelURL = "http://panel.example.com" }}, {"short secret", func(c *Config) { c.Token = "secret" }}, {"relative executable", func(c *Config) { p := c.Profiles[id]; p.Executable = "game.exe"; c.Profiles[id] = p }}, {"invalid readiness", func(c *Config) { p := c.Profiles[id]; p.ReadyPattern = "["; c.Profiles[id] = p }}, {"reserved environment", func(c *Config) { p := c.Profiles[id]; p.AllowedEnvironment = []string{"PATH"}; c.Profiles[id] = p }}, {"wildcard origin", func(c *Config) { c.Origins = []string{"*"} }}, {"origin query", func(c *Config) { c.Origins = []string{"https://panel.example.com?secret=x"} }}, {"invalid UUID", func(c *Config) { c.NodeUUID = "../bad" }}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := valid(t)
			test.mutate(&c)
			if c.Validate() == nil {
				t.Fatal("unsafe configuration accepted")
			}
		})
	}
	c := valid(t)
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if c.Profiles[id].StopTimeoutSeconds != 20 {
		t.Fatal("default stop timeout")
	}
}
func TestLoadRejectsUnknownAndTrailingJSON(t *testing.T) {
	for _, body := range []string{`{"typo":true}`, `{} {}`} {
		path := filepath.Join(t.TempDir(), "config.json")
		os.WriteFile(path, []byte(body), 0600)
		if _, err := Load(path); err == nil {
			t.Fatal("invalid JSON configuration accepted")
		}
	}
}
func TestTokenEnvironmentOverride(t *testing.T) {
	c := valid(t)
	c.Token = ""
	b, _ := json.Marshal(c)
	path := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(path, b, 0600)
	t.Setenv("WINGS_NATIVE_TOKEN", strings.Repeat("e", 32))
	got, err := Load(path)
	if err != nil || got.Token != strings.Repeat("e", 32) {
		t.Fatalf("override failed: %v", err)
	}
	if got.Profiles[id].StopTimeoutSeconds != 20 {
		t.Fatal("validated defaults were not returned by Load")
	}
}
func TestTimezoneValidationAndDefault(t *testing.T) {
	c := valid(t)
	if err := c.Validate(); err != nil || c.Timezone != "UTC" {
		t.Fatal("default timezone", err)
	}
	c.Timezone = "America/Phoenix"
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	c.Timezone = "not-a-timezone"
	if c.Validate() == nil {
		t.Fatal("invalid timezone accepted")
	}
}
func TestPanelReadinessIsExplicitOptIn(t *testing.T) {
	c := valid(t)
	p := c.Profiles[id]
	p.ReadyPattern = ""
	c.Profiles[id] = p
	if c.Validate() == nil {
		t.Fatal("missing local readiness accepted")
	}
	p.UseEggProcessConfiguration = true
	c.Profiles[id] = p
	if err := c.Validate(); err != nil {
		t.Fatal("egg policy profile rejected", err)
	}
}
