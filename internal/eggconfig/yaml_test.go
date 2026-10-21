package eggconfig

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func yamlPlan(t *testing.T, parser, rules string) File {
	t.Helper()
	plans, err := Compile([]json.RawMessage{json.RawMessage(`{"file":"server.yml","parser":"` + parser + `","replace":` + rules + `}`)}, []string{"server.yml"})
	if err != nil {
		t.Fatal(err)
	}
	return plans[0]
}
func TestYAMLPathsConditionsTypesAndPrecision(t *testing.T) {
	plan := yamlPlan(t, "yaml", `[{"match":"network.port","replace_with":"28015"},{"match":"network.enabled","replace_with":true},{"match":"worlds.*.mode","if_value":"old","replace_with":"new"},{"match":"name","replace_with":"Café 😀"},{"match":"new.items[0].enabled","replace_with":true},{"match":"absent","if_value":"old","replace_with":"added"}]`)
	input := []byte("# discarded\r\nnetwork:\r\n  port: 1\r\n  host: localhost\r\nworlds: [{mode: old}, {mode: stay}]\r\nprecise: 9007199254740993\r\nhuge: 1844674407370955161600\r\nfraction: 1.2300e+3\r\nboolean: false\r\nnothing: null\r\nname: old\r\nmultiline: |\r\n  first\r\n  second\r\n")
	output, err := Render(context.Background(), input, plan)
	if err != nil {
		t.Fatal(err)
	}
	root, err := readYAML(context.Background(), output)
	if err != nil {
		t.Fatal(err)
	}
	network := root["network"].(map[string]any)
	worlds := root["worlds"].([]any)
	if network["port"] != json.Number("28015") || network["enabled"] != true || network["host"] != "localhost" || worlds[0].(map[string]any)["mode"] != "new" || worlds[1].(map[string]any)["mode"] != "stay" || root["precise"] != json.Number("9007199254740993") || root["huge"] != json.Number("1844674407370955161600") || root["fraction"] != json.Number("1.2300e+3") || root["boolean"] != false || root["nothing"] != nil || root["name"] != "Café 😀" || root["absent"] != "added" || root["multiline"] != "first\nsecond\n" {
		t.Fatal(root)
	}
	created := root["new"].(map[string]any)["items"].([]any)[0].(map[string]any)
	if created["enabled"] != true {
		t.Fatal(created)
	}
	again, err := Render(context.Background(), output, plan)
	if err != nil || string(again) != string(output) {
		t.Fatal("unstable YAML", err)
	}
}
func TestYMLLiteralStringsAndQuotedAmbiguity(t *testing.T) {
	t.Setenv("YAML_SECRET", "never-copy-this")
	plan := yamlPlan(t, "yml", `[{"match":"added","replace_with":"${YAML_SECRET}"},{"match":"numeric","replace_with":"01"}]`)
	output, err := Render(context.Background(), []byte("literal: ${YAML_SECRET}\nyes: yes\nquoted: \"true\"\ndate: \"2026-10-21\"\npath: 'C:\\Games\\'\ncycle: '%(cycle)s'\n"), plan)
	if err != nil {
		t.Fatal(err)
	}
	root, err := readYAML(context.Background(), output)
	if err != nil {
		t.Fatal(err)
	}
	// JSON-compatible replacements coerce integral string spellings to integers.
	if root["added"] != "${YAML_SECRET}" || root["literal"] != "${YAML_SECRET}" || root["yes"] != "yes" || root["quoted"] != "true" || root["date"] != "2026-10-21" || root["path"] != "C:\\Games\\" || root["cycle"] != "%(cycle)s" || root["numeric"] != json.Number("1") || strings.Contains(string(output), "never-copy-this") {
		t.Fatal(root)
	}
}
func TestYAMLRejectsUnsupportedInput(t *testing.T) {
	cases := map[string][]byte{
		"bad-null-tag": []byte("a: !!null arbitrary"), "bad-int-tag": []byte("a: !!int 1.2"), "bad-bool-tag": []byte("a: !!bool yes"),
		"malformed": []byte("a: ["), "duplicate": []byte("a: 1\na: 2"), "nested-duplicate": []byte("a: {b: 1, b: 2}"), "scalar-root": []byte("text"), "array-root": []byte("[1,2]"), "empty": {},
		"anchor": []byte("a: &x 1"), "alias": []byte("a: &x [1]\nb: *x"), "merge": []byte("a: {<<: {b: 1}}"), "custom": []byte("a: !custom text"), "timestamp": []byte("a: 2026-10-21"), "binary": []byte("a: !!binary aGk="), "octal": []byte("a: 0o77"), "hex": []byte("a: 0xff"), "infinity": []byte("a: .inf"), "nan": []byte("a: .nan"), "numeric-key": []byte("1: value"), "complex-key": []byte("? [one, two]\n: value"),
		"multiple": []byte("a: 1\n---\nb: 2"), "trailing-invalid": []byte("a: 1\n---\n["), "nul": []byte("a: \x00"), "escaped-nul": []byte("a: \"\\0\""), "bom": {239, 187, 191}, "bad-utf8": {255},
		"depth": []byte("a: " + strings.Repeat("[", 65) + "1" + strings.Repeat("]", 65)), "nodes": []byte("a: [" + strings.Repeat("1,", 100000) + "1]"), "key-size": []byte(strings.Repeat("k", 8193) + ": value"), "size": []byte(strings.Repeat("x", MaxFileBytes+1)),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Render(context.Background(), input, yamlPlan(t, "yaml", `[]`)); err == nil {
				t.Fatal("unsupported YAML accepted")
			}
		})
	}
}
func TestYAMLOutputBoundsAndCancellation(t *testing.T) {
	var nested any = make([]any, 4000)
	for i := range nested.([]any) {
		nested.([]any)[i] = strings.Repeat("v", 200)
	}
	for i := 0; i < 40; i++ {
		nested = map[string]any{"a": nested}
	}
	input, _ := json.Marshal(nested)
	if len(input) > MaxFileBytes {
		t.Fatal("bad output-bound fixture")
	}
	if _, err := Render(context.Background(), input, yamlPlan(t, "yaml", `[]`)); err == nil || !strings.Contains(err.Error(), "output exceeds") {
		t.Fatal("output bound missing", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Render(ctx, []byte("a: 1"), yamlPlan(t, "yaml", `[]`)); err != context.Canceled {
		t.Fatal(err)
	}
}

func TestYAMLRejectsUnsafeRules(t *testing.T) {
	for name, rules := range map[string]string{
		"template": `[{"match":"a","replace_with":"{{config.x}}"}]`,
		"regex":    `[{"match":"a","if_value":"regex:.*","replace_with":"x"}]`,
		"path":     `[{"match":"a[1024]","replace_with":"x"}]`,
		"object":   `[{"match":"a","replace_with":{}}]`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Compile([]json.RawMessage{json.RawMessage(`{"file":"server.yml","parser":"yml","replace":` + rules + `}`)}, []string{"server.yml"})
			if err == nil {
				t.Fatal("unsupported rule accepted")
			}
		})
	}
}
