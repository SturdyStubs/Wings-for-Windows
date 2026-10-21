package eggconfig

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func jsonPlan(t *testing.T, rules string) File {
	t.Helper()
	plans, err := Compile([]json.RawMessage{json.RawMessage(`{"file":"server.json","parser":"json","replace":` + rules + `}`)}, []string{"server.json"})
	if err != nil {
		t.Fatal(err)
	}
	return plans[0]
}
func jsonResult(t *testing.T, input, rules string) map[string]any {
	t.Helper()
	output, err := Render(context.Background(), []byte(input), jsonPlan(t, rules))
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(output))
	decoder.UseNumber()
	var result map[string]any
	if err := decoder.Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result
}
func TestJSONScalarAndConditionContract(t *testing.T) {
	result := jsonResult(t, `{"keep":9007199254740993,"name":"Café","port":1,"flag":false,"stay":"old"}`, `[
 {"match":"port","replace_with":"28015"},
 {"match":"flag","replace_with":true},
 {"match":"float","replace_with":1.5},
 {"match":"null","replace_with":null},
 {"match":"strbool","replace_with":"true"},
 {"match":"stay","if_value":"different","replace_with":"new"},
 {"match":"missing","if_value":"old","value":"new"}]`)
	if result["port"] != json.Number("28015") || result["keep"] != json.Number("9007199254740993") || result["flag"] != true || result["float"] != "1.5" || result["null"] != "<nil>" || result["strbool"] != "true" || result["name"] != "Café" || result["stay"] != "old" || result["missing"] != "new" {
		t.Fatal(result)
	}
}
func TestJSONObjectArraysAndWildcards(t *testing.T) {
	result := jsonResult(t, `{"worlds":[{"port":1},{"port":2}],"groups":{"a":{"enabled":false},"b":{"enabled":true}},"rows":[{"v":"old"},{"v":"stay"}]}`, `[
 {"match":"worlds[1].port","replace_with":28016},
 {"match":"nested.new.key","replace_with":"value"},
 {"match":"created[0].name","replace_with":"new"},
 {"match":"groups.*.enabled","replace_with":true},
 {"match":"rows.*.v","if_value":"old","replace_with":"new"},
 {"match":"absent.*.value","replace_with":"ignored"}]`)
	worlds := result["worlds"].([]any)
	if worlds[0].(map[string]any)["port"] != json.Number("1") || worlds[1].(map[string]any)["port"] != json.Number("28016") {
		t.Fatal(worlds)
	}
	if result["nested"].(map[string]any)["new"].(map[string]any)["key"] != "value" || result["created"].([]any)[0].(map[string]any)["name"] != "new" {
		t.Fatal(result)
	}
	if result["groups"].(map[string]any)["a"].(map[string]any)["enabled"] != true || result["rows"].([]any)[1].(map[string]any)["v"] != "stay" {
		t.Fatal(result)
	}
	if _, ok := result["absent"]; ok {
		t.Fatal("wildcard created missing parent")
	}
}
func TestJSONPlanRejectsUnsupportedPathsAndRules(t *testing.T) {
	for _, match := range []string{"", "a..b", "a.*.b.*.c", "*.key", "a.*", "a[01]", "a[-1]", "a[1024]", "a[0][1]", "a]", "a.*bad"} {
		t.Run(match, func(t *testing.T) {
			raw, _ := json.Marshal(map[string]any{"file": "server.json", "parser": "json", "replace": []map[string]any{{"match": match, "replace_with": "new"}}})
			if _, err := Compile([]json.RawMessage{raw}, []string{"server.json"}); err == nil {
				t.Fatal("unsupported JSON path accepted")
			}
		})
	}
	for _, rule := range []string{`{"match":"a","replace_with":{}}`, `{"match":"a","if_value":"regex:^old","replace_with":"new"}`, `{"match":"a","replace_with":"{{config.docker.interface}}"}`, `{"match":"a"}`, `{"match":"a","replace_with":false,"unknown":true}`} {
		if _, err := Compile([]json.RawMessage{json.RawMessage(`{"file":"server.json","parser":"json","replace":[` + rule + `]}`)}, []string{"server.json"}); err == nil {
			t.Fatal("unsupported JSON rule accepted")
		}
	}
}
func TestJSONInputBoundsAndShapes(t *testing.T) {
	for _, name := range []string{"duplicate", "nested-duplicate", "array-root", "trailing", "utf8", "bom", "depth", "nodes", "shape", "array-index", "output", "wildcard-growth", "wildcard-count", "cancel"} {
		t.Run(name, func(t *testing.T) {
			input := `{"key":"old"}`
			rules := `[{"match":"key","replace_with":"new"}]`
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch name {
			case "duplicate":
				input = `{"key":1,"key":2}`
			case "nested-duplicate":
				input = `{"a":{"key":1,"key":2}}`
			case "array-root":
				input = `[]`
			case "trailing":
				input += ` {}`
			case "utf8":
				input = string([]byte{0xff})
			case "bom":
				input = "\xef\xbb\xbf" + input
			case "depth":
				input = strings.Repeat(`{"a":`, 65) + `0` + strings.Repeat(`}`, 65)
			case "nodes":
				input = `{"items":[` + strings.TrimSuffix(strings.Repeat(`0,`, 100001), ",") + `]}`
			case "shape":
				input = `{"key":1}`
				rules = `[{"match":"key.child","replace_with":"new"}]`
			case "array-index":
				input = `{"a":[1]}`
				rules = `[{"match":"a[1]","replace_with":"new"}]`
			case "output":
				input = `{"nested":{"items":[` + strings.TrimSuffix(strings.Repeat(`0,`, 90000), ",") + `]}}`
			case "wildcard-growth", "wildcard-count":
				count := 4096
				if name == "wildcard-count" {
					count = 4097
				}
				var b strings.Builder
				b.WriteString(`{"items":{`)
				for i := range count {
					if i > 0 {
						b.WriteByte(',')
					}
					fmt.Fprintf(&b, `"%d":{}`, i)
				}
				b.WriteString(`}}`)
				input = b.String()
				value := "new"
				if name == "wildcard-growth" {
					value = strings.Repeat("v", 8192)
				}
				encoded, _ := json.Marshal(value)
				rules = `[{"match":"items.*.value","replace_with":` + string(encoded) + `}]`
			case "cancel":
				cancel()
			}
			if _, err := Render(ctx, []byte(input), jsonPlan(t, rules)); err == nil {
				t.Fatal("unsafe JSON input accepted")
			}
		})
	}
}
