package setup_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagedPermissionPlanRejectsUnsafeLayouts(t *testing.T) {
	module, err := filepath.Abs("../../scripts/NodeSetup.psm1")
	if err != nil {
		t.Fatal(err)
	}
	const id = "11111111-1111-4111-8111-111111111111"
	for _, name := range []string{"valid", "external executable", "external dependency", "missing dependency", "external TLS key", "wrong data root", "unsafe profile key", "existing server data", "existing logs", "unmanaged root entry", "shared game identity", "hardlinked code", "junction code"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			for _, dir := range []string{"bin", "private", "data"} {
				if err := os.Mkdir(filepath.Join(root, dir), 0700); err != nil {
					t.Fatal(err)
				}
			}
			exe := filepath.Join(root, "bin", "daemon.exe")
			game := filepath.Join(root, "bin", "game.exe")
			for _, file := range []string{exe, game} {
				if err := os.WriteFile(file, []byte("harmless plan fixture"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			profile := map[string]any{"executable": game, "run_as": map[string]any{"sid": "S-1-5-21-1-2-3-1002"}, "requirements": map[string]any{"required_files": []string{}}}
			config := map[string]any{"data_directory": filepath.Join(root, "data"), "profiles": map[string]any{id: profile}, "tls": map[string]string{}}
			switch name {
			case "external executable":
				profile["executable"] = filepath.Join(root, "private", "outside.exe")
			case "external dependency":
				profile["requirements"] = map[string]any{"required_files": []string{filepath.Join(root, "private", "outside.dll")}}
			case "missing dependency":
				profile["requirements"] = map[string]any{"required_files": []string{filepath.Join(root, "bin", "missing.dll")}}
			case "external TLS key":
				config["tls"] = map[string]string{"key": game}
			case "wrong data root":
				config["data_directory"] = filepath.Join(root, "bin")
			case "unsafe profile key":
				config["profiles"] = map[string]any{"..\\escape": profile}
			case "existing server data":
				dir := filepath.Join(root, "data", id)
				if err := os.Mkdir(dir, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "save.dat"), nil, 0600); err != nil {
					t.Fatal(err)
				}
			case "existing logs":
				dir := filepath.Join(root, "logs")
				if err := os.Mkdir(dir, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "existing.log"), nil, 0600); err != nil {
					t.Fatal(err)
				}
			case "unmanaged root entry":
				if err := os.WriteFile(filepath.Join(root, "unmanaged.txt"), nil, 0600); err != nil {
					t.Fatal(err)
				}
			case "shared game identity":
				config["profiles"].(map[string]any)["22222222-2222-4222-8222-222222222222"] = profile
			case "hardlinked code":
				if err := os.Link(game, game+".alias"); err != nil {
					t.Fatal(err)
				}
			case "junction code":
				target := t.TempDir()
				if output, err := exec.Command("cmd.exe", "/c", "mklink", "/J", filepath.Join(root, "bin", "junction"), target).CombinedOutput(); err != nil {
					t.Fatalf("junction: %v %s", err, output)
				}
			}
			configPath := filepath.Join(root, "private", "config.json")
			body, _ := json.Marshal(config)
			if err := os.WriteFile(configPath, body, 0600); err != nil {
				t.Fatal(err)
			}
			// Identity lookup is injected; this suite changes no users, rights or
			// ACLs. Real filesystem link/containment checks still run unchanged.
			script := `param($Module,$Root,$Exe,$Config)
$ErrorActionPreference='Stop'
$m=Import-Module -Name $Module -Force -PassThru
& $m {
 function script:Get-LocalUser { param($SID) [pscustomobject]@{Name='FixtureGame';SID=$SID;Enabled=$true} }
 function script:Get-NodeLocalAccount { param($Name) [pscustomobject]@{SID=[Security.Principal.SecurityIdentifier]::new('S-1-5-21-1-2-3-1002')} }
}
$before=(Get-Acl -LiteralPath $Root).Sddl
$plan=Get-NodePermissionPlan -InstallationRoot $Root -Executable $Exe -Configuration $Config -DaemonSID 'S-1-5-21-1-2-3-1001'
if((Get-Acl -LiteralPath $Root).Sddl -ne $before){throw 'Planning changed ACLs'}
if(Test-Path -LiteralPath (Join-Path $Root 'data\11111111-1111-4111-8111-111111111111')){throw 'Planning created data'}
Write-Output ('RULES='+$plan.Rules.Count)
`
			scriptPath := filepath.Join(t.TempDir(), "plan.ps1")
			if err := os.WriteFile(scriptPath, []byte(script), 0600); err != nil {
				t.Fatal(err)
			}
			powershell := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
			output, err := exec.Command(powershell, "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", scriptPath, "-Module", module, "-Root", root, "-Exe", exe, "-Config", configPath).CombinedOutput()
			if name == "valid" {
				if err != nil || !strings.Contains(string(output), "RULES=") {
					t.Fatalf("valid plan: %v %s", err, output)
				}
			} else if err == nil {
				t.Fatalf("unsafe plan accepted: %s", output)
			}
		})
	}
}
