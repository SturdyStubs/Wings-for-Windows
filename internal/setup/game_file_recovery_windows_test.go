package setup_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"wings-windows/internal/testutil"
)

func TestStagingInspectionAndQuarantine(t *testing.T) {
	module, err := filepath.Abs("../../scripts/GameFileRecovery.psm1")
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"inspect", "quarantine", "partial-acls", "bad-name", "referenced-profile", "normalized-reference", "existing-destination", "foreign-acl", "daemon-write", "hardlink", "junction", "changed-content", "changed-acl", "changed-service", "move-fails", "destination-race", "empty-stage"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			script := `param($Module,$Root,$Case)
$ErrorActionPreference='Stop'
$m=Import-Module $Module -Force -PassThru
$name='.stage-11111111111111111111111111111111';$stage=Join-Path $Root ('bin\games\'+$name);$private=Join-Path $Root 'private';$destination=Join-Path $private ('code-quarantine\'+$name)
[IO.Directory]::CreateDirectory((Join-Path $stage 'nested'))|Out-Null;[IO.Directory]::CreateDirectory($private)|Out-Null
[IO.File]::WriteAllText((Join-Path $stage 'partial.exe'),'PARTIAL_SOURCE_BYTES');[IO.File]::WriteAllText((Join-Path $stage 'nested\dependency.dll'),'DEPENDENCY_BYTES')
$operator=[Security.Principal.WindowsIdentity]::GetCurrent().User.Value
$base='D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;FA;;;'+$operator+')'
function Set-UnitACL($Path,$SDDL){$acl=[Security.AccessControl.FileSecurity]::new();if(Test-Path -LiteralPath $Path -PathType Container){$acl=[Security.AccessControl.DirectorySecurity]::new()};$acl.SetSecurityDescriptorSddlForm($SDDL,[Security.AccessControl.AccessControlSections]::Access);if($acl -is [Security.AccessControl.DirectorySecurity]){[IO.Directory]::SetAccessControl($Path,$acl)}else{[IO.File]::SetAccessControl($Path,$acl)}}
foreach($entry in @((Get-Item -LiteralPath $stage))+@(Get-ChildItem -LiteralPath $stage -Recurse -Force)){Set-UnitACL $entry.FullName $base}
$config=[pscustomobject]@{profiles=[pscustomobject]@{}}
if($Case -eq 'referenced-profile'){$config.profiles|Add-Member -NotePropertyName '11111111-1111-4111-8111-111111111111' -NotePropertyValue @{executable=(Join-Path $stage 'partial.exe')}}
if($Case -eq 'normalized-reference'){$config.profiles|Add-Member -NotePropertyName '11111111-1111-4111-8111-111111111111' -NotePropertyValue @{executable=((Join-Path $stage 'partial.exe').Replace('\','/'))}}
if($Case -eq 'existing-destination'){[IO.Directory]::CreateDirectory($destination)|Out-Null;[IO.File]::WriteAllText((Join-Path $destination 'KEEP'),'PRESERVED')}
if($Case -eq 'foreign-acl'){Set-UnitACL (Join-Path $stage 'partial.exe') ($base+'(A;;FR;;;WD)')}
if($Case -eq 'daemon-write'){Set-UnitACL (Join-Path $stage 'partial.exe') ($base+'(A;;FW;;;S-1-5-21-1-2-3-1001)')}
if($Case -eq 'partial-acls'){Set-UnitACL (Join-Path $stage 'partial.exe') ($base+'(A;;FRFX;;;S-1-5-21-1-2-3-1001)')}
if($Case -eq 'hardlink'){cmd.exe /c mklink /H (Join-Path $stage 'alias') (Join-Path $stage 'partial.exe')|Out-Null;if($LASTEXITCODE){throw 'Hardlink creation failed'}}
if($Case -eq 'junction'){cmd.exe /c mklink /J (Join-Path $stage 'linked') $private|Out-Null;if($LASTEXITCODE){throw 'Junction creation failed'}}
if($Case -eq 'empty-stage'){[IO.File]::Delete((Join-Path $stage 'partial.exe'));[IO.File]::Delete((Join-Path $stage 'nested\dependency.dll'));[IO.Directory]::Delete((Join-Path $stage 'nested'))}
$node=[pscustomobject]@{Root=$Root;Parent=(Join-Path $Root 'bin\games');Quarantine=(Join-Path $Root 'private\code-quarantine');DaemonSID='S-1-5-21-1-2-3-1001';Config=$config}
& $m {param($node,$case,$base)
 $script:node=$node;$script:changed=$false;$script:base=$base
 function script:Get-RecoveryNode {param($root) $copy=$script:node.PSObject.Copy();if($script:changed){$copy.DaemonSID='S-1-5-21-1-2-3-1002'};$copy}
 function script:Set-RecoveryACL {param($Path)
  # Non-elevated unit stand-in for the required Administrators token.
  $acl=[Security.AccessControl.FileSecurity]::new();if(Test-Path -LiteralPath $Path -PathType Container){$acl=[Security.AccessControl.DirectorySecurity]::new()};$acl.SetSecurityDescriptorSddlForm($script:base,[Security.AccessControl.AccessControlSections]::Access);if($acl -is [Security.AccessControl.DirectorySecurity]){[IO.Directory]::SetAccessControl($Path,$acl)}else{[IO.File]::SetAccessControl($Path,$acl)}
 }
 if($case -eq 'move-fails'){function script:Move-RecoveryDirectory {param($Source,$Destination) throw 'Synthetic move failure'}}
 if($case -eq 'destination-race'){function script:Move-RecoveryDirectory {param($Source,$Destination) [IO.Directory]::CreateDirectory($Destination)|Out-Null;[IO.File]::WriteAllText((Join-Path $Destination 'KEEP'),'PRESERVED');[IO.Directory]::Move($Source,$Destination)}}
} $node $Case $base
if($Case -eq 'bad-name'){$name='..\private'}
$unsafe=$Case -in @('bad-name','referenced-profile','normalized-reference','existing-destination','foreign-acl','daemon-write','hardlink','junction')
$plan=$null;$failed=$false
try{
 try{$plan=Get-RecoveryPlan $node $name}catch{if(-not $unsafe){throw};$failed=$true}
 if($unsafe -and -not $failed){throw 'Unsafe stage accepted'}
 if($plan){
  $summary=Get-RecoverySummary $plan
  if($summary.fingerprint -notmatch '^[0-9a-f]{64}$' -or $summary.files -ne $(if($Case -eq 'empty-stage'){0}else{2})){throw 'Wrong inspection metadata'}
  $expected=$plan.Fingerprint
  if($Case -in @('changed-content','changed-acl')){
   Close-RecoveryPlan $plan
   if($Case -eq 'changed-content'){[IO.File]::WriteAllText((Join-Path $stage 'partial.exe'),'CHANGED_BYTES')}else{Set-UnitACL (Join-Path $stage 'partial.exe') ($base+'(A;;FR;;;S-1-5-21-1-2-3-1001)')}
  }
  if($Case -eq 'changed-service'){& $m {$script:changed=$true}}
  if($Case -ne 'inspect'){
   $failed=$false;try{$result=Move-RecoveryPlan $plan $expected}catch{if($Case -in @('quarantine','partial-acls','empty-stage')){throw};$failed=$true}
   if($Case -in @('changed-content','changed-acl','changed-service','move-fails','destination-race') -and -not $failed){throw 'Expected quarantine refusal'}
   if(-not $failed){
    if($result.state -ne 'quarantined' -or (Test-Path -LiteralPath $stage)){throw 'Stage not moved'}
    if($Case -ne 'empty-stage' -and [IO.File]::ReadAllText((Join-Path $destination 'partial.exe')) -cne 'PARTIAL_SOURCE_BYTES'){throw 'Quarantine changed source bytes'}
    $rules=@((Get-Acl -LiteralPath $destination).GetAccessRules($true,$true,[Security.Principal.SecurityIdentifier])|Where-Object {$_.IdentityReference.Value -eq $node.DaemonSID});if($rules.Count){throw 'Quarantine retains daemon access'}
   }else{if(-not(Test-Path -LiteralPath (Join-Path $stage 'nested\dependency.dll'))){throw 'Failed quarantine deleted staged bytes'}}
  }elseif(Test-Path -LiteralPath $node.Quarantine){throw 'Inspection created quarantine'}
 }
}finally{if($plan){Close-RecoveryPlan $plan}}
if($Case -in @('existing-destination','destination-race') -and [IO.File]::ReadAllText((Join-Path $destination 'KEEP')) -cne 'PRESERVED'){throw 'Existing quarantine overwritten'}
Write-Output 'PASSED'
`
			file := filepath.Join(t.TempDir(), "recovery-test.ps1")
			if err := os.WriteFile(file, []byte(script), 0600); err != nil {
				t.Fatal(err)
			}
			out, err := testutil.PowerShell(context.Background(), "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", file, "-Module", module, "-Root", root, "-Case", mode).CombinedOutput()
			if err != nil {
				t.Fatalf("%v\n%s", err, out)
			}
		})
	}
}
