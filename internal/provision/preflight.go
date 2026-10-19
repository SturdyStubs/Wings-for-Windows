// Package provision validates operator-approved Windows profiles without executing installers.
package provision

import (
	"context"
	"debug/pe"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"wings-windows/internal/config"
	"wings-windows/internal/native"
)

// Check performs read-only checks. It is not a security boundary or a DLL loader.
func Check(ctx context.Context, profile config.Profile) error {
	if err := profile.NormalizeRequirements(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		return fmt.Errorf("profile preflight requires Windows amd64")
	}
	f, err := os.Open(profile.Executable)
	if err != nil {
		return fmt.Errorf("executable: %w", err)
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return fmt.Errorf("executable: %w", err)
	}
	if !stat.Mode().IsRegular() {
		return fmt.Errorf("executable must be a regular file")
	}
	image, err := pe.NewFile(f)
	if err != nil {
		return fmt.Errorf("executable must be a Windows PE image: %w", err)
	}
	defer image.Close()
	if image.OptionalHeader == nil || image.Characteristics&pe.IMAGE_FILE_EXECUTABLE_IMAGE == 0 || image.Characteristics&pe.IMAGE_FILE_DLL != 0 {
		return fmt.Errorf("executable must be a PE application, not a DLL/object")
	}
	architecture := ""
	switch image.Machine {
	case pe.IMAGE_FILE_MACHINE_AMD64:
		architecture = "amd64"
	case pe.IMAGE_FILE_MACHINE_I386:
		architecture = "386"
	default:
		return fmt.Errorf("unsupported executable machine 0x%x; expected amd64 or 386", image.Machine)
	}
	if want := profile.Requirements.Architecture; want != "" && want != architecture {
		return fmt.Errorf("executable architecture is %s; profile requires %s", architecture, want)
	}
	if minimum := profile.Requirements.MinimumWindowsBuild; minimum > 0 {
		info, err := native.HostInfo(filepath.Dir(profile.Executable))
		if err != nil {
			return fmt.Errorf("Windows requirement: %w", err)
		}
		parts := strings.Split(info.KernelVersion, ".")
		build := 0
		if len(parts) == 3 {
			build, _ = strconv.Atoi(parts[2])
		}
		if build == 0 || build < minimum {
			return fmt.Errorf("Windows build %d does not satisfy minimum %d", build, minimum)
		}
	}
	for _, path := range profile.Requirements.RequiredFiles {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := readableFile(path); err != nil {
			return fmt.Errorf("required file %q: %w", path, err)
		}
	}
	return ctx.Err()
}

func readableFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("must be a readable regular file")
	}
	return nil
}
