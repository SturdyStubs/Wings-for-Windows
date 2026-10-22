package node

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"wings-windows/internal/config"
)

type installResult struct {
	Successful bool `json:"successful"`
	Reinstall  bool `json:"reinstall"`
}

func setupInstaller(t *testing.T, mode string) (*Manager, *Server, <-chan installResult) {
	t.Helper()
	callbacks := make(chan installResult, 8)
	panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test."+strings.Repeat("s", 32) {
			t.Error("incorrect callback bearer")
			w.WriteHeader(401)
			return
		}
		if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/install") {
			var wire map[string]any
			if err := json.NewDecoder(r.Body).Decode(&wire); err != nil {
				t.Error(err)
			}
			if len(wire) != 2 {
				t.Error("unexpected callback fields", wire)
			}
			success, sok := wire["successful"].(bool)
			reinstall, rok := wire["reinstall"].(bool)
			if !sok || !rok {
				t.Error("callback booleans missing", wire)
			}
			callbacks <- installResult{success, reinstall}
			w.WriteHeader(204)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/install") {
			t.Error("remote egg script must not be fetched")
			w.WriteHeader(500)
			return
		}
		remote := RemoteServer{UUID: testID, Settings: Settings{UUID: testID, Environment: map[string]any{"INSTALL_VALUE": "approved", "UNTRUSTED": "not-forwarded"}}}
		remote.Settings.SkipEggScripts = mode == "skip"
		remote.Settings.Suspended = mode == "fresh-suspended" && !strings.HasSuffix(r.URL.Path, "/servers")
		if strings.HasSuffix(r.URL.Path, "/servers") {
			json.NewEncoder(w).Encode(map[string]any{"data": []RemoteServer{remote}})
		} else {
			json.NewEncoder(w).Encode(remote)
		}
	}))
	t.Cleanup(panel.Close)
	p := config.Profile{Executable: fixture, ReadyPattern: "^SERVER_READY$", StopCommand: "stop", StopTimeoutSeconds: 1, Installer: &config.Installer{Executable: fixture, Arguments: []string{"install", mode}, AllowedEnvironment: []string{"INSTALL_VALUE"}, TimeoutSeconds: 5, MemoryMB: 128}}
	m := NewManager(config.Config{PanelURL: panel.URL, TokenID: "test", Token: strings.Repeat("s", 32), DataDirectory: t.TempDir(), Profiles: map[string]config.Profile{testID: p}})
	if err := m.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	return m, m.Get(testID), callbacks
}
func waitInstall(t *testing.T, s *Server) {
	t.Helper()
	until := time.Now().Add(7 * time.Second)
	for time.Now().Before(until) {
		s.mu.Lock()
		active := s.installing
		s.mu.Unlock()
		if !active {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("installation did not finish", s.Response()["native_install"])
}
func TestNativeInstallAndReinstallContract(t *testing.T) {
	_, s, callbacks := setupInstaller(t, "success")
	events, unsubscribe := s.Subscribe()
	defer unsubscribe()
	if err := s.EnqueueInstall(false); err != nil {
		t.Fatal(err)
	}
	waitInstall(t, s)
	select {
	case r := <-callbacks:
		if !r.Successful || r.Reinstall {
			t.Fatal(r)
		}
	default:
		t.Fatal("missing completion callback")
	}
	if b, err := os.ReadFile(filepath.Join(s.Root, "install.marker")); err != nil || string(b) != "installed" {
		t.Fatal(err)
	}
	if !slices.Contains(s.Logs(100), "[Installer] INSTALL_VALUE:approved") {
		t.Fatal(s.Logs(100))
	}
	seen := map[string]bool{}
	for len(events) > 0 {
		seen[(<-events).Event] = true
	}
	for _, name := range []string{"install started", "install output", "install completed"} {
		if !seen[name] {
			t.Fatal("missing event", name)
		}
	}
	if s.State() != "offline" || s.HasProcess() {
		t.Fatal("installer became game process")
	}
	if err := s.Power("start"); err != nil {
		t.Fatal(err)
	}
	await(t, s, "running")
	if err := s.EnqueueInstall(false); err == nil {
		t.Fatal("initial install accepted a running game")
	}
	if err := s.EnqueueInstall(true); err != nil {
		t.Fatal(err)
	}
	waitInstall(t, s)
	if r := <-callbacks; !r.Successful || !r.Reinstall {
		t.Fatal(r)
	}
	await(t, s, "offline")
	if !slices.Contains(s.Logs(100), "STOPPING") {
		t.Fatal("reinstall did not deliver graceful stop", s.Logs(100))
	}
}
func TestInstallerFailureTimeoutKillAndConcurrency(t *testing.T) {
	for _, mode := range []string{"fail", "timeout", "kill", "shutdown"} {
		t.Run(mode, func(t *testing.T) {
			runMode := mode
			if mode != "fail" {
				runMode = "hang"
			}
			m, s, callbacks := setupInstaller(t, runMode)
			if mode == "timeout" {
				s.Profile.Installer.TimeoutSeconds = 1
			}
			if err := s.EnqueueInstall(false); err != nil {
				t.Fatal(err)
			}
			if mode != "fail" {
				until := time.Now().Add(time.Second)
				for time.Now().Before(until) {
					s.mu.Lock()
					p := s.installerProcess
					s.mu.Unlock()
					if p != nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err := s.EnqueueInstall(true); err == nil {
					t.Fatal("concurrent installer accepted")
				}
				if err := s.EnqueuePower("start", 0); err == nil {
					t.Fatal("start accepted during installation")
				}
				if mode == "kill" {
					if err := s.Power("kill"); err != nil {
						t.Fatal(err)
					}
				}
				if mode == "shutdown" {
					m.Close()
				}
			}
			waitInstall(t, s)
			if r := <-callbacks; r.Successful {
				t.Fatal("failed/cancelled install reported success")
			}
			status := s.Response()["native_install"].(map[string]any)
			expected := "failed"
			if mode == "timeout" {
				expected = "timed_out"
			}
			if mode == "kill" || mode == "shutdown" {
				expected = "cancelled"
			}
			if status["stage"] != expected {
				t.Fatal(status)
			}
			if s.HasProcess() {
				t.Fatal("installer registered as game")
			}
		})
	}
}
func TestInstallerCallbackFailureDoesNotRerun(t *testing.T) {
	_, s, _ := setupInstaller(t, "success")
	var calls int
	var mu sync.Mutex
	failed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			mu.Lock()
			calls++
			mu.Unlock()
			w.WriteHeader(503)
			return
		}
		json.NewEncoder(w).Encode(RemoteServer{UUID: testID, Settings: Settings{UUID: testID, Environment: map[string]any{"INSTALL_VALUE": "approved"}}})
	}))
	defer failed.Close()
	s.panel.Config.PanelURL = failed.URL
	if err := s.EnqueueInstall(false); err != nil {
		t.Fatal(err)
	}
	waitInstall(t, s)
	status := s.Response()["native_install"].(map[string]any)
	if status["successful"] != true || status["callback_error"] != "notification_failed" {
		t.Fatal(status)
	}
	mu.Lock()
	defer mu.Unlock()
	if calls != 1 {
		t.Fatal("notification retried", calls)
	}
}
func TestInstallerOutputBoundsAndDisabledProfile(t *testing.T) {
	for _, mode := range []string{"flood", "long-line"} {
		t.Run(mode, func(t *testing.T) {
			_, s, callbacks := setupInstaller(t, mode)
			if err := s.EnqueueInstall(false); err != nil {
				t.Fatal(err)
			}
			waitInstall(t, s)
			if r := <-callbacks; !r.Successful {
				t.Fatal(r)
			}
			if len(s.Logs(1000)) > 1000 {
				t.Fatal("unbounded history")
			}
		})
	}
	_, s, _ := setup(t)
	if err := s.EnqueueInstall(false); err != ErrInstallDisabled {
		t.Fatal(err)
	}
}

