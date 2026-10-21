package eggconfig

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestApplyEggTextValidatesAllBeforeWrites(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "server.cfg")
	os.WriteFile(first, []byte("key=old\r\n"), 0600)
	plans := []File{{Name: "server.cfg", Replace: []Replacement{{"key=", "key=new"}}}, {Name: "missing.cfg"}}
	if Apply(context.Background(), root, plans, nil) == nil {
		t.Fatal("missing target accepted")
	}
	raw, _ := os.ReadFile(first)
	if string(raw) != "key=old\r\n" {
		t.Fatal("changed file before all validation completed")
	}
	if err := Apply(context.Background(), root, plans[:1], nil); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(first)
	if string(raw) != "key=new\n" {
		t.Fatal(string(raw))
	}
}
func TestApplyEggTextProtectsCodeAndAliases(t *testing.T) {
	for _, mode := range []string{"protected", "hardlink", "junction", "directory", "locked", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			outside := t.TempDir()
			target := filepath.Join(root, "server.cfg")
			os.WriteFile(target, []byte("key=old\n"), 0600)
			var locked *os.File
			var protected []string
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			name := "server.cfg"
			switch mode {
			case "protected":
				protected = []string{target}
			case "hardlink":
				if err := os.Link(target, filepath.Join(outside, "alias")); err != nil {
					t.Fatal(err)
				}
			case "junction":
				if output, err := exec.Command("cmd.exe", "/c", "mklink", "/J", filepath.Join(root, "linked"), outside).CombinedOutput(); err != nil {
					t.Fatal(err, string(output))
				}
				os.WriteFile(filepath.Join(outside, "server.cfg"), []byte("key=old\n"), 0600)
				name = "linked/server.cfg"
			case "directory":
				name = "directory"
				os.Mkdir(filepath.Join(root, name), 0700)
			case "locked":
				f, err := os.OpenFile(target, os.O_WRONLY, 0)
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				locked = f
			case "cancel":
				cancel()
			}
			if err := Apply(ctx, root, []File{{Name: name, Replace: []Replacement{{"key=", "key=new"}}}}, protected); err == nil {
				t.Fatal("unsafe rewrite accepted")
			}
			raw, _ := os.ReadFile(target)
			if string(raw) != "key=old\n" {
				t.Fatal("changed protected original")
			}
			if locked != nil {
				locked.Close()
			}
			// All ancestor handles must be released even after preflight failure.
			if err := os.Rename(root, root+"-moved"); err != nil {
				t.Fatal("leaked directory guard", err)
			}
			if err := os.Rename(root+"-moved", root); err != nil {
				t.Fatal(err)
			}
		})
	}
}
