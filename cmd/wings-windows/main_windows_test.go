package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
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

func TestRuntimeReadyOnlyAfterConfigurationSyncTLSAndBind(t *testing.T) {
	for _, stage := range []string{"configuration", "panel_sync", "tls", "listener", "ready", "tls_ready"} {
		t.Run(stage, func(t *testing.T) {
			panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if stage == "panel_sync" {
					w.WriteHeader(503)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"data":[]}`))
			}))
			defer panel.Close()
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			address := ln.Addr().String()
			defer ln.Close()
			if stage != "listener" {
				ln.Close()
			}
			directory := t.TempDir()
			executable, _ := os.Executable()
			c := config.Config{Listen: address, PanelURL: panel.URL, NodeUUID: "11111111-1111-4111-8111-111111111111", TokenID: "test", Token: strings.Repeat("s", 32), DataDirectory: filepath.Join(directory, "data"), AllowInsecureLoopback: true, Profiles: map[string]config.Profile{"11111111-1111-4111-8111-111111111111": {Executable: executable, ReadyPattern: "READY"}}}
			if stage == "tls" {
				c.TLS = config.TLS{Cert: filepath.Join(directory, "missing.crt"), Key: filepath.Join(directory, "missing.key")}
			}
			if stage == "tls_ready" {
				fixture := httptest.NewTLSServer(http.NotFoundHandler())
				defer fixture.Close()
				key, err := x509.MarshalPKCS8PrivateKey(fixture.TLS.Certificates[0].PrivateKey)
				if err != nil {
					t.Fatal(err)
				}
				c.TLS = config.TLS{Cert: filepath.Join(directory, "fixture.crt"), Key: filepath.Join(directory, "fixture.key")}
				if err := os.WriteFile(c.TLS.Cert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: fixture.Certificate().Raw}), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(c.TLS.Key, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}), 0600); err != nil {
					t.Fatal(err)
				}
			}
			body, _ := json.Marshal(c)
			if stage == "configuration" {
				body = []byte(`{"token":"invalid`)
			}
			path := filepath.Join(directory, "config.json")
			if err := os.WriteFile(path, body, 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			called := false
			err = runWithReady(ctx, path, "", func() {
				called = true
				connection, e := net.DialTimeout("tcp", address, time.Second)
				if e != nil {
					t.Error("ready without a listening socket", e)
				} else {
					connection.Close()
				}
				if stage == "tls_ready" {
					transport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} // disposable self-signed fixture only
					defer transport.CloseIdleConnections()
					client := &http.Client{Transport: transport, Timeout: time.Second}
					response, err := client.Get("https://" + address + "/api/system")
					if err != nil {
						t.Error(err)
					} else {
						response.Body.Close()
						if response.StatusCode != 401 {
							t.Error("unexpected HTTPS API status", response.StatusCode)
						}
					}
				}
				cancel()
			})
			if stage == "ready" || stage == "tls_ready" {
				if err != nil || !called {
					t.Fatal("ready run", called, err)
				}
				return
			}
			var failure *runtimeFailure
			if called || !errors.As(err, &failure) || failure.stage != stage {
				t.Fatalf("ready=%v error=%v", called, err)
			}
		})
	}
}