func TestInstallerPreflightSkipAndFreshSuspension(t *testing.T) {
	for _, mode := range []string{"missing-game", "missing-installer", "skip", "fresh-suspended", "suspension-sync", "pending-power"} {
		t.Run(mode, func(t *testing.T) {
			fixtureMode := "success"
			if mode == "skip" || mode == "fresh-suspended" {
				fixtureMode = mode
			}
			if mode == "suspension-sync" {
				fixtureMode = "hang"
			}
			_, s, callbacks := setupInstaller(t, fixtureMode)
			if mode == "missing-game" {
				s.Profile.Executable = filepath.Join(t.TempDir(), "absent.exe")
			}
			if mode == "missing-installer" {
				s.Profile.Installer.Executable = filepath.Join(t.TempDir(), "absent.exe")
			}
			if mode == "pending-power" {
				s.mu.Lock()
				s.pendingPower = 1
				s.mu.Unlock()
				if err := s.EnqueueInstall(false); err != ErrPowerBusy {
					t.Fatal(err)
				}
				s.mu.Lock()
				s.pendingPower = 0
				s.mu.Unlock()
				return
			}
			if err := s.EnqueueInstall(false); err != nil {
				t.Fatal(err)
			}
			if mode == "suspension-sync" {
				until := time.Now().Add(time.Second)
				for {
					s.mu.Lock()
					running := s.installerProcess != nil
					s.mu.Unlock()
					if running {
						break
					}
					if time.Now().After(until) {
						t.Fatal("installer not running")
					}
					time.Sleep(time.Millisecond)
				}
				suspended := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "POST" {
						w.WriteHeader(204)
						return
					}
					json.NewEncoder(w).Encode(RemoteServer{UUID: testID, Settings: Settings{UUID: testID, Suspended: true}})
				}))
				defer suspended.Close()
				s.panel.Config.PanelURL = suspended.URL
				if err := s.Sync(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
			waitInstall(t, s)
			status := s.Response()["native_install"].(map[string]any)
			if status["successful"] != (mode == "skip") {
				t.Fatal(status)
			}
			if mode != "suspension-sync" {
				if r := <-callbacks; r.Successful != (mode == "skip") {
					t.Fatal(r)
				}
			}
			if _, err := os.Stat(filepath.Join(s.Root, "install.marker")); mode != "suspension-sync" && !os.IsNotExist(err) {
				t.Fatal("preflight/skip executed installer", err)
			}
		})
	}
}
