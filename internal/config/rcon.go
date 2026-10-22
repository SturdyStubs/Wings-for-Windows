package config

import (
	"errors"
	"net"
	"regexp"
	"strconv"
	"strings"
)

// SourceRCON is local operator configuration, never populated from panel data.
type SourceRCON struct {
	Address                  string `json:"address"`
	PasswordEnvironment      string `json:"password_environment,omitempty"`
	PasswordCredentialTarget string `json:"password_credential_target,omitempty"`
	TimeoutSeconds           int    `json:"timeout_seconds,omitempty"`
}

func (r *SourceRCON) Normalize() error {
	host, port, err := net.SplitHostPort(r.Address)
	n, portErr := strconv.Atoi(port)
	if err != nil || (host != "127.0.0.1" && host != "::1") || portErr != nil || n < 1 || n > 65535 || strconv.Itoa(n) != port {
		return errors.New("source_rcon.address must use literal 127.0.0.1 or [::1] and port 1..65535")
	}
	if (r.PasswordEnvironment == "") == (r.PasswordCredentialTarget == "") {
		return errors.New("source_rcon requires exactly one password reference")
	}
	if r.PasswordEnvironment != "" && !regexp.MustCompile(`^WINGS_RCON_[A-Z0-9_]{1,100}$`).MatchString(r.PasswordEnvironment) {
		return errors.New("source_rcon.password_environment must be a WINGS_RCON_ uppercase identifier")
	}
	if r.PasswordCredentialTarget != "" && (!strings.HasPrefix(r.PasswordCredentialTarget, "WingsWindowsRCON/") || !UUID.MatchString(strings.TrimPrefix(r.PasswordCredentialTarget, "WingsWindowsRCON/"))) {
		return errors.New("source_rcon credential target must use WingsWindowsRCON/<server UUID>")
	}
	if r.TimeoutSeconds == 0 {
		r.TimeoutSeconds = 3
	}
	if r.TimeoutSeconds < 1 || r.TimeoutSeconds > 10 {
		return errors.New("source_rcon.timeout_seconds must be 1..10")
	}
	return nil
}

func (p *Profile) ValidateRCON() error {
	if p.SourceRCON == nil {
		return nil
	}
	if err := p.SourceRCON.Normalize(); err != nil {
		return err
	}
	if len(p.StopCommand) > 4086 {
		return errors.New("source_rcon stop command exceeds 4086 bytes")
	}
	for _, name := range p.AllowedEnvironment {
		if strings.HasPrefix(name, "WINGS_RCON_") {
			return errors.New("RCON secret environment names cannot be forwarded from the panel")
		}
	}
	return nil
}
