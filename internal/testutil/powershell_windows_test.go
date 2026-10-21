package testutil

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestWindowsPowerShellUsesOwnModulePaths(t *testing.T) {
	t.Setenv("PSModulePath", `C:\NotARealPowerShell7ModuleDirectory`)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	output, err := PowerShell(ctx, "-NoProfile", "-NonInteractive", "-Command", `$ErrorActionPreference='Stop';ConvertTo-SecureString 'FixtureOnly' -AsPlainText -Force|Out-Null;Get-Acl -LiteralPath $env:TEMP|Out-Null;Write-Output 'MODULES_OK'`).CombinedOutput()
	if err != nil {
		t.Fatalf("fixture module import: %v %s", err, output)
	}
	if os.Getenv("PSModulePath") != `C:\NotARealPowerShell7ModuleDirectory` {
		t.Fatal("child setup changed parent environment")
	}
}
