package config

import "testing"

func TestEggConfigFileAuthorization(t *testing.T) {
	for _, name := range []string{"../out.cfg", "/root.cfg", `a\b.cfg`, "a:stream", "CON.txt", "com1", "a/./b", "a//b", "a.", "a ", "a?", ""} {
		t.Run(name, func(t *testing.T) {
			if EggFileName(name) == nil {
				t.Fatal("unsafe path accepted")
			}
		})
	}
	p := Profile{EggConfigFiles: []string{"server.cfg"}}
	if p.ValidateEggFiles() == nil {
		t.Fatal("local execution mode authorized rewriting")
	}
	p.UseEggProcessConfiguration = true
	if err := p.ValidateEggFiles(); err != nil {
		t.Fatal(err)
	}
	p.EggConfigFiles = append(p.EggConfigFiles, "SERVER.cfg")
	if p.ValidateEggFiles() == nil {
		t.Fatal("case duplicate accepted")
	}
}
