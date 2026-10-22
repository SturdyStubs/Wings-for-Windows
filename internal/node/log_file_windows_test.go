package node

import (
	"net"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
	"wings-windows/internal/config"
)

func waitLog(t *testing.T, s *Server, line string) {
	t.Helper()
	until := time.Now().Add(3 * time.Second)
	for time.Now().Before(until) {
		if slices.Contains(s.Logs(100), line) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("log not received", line, s.Logs(100))
}
func TestLogFileReadinessSkipsStaleAndIgnoresStdout(t *testing.T) {
	_, s, _ := setup(t)
	s.Profile.Arguments = []string{"spawn"}
	s.Profile.LogFile = &config.LogFile{Path: "server.log", Readiness: true}
	if err := os.WriteFile(filepath.Join(s.Root, "server.log"), []byte("SERVER_READY\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.start(); err != nil {
		t.Fatal(err)
	}
	waitLog(t, s, "SERVER_READY")
	time.Sleep(250 * time.Millisecond)
	if s.State() != "starting" {
		t.Fatal("stale log/stdout set readiness")
	}
	if err := s.Command("log-message"); err != nil {
		t.Fatal(err)
	}
	waitLog(t, s, "[Game log] FILE_MESSAGE")
	if s.State() != "starting" {
		t.Fatal("unmatched line set readiness")
	}
	if err := s.Command("log-ready"); err != nil {
		t.Fatal(err)
	}
	await(t, s, "running")
	status := s.Response()["native_log_file"].(map[string]any)
	if status["active"] != true {
		t.Fatal(status)
	}
	if err := s.stop(false); err != nil {
		t.Fatal(err)
	}
	await(t, s, "offline")
	// Process-scoped reader pins must be released after native exit.
	moved := s.Root + "-moved"
	until := time.Now().Add(time.Second)
	var err error
	for time.Now().Before(until) {
		err = os.Rename(s.Root, moved)
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatal("tail pins leaked", err)
	}
	if err = os.Rename(moved, s.Root); err != nil {
		t.Fatal(err)
	}
}
func TestLogFileRotationTruncationAndSafeRuntimeFailure(t *testing.T) {
	_, s, _ := setup(t)
	s.Profile.LogFile = &config.LogFile{Path: "server.log"}
	if err := s.start(); err != nil {
		t.Fatal(err)
	}
	await(t, s, "running")
	for _, pair := range [][2]string{{"log-message", "[Game log] FILE_MESSAGE"}, {"log-rotate", "[Game log] ROTATED_MESSAGE"}, {"log-truncate", "[Game log] RESET"}} {
		if err := s.Command(pair[0]); err != nil {
			t.Fatal(err)
		}
		waitLog(t, s, pair[1])
	}
	if err := s.Command("log-malformed"); err != nil {
		t.Fatal(err)
	}
	waitLog(t, s, "[Windows node] log-file adapter stopped; inspect approved path, permissions and text limits")
	status := s.Response()["native_log_file"].(map[string]any)
	if status["active"] != false || status["error"] != "read_failed" || s.State() != "running" {
		t.Fatal(status, s.State())
	}
	if slices.Contains(s.Logs(100), "[Game log] \x00BROKEN") {
		t.Fatal("malformed file data exposed")
	}
	if err := s.Power("kill"); err != nil {
		t.Fatal(err)
	}
	await(t, s, "offline")
}
func TestLogFilePreflightAndEggDenylist(t *testing.T) {
	for _, mode := range []string{"missing-parent", "denylist", "protected-code"} {
		t.Run(mode, func(t *testing.T) {
			m, s, _ := setupConfigured(t, func(r *RemoteServer) {
				if mode == "denylist" {
					r.Settings.Egg.FileDenylist = []string{"server.log"}
				}
			})
			s.Profile.LogFile = &config.LogFile{Path: "server.log", Readiness: true}
			if mode == "missing-parent" {
				s.Profile.LogFile.Path = "missing/server.log"
			}
			if mode == "protected-code" {
				m.Config.SourcePath = filepath.Join(s.Root, "server.log")
				s.panel.Config.SourcePath = m.Config.SourcePath
				os.WriteFile(m.Config.SourcePath, []byte("secret"), 0600)
			}
			if err := s.start(); err == nil || s.HasProcess() {
				t.Fatal("unsafe tail launch accepted", err)
			}
		})
	}
}
func TestLogFileReadinessWithRCONCommands(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := l.Addr().String()
	l.Close()
	_, s, _ := setup(t)
	s.Profile.Arguments = []string{"rcon", address}
	s.Profile.SourceRCON = &config.SourceRCON{Address: address, PasswordEnvironment: "WINGS_RCON_FIXTURE", TimeoutSeconds: 2}
	s.Profile.LogFile = &config.LogFile{Path: "server.log", Readiness: true}
	t.Setenv("WINGS_RCON_FIXTURE", "fixture-rcon-only")
	if err := s.start(); err != nil {
		t.Fatal(err)
	}
	waitLog(t, s, "SERVER_READY")
	if s.State() != "starting" {
		t.Fatal("stdout set log-only readiness")
	}
	if err := s.Command("log-ready"); err != nil {
		t.Fatal(err)
	}
	await(t, s, "running")
	if err := s.Command("log-message"); err != nil {
		t.Fatal(err)
	}
	waitLog(t, s, "[Game log] FILE_MESSAGE")
	if err := s.stop(false); err != nil {
		t.Fatal(err)
	}
	await(t, s, "offline")
}
func TestOldProcessOutputCannotPublishOrMarkReady(t *testing.T) {
	_, s, _ := setup(t)
	if err := s.start(); err != nil {
		t.Fatal(err)
	}
	await(t, s, "running")
	s.mu.Lock()
	old := s.process
	s.mu.Unlock()
	if err := s.stop(true); err != nil {
		t.Fatal(err)
	}
	s.processLine(old, "OLD_PROCESS_OUTPUT")
	if slices.Contains(s.Logs(100), "OLD_PROCESS_OUTPUT") {
		t.Fatal("stale generation published output")
	}
}
