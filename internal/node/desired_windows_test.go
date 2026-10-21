package node

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func desiredFixture(t *testing.T) (*Manager, *Server) {
	t.Helper()
	old, _ := persistentRecoveryFixture(t)
	old.Close()
	c := old.Config
	p := c.Profiles[testID]
	p.RestoreRunning = true
	p.CrashRecovery.MaxRestarts = 3
	c.Profiles[testID] = p
	m := NewManager(c)
	if err := m.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	return m, m.Get(testID)
}
func reopenedDesired(t *testing.T, m *Manager) (*Manager, *Server) {
	t.Helper()
	m.Close()
	next := NewManager(m.Config)
	if err := next.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(next.Close)
	return next, next.Get(testID)
}
func TestDesiredRunningRestoresOnlyAfterExplicitReadiness(t *testing.T) {
	m, s := desiredFixture(t)
	if err := s.start(); err != nil {
		t.Fatal(err)
	}
	await(t, s, "running")
	next, s := reopenedDesired(t, m)
	if s.HasProcess() || s.State() != "offline" {
		t.Fatal("sync launched game")
	}
	next.RestoreRunning(context.Background())
	await(t, s, "running")
	status := s.Response()["native_restore"].(map[string]any)
	if status["desired_running"] != true || status["loaded_running"] != true {
		t.Fatal(status)
	}
	recovery := s.Response()["native_recovery"].(map[string]any)
	if recovery["attempts_in_window"] != 1 {
		t.Fatal("boot launch unreserved", recovery)
	}
	if err := s.stop(false); err != nil {
		t.Fatal(err)
	}
	await(t, s, "offline")
	third, s := reopenedDesired(t, next)
	third.RestoreRunning(context.Background())
	time.Sleep(1200 * time.Millisecond)
	if s.HasProcess() || s.Response()["native_restore"].(map[string]any)["desired_running"] != false {
		t.Fatal("manual stop forgotten")
	}
}
func TestDesiredStartupHonorsBudgetAndExplicitKill(t *testing.T) {
	for _, mode := range []string{"budget", "kill", "cancelled", "gate"} {
		t.Run(mode, func(t *testing.T) {
			m, s := desiredFixture(t)
			if err := s.start(); err != nil {
				t.Fatal(err)
			}
			await(t, s, "running")
			if mode == "budget" {
				s.recoveryStore.Save([]time.Time{time.Now(), time.Now(), time.Now()})
			}
			next, s := reopenedDesired(t, m)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "cancelled" {
				cancel()
			}
			if mode == "gate" {
				next.restoreGate <- struct{}{}
				next.restoreGate <- struct{}{}
			}
			next.RestoreRunning(ctx)
			if mode == "kill" {
				if err := s.Power("kill"); err != nil {
					t.Fatal(err)
				}
			}
			time.Sleep(1200 * time.Millisecond)
			if mode == "gate" {
				if err := s.Power("kill"); err != nil {
					t.Fatal(err)
				}
				<-next.restoreGate
				<-next.restoreGate
				time.Sleep(100 * time.Millisecond)
			}
			if s.HasProcess() {
				t.Fatal("forbidden startup")
			}
		})
	}
}

type failingDesired struct{}

func (failingDesired) Save(bool) error             { return errors.New("synthetic disk failure") }
func (failingDesired) Close(context.Context) error { return nil }
func TestDesiredStorageFailureBlocksStartButStillKillsGame(t *testing.T) {
	m, s := desiredFixture(t)
	if err := s.start(); err != nil {
		t.Fatal(err)
	}
	await(t, s, "running")
	original := s.desiredStore
	s.desiredStore = failingDesired{}
	defer func() { s.desiredStore = original }()
	if err := s.Power("kill"); err == nil {
		t.Fatal("unconfirmed stop intent acknowledged")
	}
	await(t, s, "offline")
	if err := s.start(); err == nil || s.HasProcess() {
		t.Fatal("unpersisted start allowed")
	}
	m.Close()
}
func TestDesiredStaleCleanExitCannotOverwriteNewIntent(t *testing.T) {
	_, s := desiredFixture(t)
	old := s.desiredGeneration
	if err := s.setDesired(context.Background(), true, nil); err != nil {
		t.Fatal(err)
	}
	s.clearDesiredAfterExit(old)
	if s.Response()["native_restore"].(map[string]any)["desired_running"] != true {
		t.Fatal("stale clean exit overwrote start")
	}
}

