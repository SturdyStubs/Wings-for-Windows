package eggconfig

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestUpstreamTextPrefixContract(t *testing.T) {
	raw := json.RawMessage(`{"file":"server.cfg","parser":"file","replace":[{"match":"server.port","replace_with":"server.port 28015"},{"match":"server.name","value":"server.name \"Café\""},{"match":"123","replace_with":true}]}`)
	plans, err := Compile([]json.RawMessage{raw}, []string{"server.cfg"})
	if err != nil {
		t.Fatal(err)
	}
	output, err := Render(context.Background(), []byte("# keep\r\nserver.port 1\r\nserver.name old\r\n123old\r\nother=keep"), plans[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != "# keep\nserver.port 28015\nserver.name \"Café\"\ntrue\nother=keep\n" {
		t.Fatal(string(output))
	}
	// Upstream processes every matching rule in order, without separators.
	output, err = Render(context.Background(), []byte("prefix"), File{Replace: []Replacement{{"pre", "A"}, {"prefix", "B"}}})
	if err != nil || string(output) != "AB\n" {
		t.Fatal(string(output), err)
	}
}
func TestInvalidEggTextPlans(t *testing.T) {
	cases := []string{
		`{"file":"SERVER.cfg","parser":"file","replace":[]}`,
		`{"file":"../outside.cfg","parser":"file","replace":[]}`,
		`{"file":"other.cfg","parser":"file","replace":[]}`,
		`{"file":"server.cfg","parser":"toml","replace":[]}`,
		`{"file":"server.cfg","parser":"file"}`,
		`{"file":"server.cfg","parser":"file","replace":[{"match":"","replace_with":"x"}]}`,
		`{"file":"server.cfg","parser":"file","replace":[{"match":"x","replace_with":{}}]}`,
		`{"file":"server.cfg","parser":"file","replace":[{"match":"x"}]}`,
		`{"file":"server.cfg","parser":"file","replace":[{"match":"x","replace_with":"{{config.docker.interface}}"}]}`,
		`{"file":"server.cfg","parser":"file","replace":[{"match":"x","if_value":"old","replace_with":"new"}]}`,
		`{"file":"server.cfg","parser":"file","replace":[{"match":"x","replace_with":"new","extra":true}]}`,
		`{"file":"server.cfg","parser":"file","replace":[],"extra":true}`,
		`{"file":"server.cfg","parser":"file","replace":[]} {}`,
	}
	for i, raw := range cases {
		t.Run(string(rune('A'+i)), func(t *testing.T) {
			if _, err := Compile([]json.RawMessage{json.RawMessage(raw)}, []string{"server.cfg"}); err == nil {
				t.Fatal("accepted unsafe plan")
			}
		})
	}
	raw := json.RawMessage(`{"file":"server.cfg","parser":"file","replace":[]}`)
	if _, err := Compile([]json.RawMessage{raw, raw}, []string{"server.cfg"}); err == nil {
		t.Fatal("duplicate file accepted")
	}
}

func TestEggTextPlanLimits(t *testing.T) {
	valid := json.RawMessage(`{"file":"server.cfg","parser":"file","replace":[]}`)
	files := make([]json.RawMessage, 17)
	for i := range files {
		files[i] = valid
	}
	if _, err := Compile(files, []string{"server.cfg"}); err == nil {
		t.Fatal("file count limit ignored")
	}
	for _, name := range []string{"rules", "value", "match", "total"} {
		t.Run(name, func(t *testing.T) {
			rule := map[string]any{"match": "key=", "replace_with": "key=new"}
			rules := []map[string]any{rule}
			switch name {
			case "rules":
				for range 128 {
					rules = append(rules, rule)
				}
			case "value":
				rule["replace_with"] = strings.Repeat("v", 8193)
			case "match":
				rule["match"] = strings.Repeat("m", 8193)
			case "total":
				rule["replace_with"] = strings.Repeat("v", 129<<10)
			}
			body, _ := json.Marshal(map[string]any{"file": "server.cfg", "parser": "file", "replace": rules})
			if _, err := Compile([]json.RawMessage{body}, []string{"server.cfg"}); err == nil {
				t.Fatal("plan limit ignored")
			}
		})
	}
}
func TestEggTextBoundsEncodingAndCancellation(t *testing.T) {
	for _, name := range []string{"input", "output", "nul", "utf8", "bom", "cancel"} {
		t.Run(name, func(t *testing.T) {
			data := []byte("x\n")
			plan := File{Replace: []Replacement{{"x", strings.Repeat("v", 8192)}}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch name {
			case "input":
				data = []byte(strings.Repeat("x", MaxFileBytes+1))
			case "output":
				data = []byte(strings.Repeat("x\n", 129))
			case "nul":
				data = []byte{0}
			case "utf8":
				data = []byte{0xff}
			case "bom":
				data = []byte{0xef, 0xbb, 0xbf}
			case "cancel":
				cancel()
			}
			if _, err := Render(ctx, data, plan); err == nil {
				t.Fatal("invalid input accepted")
			}
		})
	}
}
