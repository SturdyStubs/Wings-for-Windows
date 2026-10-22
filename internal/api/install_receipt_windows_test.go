package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"wings-windows/internal/config"
	"wings-windows/internal/consolelog"
	"wings-windows/internal/node"
)

func TestInstallReconciliationHTTPContract(t *testing.T) {
	a, _ := setup(t)
	route := "/api/servers/" + serverID + "/install/reconcile"
	body := `{"attempt_id":"` + serverID + `","action":"notify"}`
	if w := request(t, a, "POST", route, body, ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w := request(t, a, "POST", route, `{`, secret); w.Code != 400 {
		t.Fatal(w.Code)
	}
	if w := request(t, a, "POST", route, `{"attempt_id":"bad","action":"notify"}`, secret); w.Code != 422 {
		t.Fatal(w.Code)
	}
	if w := request(t, a, "POST", route, strings.Replace(body, "notify", "report_success", 1), secret); w.Code != 422 {
		t.Fatal(w.Code)
	}
	if w := request(t, a, "POST", route, body, secret); w.Code != 501 {
		t.Fatal(w.Code)
	}

	var calls atomic.Int32
	panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var flags struct {
				Successful bool `json:"successful"`
				Reinstall  bool `json:"reinstall"`
			}
			if json.NewDecoder(r.Body).Decode(&flags) != nil || !flags.Successful || !flags.Reinstall {
				t.Error("lost recorded callback flags")
			}
			calls.Add(1)
			w.WriteHeader(204)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"data": []node.RemoteServer{{UUID: serverID, Settings: node.Settings{UUID: serverID}}}})
	}))
	defer panel.Close()
	options := config.ConsoleLogs{Directory: filepath.Join(t.TempDir(), "private"), MaxFileBytes: 1 << 20, MaxFiles: 2}
	store, _, err := consolelog.OpenInstallState(options, serverID)
	if err != nil {
		t.Fatal(err)
	}
	record := consolelog.InstallRecord{Phase: "running", AttemptID: serverID, Reinstall: true}
	if err := store.Save(record); err != nil {
		t.Fatal(err)
	}
	record.Phase = "result"
	record.Successful = true
	if err := store.Save(record); err != nil {
		t.Fatal(err)
	}
	store.Close(context.Background())
	c := config.Config{PanelURL: panel.URL, Token: secret, DataDirectory: t.TempDir(), ConsoleLogs: &options, Profiles: map[string]config.Profile{serverID: {Executable: filepath.Join(t.TempDir(), "never-executed.exe"), ReadyPattern: "READY", Installer: &config.Installer{PersistState: true}}}}
	m := node.NewManager(c)
	if err := m.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	api := New(m)
	defer api.Close()
	if w := request(t, api, "POST", route, strings.Replace(body, serverID, userID, 1), secret); w.Code != 409 {
		t.Fatal(w.Code, w.Body)
	}
	if w := request(t, api, "POST", route, body, secret); w.Code != 204 {
		t.Fatal(w.Code, w.Body)
	}
	if w := request(t, api, "POST", route, body, secret); w.Code != 409 {
		t.Fatal(w.Code, w.Body)
	}
	if calls.Load() != 1 || m.Get(serverID).HasProcess() {
		t.Fatal("duplicate notification or execution")
	}
}
