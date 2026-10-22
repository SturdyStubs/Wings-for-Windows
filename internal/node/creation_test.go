package node

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"wings-windows/internal/config"
)

const creationID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"

func creationManager(t *testing.T, mode string) *Manager {
	t.Helper()
	panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test."+strings.Repeat("s", 32) {
			t.Error("missing node assignment credentials")
			w.WriteHeader(401)
			return
		}
		if r.Method != "GET" || r.URL.Path != "/api/remote/servers" {
			t.Error("creation performed an unexpected remote action", r.Method, r.URL.Path)
			w.WriteHeader(500)
			return
		}
		if mode == "panel-error" {
			w.WriteHeader(503)
			return
		}
		remote := RemoteServer{UUID: creationID, Settings: Settings{UUID: creationID, Invocation: "unapproved.exe", Environment: map[string]any{"UNTRUSTED": "not-execution-approval"}}}
		if mode == "bad-remote" {
			remote.UUID = testID
		}
		list := []RemoteServer{remote}
		if mode == "unassigned" {
			list = nil
		}
		json.NewEncoder(w).Encode(map[string]any{"data": list})
	}))
	t.Cleanup(panel.Close)
	m := NewManager(config.Config{AllowPendingCreation: true, PanelURL: panel.URL, TokenID: "test", Token: strings.Repeat("s", 32), DataDirectory: filepath.Join(t.TempDir(), "not-created"), Profiles: map[string]config.Profile{}})
	t.Cleanup(m.Close)
	return m
}

func TestCreationApprovalGatesAndNoExecution(t *testing.T) {
	for _, mode := range []string{"valid", "disabled", "automatic-start", "invalid", "not-v4", "unassigned", "bad-remote", "panel-error", "closed", "capacity", "approved"} {
		t.Run(mode, func(t *testing.T) {
			m := creationManager(t, mode)
			id, start := creationID, false
			var want error
			switch mode {
			case "disabled":
				m.Config.AllowPendingCreation = false
				want = ErrCreationDisabled
			case "automatic-start":
				start = true
				want = ErrCreationStart
			case "invalid":
				id = "../escape"
				want = ErrCreationUUID
			case "not-v4":
				id = "aaaaaaaa-aaaa-1aaa-8aaa-aaaaaaaaaaaa"
				want = ErrCreationUUID
			case "unassigned":
				want = ErrCreationUnassigned
			case "bad-remote":
				want = ErrCreationUUID
			case "closed":
				m.Close()
				want = ErrManagerClosed
			case "capacity":
				for i := 0; i < maxPendingCreation; i++ {
					m.pending[strings.Repeat("x", i+1)] = RemoteServer{}
				}
				want = ErrCreationCapacity
			case "approved":
				m.Config.Profiles[id] = config.Profile{}
				want = ErrCreationConflict
			}
			err := m.CreatePending(context.Background(), id, start)
			if mode == "panel-error" {
				if err == nil {
					t.Fatal("panel error accepted")
				}
			} else if !errors.Is(err, want) {
				t.Fatal(err, want)
			}
			if _, err := os.Stat(m.Config.DataDirectory); !os.IsNotExist(err) {
				t.Fatal("creation touched data", err)
			}
			if len(m.All()) != 0 {
				t.Fatal("unapproved execution object registered")
			}
			if mode == "valid" {
				if err := m.CreatePending(context.Background(), strings.ToUpper(id), false); err != nil {
					t.Fatal("pending replay not idempotent", err)
				}
				response, ok := m.Pending(id)
				if !ok || response["native_creation"].(map[string]any)["execution_approved"] != false || len(m.PendingResponses()) != 1 {
					t.Fatal(response)
				}
			}
		})
	}
}

func TestPendingDiscoveryRestartRemovalAndLocalApproval(t *testing.T) {
	m := creationManager(t, "valid")
	if err := m.CreatePending(context.Background(), creationID, false); err != nil {
		t.Fatal(err)
	}
	restarted := NewManager(m.Config)
	defer restarted.Close()
	if err := restarted.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := restarted.Pending(creationID); !ok || len(restarted.All()) != 0 {
		t.Fatal("pending assignment not reconstructed")
	}
	if _, err := os.Stat(m.Config.DataDirectory); !os.IsNotExist(err) {
		t.Fatal("discovery created data", err)
	}
	// Config is immutable in production: simulate an operator restart with a new
	// UUID-specific local profile rather than accepting code from the request.
	c := m.Config
	c.Profiles = map[string]config.Profile{creationID: {Executable: filepath.Join(t.TempDir(), "approved.exe"), ReadyPattern: "READY", StopCommand: "stop"}}
	approved := NewManager(c)
	defer approved.Close()
	if err := approved.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if approved.Get(creationID) == nil {
		t.Fatal("approved UUID not registered")
	}
	if _, ok := approved.Pending(creationID); ok {
		t.Fatal("approved UUID still pending")
	}
	removed := creationManager(t, "unassigned")
	restarted.Panel = removed.Panel
	if err := restarted.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := restarted.Pending(creationID); ok {
		t.Fatal("removed assignment retained")
	}
}

func TestConcurrentPendingCreation(t *testing.T) {
	m := creationManager(t, "valid")
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := m.CreatePending(context.Background(), creationID, false); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if len(m.PendingResponses()) != 1 || len(m.All()) != 0 {
		t.Fatal("duplicate/unapproved server registered")
	}
}