func TestDesiredCleanExitClearsAndMissingStateIsStopped(t *testing.T) {
	m, s := desiredFixture(t)
	m.RestoreRunning(context.Background())
	time.Sleep(1100 * time.Millisecond)
	if s.HasProcess() {
		t.Fatal("new storage started a game")
	}
	if err := s.start(); err != nil {
		t.Fatal(err)
	}
	await(t, s, "running")
	s.Command("stop")
	await(t, s, "offline")
	next, s := reopenedDesired(t, m)
	next.RestoreRunning(context.Background())
	time.Sleep(1100 * time.Millisecond)
	if s.HasProcess() || s.Response()["native_restore"].(map[string]any)["loaded_running"] != false {
		t.Fatal("clean exit restored")
	}
}

func TestDesiredRestoreRefreshesPanelSuspension(t *testing.T) {
	m, s := desiredFixture(t)
	if err := s.start(); err != nil {
		t.Fatal(err)
	}
	await(t, s, "running")
	next, s := reopenedDesired(t, m)
	panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(RemoteServer{Settings: Settings{UUID: testID, Suspended: true}})
	}))
	defer panel.Close()
	next.Panel.Config.PanelURL = panel.URL
	next.RestoreRunning(context.Background())
	time.Sleep(1300 * time.Millisecond)
	if s.HasProcess() || s.Response()["is_suspended"] != true {
		t.Fatal("stale panel state launched a game")
	}
}
func TestDesiredCorruptionRefusesManagerStartup(t *testing.T) {
	m, s := desiredFixture(t)
	s.setDesired(context.Background(), true, nil)
	m.Close()
	path := filepath.Join(m.Config.ConsoleLogs.Directory, testID, "intent.0.json")
	if err := os.WriteFile(path, []byte("CORRUPT"), 0600); err != nil {
		t.Fatal(err)
	}
	next := NewManager(m.Config)
	defer next.Close()
	if next.Sync(context.Background()) == nil {
		t.Fatal("corrupt intent accepted")
	}
}

type blockedDesired struct {
	once             sync.Once
	entered, release chan struct{}
	latest           atomic.Bool
}

func (b *blockedDesired) Save(running bool) error {
	b.once.Do(func() { close(b.entered); <-b.release })
	b.latest.Store(running)
	return nil
}
func (*blockedDesired) Close(context.Context) error { return nil }
func TestDesiredSlowCommitCannotDelayKillingGameTree(t *testing.T) {
	_, s := desiredFixture(t)
	if err := s.start(); err != nil {
		t.Fatal(err)
	}
	await(t, s, "running")
	original := s.desiredStore
	defer func() { s.desiredStore = original }()
	blocked := &blockedDesired{entered: make(chan struct{}), release: make(chan struct{})}
	s.desiredStore = blocked
	s.mu.Lock()
	process := s.process
	s.mu.Unlock()
	write := make(chan error, 1)
	go func() { write <- s.setDesired(context.Background(), true, nil) }()
	<-blocked.entered
	killed := make(chan error, 1)
	go func() { killed <- s.Power("kill") }()
	select {
	case <-process.Done:
	case <-time.After(2 * time.Second):
		close(blocked.release)
		t.Fatal("disk write blocked emergency kill")
	}
	close(blocked.release)
	if err := <-write; err != nil {
		t.Fatal(err)
	}
	if err := <-killed; err != nil {
		t.Fatal(err)
	}
	if blocked.latest.Load() {
		t.Fatal("older running commit overwrote kill")
	}
}
