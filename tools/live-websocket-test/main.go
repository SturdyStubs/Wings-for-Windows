package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"net/http"
	"os"
	"strings"
	"time"
)

type event struct {
	Event string   `json:"event"`
	Args  []string `json:"args"`
}
type cfg struct {
	Token string `json:"token"`
	Panel string `json:"panel_url"`
}
type claims struct {
	jwt.RegisteredClaims
	Server      string   `json:"server_uuid"`
	User        string   `json:"user_uuid"`
	Scope       string   `json:"scope"`
	Permissions []string `json:"permissions"`
}

func nonce() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic("random failed")
	}
	return hex.EncodeToString(b)
}
func check(e error) {
	if e != nil {
		panic("Live websocket check failed (details excluded to protect credentials)")
	}
}
func main() {
	if len(os.Args) != 5 || os.Args[4] != "--allow-synthetic-revocation" {
		fmt.Fprintln(os.Stderr, "arguments: native-config public-HTTPS-base disposable-demo-UUID --allow-synthetic-revocation")
		os.Exit(2)
	}
	b, e := os.ReadFile(os.Args[1])
	check(e)
	var c cfg
	check(json.Unmarshal(b, &c))
	base := strings.TrimRight(os.Args[2], "/")
	id := os.Args[3]
	report := map[string]any{"observed_utc": time.Now().UTC().Format(time.RFC3339), "identity_source": "operator-minted synthetic JWTs; not a real panel subuser", "panel_subuser_removal_tested": false, "checks": map[string]bool{}}
	checks := report["checks"].(map[string]bool)
	defer func() {
		if recover() != nil {
			report["passed"] = false
			json.NewEncoder(os.Stdout).Encode(report)
			os.Exit(1)
		}
	}()
	user := "77777777-7777-4777-8777-" + nonce()[:12]
	mk := func() claims {
		return claims{RegisteredClaims: jwt.RegisteredClaims{Issuer: c.Panel, ID: nonce(), IssuedAt: jwt.NewNumericDate(time.Now()), ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute))}, Server: id, User: user, Scope: "websocket", Permissions: []string{"websocket.connect"}}
	}
	token := func(cl claims) string {
		s, e := jwt.NewWithClaims(jwt.SigningMethodHS256, cl).SignedString([]byte(c.Token))
		check(e)
		return s
	}
	connect := func(origin string) (*websocket.Conn, *http.Response, error) {
		d := websocket.Dialer{HandshakeTimeout: 8 * time.Second}
		return d.Dial("wss"+strings.TrimPrefix(base, "https")+"/api/servers/"+id+"/ws", http.Header{"Origin": []string{origin}})
	}
	readUntil := func(w *websocket.Conn, want string) event {
		check(w.SetReadDeadline(time.Now().Add(5 * time.Second)))
		for i := 0; i < 50; i++ {
			var out event
			check(w.ReadJSON(&out))
			if out.Event == want {
				return out
			}
			if out.Event == "jwt error" && want != "jwt error" {
				panic("auth failed")
			}
		}
		panic("expected event missing")
	}
	auth := func(cl claims) *websocket.Conn {
		w, _, e := connect(c.Panel)
		check(e)
		check(w.WriteJSON(event{"auth", []string{token(cl)}}))
		readUntil(w, "auth success")
		return w
	}
	post := func(path string, payload any) {
		body, e := json.Marshal(payload)
		check(e)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		r, e := http.NewRequestWithContext(ctx, "POST", base+"/api/"+path, bytes.NewReader(body))
		check(e)
		r.Header.Set("Authorization", "Bearer "+c.Token)
		r.Header.Set("Content-Type", "application/json")
		client := http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		resp, e := client.Do(r)
		check(e)
		defer resp.Body.Close()
		if resp.StatusCode != 204 {
			panic("revocation failed")
		}
	}
	bad, resp, e := connect("https://untrusted.invalid")
	if bad != nil {
		bad.Close()
	}
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if e == nil || resp == nil || resp.StatusCode != 403 {
		panic("origin accepted")
	}
	checks["untrusted_origin_rejected"] = true
	rejected := func(cl claims) {
		w, _, e := connect(c.Panel)
		check(e)
		defer w.Close()
		check(w.WriteJSON(event{"auth", []string{token(cl)}}))
		readUntil(w, "jwt error")
	}
	wrong := mk()
	wrong.Issuer = "https://untrusted.invalid"
	rejected(wrong)
	checks["wrong_issuer_rejected"] = true
	wrong = mk()
	wrong.Server = "88888888-8888-4888-8888-888888888888"
	rejected(wrong)
	checks["wrong_server_rejected"] = true
	wrong = mk()
	wrong.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Second))
	rejected(wrong)
	checks["expired_token_rejected"] = true
	cl := mk()
	w := auth(cl)
	checks["allowed_origin_synthetic_auth"] = true
	check(w.WriteJSON(event{"send command", []string{"should-not-execute"}}))
	out := readUntil(w, "daemon error")
	if len(out.Args) != 1 || out.Args[0] != "Console permission required" {
		panic("console authorized")
	}
	checks["missing_console_permission_rejected"] = true
	check(w.WriteJSON(event{"set state", []string{"stop"}}))
	out = readUntil(w, "daemon error")
	if len(out.Args) != 1 || out.Args[0] != "Power permission required" {
		panic("power authorized")
	}
	checks["missing_power_permission_rejected"] = true
	post("servers/"+id+"/ws/deny", map[string]any{"jtis": []string{cl.ID}})
	readUntil(w, "token expired")
	w.Close()
	rejected(cl)
	checks["jti_revocation_and_replay_rejected"] = true
	cl = mk()
	w = auth(cl)
	post("deauthorize-user", map[string]any{"user": user, "servers": []string{id}})
	readUntil(w, "token expired")
	w.Close()
	rejected(cl)
	checks["synthetic_user_revocation_and_replay_rejected"] = true
	time.Sleep(1200 * time.Millisecond)
	w = auth(mk())
	w.Close()
	checks["fresh_post_revocation_token_accepted"] = true
	if len(checks) != 10 {
		check(errors.New("missing checks"))
	}
	report["passed"] = true
	json.NewEncoder(os.Stdout).Encode(report)
}
