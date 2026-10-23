package api

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPanelConfigurationOperatorPolicy(t *testing.T) {
	a, _ := setup(t)
	a.Manager.Config.IgnorePanelConfigUpdates = true
	before, err := json.Marshal(a.Manager.Config)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "private.json")
	original := []byte("operator configuration sentinel")
	if err = os.WriteFile(file, original, 0600); err != nil {
		t.Fatal(err)
	}
	a.Manager.Config.SourcePath = file
	raw, err := os.ReadFile("testdata/upstream-configuration-update.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Method, Path string
		Request      json.RawMessage
		Response     map[string]bool
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	w := request(t, a, fixture.Method, fixture.Path, string(fixture.Request), secret)
	if w.Code != 200 {
		t.Fatalf("status %d", w.Code)
	}
	var got map[string]bool
	if err = json.Unmarshal(w.Body.Bytes(), &got); err != nil || len(got) != 1 || got["applied"] || fixture.Response["applied"] {
		t.Fatalf("response %s", w.Body)
	}
	a.Manager.Config.SourcePath = ""
	after, _ := json.Marshal(a.Manager.Config)
	if !bytes.Equal(before, after) {
		t.Fatal("operator config changed")
	}
	stored, err := os.ReadFile(file)
	if err != nil || !bytes.Equal(stored, original) {
		t.Fatal("disk config changed")
	}
	if a.Manager.Panel.Config.Token != secret || a.Manager.Panel.Config.TokenID != "node" {
		t.Fatal("outgoing credentials changed")
	}
	if strings.Contains(w.Body.String(), "replacement") {
		t.Fatal("credential echoed")
	}
	if request(t, a, "GET", "/api/system", "", secret).Code != 200 || request(t, a, "GET", "/api/system", "", "replacement-secret-at-least-32-characters").Code != 401 {
		t.Fatal("incoming credentials changed")
	}
}

func TestPanelConfigurationValidation(t *testing.T) {
	a, _ := setup(t)
	a.Manager.Config.IgnorePanelConfigUpdates = true
	for _, tc := range []struct {
		name, method, body, token string
		status                    int
	}{
		{"empty object", "POST", "{}", secret, 200},
		{"missing body", "POST", "", secret, 400},
		{"null", "POST", "null", secret, 400},
		{"array", "POST", "[]", secret, 400},
		{"string", "POST", `"secret"`, secret, 400},
		{"malformed", "POST", "{", secret, 400},
		{"trailing object", "POST", "{} {}", secret, 400},
		{"oversized", "POST", `{"large":"` + strings.Repeat("x", 65536) + `"}`, secret, 413},
		{"anonymous", "POST", "{}", "", 401},
		{"bad bearer", "POST", "{}", "wrong", 401},
		{"wrong method", "PUT", "{}", secret, 501},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := request(t, a, tc.method, "/api/update", tc.body, tc.token)
			if w.Code != tc.status {
				t.Fatalf("status %d expected %d", w.Code, tc.status)
			}
		})
	}
}

func TestPanelConfigurationDisabled(t *testing.T) {
	a, _ := setup(t)
	if request(t, a, "POST", "/api/update", "{}", secret).Code != 501 {
		t.Fatal("remote mode must be explicit")
	}
	if request(t, a, "POST", "/api/update", "{}", "").Code != 401 {
		t.Fatal("authentication bypass")
	}
}
