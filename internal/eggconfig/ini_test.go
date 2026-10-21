package eggconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func iniPlan(t *testing.T, rules string) File {
	t.Helper()
	plans, err := Compile([]json.RawMessage{json.RawMessage(`{"file":"server.ini","parser":"ini","replace":` + rules + `}`)}, []string{"server.ini"})
	if err != nil {
		t.Fatal(err)
	}
	return plans[0]
}
func TestINIUpstreamPathScalarsAndExactSections(t *testing.T) {
	plan := iniPlan(t, `[{"match":"port","replace_with":28015},{"match":"network.enabled","replace_with":true},{"match":"[/Script.Game.Settings].Server.Name","value":"Café 😀"},{"match":"new.key","replace_with":null},{"match":"[parent.child].shared","replace_with":"child-only"},{"match":"dup","replace_with":"new"},{"match":"network.inner.key","replace_with":"dot-key"}]`)
	input := "; comment\r\nport:1\r\ndup=first\r\ndup=last\r\n[network]\r\nhost=127.0.0.1\r\n[network]\r\nkeep=yes\r\n[parent]\r\nshared=parent\r\n[parent.child]\r\nother=stay\r\n"
	output, err := Render(context.Background(), []byte(input), plan)
	if err != nil {
		t.Fatal(err)
	}
	expected := "port=28015\ndup=new\n[network]\nhost=127.0.0.1\nkeep=yes\nenabled=true\ninner.key=dot-key\n[parent]\nshared=parent\n[parent.child]\nother=stay\nshared=child-only\n[/Script.Game.Settings]\nServer.Name=Café 😀\n[new]\nkey=<nil>\n"
	if string(output) != expected {
		t.Fatal(string(output))
	}
	again, err := Render(context.Background(), output, plan)
	if err != nil || string(again) != expected {
		t.Fatal("unstable output", err, string(again))
	}
}
func TestINILiteralInterpolationQuotesAndWindowsPaths(t *testing.T) {
	t.Setenv("INI_SECRET", "never-serialize-this")
	plan := iniPlan(t, "[{\"match\":\"added\",\"replace_with\":\"${INI_SECRET} %(other)s\"},{\"match\":\"quoted\",\"replace_with\":\"  a;#b  \"},{\"match\":\"backtick\",\"replace_with\":\"a`b\"},{\"match\":\"empty\",\"replace_with\":\"\"}]")
	input := "literal=${INI_SECRET}\ncycle=%(cycle)s\nother=value\npath=C:\\Games\\\nquotes=`\"quoted\"`\n"
	output, err := Render(context.Background(), []byte(input), plan)
	if err != nil {
		t.Fatal(err)
	}
	sections, err := loadINI(context.Background(), output)
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{}
	for _, pair := range sections[0].pairs {
		values[pair.key] = pair.value
	}
	if values["literal"] != "${INI_SECRET}" || values["cycle"] != "%(cycle)s" || values["added"] != "${INI_SECRET} %(other)s" || values["path"] != "C:\\Games\\" || values["quoted"] != "  a;#b  " || values["backtick"] != "a`b" || values["quotes"] != `"quoted"` || values["empty"] != "" || strings.Contains(string(output), "never-serialize-this") {
		t.Fatal(values)
	}
}
func TestINIRejectsUnsafeRules(t *testing.T) {
	cases := []string{
		`{"match":"[bad.key","replace_with":"x"}`, `{"match":"[].key","replace_with":"x"}`, `{"match":"[a].","replace_with":"x"}`, `{"match":"a.","replace_with":"x"}`,
		`{"match":"a\n[b]","replace_with":"x"}`, `{"match":"-","replace_with":"x"}`, `{"match":"a=b","replace_with":"x"}`, `{"match":"a","replace_with":"x\n[b]"}`,
		`{"match":"a","replace_with":"x","if_value":"old"}`, `{"match":"a","replace_with":{}}`, `{"match":"a","replace_with":"{{config.x}}"}`, `{"match":"a","replace_with":"x","extra":true}`, `{"match":"a"}`,
	}
	for i, rule := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			_, err := Compile([]json.RawMessage{json.RawMessage(`{"file":"server.ini","parser":"ini","replace":[` + rule + `]}`)}, []string{"server.ini"})
			if err == nil {
				t.Fatal("unsafe rule accepted", rule)
			}
		})
	}
}
func TestINIEncodingShapeAndBounds(t *testing.T) {
	sections := strings.Builder{}
	for i := 0; i < 512; i++ {
		fmt.Fprintf(&sections, "[s%d]\na=1\n", i)
	}
	keys := strings.Builder{}
	for i := 0; i < 4097; i++ {
		fmt.Fprintf(&keys, "k%d=v\n", i)
	}
	cases := map[string][]byte{
		"syntax": []byte("[missing"), "boolean": []byte("no-value"), "key": []byte("`quoted`=x\n"), "multiline": []byte("a=\"\"\"first\nsecond\"\"\"\n"), "auto": []byte("-=auto\n"), "bad-utf8": {255}, "bom": {239, 187, 191}, "nul": []byte("a=\x00"),
		"input": []byte(strings.Repeat("x", MaxFileBytes+1)), "lines": []byte(strings.Repeat("\n", 8193)), "sections": []byte(sections.String()), "keys": []byte(keys.String()), "value": []byte("a=" + strings.Repeat("v", (64<<10)+1)), "long-key": []byte(strings.Repeat("k", 8193) + "=x"),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Render(context.Background(), input, iniPlan(t, `[]`)); err == nil {
				t.Fatal("invalid input accepted")
			}
		})
	}
}
func TestINIOutputGrowthAndCancellation(t *testing.T) {
	input := strings.Builder{}
	for i := 0; i < 16; i++ {
		fmt.Fprintf(&input, "k%d=%s\n", i, strings.Repeat("v", 64000))
	}
	rules := []iniRule{}
	for i := 0; i < 4; i++ {
		rules = append(rules, iniRule{"DEFAULT", fmt.Sprintf("added%d", i), strings.Repeat("x", 8192)})
	}
	if _, err := renderINI(context.Background(), []byte(input.String()), rules); err == nil || !strings.Contains(err.Error(), "output exceeds") {
		t.Fatal("output bound not enforced", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Render(ctx, []byte("a=1"), iniPlan(t, `[]`)); err != context.Canceled {
		t.Fatal(err)
	}
}

func TestINIResultStructuralBounds(t *testing.T) {
	for _, kind := range []string{"keys", "sections"} {
		t.Run(kind, func(t *testing.T) {
			var input strings.Builder
			rule := iniRule{"DEFAULT", "added", "1"}
			if kind == "keys" {
				for i := 0; i < 4096; i++ {
					fmt.Fprintf(&input, "k%d=1\n", i)
				}
			} else {
				for i := 0; i < 511; i++ {
					fmt.Fprintf(&input, "[s%d]\na=1\n", i)
				}
				rule.section = "new-section"
			}
			if _, err := renderINI(context.Background(), []byte(input.String()), []iniRule{rule}); err == nil || !strings.Contains(err.Error(), "result") {
				t.Fatal("result structural bound missing", err)
			}
		})
	}
}
