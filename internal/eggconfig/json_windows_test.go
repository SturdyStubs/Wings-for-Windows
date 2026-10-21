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
