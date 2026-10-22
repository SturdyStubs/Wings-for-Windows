package node

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strings"
	"time"
	"wings-windows/internal/native"
	"wings-windows/internal/provision"
)

var ErrInstallDisabled = errors.New("no native installer is approved for this profile")
var ErrInstallBusy = errors.New("a native installation is in progress")

// EnqueueInstall is independent of HTTP request lifetime and shares the power
// lock. The request can never supply an executable, arguments or script.
func (s *Server) EnqueueInstall(reinstall bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing {
		return errors.New("node is shutting down")
	}
	if s.Profile.Installer == nil {
		return ErrInstallDisabled
	}
	if s.settings.Suspended {
		return ErrSuspended
	}
	if s.installing {
		return ErrInstallBusy
	}
	if s.installStorageError != "" {
		return ErrInstallStorage
	}
	if s.installStore != nil && (s.installRecord.Phase == "running" || s.installRecord.Phase == "result") {
		return ErrInstallUnresolved
	}
	if s.pendingPower > 0 || (!reinstall && s.process != nil) {
		return ErrPowerBusy
	}
	if !s.power.TryLock() {
		return ErrPowerBusy
	}
	s.startupSuppressed = true
	s.cancelRecoveryLocked(true)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.Profile.Installer.TimeoutSeconds)*time.Second)
	s.powerCancel = cancel
	s.installing = true
	s.installStage = "preflight"
	s.installSuccessful = false
	s.installCallbackError = ""
	done := make(chan struct{})
	s.installDone = done
	s.outputWG.Add(1)
	go func() {
		defer s.outputWG.Done()
		defer cancel()
		beginErr := s.beginInstallRecord(reinstall)
		err := beginErr
		if err == nil {
			err = s.runInstall(ctx, reinstall)
		}
		success := err == nil && ctx.Err() == nil
		s.mu.Lock()
		s.installStage = "notifying"
		s.installSuccessful = success
		s.mu.Unlock()
		notifyCtx, notifyCancel := context.WithTimeout(context.Background(), 5*time.Second)
		notifyErr := beginErr
		if notifyErr == nil {
			notifyErr = s.completeInstallRecord(success)
		}
		if notifyErr == nil {
			notifyErr = s.notifyInstallResult(notifyCtx, success, reinstall)
		}
		notifyCancel()
		s.mu.Lock()
		s.installerProcess = nil
		s.installing = false
		s.installDone = nil
		s.powerCancel = nil
		s.installStage = "complete"
		if !success {
			s.installStage = "failed"
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				s.installStage = "timed_out"
			} else if errors.Is(ctx.Err(), context.Canceled) {
				s.installStage = "cancelled"
			}
			s.logLocked("[Windows node] native installation failed; inspect approved recipe and installer output")
		}
		if notifyErr != nil {
			s.installCallbackError = "notification_failed"
			s.logLocked("[Windows node] installation callback failed; verify panel state before retrying")
		}
		if s.installStorageError != "" {
			s.installStage = "result_unconfirmed"
			s.installCallbackError = "state_not_confirmed"
		}
		if s.process == nil {
			s.setStateLocked("offline")
		}
		s.emitLocked("install completed", "")
		s.power.Unlock()
		close(done)
		s.mu.Unlock()
	}()
	return nil
}
func (s *Server) runInstall(ctx context.Context, reinstall bool) error {
	if err := s.setDesired(ctx, false, nil); err != nil {
		return err
	}
	if reinstall {
		if err := s.stopWithContext(ctx, false); err != nil {
			return err
		}
	}
	details, err := s.panel.Details(ctx, s.ID())
	if err != nil {
		return err
	}
	if details.Settings.Suspended {
		return ErrSuspended
	}
	s.mu.Lock()
	s.settings = details.Settings
	s.mu.Unlock()
	// Protected game code is pre-provisioned in this initial data/setup runner.
	if err = provision.Check(ctx, s.Profile); err != nil {
		return err
	}
	if details.Settings.SkipEggScripts {
		return nil
	}
	profile := s.Profile.InstallerProfile()
	if err = provision.Check(ctx, profile); err != nil {
		return err
	}
	spec, err := s.specForProfile(profile)
	if err != nil {
		return err
	}
	spec.Context = ctx
	process, err := native.Start(spec)
	if err != nil {
		return err
	}
	s.mu.Lock()
	if s.closing || ctx.Err() != nil {
		s.mu.Unlock()
		process.Kill()
		<-process.Done
		process.Stdout.Close()
		return context.Canceled
	}
	s.installerProcess = process
	s.installStage = "running"
	s.emitLocked("install started", "")
	s.mu.Unlock()
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		defer process.Stdout.Close()
		scanner := bufio.NewScanner(process.Stdout)
		scanner.Buffer(make([]byte, 4096), 16<<10)
		remaining := 1 << 20
		capped := false
		for scanner.Scan() {
			line := strings.ToValidUTF8(strings.ReplaceAll(scanner.Text(), "\x00", "�"), "�")
			s.mu.Lock()
			if s.installerProcess == process && remaining >= len(line)+1 {
				remaining -= len(line) + 1
				s.logLocked("[Installer] " + line)
				s.emitLocked("install output", line)
			} else if !capped {
				capped = true
				s.logLocked("[Windows node] installer output limit reached; remaining output is drained")
			}
			s.mu.Unlock()
		}
		if scanner.Err() != nil {
			s.log("[Windows node] installer output capture stopped at the line limit; remaining output is drained")
			io.Copy(io.Discard, process.Stdout)
		}
	}()
	select {
	case <-ctx.Done():
		process.Kill()
		<-process.Done
	case <-process.Done:
	}
	<-drained
	s.mu.Lock()
	if s.installerProcess == process {
		s.installerProcess = nil
	}
	s.mu.Unlock()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if process.ExitCode != 0 || process.CleanupError != nil {
		return errors.New("installer process failed")
	}
	return provision.Check(ctx, s.Profile)
}
func (s *Server) installStatusLocked() map[string]any {
	stage := s.installStage
	if stage == "" {
		stage = "idle"
	}
	return map[string]any{"enabled": s.Profile.Installer != nil, "active": s.installing, "stage": stage, "successful": s.installSuccessful, "callback_error": s.installCallbackError, "persistent": s.installStore != nil, "attempt_id": s.installRecord.AttemptID, "receipt_phase": s.installRecord.Phase, "pending_notification": s.installRecord.Phase == "result", "storage_error": s.installStorageError}
}
