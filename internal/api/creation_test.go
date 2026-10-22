package api

import (
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

func TestPanelCreationRecordSurvivesManualApproval(t *testing.T) {
	var retained atomic.Bool
	panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer node."+secret {
			t.Error("bad node credentials")
			w.WriteHeader(401)
			return
		}
		if r.Method != "GET" || r.URL.Path != "/api/remote/servers" {
			t.Error("creation invoked installation or fetched remote script")
			w.WriteHeader(500)
			return
		}
		servers := []node.RemoteServer{}
		if retained.Load() {
			servers = append(servers, node.RemoteServer{UUID: serverID, Settings: node.Settings{UUID: serverID, Invocation: "never-execute.exe"}})
		}
		json.NewEncoder(w).Encode(map[string]any{"data": servers})
	}))
	defer panel.Close()
	root := filepath.Join(t.TempDir(), "absent")
	m := node.NewManager(config.Config{AllowPendingCreation: true, PanelURL: panel.URL, TokenID: "node", Token: secret, DataDirectory: root, Profiles: map[string]config.Profile{}})
	defer m.Close()
	a := New(m)
	defer a.Close()
	// Mirrors reviewed panel CreateServer: persist first, then delete on a
	// failed daemon create. A successful 202 must preserve the persisted record.
	retained.Store(true)
	body, err := os.ReadFile("testdata/upstream-create-server.json")
	if err != nil {
		t.Fatal(err)
	}
	w := request(t, a, "POST", "/api/servers", string(body), secret)
	if w.Code >= 400 {
		retained.Store(false)
	}
	if w.Code != 202 || !retained.Load() {
		t.Fatal(w.Code, w.Body)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("pending create wrote data", err)
	}
	w = request(t, a, "GET", "/api/servers/"+serverID, "", secret)
	var response struct {
		NativeCreation struct {
			Stage    string `json:"stage"`
			Approved bool   `json:"execution_approved"`
		} `json:"native_creation"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &response) != nil || response.NativeCreation.Stage != "awaiting_profile" || response.NativeCreation.Approved {
		t.Fatal(w.Code, w.Body)
	}
	for _, route := range []string{"power", "install", "reinstall", "files/write?file=unsafe.exe"} {
		if w := request(t, a, "POST", "/api/servers/"+serverID+"/"+route, `{"action":"start","executable":"unapproved.exe"}`, secret); w.Code != 409 {
			t.Fatal(route, w.Code, w.Body)
		}
	}
	if w := request(t, a, "POST", "/api/servers", string(body), secret); w.Code != 202 {
		t.Fatal("pending retry", w.Code)
	}
	retained.Store(false)
	if w := request(t, a, "POST", "/api/servers", string(body), secret); w.Code != 404 {
		t.Fatal("stale assignment accepted", w.Code)
	}
}

func TestCreationHTTPValidationAndGating(t *testing.T) {
	a, _ := setup(t)
	for _, tc := range []struct {
		name, body, token string
		status            int
	}{
		{"unauthorized", `{}`, "", 401},
		{"malformed", `{`, secret, 400},
		{"trailing", `{} {}`, secret, 400},
		{"oversized", `{"padding":"` + strings.Repeat("x", 65536) + `"}`, secret, 413},
		{"invalid UUID", `{"uuid":"../escape"}`, secret, 422},
		{"disabled", `{"uuid":"` + serverID + `"}`, secret, 501},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if w := request(t, a, "POST", "/api/servers", tc.body, tc.token); w.Code != tc.status {
				t.Fatal(w.Code, w.Body)
			}
		})
	}
	a.Manager.Config.AllowPendingCreation = true
	if w := request(t, a, "POST", "/api/servers", `{"uuid":"`+serverID+`","start_on_completion":true}`, secret); w.Code != 422 {
		t.Fatal(w.Code, w.Body)
	}
	if w := request(t, a, "POST", "/api/servers", `{"uuid":"`+serverID+`"}`, secret); w.Code != 409 {
		t.Fatal(w.Code, w.Body)
	}
}
