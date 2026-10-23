package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata"
	"wings-windows/internal/identity"
	"wings-windows/internal/networkpolicy"
)

const Version = "0.1.0-alpha.41"

var UUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type TLS struct {
	Cert string `json:"cert"`
	Key  string `json:"key"`
}
type Profile struct {
	Installer                  *Installer           `json:"installer,omitempty"`
	LogFile                    *LogFile             `json:"log_file,omitempty"`
	SourceRCON                 *SourceRCON          `json:"source_rcon,omitempty"`
	RestoreRunning             bool                 `json:"restore_running,omitempty"`
	EggConfigFiles             []string             `json:"egg_config_files,omitempty"`
	CrashRecovery              *CrashRecovery       `json:"crash_recovery,omitempty"`
	NetworkPolicy              networkpolicy.Policy `json:"network_policy,omitempty"`
	RunAs                      *identity.Reference  `json:"run_as,omitempty"`
	SchemaVersion              int                  `json:"schema_version,omitempty"`
	Requirements               Requirements         `json:"requirements,omitempty"`
	UseEggProcessConfiguration bool                 `json:"use_egg_process_configuration,omitempty"`
	Executable                 string               `json:"executable"`
	Arguments                  []string             `json:"arguments"`
	AllowedEnvironment         []string             `json:"allowed_environment"`
	StopCommand                string               `json:"stop_command"`
	ReadyPattern               string               `json:"ready_pattern"`
	StopTimeoutSeconds         int                  `json:"stop_timeout_seconds"`
	MemoryMB                   int                  `json:"memory_mb"`
}
type Config struct {
	AllowPendingCreation    bool               `json:"allow_pending_creation,omitempty"`
	ConsoleLogs             *ConsoleLogs       `json:"console_logs,omitempty"`
	SourcePath              string             `json:"-"`
	Listen                  string             `json:"listen"`
	PanelURL                string             `json:"panel_url"`
	NodeUUID                string             `json:"node_uuid"`
	TokenID                 string             `json:"token_id"`
	Token                   string             `json:"token"`
	DataDirectory           string             `json:"data_directory"`
	Timezone                string             `json:"timezone,omitempty"`
	TLS                     TLS                `json:"tls"`
	AllowInsecureLoopback   bool               `json:"allow_insecure_loopback"`
	Origins                 []string           `json:"allowed_origins"`
	AllowCORSPrivateNetwork bool               `json:"allow_cors_private_network,omitempty"`
	Profiles                map[string]Profile `json:"profiles"`
}

