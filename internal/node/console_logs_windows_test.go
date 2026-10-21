package node

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
	"time"
	"wings-windows/internal/config"
)

func TestPersistentConsoleRestoredAfterManagerRestart(t *testing.T) {
	old, _, _ := setup(t)
	old.Close()
	c := old.Config
	c.ConsoleLogs = &config.ConsoleLogs{Directory: filepath.Join(t.TempDir(), "console"), MaxFileBytes: 1 << 20, MaxFiles: 2}
	m := NewManager(c)
	if err := m.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	s := m.Get(testID)
	if err := s.start(); err != nil {
		t.Fatal(err)
	}
	await(t, s, "running")
	if err := s.Command("persistent-marker"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for !slices.Contains(s.Logs(100), "ECHO:persistent-marker") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	m.Close()
	m = NewManager(c)
	if err := m.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	s = m.Get(testID)
	if !slices.Contains(s.Logs(100), "ECHO:persistent-marker") || s.State() != "offline" || s.HasProcess() {
		t.Fatal("history was not restored independently of game state")
	}
	if s.consoleStore.Status().Restored == 0 {
		t.Fatal("restored history missing from status")
	}
}
