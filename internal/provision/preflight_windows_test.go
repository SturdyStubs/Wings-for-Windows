package provision

import (
	"context"
	"debug/pe"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"wings-windows/internal/config"
)

func TestPreflightRequirements(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dependency := filepath.Join(t.TempDir(), "required.dll")
	if err := os.WriteFile(dependency, []byte("declared dependency fixture; never loaded"), 0600); err != nil {
		t.Fatal(err)
	}
	base := config.Profile{Executable: exe, Requirements: config.Requirements{Architecture: "amd64", MinimumWindowsBuild: 1, RequiredFiles: []string{dependency}}}
	for _, test := range []struct {
		name   string
		change func(*config.Profile)
		want   string
	}{
		{"valid legacy version", func(p *config.Profile) {}, ""},
		{"explicit v1", func(p *config.Profile) { p.SchemaVersion = 1 }, ""},
		{"future schema", func(p *config.Profile) { p.SchemaVersion = 2 }, "schema_version"},
		{"missing executable", func(p *config.Profile) { p.Executable = filepath.Join(t.TempDir(), "absent.exe") }, "executable"},
		{"directory executable", func(p *config.Profile) { p.Executable = t.TempDir() }, "regular file"},
		{"wrong architecture", func(p *config.Profile) { p.Requirements.Architecture = "386" }, "architecture"},
		{"newer Windows", func(p *config.Profile) { p.Requirements.MinimumWindowsBuild = 1_000_000 }, "minimum"},
		{"missing dependency", func(p *config.Profile) {
			p.Requirements.RequiredFiles = []string{filepath.Join(t.TempDir(), "missing.dll")}
		}, "required file"},
		{"directory dependency", func(p *config.Profile) { p.Requirements.RequiredFiles = []string{t.TempDir()} }, "regular file"},
		{"relative dependency", func(p *config.Profile) { p.Requirements.RequiredFiles = []string{"file.dll"} }, "absolute"},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := base
			test.change(&p)
			err := Check(context.Background(), p)
			if test.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("want %q, got %v", test.want, err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Check(ctx, base); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation ignored", err)
	}
}

func TestPreflightRejectsNonApplications(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	image, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	header := int(binary.LittleEndian.Uint32(image[0x3c:]))
	for _, kind := range []string{"text", "DLL", "ARM64"} {
		t.Run(kind, func(t *testing.T) {
			data := append([]byte(nil), image...)
			switch kind {
			case "text":
				data = []byte("not an EXE")
			case "DLL":
				pos := header + 22
				binary.LittleEndian.PutUint16(data[pos:], binary.LittleEndian.Uint16(data[pos:])|pe.IMAGE_FILE_DLL)
			case "ARM64":
				binary.LittleEndian.PutUint16(data[header+4:], pe.IMAGE_FILE_MACHINE_ARM64)
			}
			path := filepath.Join(t.TempDir(), "bad.exe")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			if err := Check(context.Background(), config.Profile{Executable: path}); err == nil {
				t.Fatal("invalid executable accepted")
			}
		})
	}
}
