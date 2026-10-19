package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"wings-windows/internal/config"
	"wings-windows/internal/node"
)

func TestInstallationAcknowledgementRequiresPreflight(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"
	var callbacks atomic.Int32
	panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/api/remote/servers/"+id+"/install" {
			callbacks.Add(1)
			w.WriteHeader(204)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"data": []node.RemoteServer{{UUID: id, Settings: node.Settings{UUID: id}}}})
	}))
	defer panel.Close()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	dependency := filepath.Join(directory, "runtime.dll")
	c := config.Config{Listen: "127.0.0.1:8080", PanelURL: panel.URL, NodeUUID: id, TokenID: "test", Token: strings.Repeat("s", 32), DataDirectory: filepath.Join(directory, "data"), AllowInsecureLoopback: true, Profiles: map[string]config.Profile{id: {Executable: exe, ReadyPattern: "READY", Requirements: config.Requirements{RequiredFiles: []string{dependency}}}}}
	path := filepath.Join(directory, "config.json")
	body, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	if err := run(context.Background(), path, id); err == nil || !strings.Contains(err.Error(), "preflight") {
		t.Fatal("unmet requirement accepted", err)
	}
	if callbacks.Load() != 0 {
		t.Fatal("sent success callback despite missing requirement")
	}
	if err := os.WriteFile(dependency, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run(context.Background(), path, id); err != nil {
		t.Fatal(err)
	}
	if callbacks.Load() != 1 {
		t.Fatal("expected one explicit success callback")
	}
}
