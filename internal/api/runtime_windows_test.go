package api

import (
	"github.com/gorilla/websocket"
	"net/http"
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