func Load(path string) (Config, error) {
	var c Config
	f, err := os.Open(path)
	if err != nil {
		return c, err
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil {
		return c, err
	} else if info.Size() > 1<<20 {
		return c, errors.New("configuration exceeds 1 MiB")
	}
	dec := json.NewDecoder(io.LimitReader(f, 1<<20))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&c); err != nil {
		return c, err
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return c, errors.New("configuration must contain one JSON object")
	}
	if value := os.Getenv("WINGS_NATIVE_TOKEN"); value != "" {
		c.Token = value
	}
	err = c.Validate()
	if err == nil {
		c.SourcePath, err = filepath.Abs(path)
		if err == nil && c.ConsoleLogs != nil {
			err = c.ConsoleLogs.Protect(c.SourcePath)
		}
	}
	return c, err
}
func (c *Config) Validate() error {
	if c.Listen == "" {
		c.Listen = "127.0.0.1:8080"
	}
	host, _, err := net.SplitHostPort(c.Listen)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	loopback := net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback()
	if (c.TLS.Cert == "") != (c.TLS.Key == "") {
		return errors.New("TLS certificate and key must be configured together")
	}
	if c.TLS.Cert != "" && (!filepath.IsAbs(c.TLS.Cert) || !filepath.IsAbs(c.TLS.Key)) {
		return errors.New("TLS certificate and key paths must be absolute")
	}
	if (c.TLS.Cert == "" || c.TLS.Key == "") && !(loopback && c.AllowInsecureLoopback) {
		return errors.New("TLS cert/key required unless explicitly allowing loopback HTTP")
	}
	u, err := url.Parse(c.PanelURL)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("panel_url must be an absolute URL without credentials/query/fragment")
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && c.AllowInsecureLoopback && net.ParseIP(u.Hostname()) != nil && net.ParseIP(u.Hostname()).IsLoopback()) {
		return errors.New("panel_url must use HTTPS (explicit loopback development HTTP excepted)")
	}
	c.PanelURL = strings.TrimRight(c.PanelURL, "/")
	if c.Timezone == "" {
		c.Timezone = "UTC"
	}
	if strings.ContainsAny(c.Timezone, "\x00\r\n") {
		return errors.New("invalid timezone")
	}
	if _, err := time.LoadLocation(c.Timezone); err != nil {
		return fmt.Errorf("timezone must be a valid IANA name: %w", err)
	}
	if len(c.Token) < 32 || c.TokenID == "" || !UUID.MatchString(c.NodeUUID) {
		return errors.New("node_uuid, token_id and a node token of at least 32 characters are required")
	}
	if !filepath.IsAbs(c.DataDirectory) {
		return errors.New("data_directory must be absolute")
	}
	if c.ConsoleLogs != nil {
		if err := c.ConsoleLogs.Normalize(c.DataDirectory); err != nil {
			return err
		}
	}
	if len(c.Profiles) == 0 && !c.AllowPendingCreation {
		return errors.New("at least one local executable profile is required")
	}
	normalized := make(map[string]Profile, len(c.Profiles))
	identities := make(map[string]bool)
	rconAddresses := make(map[string]bool)
	for id, p := range c.Profiles {
		if p.Installer != nil && p.Installer.PersistState && c.ConsoleLogs == nil {
			return fmt.Errorf("server %s: installer.persist_state requires console_logs private storage", id)
		}
		if err := p.NormalizeInstaller(); err != nil {
			return fmt.Errorf("server %s: %w", id, err)
		}
		if err := p.ValidateLogFile(); err != nil {
			return fmt.Errorf("server %s: %w", id, err)
		}
		if err := p.ValidateRCON(); err != nil {
			return fmt.Errorf("server %s: %w", id, err)
		}
		if p.SourceRCON != nil {
			if p.SourceRCON.PasswordCredentialTarget != "" && p.SourceRCON.PasswordCredentialTarget != "WingsWindowsRCON/"+strings.ToLower(id) {
				return errors.New("source_rcon credential target must match this server UUID")
			}
			if rconAddresses[p.SourceRCON.Address] {
				return errors.New("source_rcon addresses must be unique per server")
			}
			rconAddresses[p.SourceRCON.Address] = true
		}
		if err := p.ValidateEggFiles(); err != nil {
			return fmt.Errorf("server %s: %w", id, err)
		}
		if p.RestoreRunning && (c.ConsoleLogs == nil || p.CrashRecovery == nil || !p.CrashRecovery.Enabled || !p.CrashRecovery.PersistHistory) {
			return fmt.Errorf("server %s: restore_running requires console_logs and enabled persistent crash recovery", id)
		}
		if p.CrashRecovery != nil && p.CrashRecovery.PersistHistory && c.ConsoleLogs == nil {
			return fmt.Errorf("server %s: crash_recovery.persist_history requires console_logs private storage", id)
		}
		if c.ConsoleLogs != nil {
			if p.Installer != nil {
				if err := c.ConsoleLogs.Protect(append([]string{p.Installer.Executable}, p.Installer.Requirements.RequiredFiles...)...); err != nil {
					return err
				}
			}
			if err := c.ConsoleLogs.Protect(append([]string{c.SourcePath, c.TLS.Cert, c.TLS.Key, p.Executable}, p.Requirements.RequiredFiles...)...); err != nil {
				return err
			}
		}
		if p.CrashRecovery != nil {
			if err := p.CrashRecovery.Normalize(); err != nil {
				return fmt.Errorf("%s: %w", id, err)
			}
		}
		if err := p.NetworkPolicy.Validate(p.RunAs); err != nil {
			return fmt.Errorf("%s: %w", id, err)
		}
		if err := p.NormalizeRequirements(); err != nil {
			return fmt.Errorf("%s: %w", id, err)
		}
		if !UUID.MatchString(id) {
			return fmt.Errorf("profile key %q is not a server UUID", id)
		}
		if p.RunAs != nil {
			if err := p.RunAs.Validate(); err != nil {
				return fmt.Errorf("%s: %w", id, err)
			}
			if !p.RunAs.MatchesServer(id) {
				return fmt.Errorf("%s: credential target must match this server UUID", id)
			}
			if identities[p.RunAs.SID] {
				return errors.New("each run_as server must use a distinct account SID")
			}
			identities[p.RunAs.SID] = true
		}
		if !filepath.IsAbs(p.Executable) || !strings.EqualFold(filepath.Ext(p.Executable), ".exe") {
			return fmt.Errorf("%s: executable must be an absolute .exe path", id)
		}
		if len(p.Arguments) > 256 || p.MemoryMB < 0 {
			return fmt.Errorf("%s: invalid arguments/memory limit", id)
		}
		if p.ReadyPattern == "" && !p.UseEggProcessConfiguration {
			return fmt.Errorf("%s: ready_pattern is required", id)
		}
		if _, err := regexp.Compile(p.ReadyPattern); err != nil && !p.UseEggProcessConfiguration {
			return fmt.Errorf("%s ready_pattern: %w", id, err)
		}
		if p.StopTimeoutSeconds == 0 {
			p.StopTimeoutSeconds = 20
		}
		if p.StopTimeoutSeconds < 1 || p.StopTimeoutSeconds > 300 {
			return errors.New("stop_timeout_seconds must be 1..300")
		}
		if len(p.StopCommand) > 4096 || strings.ContainsAny(p.StopCommand, "\r\n\x00") {
			return errors.New("stop_command must be a single console line")
		}
		for _, key := range p.AllowedEnvironment {
			if !regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`).MatchString(key) {
				return errors.New("allowed_environment names must be uppercase identifiers")
			}
			switch key {
			case "PATH", "SYSTEMROOT", "WINDIR", "TEMP", "TMP", "COMSPEC":
				return fmt.Errorf("reserved environment variable %s", key)
			}
		}
		key := strings.ToLower(id)
		if _, exists := normalized[key]; exists {
			return errors.New("duplicate case-insensitive server UUID")
		}
		normalized[key] = p
	}
	c.Profiles = normalized
	for _, origin := range c.Origins {
		v, err := url.Parse(origin)
		if err != nil || (v.Scheme != "https" && v.Scheme != "http") || v.Host == "" || v.User != nil || v.RawQuery != "" || v.Fragment != "" || v.Path != "" {
			return errors.New("allowed_origins must be exact HTTP(S) origins")
		}
	}
	return nil
}
