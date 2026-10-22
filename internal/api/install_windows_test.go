package api

import (
	"os/exec"
	"path/filepath"
	"testing"
	"time"
	"wings-windows/internal/config"
	"wings-windows/internal/node"
)

func TestApprovedInstallHTTPContract(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "fixture.exe")
	if out, err := exec.Command("go", "build", "-o", exe, "../../cmd/testserver").CombinedOutput(); err != nil {
		t.Fatalf("fixture build: %v %s", err, out)
	}
	for _, action := range []string{"install", "reinstall"} {
		t.Run(action, func(t *testing.T) {
			a, _ := setup(t)
			route := "/api/servers/" + serverID + "/" + action
			if w := request(t, a, "POST", route, "{}", ""); w.Code != 401 {
				t.Fatal(w.Code)
			}
			if w := request(t, a, "POST", route, "{}", secret); w.Code != 501 {
				t.Fatal(w.Code)
			}
			server := a.Manager.Get(serverID)
			server.Profile.Executable = exe
			server.Profile.Installer = &config.Installer{Executable: exe, Arguments: []string{"install", "hang"}, TimeoutSeconds: 5, MemoryMB: 128}
			// Request fields are never executable approval or argument overrides.
			if w := request(t, a, "POST", route, `{"executable":"unapproved.exe","arguments":["arbitrary"]}`, secret); w.Code != 202 {
				t.Fatal(w.Code, w.Body)
			}
			if w := request(t, a, "POST", route, "{}", secret); w.Code != 409 {
				t.Fatal(w.Code, w.Body)
			}
			if w := request(t, a, "POST", "/api/servers/"+serverID+"/power", `{"action":"start"}`, secret); w.Code != 409 {
				t.Fatal(w.Code, w.Body)
			}
			if w := request(t, a, "POST", "/api/servers/"+serverID+"/power", `{"action":"kill"}`, secret); w.Code != 202 {
				t.Fatal(w.Code, w.Body)
			}
			until := time.Now().Add(7 * time.Second)
			for server.Response()["native_install"].(map[string]any)["active"] == true {
				if time.Now().After(until) {
					t.Fatal("cancel did not drain")
				}
				time.Sleep(10 * time.Millisecond)
			}
			if server.HasProcess() {
				t.Fatal("installer became game process")
			}
		})
	}
	t.Run("suspended", func(t *testing.T) {
		a, _ := setupWithSettings(t, func(s *node.Settings) { s.Suspended = true })
		a.Manager.Get(serverID).Profile.Installer = &config.Installer{Executable: exe}
		if w := request(t, a, "POST", "/api/servers/"+serverID+"/install", "{}", secret); w.Code != 400 {
			t.Fatal(w.Code, w.Body)
		}
	})
}
