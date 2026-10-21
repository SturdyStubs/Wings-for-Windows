package node

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEggTextRewriteBeforeNativeLaunch(t *testing.T) {
	_, s, _ := setupConfigured(t, func(remote *RemoteServer) {
		remote.ProcessConfiguration = processFixture(t)
		remote.ProcessConfiguration.Startup.Done = []string{"regex:^SERVER_READY$"}
		remote.ProcessConfiguration.Configs = []json.RawMessage{json.RawMessage(`{"file":"server.cfg","parser":"file","replace":[{"match":"key=","replace_with":"key=new"}]}`)}
	})
	s.Profile.UseEggProcessConfiguration = true
	s.Profile.EggConfigFiles = []string{"server.cfg"}
	if err := s.start(); err == nil || s.HasProcess() {
		t.Fatal("launched without required existing config")
	}
	file := filepath.Join(s.Root, "server.cfg")
	os.WriteFile(file, []byte("key=old\n"), 0600)
	if err := s.start(); err != nil {
		t.Fatal(err)
	}
	await(t, s, "running")
	data, _ := os.ReadFile(file)
	if string(data) != "key=new\n" {
		t.Fatal("rewrite missing before running", string(data))
	}
}

func TestEggPropertiesRejectsMalformedThenLaunches(t *testing.T) {
	_, s, _ := setupConfigured(t, func(remote *RemoteServer) {
		remote.ProcessConfiguration = processFixture(t)
		remote.ProcessConfiguration.Startup.Done = []string{"regex:^SERVER_READY$"}
		remote.ProcessConfiguration.Configs = []json.RawMessage{json.RawMessage(`{"file":"server.properties","parser":"properties","replace":[{"match":"port","replace_with":28015}]}`)}
	})
	s.Profile.UseEggProcessConfiguration = true
	s.Profile.EggConfigFiles = []string{"server.properties"}
	path := filepath.Join(s.Root, "server.properties")
	os.WriteFile(path, []byte(`bad=\u00xz`), 0600)
	if err := s.start(); err == nil || s.HasProcess() {
		t.Fatal("launched with malformed properties")
	}
	os.WriteFile(path, []byte("keep=yes\n"), 0600)
	if err := s.start(); err != nil {
		t.Fatal(err)
	}
	await(t, s, "running")
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "port=28015\n") || !strings.Contains(string(raw), "keep=yes\n") {
		t.Fatal(string(raw))
	}
}

func TestEggJSONBlocksMalformedInputThenLaunches(t *testing.T) {
	_, s, _ := setupConfigured(t, func(remote *RemoteServer) {
		remote.ProcessConfiguration = processFixture(t)
		remote.ProcessConfiguration.Startup.Done = []string{"regex:^SERVER_READY$"}
		remote.ProcessConfiguration.Configs = []json.RawMessage{json.RawMessage(`{"file":"server.json","parser":"json","replace":[{"match":"network.port","replace_with":"28015"}]}`)}
	})
	s.Profile.UseEggProcessConfiguration = true
	s.Profile.EggConfigFiles = []string{"server.json"}
	path := filepath.Join(s.Root, "server.json")
	os.WriteFile(path, []byte(`{"invalid":`), 0600)
	if err := s.start(); err == nil || s.HasProcess() {
		t.Fatal("launched with malformed JSON")
	}
	os.WriteFile(path, []byte(`{"keep":true}`), 0600)
	if err := s.start(); err != nil {
		t.Fatal(err)
	}
	await(t, s, "running")
	raw, _ := os.ReadFile(path)
	var result struct {
		Keep    bool
		Network struct{ Port int }
	}
	if err := json.Unmarshal(raw, &result); err != nil || !result.Keep || result.Network.Port != 28015 {
		t.Fatal(string(raw), err)
	}
}

func TestEggINIRejectsMalformedThenLaunches(t *testing.T) {
	_, s, _ := setupConfigured(t, func(remote *RemoteServer) {
		remote.ProcessConfiguration = processFixture(t)
		remote.ProcessConfiguration.Startup.Done = []string{"regex:^SERVER_READY$"}
		remote.ProcessConfiguration.Configs = []json.RawMessage{json.RawMessage(`{"file":"server.ini","parser":"ini","replace":[{"match":"port","replace_with":28015}]}`)}
	})
	s.Profile.UseEggProcessConfiguration = true
	s.Profile.EggConfigFiles = []string{"server.ini"}
	path := filepath.Join(s.Root, "server.ini")
	os.WriteFile(path, []byte(`[invalid`), 0600)
	if err := s.start(); err == nil || s.HasProcess() {
		t.Fatal("launched with malformed INI")
	}
	os.WriteFile(path, []byte("keep=yes\n"), 0600)
	if err := s.start(); err != nil {
		t.Fatal(err)
	}
	await(t, s, "running")
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "port=28015\n") || !strings.Contains(string(raw), "keep=yes\n") {
		t.Fatal(string(raw))
	}
}

func TestEggYAMLRejectsMalformedThenLaunches(t *testing.T) {
	_, s, _ := setupConfigured(t, func(remote *RemoteServer) {
		remote.ProcessConfiguration = processFixture(t)
		remote.ProcessConfiguration.Startup.Done = []string{"regex:^SERVER_READY$"}
		remote.ProcessConfiguration.Configs = []json.RawMessage{json.RawMessage(`{"file":"server.yml","parser":"yaml","replace":[{"match":"port","replace_with":28015}]}`)}
	})
	s.Profile.UseEggProcessConfiguration = true
	s.Profile.EggConfigFiles = []string{"server.yml"}
	path := filepath.Join(s.Root, "server.yml")
	os.WriteFile(path, []byte(`bad: [`), 0600)
	if err := s.start(); err == nil || s.HasProcess() {
		t.Fatal("launched with malformed YAML")
	}
	os.WriteFile(path, []byte("keep: yes\n"), 0600)
	if err := s.start(); err != nil {
		t.Fatal(err)
	}
	await(t, s, "running")
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "\"port\": 28015\n") || !strings.Contains(string(raw), "\"keep\": \"yes\"\n") {
		t.Fatal(string(raw))
	}
}

func TestEggXMLRejectsMalformedThenLaunches(t *testing.T) {
	_, s, _ := setupConfigured(t, func(remote *RemoteServer) {
		remote.ProcessConfiguration = processFixture(t)
		remote.ProcessConfiguration.Startup.Done = []string{"regex:^SERVER_READY$"}
		remote.ProcessConfiguration.Configs = []json.RawMessage{json.RawMessage(`{"file":"server.xml","parser":"xml","replace":[{"match":"server.port","replace_with":28015}]}`)}
	})
	s.Profile.UseEggProcessConfiguration = true
	s.Profile.EggConfigFiles = []string{"server.xml"}
	path := filepath.Join(s.Root, "server.xml")
	os.WriteFile(path, []byte(`<invalid>`), 0600)
	if err := s.start(); err == nil || s.HasProcess() {
		t.Fatal("launched with malformed XML")
	}
	os.WriteFile(path, []byte("<server><keep>yes</keep></server>"), 0600)
	if err := s.start(); err != nil {
		t.Fatal(err)
	}
	await(t, s, "running")
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "<port>28015</port>") || !strings.Contains(string(raw), "<keep>yes</keep>") {
		t.Fatal(string(raw))
	}
}
