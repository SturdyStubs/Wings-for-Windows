package eggconfig

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMixedJSONAndTextBatchPreflight(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "server.cfg"), []byte("key=old\n"), 0600)
	os.WriteFile(filepath.Join(root, "server.json"), []byte(`{"bad":`), 0600)
	plans, err := Compile([]json.RawMessage{json.RawMessage(`{"file":"server.cfg","parser":"file","replace":[{"match":"key=","replace_with":"key=new"}]}`), json.RawMessage(`{"file":"server.json","parser":"json","replace":[{"match":"port","replace_with":"28015"}]}`)}, []string{"server.cfg", "server.json"})
	if err != nil {
		t.Fatal(err)
	}
	if Apply(context.Background(), root, plans, nil) == nil {
		t.Fatal("malformed JSON accepted")
	}
	raw, _ := os.ReadFile(filepath.Join(root, "server.cfg"))
	if string(raw) != "key=old\n" {
		t.Fatal("text was changed before JSON validated")
	}
	os.WriteFile(filepath.Join(root, "server.json"), []byte(`{}`), 0600)
	if err := Apply(context.Background(), root, plans, nil); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(filepath.Join(root, "server.json"))
	var result map[string]int
	if err := json.Unmarshal(raw, &result); err != nil || result["port"] != 28015 {
		t.Fatal(string(raw), err)
	}
}

func TestMixedPropertiesPreflightPreservesEarlierFile(t *testing.T) {
	root := t.TempDir()
	textPath := filepath.Join(root, "server.cfg")
	propPath := filepath.Join(root, "server.properties")
	os.WriteFile(textPath, []byte("key=old\n"), 0600)
	os.WriteFile(propPath, []byte(`bad=\u0000`), 0600)
	plans, err := Compile([]json.RawMessage{json.RawMessage(`{"file":"server.cfg","parser":"file","replace":[{"match":"key=","replace_with":"key=new"}]}`), json.RawMessage(`{"file":"server.properties","parser":"properties","replace":[{"match":"port","replace_with":28015}]}`)}, []string{"server.cfg", "server.properties"})
	if err != nil {
		t.Fatal(err)
	}
	if Apply(context.Background(), root, plans, nil) == nil {
		t.Fatal("escaped NUL accepted")
	}
	raw, _ := os.ReadFile(textPath)
	if string(raw) != "key=old\n" {
		t.Fatal("earlier target changed")
	}
	os.WriteFile(propPath, []byte("keep=yes\n"), 0600)
	if err := Apply(context.Background(), root, plans, nil); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(propPath)
	values := readPropertyResult(t, raw)
	if values["port"] != "28015" || values["keep"] != "yes" {
		t.Fatal(values)
	}
}

func TestMixedINIPreflightPreservesEarlierFile(t *testing.T) {
	root := t.TempDir()
	textPath := filepath.Join(root, "server.cfg")
	propPath := filepath.Join(root, "server.ini")
	os.WriteFile(textPath, []byte("key=old\n"), 0600)
	os.WriteFile(propPath, []byte(`[invalid`), 0600)
	plans, err := Compile([]json.RawMessage{json.RawMessage(`{"file":"server.cfg","parser":"file","replace":[{"match":"key=","replace_with":"key=new"}]}`), json.RawMessage(`{"file":"server.ini","parser":"ini","replace":[{"match":"port","replace_with":28015}]}`)}, []string{"server.cfg", "server.ini"})
	if err != nil {
		t.Fatal(err)
	}
	if Apply(context.Background(), root, plans, nil) == nil {
		t.Fatal("malformed INI accepted")
	}
	raw, _ := os.ReadFile(textPath)
	if string(raw) != "key=old\n" {
		t.Fatal("earlier target changed")
	}
	os.WriteFile(propPath, []byte("keep=yes\n"), 0600)
	if err := Apply(context.Background(), root, plans, nil); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(propPath)
	if string(raw) != "keep=yes\nport=28015\n" {
		t.Fatal(string(raw))
	}
}

func TestMixedYAMLPreflightPreservesEarlierFile(t *testing.T) {
	root := t.TempDir()
	textPath := filepath.Join(root, "server.cfg")
	propPath := filepath.Join(root, "server.yml")
	os.WriteFile(textPath, []byte("key=old\n"), 0600)
	os.WriteFile(propPath, []byte(`bad: [`), 0600)
	plans, err := Compile([]json.RawMessage{json.RawMessage(`{"file":"server.cfg","parser":"file","replace":[{"match":"key=","replace_with":"key=new"}]}`), json.RawMessage(`{"file":"server.yml","parser":"yaml","replace":[{"match":"port","replace_with":28015}]}`)}, []string{"server.cfg", "server.yml"})
	if err != nil {
		t.Fatal(err)
	}
	if Apply(context.Background(), root, plans, nil) == nil {
		t.Fatal("malformed YAML accepted")
	}
	raw, _ := os.ReadFile(textPath)
	if string(raw) != "key=old\n" {
		t.Fatal("earlier target changed")
	}
	os.WriteFile(propPath, []byte("keep: yes\n"), 0600)
	if err := Apply(context.Background(), root, plans, nil); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(propPath)
	values, err := readYAML(context.Background(), raw)
	if err != nil || values["keep"] != "yes" || values["port"] != json.Number("28015") {
		t.Fatal(string(raw), err)
	}
}

func TestMixedXMLPreflightPreservesEarlierFile(t *testing.T) {
	root := t.TempDir()
	textPath := filepath.Join(root, "server.cfg")
	propPath := filepath.Join(root, "server.xml")
	os.WriteFile(textPath, []byte("key=old\n"), 0600)
	os.WriteFile(propPath, []byte(`<invalid>`), 0600)
	plans, err := Compile([]json.RawMessage{json.RawMessage(`{"file":"server.cfg","parser":"file","replace":[{"match":"key=","replace_with":"key=new"}]}`), json.RawMessage(`{"file":"server.xml","parser":"xml","replace":[{"match":"server.port","replace_with":28015}]}`)}, []string{"server.cfg", "server.xml"})
	if err != nil {
		t.Fatal(err)
	}
	if Apply(context.Background(), root, plans, nil) == nil {
		t.Fatal("malformed XML accepted")
	}
	raw, _ := os.ReadFile(textPath)
	if string(raw) != "key=old\n" {
		t.Fatal("earlier target changed")
	}
	os.WriteFile(propPath, []byte("<server><keep>yes</keep></server>"), 0600)
	if err := Apply(context.Background(), root, plans, nil); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(propPath)
	values, err := readXML(context.Background(), raw)
	if err != nil || string(values.children[0].text) != "yes" || string(values.children[1].text) != "28015" {
		t.Fatal(string(raw), err)
	}
}
