package node

import (
	"encoding/json"
	"testing"
	"wings-windows/internal/config"
)

func processFixture(t *testing.T) *ProcessConfiguration {
	t.Helper()
	var p ProcessConfiguration
	if err := json.Unmarshal([]byte(`{"startup":{"done":["literal.ready","regex:^Ready [0-9]+$","regex:[invalid"],"strip_ansi":true,"user_interaction":[]},"stop":{"type":"command","value":"stop"},"configs":[]}`), &p); err != nil {
		t.Fatal(err)
	}
	return &p
}
func TestEggReadinessAndStopPolicy(t *testing.T) {
	remote := processFixture(t)
	policy, err := newProcessPolicy(config.Profile{UseEggProcessConfiguration: true}, remote)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		line  string
		ready bool
	}{{"Boot literal.ready now", true}, {"literalXready", false}, {"Ready 123", true}, {"Ready no", false}, {"[invalid", true}, {"\x1b[32mReady 456\x1b[0m", true}} {
		if got := policy.ready(test.line); got != test.ready {
			t.Errorf("%q: ready=%v", test.line, got)
		}
	}
	if policy.stopCommand != "stop" || policy.forceStop {
		t.Fatal("wrong egg command stop")
	}
	remote.Stop.Type = "signal"
	remote.Stop.Value = "SIGKILL"
	policy, err = newProcessPolicy(config.Profile{UseEggProcessConfiguration: true}, remote)
	if err != nil || !policy.forceStop {
		t.Fatal("SIGKILL mapping", err)
	}
}
func TestUnsupportedEggStrategiesFailExplicitly(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*ProcessConfiguration)
	}{{"Linux SIGTERM", func(p *ProcessConfiguration) { p.Stop.Type = "signal"; p.Stop.Value = "SIGTERM" }}, {"Docker native stop", func(p *ProcessConfiguration) { p.Stop.Type = "stop" }}, {"rewrites", func(p *ProcessConfiguration) {
		p.Configs = []json.RawMessage{json.RawMessage(`{"file":"server.cfg","parser":"file"}`)}
	}}, {"no readiness", func(p *ProcessConfiguration) { p.Startup.Done = nil }}, {"multiline command", func(p *ProcessConfiguration) { p.Stop.Value = "stop\nother" }}} {
		t.Run(test.name, func(t *testing.T) {
			remote := processFixture(t)
			test.change(remote)
			if _, err := newProcessPolicy(config.Profile{UseEggProcessConfiguration: true}, remote); err == nil {
				t.Fatal("unsupported egg silently accepted")
			}
		})
	}
	if _, err := newProcessPolicy(config.Profile{UseEggProcessConfiguration: true}, nil); err == nil {
		t.Fatal("missing process configuration accepted")
	}
}
func TestLocalPolicyRetainsLocalAuthority(t *testing.T) {
	remote := processFixture(t)
	remote.Stop.Type = "signal"
	remote.Stop.Value = "SIGTERM"
	policy, err := newProcessPolicy(config.Profile{ReadyPattern: "^LOCAL_READY$", StopCommand: "quit"}, remote)
	if err != nil || policy.stopCommand != "quit" || !policy.ready("LOCAL_READY") || policy.ready("Ready 123") {
		t.Fatal("local policy changed", err)
	}
}
