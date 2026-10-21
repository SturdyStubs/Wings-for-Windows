package node

import (
	"context"
	"fmt"
	"time"
)

type recoveryAttempt struct {
	cancel context.CancelFunc
	next   time.Time
}

// Caller holds mu. An explicit operator action always supersedes recovery.
func (s *Server) cancelRecoveryLocked(stopped bool) {
	s.recoveryStopped = stopped
	if s.recovery != nil {
		s.recovery.cancel()
		s.recovery = nil
	}
}

func (s *Server) pruneRecoveryLocked(now time.Time) {
	policy := s.Profile.CrashRecovery
	if policy == nil {
		return
	}
	cutoff := now.Add(-time.Duration(policy.WindowSeconds) * time.Second)
	kept := s.recoveryHistory[:0]
	for _, attempted := range s.recoveryHistory {
		if attempted.After(cutoff) {
			kept = append(kept, attempted)
		}
	}
	s.recoveryHistory = kept
}

func (s *Server) scheduleRecoveryLocked() {
	policy := s.Profile.CrashRecovery
	if policy == nil || !policy.Enabled || s.recoveryStopped || s.closing || s.settings.Suspended {
		return
	}
	now := time.Now()
	s.pruneRecoveryLocked(now)
	if len(s.recoveryHistory) >= policy.MaxRestarts {
		s.logLocked("[Windows node] automatic restart limit reached; server remains offline")
		return
	}
	if s.recovery != nil {
		s.recovery.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	attempt := &recoveryAttempt{cancel: cancel, next: now.Add(time.Duration(policy.DelaySeconds) * time.Second)}
	s.recovery = attempt
	s.logLocked(fmt.Sprintf("[Windows node] unexpected exit; automatic restart scheduled in %d seconds", policy.DelaySeconds))
	go s.recoverProcess(ctx, attempt)
}

func (s *Server) recoverProcess(ctx context.Context, attempt *recoveryAttempt) {
	defer func() {
		attempt.cancel()
		s.mu.Lock()
		if s.recovery == attempt {
			s.recovery = nil
		}
		s.mu.Unlock()
	}()
	timer := time.NewTimer(time.Until(attempt.next))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}
	// Bound power-lock contention independently of the normal startup timeout.
	wait, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	poll := time.NewTicker(10 * time.Millisecond)
	defer poll.Stop()
	for {
		s.mu.Lock()
		if wait.Err() != nil || s.recovery != attempt || s.recoveryStopped || s.closing || s.settings.Suspended || s.process != nil {
			s.mu.Unlock()
			if wait.Err() != nil && ctx.Err() == nil {
				s.log("[Windows node] automatic restart timed out; manual start required")
			}
			return
		}
		if !s.power.TryLock() {
			s.mu.Unlock()
			select {
			case <-wait.Done():
				if ctx.Err() == nil {
					s.log("[Windows node] automatic restart timed out waiting for power action; manual start required")
				}
				return
			case <-poll.C:
				continue
			}
		}
		s.pruneRecoveryLocked(time.Now())
		if len(s.recoveryHistory) >= s.Profile.CrashRecovery.MaxRestarts {
			s.power.Unlock()
			s.mu.Unlock()
			return
		}
		s.recoveryHistory = append(s.recoveryHistory, time.Now())
		s.logLocked("[Windows node] attempting automatic restart")
		s.mu.Unlock()
		err := s.startWithContext(wait)
		s.power.Unlock()
		if err != nil && ctx.Err() == nil {
			s.log("[Windows node] automatic restart failed; manual start required: " + err.Error())
		}
		return
	}
}
