package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"wings-windows/internal/config"
)

func TestCORSPreflightAndExactOrigins(t *testing.T) {
	a, _ := setup(t)
	a.Manager.Config.Origins = []string{"https://other-panel.example"}
	for _, origin := range []string{a.Manager.Config.PanelURL, "https://other-panel.example", "https://untrusted.example"} {
		r := httptest.NewRequest("OPTIONS", "/upload/file", nil)
		r.Header.Set("Origin", origin)
		r.Header.Set("Access-Control-Request-Method", "POST")
		r.Header.Set("Access-Control-Request-Headers", "content-type")
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code != 204 {
			t.Fatal("preflight requires credentials", w.Code)
		}
		allowed := w.Header().Get("Access-Control-Allow-Origin")
		if origin == "https://untrusted.example" {
			if allowed == origin || allowed == "*" {
				t.Fatal("untrusted origin reflected")
			}
		} else if allowed != origin {
			t.Fatal("allowed origin rejected", allowed)
		}
		if w.Header().Get("Access-Control-Allow-Credentials") != "true" || w.Header().Get("Vary") != "Origin" {
			t.Fatal(w.Header())
		}
		if w.Header().Get("Access-Control-Allow-Private-Network") != "" {
			t.Fatal("private network access was enabled implicitly")
		}
	}
	a.Manager.Config.AllowCORSPrivateNetwork = true
	r := httptest.NewRequest("OPTIONS", "/upload/file", nil)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Header().Get("Access-Control-Allow-Private-Network") != "true" {
		t.Fatal("explicit private-network configuration ignored")
	}
	if request(t, a, "POST", "/api/servers/"+serverID+"/power", `{"action":"start"}`, "").Code != 401 {
		t.Fatal("preflight bypassed actual route authentication")
	}
}
func TestRequestIDsAndMalformedBodyContract(t *testing.T) {
	a, _ := setup(t)
	seen := map[string]bool{}
	for _, test := range []struct {
		body   string
		status int
	}{{`{`, 400}, {`{} {}`, 400}, {`{"action":"bogus"}`, 422}, {`{"action":"start","padding":"` + strings.Repeat("x", 70*1024) + `"}`, 413}} {
		w := request(t, a, "POST", "/api/servers/"+serverID+"/power", test.body, secret)
		if w.Code != test.status {
			t.Fatalf("expected %d, got %d: %s", test.status, w.Code, w.Body)
		}
		id := w.Header().Get("X-Request-Id")
		if !config.UUID.MatchString(id) || seen[id] {
			t.Fatal("missing/nonunique request ID", id)
		}
		seen[id] = true
		var body map[string]any
		json.Unmarshal(w.Body.Bytes(), &body)
		if body["request_id"] != id || body["error"] == nil {
			t.Fatal("uncorrelated error", body)
		}
	}
	w := request(t, a, "GET", "/api/system", "", "")
	if w.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatal("missing bearer challenge")
	}
	r := httptest.NewRequest("GET", "/api/system", nil)
	r.Header.Set("X-Request-Id", "untrusted-client-id")
	r.Header.Set("Authorization", "Bearer "+secret)
	out := httptest.NewRecorder()
	a.ServeHTTP(out, r)
	if !config.UUID.MatchString(out.Header().Get("X-Request-Id")) {
		t.Fatal("trusted a client-provided request ID")
	}
}
func TestSystemV2MatchesUpstreamShapeAndNativeReality(t *testing.T) {
	a, _ := setup(t)
	fixture, err := os.ReadFile("testdata/upstream-system-v2.json")
	if err != nil {
		t.Fatal(err)
	}
	var expected, actual map[string]any
	json.Unmarshal(fixture, &expected)
	w := request(t, a, "GET", "/api/system?v=2", "", secret)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if err = json.Unmarshal(w.Body.Bytes(), &actual); err != nil {
		t.Fatal(err)
	}
	var compare func(map[string]any, map[string]any, string)
	compare = func(want, got map[string]any, prefix string) {
		for key, value := range want {
			received, ok := got[key]
			if !ok || reflect.TypeOf(received) != reflect.TypeOf(value) {
				t.Errorf("system contract mismatch %s%s: %#v", prefix, key, received)
				continue
			}
			if object, ok := value.(map[string]any); ok {
				compare(object, received.(map[string]any), prefix+key+".")
			}
		}
	}
	compare(expected, actual, "")
	system := actual["system"].(map[string]any)
	if system["os_type"] != "windows" || system["memory_bytes"].(float64) <= 0 || system["cpu_threads"].(float64) <= 0 || system["kernel_version"] == "Windows NT" {
		t.Fatal("host information was not measured", system)
	}
	docker := actual["docker"].(map[string]any)
	if docker["containers"].(map[string]any)["total"] != float64(0) {
		t.Fatal("native processes reported as Docker containers")
	}
	native := actual["native"].(map[string]any)
	counts := native["servers"].(map[string]any)
	if counts["total"] != float64(1) || counts["stopped"] != float64(1) || native["container_isolation"] != false {
		t.Fatal("incorrect native capability reporting", native)
	}
	legacy := request(t, a, "GET", "/api/system", "", secret)
	var old map[string]any
	json.Unmarshal(legacy.Body.Bytes(), &old)
	if old["os"] != "windows" || old["cpu_count"] != system["cpu_threads"] {
		t.Fatal("legacy system mismatch", old)
	}
}
func TestStoppedCommandsReturnBadGateway(t *testing.T) {
	a, _ := setup(t)
	for _, body := range []string{`{"commands":[]}`, `{"commands":["say hi"]}`} {
		w := request(t, a, "POST", "/api/servers/"+serverID+"/commands", body, secret)
		if w.Code != http.StatusBadGateway {
			t.Fatal("offline command route", w.Code)
		}
	}
}
