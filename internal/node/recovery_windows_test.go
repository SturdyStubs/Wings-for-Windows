package node

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"wings-windows/internal/config"
)

func recoveryFixture(t *testing.T) (*Manager, *Server) {
	m, s, _ := setup(t)
	s.Profile.CrashRecovery = &config.CrashRecovery{Enabled: true, MaxRestarts: 2, WindowSeconds: 30, DelaySeconds: 1}
	return m, s
}
func recoveryWait(t *testing.T, s *Server, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		ok := condition()
		s.mu.Unlock()
		if ok {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("recovery condition timed out: %v", s.Logs(100))
}
func TestCrashRecoveryRestartsFailedProcess(t *testing.T) {
	_, s := recoveryFixture(t)
	if err := s.start(); err != nil {
		t.Fatal(err)
	}
	await(t, s, "running")
	s.mu.Lock()
	old := s.process
	s.mu.Unlock()
	if err := s.Command("crash"); err != nil {
		t.Fatal(err)
	}
	recoveryWait(t, s, func() bool { return s.process != nil && s.process != old && s.state == "running" })
	recoveryWait(t, s, func() bool { return s.recovery == nil })
	status := s.Response()["native_recovery"].(map[string]any)
	if status["attempts_in_window"] != 1 || status["pending"] != false {
		t.Fatalf("bad recovery status: %v", status)
	}
}
func TestCrashRecoveryBoundsStartupCrashLoop(t *testing.T) {
	_, s := recoveryFixture(t)
	s.Profile.Arguments = []string{"crash-on-start"}
	if err := s.start(); err != nil {
		t.Fatal(err)
	}
	recoveryWait(t, s, func() bool {
		return len(s.recoveryHistory) == 2 && s.process == nil && s.recovery == nil && strings.Contains(strings.Join(s.logs, "\n"), "limit reached")
	})
	if count := strings.Count(strings.Join(s.Logs(100), "\n"), "process exited (23)"); count != 3 {
		t.Fatalf("expected initial launch and two retries, got %d", count)
	}
}
func TestCrashRecoveryCancellation(t *testing.T) {
	for _, action := range []string{"stop", "kill", "queued stop", "shutdown", "suspend"} {
		t.Run(action, func(t *testing.T) {
			m, s := recoveryFixture(t)
			if err := s.start(); err != nil {
				t.Fatal(err)
			}
			await(t, s, "running")
			if err := s.Command("crash"); err != nil {
				t.Fatal(err)
			}
			recoveryWait(t, s, func() bool { return s.recovery != nil && s.process == nil })
			switch action {
			case "shutdown":
				m.Close()
			case "suspend":
				s.mu.Lock()
				s.settings.Suspended = true
				s.mu.Unlock()
				if err := s.Power("kill"); err != nil {
					t.Fatal(err)
				}
			case "queued stop":
				if err := s.EnqueuePower("stop", 1); err != nil {
					t.Fatal(err)
				}
			default:
				if err := s.Power(action); err != nil {
					t.Fatal(err)
				}
			}
			time.Sleep(1200 * time.Millisecond)
			s.mu.Lock()
			defer s.mu.Unlock()
			if s.process != nil || s.recovery != nil || len(s.recoveryHistory) != 0 {
				t.Fatal("explicit action allowed recovery")
			}
		})
	}
}
func TestCrashRecoveryDoesNotRestartCleanOrDisabledExit(t *testing.T) {
	for _, command := range []string{"stop", "crash"} {
		t.Run(command, func(t *testing.T) {
			_, s := recoveryFixture(t)
			if command == "crash" {
				s.Profile.CrashRecovery = nil
			}
			if err := s.start(); err != nil {
				t.Fatal(err)
			}
			await(t, s, "running")
			if err := s.Command(command); err != nil {
				t.Fatal(err)
			}
			await(t, s, "offline")
			time.Sleep(1200 * time.Millisecond)
			s.mu.Lock()
			defer s.mu.Unlock()
			if s.process != nil || s.recovery != nil || len(s.recoveryHistory) != 0 {
				t.Fatal("clean or disabled exit restarted")
			}
		})
	}
}
func TestCrashRecoveryRechecksPanelSuspension(t *testing.T) {
	_, s, suspended := setup(t)
	s.Profile.CrashRecovery = &config.CrashRecovery{Enabled: true, MaxRestarts: 2, WindowSeconds: 30, DelaySeconds: 1}
	if err := s.start(); err != nil {
		t.Fatal(err)
	}
	await(t, s, "running")
	suspended.Store(true)
	if err := s.Command("crash"); err != nil {
		t.Fatal(err)
	}
	recoveryWait(t, s, func() bool { return len(s.recoveryHistory) == 1 && s.recovery == nil })
	if s.HasProcess() {
		t.Fatal("suspended game restarted")
	}
	if !strings.Contains(strings.Join(s.Logs(100), "\n"), "manual start required") {
		t.Fatal("failure not reported")
	}
}
func TestCrashRecoveryPrunesOldAttempts(t *testing.T) {
	s := &Server{Profile: config.Profile{CrashRecovery: &config.CrashRecovery{WindowSeconds: 30}}}
	now := time.Now()
	s.recoveryHistory = []time.Time{now.Add(-31 * time.Second), now.Add(-time.Second)}
	s.pruneRecoveryLocked(now)
	if len(s.recoveryHistory) != 1 {
		t.Fatal("sliding restart window did not expire old attempts")
	}
}
func TestCrashRecoveryStartupFailureDoesNotLoop(t *testing.T) {
	_, s := recoveryFixture(t)
	s.Profile.Executable += ".missing.exe"
	if err := s.startWithContext(context.Background()); err == nil {
		t.Fatal("missing executable accepted")
	}
	if s.recovery != nil || len(s.recoveryHistory) != 0 {
		t.Fatal("preflight failure scheduled recovery")
	}
}

func TestCrashRecoveryKillCancelsActiveLaunch(t *testing.T) {
	var block atomic.Bool
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	_, s, _ := setupConfigured(t, func(*RemoteServer) {
		if block.Load() {
			once.Do(func() { close(entered) })
			<-release
		}
	})
	defer close(release)
	s.Profile.CrashRecovery = &config.CrashRecovery{Enabled: true, MaxRestarts: 2, WindowSeconds: 30, DelaySeconds: 1}
	if err := s.start(); err != nil {
		t.Fatal(err)
	}
	await(t, s, "running")
	block.Store(true)
	if err := s.Command("crash"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(4 * time.Second):
		t.Fatal("automatic launch never fetched fresh settings")
	}
	if err := s.Power("kill"); err != nil {
		t.Fatal(err)
	}
	recoveryWait(t, s, func() bool {
		if s.recovery != nil || s.process != nil {
			return false
		}
		if s.power.TryLock() {
			s.power.Unlock()
			return true
		}
		return false
	})
}
