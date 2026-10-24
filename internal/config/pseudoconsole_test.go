package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPseudoconsoleProfileOptIn(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		c := valid(t)
		for id, p := range c.Profiles {
			p.Pseudoconsole = enabled
			c.Profiles[id] = p
		}
		data, e := json.Marshal(c)
		if e != nil {
			t.Fatal(e)
		}
		filename := filepath.Join(t.TempDir(), "config.json")
		if e = os.WriteFile(filename, data, 0600); e != nil {
			t.Fatal(e)
		}
		loaded, e := Load(filename)
		if e != nil {
			t.Fatal(e)
		}
		for _, p := range loaded.Profiles {
			if p.Pseudoconsole != enabled {
				t.Fatal("console choice lost")
			}
		}
	}
}
