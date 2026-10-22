package node

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"wings-windows/internal/config"
)

var fixture string

const testID = "11111111-1111-4111-8111-111111111111"

func TestMain(m *testing.M) {
	directory, err := os.MkdirTemp("", "native-node-test-")
	if err != nil {
		panic(err)
	}
	fixture = filepath.Join(directory, "fixture.exe")
	if b, err := exec.Command("go", "build", "-o", fixture, "../../cmd/testserver").CombinedOutput(); err != nil {
		panic(fmt.Sprintf("fixture build: %v %s", err, b))
	}
	code := m.Run()
	os.RemoveAll(directory)
	os.Exit(code)
}
func setup(t *testing.T) (*Manager, *Server, *atomic.Bool) {
	return setupConfigured(t, nil)
}
func setupConfigured(t *testing.T, configure func(*RemoteServer)) (*Manager, *Server, *atomic.Bool) {
	return setupConfiguredID(t, testID, configure)
}
func setupConfiguredID(t *testing.T, serverID string, configure func(*RemoteServer)) (*Manager, *Server, *atomic.Bool) {
	t.Helper()
	suspended := &atomic.Bool{}
	panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		settings := Settings{UUID: serverID, Environment: map[string]any{"PORT": "28015", "UNTRUSTED": "ignored"}, Suspended: suspended.Load()}
		remote := RemoteServer{UUID: serverID, Settings: settings}
		if configure != nil {
			configure(&remote)
		}
		if strings.HasSuffix(r.URL.Path, "/servers") {
			json.NewEncoder(w).Encode(map[string]any{"data": []RemoteServer{remote}})
		} else {
			json.NewEncoder(w).Encode(remote)
		}
	}))
	t.Cleanup(panel.Close)
	m := NewManager(config.Config{PanelURL: panel.URL, TokenID: "test", Token: strings.Repeat("s", 32), DataDirectory: t.TempDir(), Profiles: map[string]config.Profile{serverID: {Executable: fixture, ReadyPattern: "^SERVER_READY$", StopCommand: "stop", StopTimeoutSeconds: 1, AllowedEnvironment: []string{"PORT"}, MemoryMB: 128}}})
	if err := m.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	return m, m.Get(serverID), suspended
}
func await(t *testing.T, s *Server, state string) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if s.State() == state {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("expected %s; state %s; logs %v", state, s.State(), s.Logs(100))
}
func TestNativeServerLifecycle(t *testing.T) {
	_, s, _ := setup(t)
	if err := s.start(); err != nil {
		t.Fatal(err)
	}
	await(t, s, "running")
	if err := s.Command("hello"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for !slices.Contains(s.Logs(100), "ECHO:hello") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !slices.Contains(s.Logs(100), "ECHO:hello") {
		t.Fatal("missing console echo")
	}
	stats := s.Response()["utilization"].(map[string]any)
	if stats["memory_bytes"].(uint64) == 0 {
		t.Fatal("no process metrics")
	}
	if err := s.stop(false); err != nil {
		t.Fatal(err)
	}
	await(t, s, "offline")
	if err := s.start(); err != nil {
		t.Fatal(err)
	}
	await(t, s, "running")
	if err := s.stop(true); err != nil {
		t.Fatal(err)
	}
	await(t, s, "offline")
}
func TestSuspensionAndShutdownPreventStart(t *testing.T) {
	m, s, suspended := setup(t)
	suspended.Store(true)
	if err := s.start(); err == nil {
		t.Fatal("suspended server started")
	}
	suspended.Store(false)
	m.Close()
	if err := s.start(); err == nil {
		t.Fatal("server started after node close")
	}
}
func TestArgumentTemplatesAndSecretIsolation(t *testing.T) {
	_, s, _ := setup(t)
	t.Setenv("WINGS_NATIVE_TOKEN", "do-not-pass-to-child")
	s.Profile.Arguments = []string{"--port", "{{PORT}}", "--name=hello world"}
	spec, err := s.spec()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(spec.Arguments, []string{"--port", "28015", "--name=hello world"}) {
		t.Fatal(spec.Arguments)
	}
	for _, entry := range spec.Environment {
		if strings.HasPrefix(entry, "WINGS_NATIVE_TOKEN=") || strings.HasPrefix(entry, "UNTRUSTED=") {
			t.Fatal("secret/unapproved environment leaked")
		}
	}
	s.Profile.Arguments = []string{"{{MISSING}}"}
	if _, err = s.spec(); err == nil {
		t.Fatal("missing variable accepted")
	}
}
func TestUnapprovedServersAreNotRegistered(t *testing.T) {
	m, _, _ := setup(t)
	if m.Get("22222222-2222-4222-8222-222222222222") != nil {
		t.Fatal("unapproved server registered")
	}
	if err := m.Get(testID).Power("shell"); err == nil {
		t.Fatal("unknown action accepted")
	}
}
func TestEmergencyKillInterruptsStopAndRestart(t *testing.T) {
	for _, action := range []string{"stop", "restart"} {
		t.Run(action, func(t *testing.T) {
			_, s, _ := setup(t)
			s.Profile.Arguments = []string{"noinput"}
			s.Profile.StopTimeoutSeconds = 30
			if err := s.start(); err != nil {
				t.Fatal(err)
			}
			await(t, s, "running")
			if err := s.Power(action); err != nil {
				t.Fatal(err)
			}
			await(t, s, "stopping")
			started := time.Now()
			if err := s.Power("kill"); err != nil {
				t.Fatal("kill was blocked by power lock", err)
			}
			await(t, s, "offline")
			if time.Since(started) > 2*time.Second {
				t.Fatal("kill waited for graceful timeout")
			}
			deadline := time.Now().Add(3 * time.Second)
			released := false
			for time.Now().Before(deadline) {
				if s.power.TryLock() {
					s.power.Unlock()
					released = true
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if !released {
				t.Fatal("interrupted power action did not release its lock")
			}
			if s.State() != "offline" {
				t.Fatal("cancelled restart relaunched the game")
			}
		})
	}
}
func TestEmergencyKillCancelsPendingStart(t *testing.T) {
	_, s, _ := setup(t)
	entered := make(chan struct{})
	cancelled := make(chan struct{})
	panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(entered); <-r.Context().Done(); close(cancelled) }))
	defer panel.Close()
	s.panel.Config.PanelURL = panel.URL
	if err := s.Power("start"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("start did not fetch configuration")
	}
	if err := s.Power("kill"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-cancelled:
	case <-time.After(3 * time.Second):
		t.Fatal("kill did not cancel pending panel fetch")
	}
	if s.State() != "offline" {
		t.Fatal("cancelled start launched process")
	}
}
func TestDerivedRuntimeEnvironment(t *testing.T) {
	_, s, _ := setup(t)
	s.mu.Lock()
	s.settings.Allocations = json.RawMessage(`{"default":{"ip":"192.0.2.10","port":28015}}`)
	s.settings.Build.Memory = 1024
	s.settings.Invocation = "panel invocation retained as metadata"
	s.settings.Environment["SERVER_PORT"] = "malicious override"
	s.mu.Unlock()
	s.Profile.AllowedEnvironment = append(s.Profile.AllowedEnvironment, "SERVER_PORT")
	s.Profile.Arguments = []string{"--ip", "{{SERVER_IP}}", "--port", "{{SERVER_PORT}}", "--memory", "{{SERVER_MEMORY}}", "--zone", "{{TZ}}"}
	s.panel.Config.Timezone = "America/Phoenix"
	spec, err := s.spec()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(spec.Arguments, []string{"--ip", "192.0.2.10", "--port", "28015", "--memory", "1024", "--zone", "America/Phoenix"}) {
		t.Fatal("derived argument substitution", spec.Arguments)
	}
	if !slices.Contains(spec.Environment, "STARTUP=panel invocation retained as metadata") {
		t.Fatal("missing startup environment")
	}
	if slices.Contains(spec.Environment, "SERVER_PORT=malicious override") {
		t.Fatal("panel variable overrode allocation")
	}
}
