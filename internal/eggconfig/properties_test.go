package eggconfig

import (
	"context"
	"encoding/json"
	"github.com/magiconair/properties"
	"strings"
	"testing"
)

func propertiesPlan(t *testing.T, rules string) File {
	t.Helper()
	plans, err := Compile([]json.RawMessage{json.RawMessage(`{"file":"server.properties","parser":"properties","replace":` + rules + `}`)}, []string{"server.properties"})
	if err != nil {
		t.Fatal(err)
	}
	return plans[0]
}
func readPropertyResult(t *testing.T, output []byte) map[string]string {
	t.Helper()
	normalized, err := normalizeSurrogates(context.Background(), output)
	if err != nil {
		t.Fatal(err)
	}
	loader := properties.Loader{Encoding: properties.UTF8, DisableExpansion: true}
	values, err := loader.LoadBytes(normalized)
	if err != nil {
		t.Fatal(err)
	}
	return values.Map()
}
func TestPropertiesContractConditionsAndEscapes(t *testing.T) {
	input := "# initial C:\\users\\name\r\n! keep header\r\n\r\nserver-port: 1\r\nname = old\r\njoined=first\\\r\n   second\r\nescaped\\ key\\=part=old\r\ndup=first\r\n# inline removed\r\ndup=last\r\nmode=stay\r\n"
	plan := propertiesPlan(t, `[{"match":"server-port","replace_with":28015},{"match":"name","replace_with":"Café 😀"},{"match":"escaped key=part","value":"new"},{"match":"mode","if_value":"old","replace_with":"new"},{"match":"absent","if_value":"old","replace_with":"no"},{"match":"enabled","replace_with":true},{"match":"nullable","replace_with":null},{"match":"spaces","replace_with":"  keep  "},{"match":"multiline","replace_with":"a\nb\tc\\d"}]`)
	output, err := Render(context.Background(), []byte(input), plan)
	if err != nil {
		t.Fatal(err)
	}
	values := readPropertyResult(t, output)
	if values["server-port"] != "28015" || values["name"] != "Café 😀" || values["joined"] != "firstsecond" || values["escaped key=part"] != "new" || values["dup"] != "last" || values["mode"] != "stay" || values["enabled"] != "true" || values["nullable"] != "<nil>" || values["spaces"] != "  keep  " || values["multiline"] != "a\nb\tc\\d" {
		t.Fatal(values)
	}
	if _, exists := values["absent"]; exists {
		t.Fatal("conditional properties rule created missing key")
	}
	if !strings.HasPrefix(string(output), "# initial C:\\users\\name\n! keep header\n\n") || strings.Contains(string(output), "inline removed") || !strings.Contains(string(output), `Caf\u00e9 \ud83d\ude00`) {
		t.Fatal(string(output))
	}
	again, err := Render(context.Background(), output, plan)
	if err != nil || string(again) != string(output) {
		t.Fatal("properties rewrite is not stable", err, string(again))
	}
}
func TestPropertiesDoNotExpandDaemonEnvironmentOrCycles(t *testing.T) {
	t.Setenv("WINGS_PROPERTY_SECRET", "daemon-secret-must-not-leak")
	plan := propertiesPlan(t, `[{"match":"added","replace_with":"${WINGS_PROPERTY_SECRET}"},{"match":"cycle","replace_with":"${cycle}"}]`)
	output, err := Render(context.Background(), []byte("literal=${WINGS_PROPERTY_SECRET}\ncycle=${cycle}\n"), plan)
	if err != nil {
		t.Fatal(err)
	}
	values := readPropertyResult(t, output)
	if values["literal"] != "${WINGS_PROPERTY_SECRET}" || values["added"] != "${WINGS_PROPERTY_SECRET}" || values["cycle"] != "${cycle}" || strings.Contains(string(output), "daemon-secret-must-not-leak") {
		t.Fatal(values)
	}
}
func TestPropertiesSurrogateEscapesAndComments(t *testing.T) {
	input := []byte("# literal bad escape \\users\\n\nemoji=\\uD83D\\uDE00\nliteral=\\\\uD800\nseparator\\u003dkey=value\n")
	output, err := Render(context.Background(), input, propertiesPlan(t, `[]`))
	if err != nil {
		t.Fatal(err)
	}
	values := readPropertyResult(t, output)
	if values["emoji"] != "😀" || values["literal"] != `\uD800` || values["separator=key"] != "value" {
		t.Fatal(values)
	}
	// A comment-looking continuation is value data, not a comment.
	output, err = Render(context.Background(), []byte("key=before\\\r\n#\\uD83D\\uDE00\r\n"), propertiesPlan(t, `[]`))
	if err != nil {
		t.Fatal(err)
	}
	if readPropertyResult(t, output)["key"] != "before#😀" {
		t.Fatal(string(output))
	}
	output, err = Render(context.Background(), []byte("split=\\u00\\\r\n e9\r\nlong\\\n key=value\n"), propertiesPlan(t, `[]`))
	if err != nil {
		t.Fatal(err)
	}
	values = readPropertyResult(t, output)
	if values["split"] != "é" || values["longkey"] != "value" {
		t.Fatal(values)
	}
}
func TestPropertiesRejectMalformedAndOversizeInput(t *testing.T) {
	for _, name := range []string{"unicode", "high-surrogate", "low-surrogate", "wrong-pair", "nul", "escaped-nul", "utf8", "bom", "input", "output", "lines", "keys", "value", "cancel"} {
		t.Run(name, func(t *testing.T) {
			input := []byte("key=value\n")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch name {
			case "unicode":
				input = []byte(`key=\u00xz`)
			case "high-surrogate":
				input = []byte(`key=\ud800`)
			case "low-surrogate":
				input = []byte(`key=\udc00`)
			case "wrong-pair":
				input = []byte(`key=\ud800\u0041`)
			case "nul":
				input = []byte{0}
			case "escaped-nul":
				input = []byte(`key=\u0000`)
			case "utf8":
				input = []byte{0xff}
			case "bom":
				input = []byte{0xef, 0xbb, 0xbf}
			case "input":
				input = []byte(strings.Repeat("v", MaxFileBytes+1))
			case "output":
				input = []byte(strings.Repeat("#header\n", 10000))
				var lines strings.Builder
				lines.Write(input)
				for i := 0; i < 8; i++ {
					lines.WriteString(string(rune('a'+i)) + "=" + strings.Repeat("é", 32000) + "\n")
				}
				input = []byte(lines.String())
			case "lines":
				input = []byte(strings.Repeat("\n", 65537))
			case "keys":
				var b strings.Builder
				for i := range 16385 {
					raw, _ := json.Marshal(i)
					b.Write(raw)
					b.WriteString("=v\n")
				}
				input = []byte(b.String())
			case "value":
				input = []byte("key=" + strings.Repeat("v", 65537))
			case "cancel":
				cancel()
			}
			if _, err := Render(ctx, input, propertiesPlan(t, `[]`)); err == nil {
				t.Fatal("invalid properties input accepted")
			}
		})
	}
}
func TestPropertiesPlanValidation(t *testing.T) {
	for _, rule := range []string{`{"match":"","replace_with":"new"}`, `{"match":"key","replace_with":{}}`, `{"match":"key"}`, `{"match":"key","replace_with":"{{config.docker.interface}}"}`, `{"match":"key","replace_with":"new","unexpected":true}`, `{"match":"key","replace_with":"new","if_value":[]}`} {
		if _, err := Compile([]json.RawMessage{json.RawMessage(`{"file":"server.properties","parser":"properties","replace":[` + rule + `]}`)}, []string{"server.properties"}); err == nil {
			t.Fatal("unsafe properties rule accepted")
		}
	}
}
