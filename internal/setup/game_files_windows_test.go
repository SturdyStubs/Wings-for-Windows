package setup_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"wings-windows/internal/testutil"
)

func TestProtectedGameFileImport(t *testing.T) {
	module, err := filepath.Abs("../../scripts/GameFiles.psm1")
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"plan-only", "import", "bad-manifest-hash", "bad-source-hash", "unknown-field", "duplicate-path", "traversal", "reserved-name", "existing-target", "existing-profile", "hardlink", "junction", "copy-fails", "promotion-fails", "stale-service", "source-pinned", "destination-race", "existing-stage", "bad-version", "empty-files", "too-many-files", "oversized-manifest"} {
		t.Run(mode, func(t *testing.T) {
			root, source := t.TempDir(), t.TempDir()
			for _, dir := range []string{"bin", "private", "data", "logs"} {
				if err := os.Mkdir(filepath.Join(root, dir), 0700); err != nil {
					t.Fatal(err)
				}
			}
			script := `param($Module,$Root,$Source,$Case)
$ErrorActionPreference='Stop'
$m=Import-Module $Module -Force -PassThru
$id='11111111-1111-4111-8111-111111111111';$config=Join-Path $Root 'private\config.json'
@{profiles=@{}}|ConvertTo-Json|Set-Content -LiteralPath $config
if($Case -eq 'existing-profile'){@{profiles=@{$id=@{}}}|ConvertTo-Json -Depth 5|Set-Content -LiteralPath $config}
[IO.Directory]::CreateDirectory((Join-Path $Source 'deps'))|Out-Null
[IO.File]::WriteAllText((Join-Path $Source 'server.exe'),'APPROVED_FIXTURE')
[IO.File]::WriteAllText((Join-Path $Source 'deps\runtime.dll'),'APPROVED_DEPENDENCY')
$entries=@(@{path='server.exe';sha256=(Get-FileHash -LiteralPath (Join-Path $Source 'server.exe') -Algorithm SHA256).Hash},@{path='deps/runtime.dll';sha256=(Get-FileHash -LiteralPath (Join-Path $Source 'deps\runtime.dll') -Algorithm SHA256).Hash})
if($Case -eq 'bad-source-hash'){$entries[0].sha256='0'*64}
if($Case -eq 'duplicate-path'){$entries+=@(@{path='SERVER.EXE';sha256=$entries[0].sha256})}
if($Case -eq 'traversal'){$entries[0].path='..\server.exe'}
if($Case -eq 'reserved-name'){$entries[0].path='CON.exe'}
if($Case -eq 'hardlink'){[IO.File]::WriteAllText((Join-Path $Source 'original'),'LINK');cmd.exe /c mklink /H (Join-Path $Source 'linked') (Join-Path $Source 'original')|Out-Null;if($LASTEXITCODE){throw 'Hardlink fixture failed'};$entries[0].path='linked'}
if($Case -eq 'junction'){$outside=Join-Path $Root 'data';cmd.exe /c mklink /J (Join-Path $Source 'linked') $outside|Out-Null;if($LASTEXITCODE){throw 'Junction fixture failed'};$entries[0].path='linked\server.exe'}
$target=Join-Path $Root ('bin\games\'+$id)
if($Case -eq 'existing-target'){[IO.Directory]::CreateDirectory($target)|Out-Null;[IO.File]::WriteAllText((Join-Path $target 'KEEP'),'PRESERVED')}
if($Case -eq 'existing-stage'){[IO.Directory]::CreateDirectory((Join-Path $Root 'bin\games\.stage-fixture'))|Out-Null}
$manifest=Join-Path $Root 'manifest-outside.json'
# Manifest must be outside the managed root; use a sibling temp test file.
$manifest=Join-Path $Source 'manifest.json';$value=@{schema_version=1;files=$entries};if($Case -eq 'unknown-field'){$value.extra=$true}
if($Case -eq 'bad-version'){$value.schema_version=$true}
if($Case -eq 'empty-files'){$value.files=@()}
if($Case -eq 'too-many-files'){$value.files=@($entries[0])*1001}
[IO.File]::WriteAllText($manifest,($value|ConvertTo-Json -Depth 6),[Text.UTF8Encoding]::new($false))
if($Case -eq 'oversized-manifest'){[IO.File]::WriteAllText($manifest,(' '*1048577))}
$hash=(Get-FileHash -LiteralPath $manifest -Algorithm SHA256).Hash;if($Case -eq 'bad-manifest-hash'){$hash='0'*64}
& $m {param($root,$case)
 $script:root=$root;$script:changed=$false;$script:case=$case
 function script:Get-GamePermissionService {param($p) $sid='S-1-5-21-1-2-3-1001';if($script:changed){$sid='S-1-5-21-1-2-3-1003'};[pscustomobject]@{Root=$script:root;Configuration=(Join-Path $script:root 'private\config.json');Executable=(Join-Path $script:root 'bin\daemon.exe');DaemonSID=$sid}}
 function script:Assert-GameFileACL {param($Path,$Games,$DaemonSID)}
 function script:Set-GameFileACL {param($Path,$SDDL)
  # Stand in for the required elevated Administrators token on this unit host.
  $operator=[Security.Principal.WindowsIdentity]::GetCurrent().User.Value;$SDDL+='(A;OICI;FA;;;'+$operator+')'
  $acl=[Security.AccessControl.FileSecurity]::new();if(Test-Path -LiteralPath $Path -PathType Container){$acl=[Security.AccessControl.DirectorySecurity]::new()}
  $acl.SetSecurityDescriptorSddlForm($SDDL,[Security.AccessControl.AccessControlSections]::Access)
  if($acl -is [Security.AccessControl.DirectorySecurity]){[IO.Directory]::SetAccessControl($Path,$acl)}else{[IO.File]::SetAccessControl($Path,$acl)}
 }
 if($case -eq 'copy-fails'){function script:Copy-GameFileStream {param($Source,$Destination) $Destination.WriteByte(1);throw 'Synthetic partial-copy failure'}}
 if($case -eq 'promotion-fails'){function script:Publish-GameFileDirectory {param($Stage,$Destination) throw 'Synthetic promotion failure'}}
 if($case -eq 'destination-race'){function script:Publish-GameFileDirectory {param($Stage,$Destination) [IO.Directory]::CreateDirectory($Destination)|Out-Null;[IO.File]::WriteAllText((Join-Path $Destination 'KEEP'),'PRESERVED');[IO.Directory]::Move($Stage,$Destination)}}
} $Root $Case
$unsafe=$Case -in @('bad-manifest-hash','bad-source-hash','unknown-field','duplicate-path','traversal','reserved-name','existing-target','existing-profile','hardlink','junction','existing-stage','bad-version','empty-files','too-many-files','oversized-manifest')
$plan=$null;$failed=$false
try{
 try{$plan=Get-GameFilePlan $Root $id $Source $manifest $hash}catch{if(-not $unsafe){throw};$failed=$true}
 if($unsafe -and -not $failed){throw 'Unsafe import accepted'}
 if($Case -eq 'plan-only' -and (Test-Path -LiteralPath (Join-Path $Root 'bin\games'))){throw 'Planning created code directories'}
 if($Case -eq 'source-pinned'){
  $blocked=$false;try{$writer=[IO.File]::Open((Join-Path $Source 'server.exe'),[IO.FileMode]::Open,[IO.FileAccess]::Write,[IO.FileShare]::ReadWrite);$writer.Dispose()}catch{$blocked=$true};if(-not $blocked){throw 'Approved source allows concurrent writes'}
 }
 if($Case -eq 'stale-service'){& $m {$script:changed=$true}}
 if($Case -in @('import','copy-fails','promotion-fails','stale-service','destination-race')){
  $failed=$false;try{$result=Import-GameFilePlan $plan}catch{if($Case -eq 'import'){throw};$failed=$true}
  if($Case -ne 'import' -and -not $failed){throw 'Expected import failure'}
  if($Case -eq 'import'){
   if(-not $result.imported -or $result.files -ne 2 -or [IO.File]::ReadAllText((Join-Path $target 'deps\runtime.dll')) -cne 'APPROVED_DEPENDENCY'){throw 'Wrong promoted code'}
   $rules=(Get-Acl -LiteralPath (Join-Path $target 'server.exe')).GetAccessRules($true,$true,[Security.Principal.SecurityIdentifier])|Where-Object {$_.IdentityReference.Value -eq 'S-1-5-21-1-2-3-1001'}
   if(-not $rules -or ([int]$rules.FileSystemRights -band 0x116) -ne 0){throw 'Daemon code ACL is writable or absent'}
  }
  if($Case -in @('copy-fails','promotion-fails','stale-service') -and (Test-Path -LiteralPath $target)){throw 'Failed import published code'}
  if(Test-Path -LiteralPath (Join-Path $Root 'bin\games')){if(@(Get-ChildItem -LiteralPath (Join-Path $Root 'bin\games') -Filter '.stage-*').Count){throw 'Staged files leaked'}}
 }
 if($Case -in @('existing-target','destination-race') -and [IO.File]::ReadAllText((Join-Path $target 'KEEP')) -cne 'PRESERVED'){throw 'Existing destination changed'}
}finally{if($plan){Close-GameFilePlan $plan}}
$writer=[IO.File]::Open((Join-Path $Source 'server.exe'),[IO.FileMode]::Open,[IO.FileAccess]::Write,[IO.FileShare]::ReadWrite);$writer.Dispose()
Write-Output 'PASSED'
`
			file := filepath.Join(t.TempDir(), "game-files-test.ps1")
			if err := os.WriteFile(file, []byte(script), 0600); err != nil {
				t.Fatal(err)
			}
			out, err := testutil.PowerShell(context.Background(), "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", file, "-Module", module, "-Root", root, "-Source", source, "-Case", mode).CombinedOutput()
			if err != nil {
				t.Fatalf("%v\n%s", err, out)
			}
		})
	}
}
