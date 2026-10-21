package node

import (
	"context"
	"errors"
	"sort"
)

// setDesired serializes disk commits outside the state lock. A stale clean-exit
// write cannot overwrite a later explicit action. Kill cancels/kills first.
func (s *Server) setDesired(ctx context.Context, running bool, expected *uint64) error {
	s.desiredWrite.Lock()
	defer s.desiredWrite.Unlock()
	s.mu.Lock()
	if s.desiredStore == nil {
		s.mu.Unlock()
		return nil
	}
	if expected != nil && (*expected != s.desiredGeneration) {
		s.mu.Unlock()
		return nil
	}
	if err := ctx.Err(); err != nil {
		s.mu.Unlock()
		return err
	}
	s.desiredGeneration++
	s.desiredRunning = running
	store := s.desiredStore
	s.mu.Unlock()
	err := store.Save(running)
	if err != nil {
		s.mu.Lock()
		s.desiredStorageError = "intent_write_failed"
		s.mu.Unlock()
		return errors.New("desired intent could not be persisted; inspect storage before daemon restart")
	}
	return ctx.Err()
}
func (s *Server) clearDesiredAfterExit(generation uint64) {
	if err := s.setDesired(context.Background(), false, &generation); err != nil {
		s.log("[Windows node] clean-exit intent could not be persisted; inspect storage")
	}
}

// Called once only after listener binding/readiness, never by diagnostics or
// mark-installed. Explicit actions accepted before this call suppress replay.
func (m *Manager) RestoreRunning(ctx context.Context) {
	m.restoreOnce.Do(func() {
		servers := m.All()
		sort.Slice(servers, func(i, j int) bool { return servers[i].ID() < servers[j].ID() })
		for _, s := range servers {
			s.mu.Lock()
			if ctx.Err() == nil && s.Profile.RestoreRunning && s.desiredRestored && s.desiredRunning && s.desiredStore != nil && s.recoveryStore != nil && !s.startupSuppressed && !s.closing && !s.settings.Suspended && s.process == nil {
				s.logLocked("[Windows node] restoring persisted running intent through bounded recovery")
				s.scheduleRecoveryLockedWithGate(m.restoreGate, ctx)
			}
			s.mu.Unlock()
		}
	})
}
func (s *Server) restoreStatusLocked() map[string]any {
	status := map[string]any{"enabled": s.Profile.RestoreRunning, "desired_running": s.desiredRunning, "loaded_running": s.desiredRestored, "pending": s.recovery != nil && s.recovery.gate != nil}
	if s.desiredStorageError != "" {
		status["storage_error"] = s.desiredStorageError
	}
	return status
}
