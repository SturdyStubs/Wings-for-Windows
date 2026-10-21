// Package testutil contains developer-fixture helpers, not daemon runtime code.
package testutil

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// PowerShell starts the Windows 5.1 fixture host with its own module defaults.
// PowerShell 7 hosts can otherwise leak incompatible bundled module paths to
// children launched directly by Go rather than by PowerShell's native invoker.
func PowerShell(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe"), args...)
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		if !strings.EqualFold(key, "PSModulePath") {
			cmd.Env = append(cmd.Env, value)
		}
	}
	return cmd
}
