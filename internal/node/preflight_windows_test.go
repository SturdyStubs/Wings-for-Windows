package node

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStartRechecksDeclaredRequirements(t *testing.T) {
	_, s, _ := setup(t)
	dependency := filepath.Join(t.TempDir(), "runtime.dll")
	s.Profile.Requirements.RequiredFiles = []string{dependency}
	if err := s.start(); err == nil || !strings.Contains(err.Error(), "required file") {
		t.Fatal("missing dependency was accepted", err)
	}
	if s.State() != "offline" || s.HasProcess() {
		t.Fatal("failed preflight launched a game")
	}
	if err := os.WriteFile(dependency, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.start(); err != nil {
		t.Fatal(err)
	}
	await(t, s, "running")
	if err := s.stop(true); err != nil {
		t.Fatal(err)
	}
	await(t, s, "offline")
	if err := os.Remove(dependency); err != nil {
		t.Fatal(err)
	}
	if err := s.start(); err == nil {
		t.Fatal("requirements were not rechecked on next start")
	}
}
