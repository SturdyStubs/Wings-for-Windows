package api

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"wings-windows/internal/node"
)

func TestNativeControlThroughHTTPAndWebsocket(t *testing.T) {
	a, httpServer := setup(t)
	exe := filepath.Join(t.TempDir(), "fixture.exe")
	if out, err := exec.Command("go", "build", "-o", exe, "../../cmd/testserver").CombinedOutput(); err != nil {
		t.Fatalf("fixture build: %v %s", err, out)
	}
	server := a.Manager.Get(serverID)
	server.Profile.Executable = exe
	server.Profile.StopCommand = "stop"
	endpoint := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/api/servers/" + serverID + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(endpoint, http.Header{"Origin": []string{a.Manager.Config.PanelURL}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	token := signed(t, a, func(c *Claims) { c.Permissions = append(c.Permissions, "control.console") })
	conn.WriteJSON(node.Event{Event: "auth", Args: []string{token}})
	var event node.Event
	if err = conn.ReadJSON(&event); err != nil || event.Event != "auth success" {
		t.Fatal(event, err)
	}
	if w := request(t, a, "POST", "/api/servers/"+serverID+"/power", `{"action":"start"}`, secret); w.Code != 202 {
		t.Fatal(w.Code, w.Body)
	}
	for {
		if err = conn.ReadJSON(&event); err != nil {
			t.Fatal(err)
		}
		if event.Event == "status" && len(event.Args) > 0 && event.Args[0] == "running" {
			break
		}
	}
	if err = conn.WriteJSON(node.Event{Event: "send command", Args: []string{"hello through panel"}}); err != nil {
		t.Fatal(err)
	}
	for {
		if err = conn.ReadJSON(&event); err != nil {
			t.Fatal(err)
		}
		if event.Event == "console output" && len(event.Args) > 0 && event.Args[0] == "ECHO:hello through panel" {
			break
		}
	}
	if w := request(t, a, "POST", "/api/servers/"+serverID+"/power", `{"action":"stop"}`, secret); w.Code != 202 {
		t.Fatal(w.Code, w.Body)
	}
	for {
		if err = conn.ReadJSON(&event); err != nil {
			t.Fatal(err)
		}
		if event.Event == "status" && len(event.Args) > 0 && event.Args[0] == "offline" {
			break
		}
	}
}
func TestListingReportsWindowsCreationTime(t *testing.T) {
	a, _ := setup(t)
	name := filepath.Join(a.Manager.Get(serverID).Root, "old.cfg")
	if err := os.WriteFile(name, []byte("settings"), 0600); err != nil {
		t.Fatal(err)
	}
	modified := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	if err := os.Chtimes(name, modified, modified); err != nil {
		t.Fatal(err)
	}
	w := request(t, a, "GET", "/api/servers/"+serverID+"/files/list-directory?directory=/", "", secret)
	var entries []map[string]any
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &entries) != nil || len(entries) != 1 {
		t.Fatal(w.Code, w.Body)
	}
	created, err := time.Parse(time.RFC3339, entries[0]["created"].(string))
	if err != nil || !created.After(modified) || entries[0]["modified"] != modified.Format(time.RFC3339) {
		t.Fatal("creation time was replaced by modification time", entries)
	}
}
