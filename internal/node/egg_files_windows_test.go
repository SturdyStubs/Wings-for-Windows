package node

import (
	"encoding/json"
	"os"
	"path/filepath"
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
