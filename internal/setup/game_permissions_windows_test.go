package setup_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"wings-windows/internal/testutil"
)

func TestIncrementalGamePermissionPlanningAndRollback(t *testing.T) {
	module, err := filepath.Abs("../../scripts/GamePermissions.psm1")
	if err != nil {
		t.Fatal(err)
	}
	const id = "11111111-1111-4111-8111-111111111111"
	const other = "22222222-2222-4222-8222-222222222222"
	for _, mode := range []string{"plan-only", "apply", "rollback", "preexisting-right-rollback", "unexpected-acl", "deny-batch", "populated-target", "external-exe", "duplicate-sid", "hardlink", "junction", "stale-plan", "staged-code"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			for _, dir := range []string{"bin", "private", "data", "logs", filepath.Join("data", other), filepath.Join("logs", "console", other)} {
				if err := os.MkdirAll(filepath.Join(root, dir), 0700); err != nil {
					t.Fatal(err)
				}
			}
			exe := filepath.Join(root, "bin", "fixture.exe")
			if mode == "staged-code" {
				if err := os.Mkdir(filepath.Join(root, "bin", ".stage-fixture"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			for _, file := range []string{exe, filepath.Join(root, "data", other, "save.txt"), filepath.Join(root, "logs", "console", other, "console.jsonl")} {
				if err := os.WriteFile(file, []byte("KEEP_EXISTING_DATA"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			profile := func(sid, uuid string) map[string]any {
				return map[string]any{"executable": exe, "run_as": map[string]string{"sid": sid, "credential_target": "WingsWindows/" + uuid}}
			}
			selected := profile("S-1-5-21-1-2-3-1003", id)
			if mode == "external-exe" {
				selected["executable"] = filepath.Join(root, "private", "config.json")
			}
			if mode == "duplicate-sid" {
				selected = profile("S-1-5-21-1-2-3-1002", id)
			}
			if mode == "populated-target" {
				os.Mkdir(filepath.Join(root, "data", id), 0700)
				os.WriteFile(filepath.Join(root, "data", id, "save.txt"), []byte("DON'T_TOUCH"), 0600)
			}
			if mode == "hardlink" {
				if err := os.Link(exe, exe+".alias"); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "junction" {
				if out, err := exec.Command("cmd.exe", "/c", "mklink", "/J", filepath.Join(root, "bin", "junction"), t.TempDir()).CombinedOutput(); err != nil {
					t.Fatalf("%v %s", err, out)
				}
			}
			c := map[string]any{"data_directory": filepath.Join(root, "data"), "profiles": map[string]any{id: selected, other: profile("S-1-5-21-1-2-3-1002", other)}}
			b, _ := json.Marshal(c)
			os.WriteFile(filepath.Join(root, "private", "config.json"), b, 0600)
			script := `param($Module,$Root,$Case)
$ErrorActionPreference='Stop'
$m=Import-Module $Module -Force -PassThru
$id='11111111-1111-4111-8111-111111111111';$other='22222222-2222-4222-8222-222222222222'
$daemon='S-1-5-21-1-2-3-1001';$game='S-1-5-21-1-2-3-1003';$old='S-1-5-21-1-2-3-1002'
$operator=[Security.Principal.WindowsIdentity]::GetCurrent().User.Value
$base='D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;FA;;;'+$operator+')'
# Mutations are confined to this temporary tree. Identity and LSA calls are
# injected; these tests never create accounts or grant real user rights.
foreach($p in @((Get-Item -LiteralPath $Root))+@(Get-ChildItem -LiteralPath $Root -Recurse -Force)){
 if($p.Name -eq 'junction'){continue}
 $acl=[Security.AccessControl.FileSecurity]::new();if($p.PSIsContainer){$acl=[Security.AccessControl.DirectorySecurity]::new()}
 $sddl=$base+'(A;OICI;FRFX;;;'+$daemon+')(A;OICI;FRFX;;;'+$old+')'
 if($p.FullName.StartsWith((Join-Path $Root 'private'))){$sddl=$base+'(A;OICI;FRFX;;;'+$daemon+')'}
 if($p.FullName.StartsWith((Join-Path $Root 'logs'))){$sddl=$base+'(A;OICI;0x1301bf;;;'+$daemon+')'}
 if($p.FullName.StartsWith((Join-Path $Root ('data\'+$other)))){$sddl=$base+'(A;OICI;0x1301bf;;;'+$daemon+')(A;OICI;0x1301bf;;;'+$old+')'}
 $acl.SetSecurityDescriptorSddlForm($sddl,[Security.AccessControl.AccessControlSections]::Access);if($p.PSIsContainer){[IO.Directory]::SetAccessControl($p.FullName,$acl)}else{[IO.File]::SetAccessControl($p.FullName,$acl)}
}
& $m {param($case)
 $script:added=0;$script:removed=0;$script:batch=($case -eq 'preexisting-right-rollback');$script:deny=($case -eq 'deny-batch');$script:failAt=0;$script:sets=0
 function script:Get-GamePermissionAccount {param($SID) [pscustomobject]@{SID=[Security.Principal.SecurityIdentifier]::new($SID)}}
 function script:Get-GamePermissionRights {param($SID) if($script:deny){'SeDenyBatchLogonRight'};if($script:batch){'SeBatchLogonRight'}}
 function script:Set-GamePermissionBatchRight {param($SID,$Remove) if($Remove){$script:removed++;$script:batch=$false}else{$script:added++;$script:batch=$true}}
 function script:Set-GamePermissionACL {param($Path,$SDDL)
  $script:sets++;if($script:sets -eq $script:failAt){throw 'Synthetic ACL mutation failure'}
  # The unit host is not elevated. Stand in for the public command's required
  # Administrators token on newly protected empty target directories only.
  $operator=[Security.Principal.WindowsIdentity]::GetCurrent().User.Value
  if(-not $SDDL.Contains(';;;'+$operator+')')){$SDDL+='(A;OICI;FA;;;'+$operator+')'}

  $acl=[Security.AccessControl.FileSecurity]::new();if(Test-Path -LiteralPath $Path -PathType Container){$acl=[Security.AccessControl.DirectorySecurity]::new()};$acl.SetSecurityDescriptorSddlForm($SDDL,[Security.AccessControl.AccessControlSections]::Access);if($acl -is [Security.AccessControl.DirectorySecurity]){[IO.Directory]::SetAccessControl($Path,$acl)}else{[IO.File]::SetAccessControl($Path,$acl)}
 }
} $Case
if($Case -eq 'unexpected-acl'){$path=Join-Path $Root 'private';$oldSDDL=(Get-Acl -LiteralPath $path).GetSecurityDescriptorSddlForm([Security.AccessControl.AccessControlSections]::Access);$acl=[Security.AccessControl.DirectorySecurity]::new();$acl.SetSecurityDescriptorSddlForm($oldSDDL,[Security.AccessControl.AccessControlSections]::Access);$acl.AddAccessRule([Security.AccessControl.FileSystemAccessRule]::new([Security.Principal.SecurityIdentifier]::new('S-1-1-0'),'Read','Allow'));[IO.Directory]::SetAccessControl($path,$acl)}
$before=@{};foreach($p in @((Get-Item -LiteralPath $Root))+@(Get-ChildItem -LiteralPath $Root -Recurse -Force)){if($p.Name -ne 'junction'){$before[$p.FullName]=(Get-Acl -LiteralPath $p.FullName).Sddl}}
$valid=$Case -in @('plan-only','apply','rollback','preexisting-right-rollback','stale-plan')
$failed=$false
try{$plan=Get-GamePermissionPlan $Root $id $daemon}catch{if($valid){throw};$failed=$true}
if(-not $valid -and -not $failed){throw 'Unsafe plan accepted'}
foreach($p in $before.Keys){if((Get-Acl -LiteralPath $p).Sddl -cne $before[$p]){throw 'Planning changed ACLs'}}
if($Case -eq 'plan-only' -and (Test-Path -LiteralPath (Join-Path $Root ('data\'+$id)))){throw 'Planning created data'}
if($Case -in @('rollback','preexisting-right-rollback')){& $m {param($n) $script:failAt=$n} $plan.Rules.Count}
if($Case -eq 'stale-plan'){$path=Join-Path $Root 'bin';$oldSDDL=(Get-Acl -LiteralPath $path).GetSecurityDescriptorSddlForm([Security.AccessControl.AccessControlSections]::Access);$acl=[Security.AccessControl.DirectorySecurity]::new();$acl.SetSecurityDescriptorSddlForm($oldSDDL,[Security.AccessControl.AccessControlSections]::Access);$acl.AddAccessRule([Security.AccessControl.FileSystemAccessRule]::new([Security.Principal.SecurityIdentifier]::new($game),'ReadAndExecute','Allow'));[IO.Directory]::SetAccessControl($path,$acl)}
if($Case -in @('apply','rollback','preexisting-right-rollback','stale-plan')){
 $failed=$false;try{$result=Enable-GamePermissionPlan $plan}catch{if($Case -eq 'apply'){throw};$failed=$true}
 if($Case -ne 'apply' -and -not $failed){throw 'Expected mutation failure'}
 if($Case -in @('rollback','preexisting-right-rollback')){foreach($p in $before.Keys){if((Get-Acl -LiteralPath $p).Sddl -cne $before[$p]){throw ('Rollback changed '+$p)}}}
 if($Case -eq 'apply'){
  if(-not $result.prepared){throw 'Preparation not reported'}
  foreach($p in @((Join-Path $Root ('data\'+$other)),(Join-Path $Root ('data\'+$other+'\save.txt')),(Join-Path $Root 'private\config.json'),(Join-Path $Root ('logs\console\'+$other+'\console.jsonl')))){if((Get-Acl -LiteralPath $p).Sddl -cne $before[$p]){throw 'Existing data/private/log ACL changed'}}
  $acl=Get-Acl -LiteralPath (Join-Path $Root ('data\'+$id));$rules=@($acl.GetAccessRules($true,$true,[Security.Principal.SecurityIdentifier])|Where-Object{$_.IdentityReference.Value -eq $game})
  if($rules.Count -ne 1 -or ([int]$rules[0].FileSystemRights -band 0x1301bf) -ne 0x1301bf){throw 'New game data permission missing'}
  $plan=Get-GamePermissionPlan $Root $id $daemon;$again=Enable-GamePermissionPlan $plan
  if($again.batch_right_added){throw 'Reapplication granted a preexisting right'}
 }
}
$counts=& $m { @($script:added,$script:removed) }
if($Case -eq 'rollback' -and ($counts[0] -ne 1 -or $counts[1] -ne 1)){throw 'Added right not rolled back'}
if($Case -eq 'preexisting-right-rollback' -and ($counts[0]+$counts[1] -ne 0)){throw 'Preexisting right modified'}
if([IO.File]::ReadAllText((Join-Path $Root ('data\'+$other+'\save.txt'))) -ne 'KEEP_EXISTING_DATA'){throw 'Existing data changed'}
Write-Output 'PASSED'
`
			file := filepath.Join(t.TempDir(), "game-permission-test.ps1")
			os.WriteFile(file, []byte(script), 0600)
			out, err := testutil.PowerShell(context.Background(), "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", file, "-Module", module, "-Root", root, "-Case", mode).CombinedOutput()
			if err != nil {
				t.Fatalf("%v\n%s", err, out)
			}
		})
	}
}
