package node

import (
	"context"
	"time"
	"wings-windows/internal/logtail"
	"wings-windows/internal/native"
)

func (s *Server) readLogFile(p *native.Process, reader *logtail.Reader) {
	defer s.outputWG.Done()
	defer reader.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-p.Done:
			cancel()
		case <-ctx.Done():
		}
	}()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	defer func() {
		s.mu.Lock()
		if s.process == p {
			s.logFileActive = false
		}
		s.mu.Unlock()
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		lines, err := reader.Poll(ctx)
		s.mu.Lock()
		if s.process != p || s.closing || ctx.Err() != nil {
			s.mu.Unlock()
			return
		}
		if err != nil || s.filePolicy.Denied(s.Profile.LogFile.Path, false) {
			s.logFileActive = false
			s.logFileError = "read_failed"
			s.logLocked("[Windows node] log-file adapter stopped; inspect approved path, permissions and text limits")
			s.mu.Unlock()
			return
		}
		for _, line := range lines {
			s.logLocked("[Game log] " + line)
			if s.Profile.LogFile.Readiness && s.state == "starting" && s.processPolicy.ready(line) {
				s.setStateLocked("running")
			}
		}
		s.mu.Unlock()
	}
}
