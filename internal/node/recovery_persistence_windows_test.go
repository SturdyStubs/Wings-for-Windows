package node

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
	"wings-windows/internal/config"
	"wings-windows/internal/consolelog"
)

func persistentRecoveryFixture(t *testing.T) (*Manager, *Server) {
	t.Helper()
	old, _, _ := setup(t)
	old.Close()
	c := old.Config
	c.ConsoleLogs = &config.ConsoleLogs{Directory: filepath.Join(t.TempDir(), "console"), MaxFileBytes: 1 << 20, MaxFiles: 2}
	p := c.Profiles[testID]
	p.CrashRecovery = &config.CrashRecovery{Enabled: true, PersistHistory: true, MaxRestarts: 1, WindowSeconds: 3600, DelaySeconds: 1}
	c.Profiles[testID] = p
	m := NewManager(c)
	if err := m.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	return m, m.Get(testID)
}
func TestRecoveryBudgetSurvivesManagerRestart(t *testing.T) {
	m, s := persistentRecoveryFixture(t)
	if err := s.start(); err != nil {
		t.Fatal(err)
	}
	await(t, s, "running")
	if err := s.Command("crash"); err != nil {
		t.Fatal(err)
	}
	recoveryWait(t, s, func() bool { return s.state == "running" && len(s.recoveryHistory) == 1 && s.recovery == nil })
	m.Close()
	next := NewManager(m.Config)
	if err := next.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	s = next.Get(testID)
	status := s.Response()["native_recovery"].(map[string]any)
	if s.State() != "offline" || s.HasProcess() || status["attempts_in_window"] != 1 || status["restored_attempts"] != 1 || status["persistent_history"] != true {
		t.Fatal(status)
	}
	if err := s.start(); err != nil {
		t.Fatal("manual start denied", err)
	}
	await(t, s, "running")
	s.Command("crash")
	await(t, s, "offline")
	time.Sleep(1200 * time.Millisecond)
	status = s.Response()["native_recovery"].(map[string]any)
	if s.HasProcess() || status["pending"] != false || status["attempts_in_window"] != 1 {
		t.Fatal("restart reset budget", status)
	}
}
func TestRestoredRecoveryWindowAndClockRollback(t *testing.T) {
	for _, name := range []string{"expired", "future"} {
		t.Run(name, func(t *testing.T) {
			m, s := persistentRecoveryFixture(t)
			when := time.Now().Add(-2 * time.Hour)
			want := 0
			if name == "future" {
				when = time.Now().Add(time.Hour)
				want = 1
			}
			if err := s.recoveryStore.Save([]time.Time{when}); err != nil {
				t.Fatal(err)
			}
			m.Close()
			next := NewManager(m.Config)
			if err := next.Sync(context.Background()); err != nil {
				t.Fatal(err)
			}
			defer next.Close()
			s = next.Get(testID)
			status := s.Response()["native_recovery"].(map[string]any)
			if status["attempts_in_window"] != want || s.HasProcess() {
				t.Fatal(status)
			}
		})
	}
}
func TestCorruptRecoveryFailsSyncAndReleasesConsoleStore(t *testing.T) {
	m, _ := persistentRecoveryFixture(t)
	m.Close()
	options := *m.Config.ConsoleLogs
	if err := os.WriteFile(filepath.Join(options.Directory, testID, "recovery.1.json"), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	next := NewManager(m.Config)
	defer next.Close()
	if next.Sync(context.Background()) == nil || next.Get(testID) != nil {
		t.Fatal("corrupt state published a server")
	}
	store, _, err := consolelog.Open(options, testID)
	if err != nil {
		t.Fatal("failed sync leaked console lease", err)
	}
	defer store.Close(context.Background())
}

type gatedRecoveryBudget struct {
	entered chan struct{}
	release chan struct{}
	failure bool
}

func (b *gatedRecoveryBudget) Save([]time.Time) error {
	close(b.entered)
	<-b.release
	if b.failure {
		return errors.New("secret storage path")
	}
	return nil
}
func (b *gatedRecoveryBudget) Close(context.Context) error { return nil }
func TestRecoveryReservationPrecedesLaunchAndRemainsCancellable(t *testing.T) {
	for _, name := range []string{"cancel", "failure"} {
		t.Run(name, func(t *testing.T) {
			_, s := recoveryFixture(t)
			budget := &gatedRecoveryBudget{entered: make(chan struct{}), release: make(chan struct{}), failure: name == "failure"}
			s.recoveryStore = budget
			if err := s.start(); err != nil {
				t.Fatal(err)
			}
			await(t, s, "running")
			s.Command("crash")
			select {
			case <-budget.entered:
			case <-time.After(5 * time.Second):
				t.Fatal("no reservation")
			}
			if s.HasProcess() {
				t.Fatal("launched before persisted reservation")
			}
			if name == "cancel" {
				done := make(chan error, 1)
				go func() { done <- s.Power("kill") }()
				select {
				case err := <-done:
					if err != nil {
						t.Fatal(err)
					}
				case <-time.After(time.Second):
					t.Fatal("storage held server lock during kill")
				}
			}
			close(budget.release)
			recoveryWait(t, s, func() bool { return s.recovery == nil })
			status := s.Response()["native_recovery"].(map[string]any)
			if s.HasProcess() || status["attempts_in_window"] != 1 {
				t.Fatal(status)
			}
			if name == "failure" {
				if status["storage_error"] != "reservation_write_failed" {
					t.Fatal(status)
				}
				if err := s.start(); err != nil {
					t.Fatal("manual start unavailable", err)
				}
				await(t, s, "running")
				s.Command("crash")
				await(t, s, "offline")
				time.Sleep(1200 * time.Millisecond)
				if s.HasProcess() {
					t.Fatal("storage failure did not disable automatic recovery")
				}
			}
		})
	}
}
