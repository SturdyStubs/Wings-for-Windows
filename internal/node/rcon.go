package node

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"
	"wings-windows/internal/identity"
	"wings-windows/internal/native"
	"wings-windows/internal/sourcercon"
)

func (s *Server) rconCommand(ctx context.Context, p *native.Process, command string) error {
	policy := s.Profile.SourceRCON
	select {
	case <-p.Done:
		return errors.New("server process exited; no RCON command was sent")
	default:
	}
	password := os.Getenv(policy.PasswordEnvironment)
	if policy.PasswordCredentialTarget != "" {
		target, err := identity.RCONCredentialTarget(s.ID())
		if err != nil || target != policy.PasswordCredentialTarget {
			return errors.New("RCON credential reference is invalid")
		}
		password, err = identity.RCONPassword(s.ID())
		if err != nil {
			return err
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-p.Done:
			cancel()
		case <-ctx.Done():
		}
	}()
	response, err := sourcercon.Execute(ctx, policy.Address, password, command, time.Duration(policy.TimeoutSeconds)*time.Second)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.process != p || ctx.Err() != nil {
		return errors.New("RCON process changed or exited; result discarded")
	}
	// Fixed prefix keeps output distinct from runtime diagnostics. Credentials are
	// redacted before publication; RCON never drives readiness or exit detection.
	for _, line := range strings.Split(strings.ReplaceAll(response, "\r\n", "\n"), "\n") {
		if line != "" {
			s.logLocked("[RCON] " + strings.TrimSuffix(line, "\r"))
		}
	}
	return nil
}
