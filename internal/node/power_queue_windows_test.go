package node

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func waitPending(t *testing.T, s *Server, wanted int) {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		n := s.pendingPower
		s.mu.Unlock()
		if n == wanted {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("power waiter count did not become %d", wanted)
}
func TestPowerWaitAcquiresReleasedLock(t *testing.T) {
	_, s, _ := setup(t)
	s.power.Lock()
	if err := s.EnqueuePower("start", 2); err != nil {
		s.power.Unlock()
		t.Fatal(err)
	}
	if s.State() != "offline" {
		s.power.Unlock()
		t.Fatal("start bypassed lock")
	}
	s.power.Unlock()
	await(t, s, "running")
	waitPending(t, s, 0)
}
func TestZeroWaitDoesNotRunLater(t *testing.T) {
	_, s, _ := setup(t)
	s.power.Lock()
	if err := s.EnqueuePower("start", 0); err != nil {
		s.power.Unlock()
		t.Fatal(err)
	}
	waitPending(t, s, 0)
	s.power.Unlock()
	if s.HasProcess() {
		t.Fatal("zero-wait request was deferred")
	}
}
func TestPowerWaitTimesOut(t *testing.T) {
	_, s, _ := setup(t)
	s.power.Lock()
	started := time.Now()
	if err := s.EnqueuePower("start", 1); err != nil {
		s.power.Unlock()
		t.Fatal(err)
	}
	waitPending(t, s, 0)
	s.power.Unlock()
	if time.Since(started) < 900*time.Millisecond || s.HasProcess() {
		t.Fatal("wait deadline not honored")
	}
	if !strings.Contains(strings.Join(s.Logs(100), "\n"), "timed out") {
		t.Fatal("timeout not observable")
	}
}
func TestKillCancelsQueuedStartsAndRestarts(t *testing.T) {
	for _, action := range []string{"start", "restart"} {
		t.Run(action, func(t *testing.T) {
			_, s, _ := setup(t)
			s.power.Lock()
			if err := s.EnqueuePower(action, 3); err != nil {
				s.power.Unlock()
				t.Fatal(err)
			}
			if err := s.Power("kill"); err != nil {
				s.power.Unlock()
				t.Fatal(err)
			}
			s.power.Unlock()
			waitPending(t, s, 0)
			if s.HasProcess() || s.State() != "offline" {
				t.Fatal("cancelled queued request launched server")
			}
		})
	}
}
func TestPowerWaitQueueIsBoundedAndStopsOnShutdown(t *testing.T) {
	m, s, _ := setup(t)
	s.power.Lock()
	for i := 0; i < 16; i++ {
		if err := s.EnqueuePower("start", 300); err != nil {
			s.power.Unlock()
			t.Fatal(err)
		}
	}
	if err := s.EnqueuePower("start", 300); !errors.Is(err, ErrPowerQueueFull) {
		s.power.Unlock()
		t.Fatal("unbounded queue", err)
	}
	m.Close()
	s.power.Unlock()
	waitPending(t, s, 0)
	if s.HasProcess() {
		t.Fatal("shutdown waiter launched process")
	}
}
func TestWaitSecondsNormalization(t *testing.T) {
	for _, test := range []struct{ input, want int }{{-1, 30}, {301, 30}, {0, 0}, {1, 1}, {300, 300}} {
		if got := normalizeWaitSeconds(test.input); got != test.want {
			t.Fatal(test, got)
		}
	}
}
