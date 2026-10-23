package setup_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"wings-windows/internal/testutil"
)

func TestGameAccountCreationGuardsAndRollback(t *testing.T) {
	module, err := filepath.Abs("../../scripts/GameAccounts.psm1")
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"plan-only", "create", "existing-account", "existing-profile", "existing-data", "bad-name", "bad-uuid", "empty-password", "overlong-password", "create-fails", "validation-fails", "identity-mismatch", "daemon-mismatch", "rollback-fails", "stale-account", "stale-service"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			for _, dir := range []string{"private", "data"} {
				if err := os.Mkdir(filepath.Join(root, dir), 0700); err != nil {
					t.Fatal(err)
				}
			}
			config := map[string]any{"data_directory": filepath.Join(root, "data"), "profiles": map[string]any{}}
			b, _ := json.Marshal(config)
			if err := os.WriteFile(filepath.Join(root, "private", "config.json"), b, 0600); err != nil {
				t.Fatal(err)
			}
			script := `param($Module,$Root,$Case)
$ErrorActionPreference='Stop'
$m=Import-Module $Module -Force -PassThru
$id='11111111-1111-4111-8111-111111111111';$name='WwUnitGame'
& $m {param($root,$case)
 $script:root=$root;$script:case=$case;$script:creates=0;$script:removes=0;$script:exists=($case -eq 'existing-account');$script:changed=$false
 $script:realCreate=${function:New-GameAccountUser}
 function script:New-LocalUser {
  [CmdletBinding()]param($Name,[Security.SecureString]$Password,[ValidateLength(0,48)][string]$Description,[switch]$PasswordNeverExpires,[switch]$UserMayNotChangePassword)
  if(-not $PasswordNeverExpires -or -not $UserMayNotChangePassword -or -not $Description.EndsWith('11111111-1111-4111-8111-111111111111')){throw 'Account flags or UUID description invalid'}
  [pscustomobject]@{Name=$Name;SID=[Security.Principal.SecurityIdentifier]::new('S-1-5-21-1-2-3-1002')}
 }
 function script:Get-GamePermissionService {param($p)
  $daemon='S-1-5-21-1-2-3-1001';if($script:changed){$daemon='S-1-5-21-1-2-3-1010'}
  [pscustomobject]@{Root=$script:root;Configuration=(Join-Path $script:root 'private\config.json');Executable=(Join-Path $script:root 'bin\daemon.exe');DaemonSID=$daemon}
 }
 function script:Get-GameAccountByName {param($name) if($script:exists){[pscustomobject]@{Name=$name;SID=[Security.Principal.SecurityIdentifier]::new('S-1-5-21-1-2-3-1099')}}}
 function script:New-GameAccountUser {param($Name,$Password,$UUID)
  $script:creates++
  if($Password -isnot [Security.SecureString] -or $UUID -ne '11111111-1111-4111-8111-111111111111'){throw 'Invalid argument forwarding'}
  if($script:case -eq 'create-fails'){throw 'Synthetic create failure'}
  if($script:case -eq 'create'){return (& $script:realCreate $Name $Password $UUID)}
  [pscustomobject]@{Name=$Name;SID=[Security.Principal.SecurityIdentifier]::new('S-1-5-21-1-2-3-1002')}
 }
 function script:Assert-GameAccountUser {param($Name)
  if($script:case -in @('validation-fails','rollback-fails')){throw 'Synthetic validation failure'}
  $sid='S-1-5-21-1-2-3-1002';if($script:case -eq 'identity-mismatch'){$sid='S-1-5-21-1-2-3-1003'};if($script:case -eq 'daemon-mismatch'){$sid='S-1-5-21-1-2-3-1001'}
  [pscustomobject]@{SID=[Security.Principal.SecurityIdentifier]::new($sid)}
 }
 function script:Remove-GameAccountUser {param($SID)
  if($SID -cne 'S-1-5-21-1-2-3-1002'){throw 'Tried deleting an unowned account'}
  $script:removes++;if($script:case -eq 'rollback-fails'){throw 'Synthetic removal failure'}
 }
} $Root $Case
$file=Join-Path $Root 'private\config.json'
if($Case -eq 'existing-profile'){$c=Get-Content -LiteralPath $file -Raw|ConvertFrom-Json;$c.profiles|Add-Member -NotePropertyName $id -NotePropertyValue @{};$c|ConvertTo-Json -Depth 10|Set-Content -LiteralPath $file}
if($Case -eq 'existing-data'){New-Item -ItemType Directory -Path (Join-Path $Root ('data\'+$id))|Out-Null}
if($Case -eq 'bad-name'){$name='..\Administrator'}
if($Case -eq 'bad-uuid'){$id='short'}
$before=[IO.File]::ReadAllText($file)
$unsafe=$Case -in @('existing-account','existing-profile','existing-data','bad-name','bad-uuid')
$failed=$false;try{$plan=Get-ServiceGameAccountPlan $Root $id $name}catch{if(-not $unsafe){throw};$failed=$true}
if($unsafe -and -not $failed){throw 'Unsafe plan accepted'}
$password=[Security.SecureString]::new()
try{
 if($Case -ne 'empty-password'){foreach($n in 1..24){$password.AppendChar([char]65)}}
 if($Case -eq 'overlong-password'){foreach($n in 1..1280){$password.AppendChar([char]65)}}
 if($Case -eq 'stale-account'){& $m {$script:exists=$true}}
 if($Case -eq 'stale-service'){& $m {$script:changed=$true}}
 if(-not $unsafe -and $Case -ne 'plan-only'){
  $failed=$false;try{$result=New-ServiceGameAccount $plan $password}catch{if($Case -eq 'create'){throw};$failed=$true;$message=$_.Exception.Message}
  if($Case -ne 'create' -and -not $failed){throw 'Expected creation failure'}
  if($Case -eq 'create'){
   if($result.username -cne '.\WwUnitGame' -or $result.run_as.sid -cne $result.sid -or $result.run_as.credential_target -cne ('WingsWindows/'+$id)){throw 'Wrong profile binding'}
   if(($result|ConvertTo-Json -Depth 5) -match 'AAAA'){throw 'Output contains password material'}
  }
  if($Case -eq 'rollback-fails' -and $message -notmatch 'rollback could not remove'){throw 'Uncertain rollback not surfaced'}
 }
}finally{$password.Dispose()}
$counts=& $m {@($script:creates,$script:removes)}
$expectedCreate=[int]($Case -in @('create','create-fails','validation-fails','identity-mismatch','daemon-mismatch','rollback-fails'))
$expectedRemove=[int]($Case -in @('validation-fails','identity-mismatch','daemon-mismatch','rollback-fails'))
if($counts[0] -ne $expectedCreate -or $counts[1] -ne $expectedRemove){throw 'Incorrect account mutation/rollback count'}
if([IO.File]::ReadAllText($file) -cne $before){throw 'Account helper rewrote configuration'}
Write-Output 'PASSED'
`
			file := filepath.Join(t.TempDir(), "account-test.ps1")
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
