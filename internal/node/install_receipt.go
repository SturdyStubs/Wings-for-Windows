package node

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
	"wings-windows/internal/consolelog"
)

var (
	ErrInstallUnresolved      = errors.New("resolve the previous persistent installation result before starting another installation")
	ErrInstallStorage         = errors.New("installation state storage failed; inspect private state before continuing")
	ErrInstallPersistDisabled = errors.New("persistent installation state is not enabled for this profile")
	ErrInstallAttempt         = errors.New("installation attempt does not match the pending record")
	ErrInstallNoNotification  = errors.New("no matching installation notification is pending")
	ErrInstallNotification    = errors.New("panel installation notification failed; inspect panel state before retrying")
)

func (s *Server) restoreInstallRecord() {
	r := s.installRecord
	s.installSuccessful = r.Successful
	switch r.Phase {
	case "running":
		s.installStage = "interrupted"
		s.installCallbackError = "outcome_unknown"
	case "result":
		s.installStage = "awaiting_notification"
		s.installCallbackError = "notification_pending"
	case "acknowledged":
		s.installStage = "complete"
		if !r.Successful {
			s.installStage = "failed"
		}
	}
}

func (s *Server) saveInstallRecord(record consolelog.InstallRecord) error {
	if s.installStore == nil {
		return nil
	}
	err := s.installStore.Save(record)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.installStorageError = "write_failed"
		return ErrInstallStorage
	}
	s.installRecord = record
	return nil
}
func (s *Server) beginInstallRecord(reinstall bool) error {
	if s.installStore == nil {
		return nil
	}
	return s.saveInstallRecord(consolelog.InstallRecord{Phase: "running", AttemptID: newInstallAttempt(), Reinstall: reinstall})
}
func (s *Server) completeInstallRecord(success bool) error {
	if s.installStore == nil {
		return nil
	}
	s.mu.Lock()
	record := s.installRecord
	s.mu.Unlock()
	record.Phase = "result"
	record.Successful = success
	return s.saveInstallRecord(record)
}
func (s *Server) notifyInstallResult(ctx context.Context, success, reinstall bool) error {
	if err := s.panel.InstallResult(ctx, s.ID(), success, reinstall); err != nil {
		return ErrInstallNotification
	}
	if s.installStore != nil {
		s.mu.Lock()
		record := s.installRecord
		s.mu.Unlock()
		record.Phase = "acknowledged"
		if err := s.saveInstallRecord(record); err != nil {
			return err
		}
	}
	return nil
}

// ReconcileInstall never executes or repairs an installer. The explicit failed
// resolution is conservative: an interrupted process has no trustworthy result.
func (s *Server) ReconcileInstall(parent context.Context, attempt string, reportFailed bool) error {
	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		return ErrManagerClosed
	}
	if s.installStore == nil {
		s.mu.Unlock()
		return ErrInstallPersistDisabled
	}
	if s.installStorageError != "" {
		s.mu.Unlock()
		return ErrInstallStorage
	}
	if s.installRecord.AttemptID != attempt {
		s.mu.Unlock()
		return ErrInstallAttempt
	}
	record := s.installRecord
	if (!reportFailed && record.Phase != "result") || (reportFailed && record.Phase != "running") {
		s.mu.Unlock()
		return ErrInstallNoNotification
	}
	if s.installing || s.pendingPower > 0 || !s.power.TryLock() {
		s.mu.Unlock()
		return ErrInstallBusy
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	done := make(chan struct{})
	s.installing = true
	s.installStage = "notifying"
	s.installDone = done
	s.powerCancel = cancel
	s.outputWG.Add(1)
	s.mu.Unlock()
	defer s.outputWG.Done()
	defer cancel()
	var err error
	if reportFailed {
		record.Phase = "result"
		record.Successful = false
		err = s.saveInstallRecord(record)
	}
	if err == nil {
		err = s.notifyInstallResult(ctx, record.Successful, record.Reinstall)
	}
	s.mu.Lock()
	s.installSuccessful = record.Successful
	s.installCallbackError = ""
	s.installStage = "complete"
	if !record.Successful {
		s.installStage = "failed"
	}
	if err != nil {
		s.installStage = "awaiting_notification"
		s.installCallbackError = "notification_failed"
	}
	if errors.Is(err, ErrInstallStorage) {
		s.installStage = "result_unconfirmed"
		s.installCallbackError = "state_not_confirmed"
	}
	s.installing = false
	s.installDone = nil
	s.powerCancel = nil
	s.power.Unlock()
	close(done)
	s.mu.Unlock()
	return err
}

func newInstallAttempt() string {
	var b [16]byte
	rand.Read(b[:])
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	v := hex.EncodeToString(b[:])
	return v[:8] + "-" + v[8:12] + "-" + v[12:16] + "-" + v[16:20] + "-" + v[20:]
}
