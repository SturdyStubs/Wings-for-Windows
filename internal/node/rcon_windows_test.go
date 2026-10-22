package node

import (
	"context"
	"net"
	"slices"
	"strings"
	"testing"
	"time"
	"wings-windows/internal/config"
)

func setupRCON(t *testing.T) (*Server, string) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := l.Addr().String()
	l.Close()
	_, s, _ := setup(t)
	s.Profile.Arguments = []string{"rcon", address}
	s.Profile.SourceRCON = &config.SourceRCON{Address: address, PasswordEnvironment: "WINGS_RCON_FIXTURE", TimeoutSeconds: 2}
	t.Setenv("WINGS_RCON_FIXTURE", "fixture-rcon-only")
	if err := s.start(); err != nil {
		t.Fatal(err)
	}
	await(t, s, "running")
	return s, address
}
func TestRCONConsoleAndGracefulStop(t *testing.T) {
	s, _ := setupRCON(t)
	if err := s.Command("hello"); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(s.Logs(100), "[RCON] ECHO:hello") {
		t.Fatal(s.Logs(100))
	}
	if err := s.Command("echo-secret"); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(s.Logs(100), "[RCON] [REDACTED]") {
		t.Fatal("secret not redacted")
	}
	// The host secret must not be inherited by the game executable.
	spec, err := s.spec()
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range spec.Environment {
		if strings.HasPrefix(line, "WINGS_RCON_") {
			t.Fatal("host secret inherited")
		}
	}
	if err := s.stop(false); err != nil {
		t.Fatal(err)
	}
	await(t, s, "offline")
	if !slices.Contains(s.Logs(100), "RCON_COMMAND:stop") {
		t.Fatal("stop did not use RCON")
	}
}
func TestRCONKillCancelsBlockedCommandAndBusyRejects(t *testing.T) {
	s, _ := setupRCON(t)
	done := make(chan error, 1)
	go func() { done <- s.Command("hang") }()
	deadline := time.Now().Add(time.Second)
	for !slices.Contains(s.Logs(100), "RCON_COMMAND:hang") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !slices.Contains(s.Logs(100), "RCON_COMMAND:hang") {
		t.Fatal("command never started")
	}
	if err := s.Command("must-not-run"); err == nil {
		t.Fatal("busy command accepted")
	}
	if err := s.Power("kill"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("blocked command succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("kill did not cancel RCON")
	}
	await(t, s, "offline")
}
func TestRCONAuthenticationFailureAndStopFallback(t *testing.T) {
	s, _ := setupRCON(t)
	t.Setenv("WINGS_RCON_FIXTURE", "wrong-fixture-password")
	if err := s.Command("not-sent"); err == nil {
		t.Fatal("wrong password accepted")
	}
	if slices.Contains(s.Logs(100), "RCON_COMMAND:not-sent") {
		t.Fatal("unauthenticated command executed")
	}
	if err := s.stopWithContext(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	await(t, s, "offline")
	if !slices.Contains(s.Logs(100), "[Windows node] stop command delivery failed; waiting for exit before forced stop") {
		t.Fatal("missing safe stop failure")
	}
}
