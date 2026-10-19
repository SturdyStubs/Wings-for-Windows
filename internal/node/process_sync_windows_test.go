package node

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestEggProcessConfigurationDrivesNativeServer(t *testing.T) {
	_, s, _ := setupConfigured(t, func(remote *RemoteServer) {
		remote.ProcessConfiguration = processFixture(t)
		remote.ProcessConfiguration.Startup.Done = []string{"regex:^SERVER_READY$"}
	})
	s.Profile.UseEggProcessConfiguration = true
	s.Profile.ReadyPattern = "NEVER"
	s.Profile.StopCommand = "wrong-local-command"
	s.Profile.Arguments = []string{"colored"}
	if err := s.start(); err != nil {
		t.Fatal(err)
	}
	await(t, s, "running")
	p := s.process
	if err := s.stop(false); err != nil {
		t.Fatal(err)
	}
	<-p.Done
	if p.ExitCode != 0 {
		t.Fatal("egg stop command was not used", p.ExitCode)
	}
}
func TestLiveSyncChangesJobLimitsWithoutRestart(t *testing.T) {
	memory := &atomic.Int64{}
	cpu := &atomic.Int64{}
	memory.Store(128)
	cpu.Store(100)
	_, s, _ := setupConfigured(t, func(remote *RemoteServer) {
		remote.Settings.Build.Memory = int(memory.Load())
		remote.Settings.Build.CPU = int(cpu.Load())
	})
	s.Profile.MemoryMB = 0
	if err := s.start(); err != nil {
		t.Fatal(err)
	}
	await(t, s, "running")
	original := s.process
	started := s.started
	for _, limits := range [][2]int{{96, 50}, {256, 200}, {0, 0}} {
		memory.Store(int64(limits[0]))
		cpu.Store(int64(limits[1]))
		if err := s.Sync(context.Background()); err != nil {
			t.Fatal(err)
		}
		s.mu.Lock()
		same := s.process == original && s.started.Equal(started)
		s.mu.Unlock()
		if !same {
			t.Fatal("sync restarted the server")
		}
		stats := s.Response()["utilization"].(map[string]any)
		if stats["memory_limit_bytes"] != int64(limits[0])*1024*1024 {
			t.Fatal("reported limit does not match applied limit", stats)
		}
	}
	memory.Store(512)
	cpu.Store(-1)
	if err := s.Sync(context.Background()); err == nil {
		t.Fatal("invalid update accepted")
	}
	if got := s.Response()["utilization"].(map[string]any)["memory_limit_bytes"]; got != int64(0) {
		t.Fatal("failed update changed displayed enforcement", got)
	}
	if s.State() != "running" {
		t.Fatal("invalid update killed server")
	}
}
func TestProfileCapStillAppliesDuringSync(t *testing.T) {
	memory := &atomic.Int64{}
	memory.Store(128)
	_, s, _ := setupConfigured(t, func(remote *RemoteServer) { remote.Settings.Build.Memory = int(memory.Load()) })
	s.Profile.MemoryMB = 96
	if err := s.start(); err != nil {
		t.Fatal(err)
	}
	await(t, s, "running")
	memory.Store(512)
	if err := s.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if limit := s.Response()["utilization"].(map[string]any)["memory_limit_bytes"]; limit != int64(96*1024*1024) {
		t.Fatal("local cap removed", limit)
	}
}
func TestEggReadinessUpdatesWhileStarting(t *testing.T) {
	ready := &atomic.Bool{}
	_, s, _ := setupConfigured(t, func(remote *RemoteServer) {
		remote.ProcessConfiguration = processFixture(t)
		remote.ProcessConfiguration.Startup.Done = []string{"NEVER"}
		if ready.Load() {
			remote.ProcessConfiguration.Startup.Done = []string{"ECHO:complete"}
		}
	})
	s.Profile.UseEggProcessConfiguration = true
	if err := s.start(); err != nil {
		t.Fatal(err)
	}
	if s.State() != "starting" {
		t.Fatal("server became ready too early")
	}
	ready.Store(true)
	if err := s.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.Command("complete"); err != nil {
		t.Fatal(err)
	}
	await(t, s, "running")
}
func TestSuspensionWinsOverInvalidEggConfiguration(t *testing.T) {
	_, s, suspended := setupConfigured(t, func(remote *RemoteServer) {
		remote.ProcessConfiguration = processFixture(t)
		remote.ProcessConfiguration.Startup.Done = []string{"SERVER_READY"}
		if remote.Settings.Suspended {
			remote.ProcessConfiguration.Stop.Type = "signal"
			remote.ProcessConfiguration.Stop.Value = "SIGTERM"
		}
	})
	s.Profile.UseEggProcessConfiguration = true
	if err := s.start(); err != nil {
		t.Fatal(err)
	}
	await(t, s, "running")
	p := s.process
	suspended.Store(true)
	if err := s.Sync(context.Background()); err == nil {
		t.Fatal("unsupported egg configuration accepted")
	}
	select {
	case <-p.Done:
	case <-time.After(3 * time.Second):
		t.Fatal("invalid egg configuration prevented suspension")
	}
	await(t, s, "offline")
}
