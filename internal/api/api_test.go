package api

import (
	"context"
	"encoding/json"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"wings-windows/internal/config"
	"wings-windows/internal/node"
)

const serverID = "11111111-1111-4111-8111-111111111111"
const userID = "22222222-2222-4222-8222-222222222222"
const secret = "local-testing-token-at-least-32-characters"

func setup(t *testing.T) (*API, *httptest.Server) {
	t.Helper()
	panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer node."+secret {
			t.Error("incorrect panel bearer authentication")
			w.WriteHeader(401)
			return
		}
		settings := node.Settings{UUID: serverID, Environment: map[string]any{"PORT": "28015"}}
		if strings.HasSuffix(r.URL.Path, "/servers") {
			json.NewEncoder(w).Encode(map[string]any{"data": []node.RemoteServer{{UUID: serverID, Settings: settings}}, "meta": map[string]int{"last_page": 1}})
		} else {
			json.NewEncoder(w).Encode(map[string]any{"settings": settings})
		}
	}))
	t.Cleanup(panel.Close)
	c := config.Config{PanelURL: panel.URL, TokenID: "node", Token: secret, DataDirectory: t.TempDir(), Profiles: map[string]config.Profile{serverID: {Executable: filepath.Join(t.TempDir(), "game.exe"), ReadyPattern: "READY", StopTimeoutSeconds: 1}}}
	m := node.NewManager(c)
	if err := m.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	a := New(m)
	s := httptest.NewServer(a)
	t.Cleanup(func() { a.Close(); s.Close(); m.Close() })
	return a, s
}
func request(t *testing.T, a *API, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}
func TestBearerAuthenticationAndUnsupportedOperations(t *testing.T) {
	a, _ := setup(t)
	for _, token := range []string{"", "wrong", "node." + secret} {
		if got := request(t, a, "GET", "/api/system", "", token).Code; got != 401 {
			t.Fatalf("bad bearer status %d", got)
		}
	}
	if got := request(t, a, "GET", "/api/system", "", secret).Code; got != 200 {
		t.Fatal(got)
	}
	if got := request(t, a, "POST", "/api/servers/"+serverID+"/reinstall", "{}", secret).Code; got != 501 {
		t.Fatal(got)
	}
	if got := request(t, a, "POST", "/api/servers/"+serverID+"/power", `{"action":"execute"}`, secret).Code; got != 422 {
		t.Fatal(got)
	}
}
func TestWindowsPathRejections(t *testing.T) {
	for _, name := range []string{"../secret", "/../secret", "C:/secret", `dir\secret`, "file.txt:stream", "NUL", "con.txt", "COM1.log", "LPT9", "dir./file", "dir /file", "a//b"} {
		if _, err := filePath(name); err == nil {
			t.Errorf("accepted %q", name)
		}
	}
	for _, name := range []string{"/", "/server/config.json", "logs/latest.log"} {
		if _, err := filePath(name); err != nil {
			t.Errorf("rejected %q", name)
		}
	}
}
func TestConfinedFileReadWrite(t *testing.T) {
	a, _ := setup(t)
	base := "/api/servers/" + serverID + "/files/"
	if w := request(t, a, "POST", base+"write?file=/test.cfg", "server config", secret); w.Code != 204 {
		t.Fatal(w.Code, w.Body)
	}
	w := request(t, a, "GET", base+"contents?file=/test.cfg", "", secret)
	if w.Code != 200 || w.Body.String() != "server config" {
		t.Fatal(w.Code, w.Body)
	}
	if w = request(t, a, "GET", base+"list?directory=/", "", secret); w.Code != 200 || !strings.Contains(w.Body.String(), "test.cfg") {
		t.Fatal(w.Code, w.Body)
	}
	if w = request(t, a, "POST", base+"write?file=../escape.txt", "bad", secret); w.Code != 422 {
		t.Fatal(w.Code)
	}
}
func TestSymlinkEscapeRejected(t *testing.T) {
	a, _ := setup(t)
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("private"), 0600)
	if err := createTestLink(outside, filepath.Join(a.Manager.Get(serverID).Root, "escape")); err != nil {
		t.Fatalf("Cannot create filesystem escape fixture: %v", err)
	}
	if w := request(t, a, "GET", "/api/servers/"+serverID+"/files/contents?file=escape/secret.txt", "", secret); w.Code == 200 {
		t.Fatal("symlink escaped server root")
	}
	if w := request(t, a, "POST", "/api/servers/"+serverID+"/files/write?file=escape/secret.txt", "changed", secret); w.Code == 204 {
		t.Fatal("junction write escaped server root")
	}
	if b, err := os.ReadFile(filepath.Join(outside, "secret.txt")); err != nil || string(b) != "private" {
		t.Fatal("outside file changed", err)
	}
}
func signed(t *testing.T, a *API, mutate func(*Claims)) string {
	t.Helper()
	c := &Claims{RegisteredClaims: jwt.RegisteredClaims{Issuer: a.Manager.Config.PanelURL, IssuedAt: jwt.NewNumericDate(time.Now()), ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)), ID: "test-session"}, Server: serverID, User: userID, Scope: "websocket", Permissions: []string{"websocket.connect"}}
	if mutate != nil {
		mutate(c)
	}
	value, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return value
}
func TestWebsocketJWTValidation(t *testing.T) {
	a, _ := setup(t)
	for _, test := range []struct {
		name   string
		change func(*Claims)
	}{{"expired", func(c *Claims) { c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Second)) }}, {"cross-server", func(c *Claims) { c.Server = userID }}, {"missing permission", func(c *Claims) { c.Permissions = nil }}, {"wrong scope", func(c *Claims) { c.Scope = "file-upload" }}, {"missing issued-at", func(c *Claims) { c.IssuedAt = nil }}, {"preboot", func(c *Claims) { c.IssuedAt = jwt.NewNumericDate(time.Now().Add(-time.Hour)) }}, {"wrong issuer", func(c *Claims) { c.Issuer = "https://evil.invalid" }}, {"future issued-at", func(c *Claims) { c.IssuedAt = jwt.NewNumericDate(time.Now().Add(time.Hour)) }}} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := a.parse(signed(t, a, test.change), serverID); err == nil {
				t.Fatal("invalid claims accepted")
			}
		})
	}
	token := signed(t, a, nil)
	if _, err := a.parse(token, serverID); err != nil {
		t.Fatal(err)
	}
	w := request(t, a, "POST", "/api/deauthorize-user", `{"user":"`+userID+`","servers":[]}`, secret)
	if w.Code != 204 {
		t.Fatal(w.Code)
	}
	if _, err := a.parse(token, serverID); err == nil {
		t.Fatal("revoked token accepted")
	}
}
func TestWebsocketConsolePermissionAndOrigin(t *testing.T) {
	a, s := setup(t)
	endpoint := "ws" + strings.TrimPrefix(s.URL, "http") + "/api/servers/" + serverID + "/ws"
	headers := http.Header{"Origin": []string{"https://evil.invalid"}}
	if conn, _, err := websocket.DefaultDialer.Dial(endpoint, headers); err == nil {
		conn.Close()
		t.Fatal("cross-origin websocket accepted")
	}
	headers.Set("Origin", a.Manager.Config.PanelURL)
	conn, _, err := websocket.DefaultDialer.Dial(endpoint, headers)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	conn.WriteJSON(node.Event{Event: "auth", Args: []string{signed(t, a, nil)}})
	var event node.Event
	if err = conn.ReadJSON(&event); err != nil || event.Event != "auth success" {
		t.Fatal(event, err)
	}
	conn.ReadJSON(&event)
	conn.WriteJSON(node.Event{Event: "send command", Args: []string{"stop"}})
	if err = conn.ReadJSON(&event); err != nil || event.Event != "daemon error" {
		t.Fatal("permission bypass", event, err)
	}
	request(t, a, "POST", "/api/deauthorize-user", `{"user":"`+userID+`"}`, secret)
	for {
		if err = conn.ReadJSON(&event); err != nil {
			break
		}
		if event.Event == "token expired" {
			return
		}
	}
	t.Fatal("websocket did not report revocation")
}
