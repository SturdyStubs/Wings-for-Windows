package setup_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"wings-windows/internal/testutil"
)

func runCredentialScript(t *testing.T, script string, args ...string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "credential-test.ps1")
	if err := os.WriteFile(file, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	command := append([]string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", file}, args...)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	output, err := testutil.PowerShell(ctx, command...).CombinedOutput()
	if err != nil {
		t.Fatalf("credential fixture: %v\n%s", err, output)
	}
}

func TestCredentialPipeFrames(t *testing.T) {
	module, err := filepath.Abs("../../scripts/CredentialBridge.psm1")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"roundtrip", "zero", "negative", "oversize", "truncated", "invalid utf8"} {
		t.Run(name, func(t *testing.T) {
			runCredentialScript(t, `param($Module,$Case)
$ErrorActionPreference='Stop'
Import-Module $Module -Force;Initialize-CredentialBridge
Add-Type -TypeDefinition @'
using System.IO.Pipes;
using System.Threading.Tasks;
public static class FixturePipeWriter {
 public static Task Send(PipeStream pipe, byte[] data, bool close) {return Task.Run(()=>{pipe.Write(data,0,data.Length);if(close)pipe.Dispose();});}
 public static Task Framed(PipeStream pipe,string text,System.Type bridge) {return Task.Run(()=>bridge.GetMethod("Send").Invoke(null,new object[]{pipe,text}));}
}
'@
$name='WingsFrameTest_'+[guid]::NewGuid().ToString('N')
$server=[IO.Pipes.NamedPipeServerStream]::new($name,[IO.Pipes.PipeDirection]::InOut,1,[IO.Pipes.PipeTransmissionMode]::Byte,[IO.Pipes.PipeOptions]::Asynchronous)
$client=[IO.Pipes.NamedPipeClientStream]::new('.',$name,[IO.Pipes.PipeDirection]::InOut,[IO.Pipes.PipeOptions]::Asynchronous)
try{
 $pending=$server.BeginWaitForConnection($null,$null);$client.Connect(1000);$server.EndWaitForConnection($pending)
 if([WingsWindows.CredentialPipe]::Peer($server,$true) -ne $PID -or [WingsWindows.CredentialPipe]::Peer($client,$false) -ne $PID){throw 'Peer PID mismatch'}
 if($Case -eq 'roundtrip'){
  $text='fixture-'+[char]0x2603
  $write=[FixturePipeWriter]::Framed($client,$text,[WingsWindows.CredentialPipe])
  if([WingsWindows.CredentialPipe]::Receive($server) -cne $text){throw 'UTF8 request changed'}
  if(-not $write.Wait(1000)){throw 'Request writer stuck'}
  $write=[FixturePipeWriter]::Framed($server,'reply',[WingsWindows.CredentialPipe])
  if([WingsWindows.CredentialPipe]::Receive($client) -cne 'reply'){throw 'Reply changed'}
  if(-not $write.Wait(1000)){throw 'Reply writer stuck'}
 }else{
  $size=0;switch($Case){'negative'{$size=-1};'oversize'{$size=32769};'truncated'{$size=5};'invalid utf8'{$size=1}}
  [byte[]]$bytes=[BitConverter]::GetBytes([int]$size)
  if($Case -eq 'truncated'){$bytes+=[byte]65}
  if($Case -eq 'invalid utf8'){$bytes+=[byte]255}
  $write=[FixturePipeWriter]::Send($client,$bytes,($Case -eq 'truncated'))
  $rejected=$false;try{[WingsWindows.CredentialPipe]::Receive($server)|Out-Null}catch{$rejected=$true}
  if(-not $rejected){throw 'Malformed credential frame accepted'}
  if(-not $write.Wait(1000)){throw 'Malformed frame writer stuck'}
 }
}finally{$client.Dispose();$server.Dispose()}
`, "-Module", module, "-Case", name)
		})
	}
}

func TestServiceCredentialPlan(t *testing.T) {
	module, err := filepath.Abs("../../scripts/CredentialBridge.psm1")
	if err != nil {
		t.Fatal(err)
	}
	node, err := filepath.Abs("../../scripts/NodeSetup.psm1")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"store", "inspect", "remove", "running", "daemon mismatch", "game mismatch", "wrong target", "missing worker", "wrong registration", "invalid uuid", "missing profile", "newline uuid", "rcon store", "rcon inspect", "rcon remove", "rcon wrong target", "rcon both", "rcon missing store"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			for _, dir := range []string{"bin/scripts", "private"} {
				if err := os.MkdirAll(filepath.Join(root, dir), 0700); err != nil {
					t.Fatal(err)
				}
			}
			for _, file := range []string{"bin/wings-windows.exe", "bin/scripts/CredentialTask.ps1", "bin/scripts/CredentialBridge.psm1", "bin/scripts/Store-GameCredential.ps1", "bin/scripts/Store-RCONCredential.ps1"} {
				if err := os.WriteFile(filepath.Join(root, file), nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			id := "11111111-1111-4111-8111-111111111111"
			target := "WingsWindows/" + id
			if name == "wrong target" {
				target += "-wrong"
			}
			profiles := map[string]any{id: map[string]any{"run_as": map[string]string{"sid": "S-1-5-21-1-2-3-1002", "credential_target": target}}}
			if strings.HasPrefix(name, "rcon ") {
				rcon := map[string]string{"password_credential_target": "WingsWindowsRCON/" + id}
				if name == "rcon wrong target" {
					rcon["password_credential_target"] += "-wrong"
				}
				if name == "rcon both" {
					rcon["password_environment"] = "WINGS_RCON_TEST"
				}
				profiles[id].(map[string]any)["source_rcon"] = rcon
			}
			if name == "rcon missing store" {
				os.Remove(filepath.Join(root, "bin/scripts/Store-RCONCredential.ps1"))
			}
			if name == "missing profile" {
				profiles = map[string]any{}
			}
			body, _ := json.Marshal(map[string]any{"profiles": profiles})
			if err := os.WriteFile(filepath.Join(root, "private/config.json"), body, 0600); err != nil {
				t.Fatal(err)
			}
			if name == "missing worker" {
				if err := os.Remove(filepath.Join(root, "bin/scripts/CredentialTask.ps1")); err != nil {
					t.Fatal(err)
				}
			}
			runCredentialScript(t, `param($Module,$Node,$Root,$Case)
$ErrorActionPreference='Stop'
Import-Module $Node -Force
$m=Import-Module $Module -Force -PassThru
& $m {
 param($root,$case)
 $script:fixtureRoot=$root;$script:fixtureCase=$case
 function script:Get-NodeLocalAccount {param($Name)
  $sid=[Security.Principal.WindowsIdentity]::GetCurrent().User
  if($Name -eq '.\FixtureGame'){$sid=[Security.Principal.SecurityIdentifier]::new('S-1-5-21-1-2-3-1002')}
  if($script:fixtureCase -eq 'daemon mismatch' -or ($Name -eq '.\FixtureGame' -and $script:fixtureCase -eq 'game mismatch')){$sid=[Security.Principal.SecurityIdentifier]::new('S-1-5-21-1-2-3-1003')}
  [pscustomobject]@{SID=$sid}
 }
 function script:Get-CimInstance {param($ClassName,$Filter)
  $state='Stopped';if($script:fixtureCase -eq 'running'){$state='Running'}
  $path='"'+(Join-Path $script:fixtureRoot 'bin\wings-windows.exe')+'" --config "'+(Join-Path $script:fixtureRoot 'private\config.json')+'"'
  if($script:fixtureCase -eq 'wrong registration'){$path='different.exe'}
  [pscustomobject]@{StartName=[Security.Principal.WindowsIdentity]::GetCurrent().Name;State=$state;PathName=$path}
 }
} $Root $Case
$password=ConvertTo-SecureString 'FixtureOnly1!' -AsPlainText -Force
$service=[PSCredential]::new('.\FixtureDaemon',$password);$game=[PSCredential]::new('.\FixtureGame',$password)
$operation='Store';if($Case -eq 'inspect'){$operation='Inspect';$game=$null};if($Case -eq 'remove'){$operation='Remove';$game=$null}
$id='11111111-1111-4111-8111-111111111111';if($Case -eq 'invalid uuid'){$id='short'};if($Case -eq 'newline uuid'){$id+=[char]10}
$purpose='Game';if($Case -like 'rcon *'){$purpose='RCON';$game=[PSCredential]::new('.\WingsRCONSecret-v1',$password);if($Case -eq 'rcon inspect'){$operation='Inspect';$game=$null};if($Case -eq 'rcon remove'){$operation='Remove';$game=$null}}
$before=(Get-Acl -LiteralPath $Root).Sddl
$accepted=$false;try{$plan=Get-ServiceCredentialPlan -InstallationRoot $Root -ServerUUID $id -ServiceCredential $service -GameCredential $game -Operation $operation -Purpose $purpose;$accepted=$true}catch{if($Case -in @('store','inspect','remove','rcon store','rcon inspect','rcon remove')){throw}}
if($accepted -ne ($Case -in @('store','inspect','remove','rcon store','rcon inspect','rcon remove'))){throw 'Unexpected planner acceptance'}
if((Get-Acl -LiteralPath $Root).Sddl -ne $before){throw 'Planning changed DACLs'}
$password.Dispose()
`, "-Module", module, "-Node", node, "-Root", root, "-Case", name)
		})
	}
}

func TestServiceCredentialRegistrationFailure(t *testing.T) {
	module, _ := filepath.Abs("../../scripts/CredentialBridge.psm1")
	node, _ := filepath.Abs("../../scripts/NodeSetup.psm1")
	for _, name := range []string{"absent task", "partial task", "existing batch right", "right cleanup failure"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, "private"), 0700); err != nil {
				t.Fatal(err)
			}
			runCredentialScript(t, `param($Module,$Node,$Root,$Case)
# All account-right and scheduler operations below are fake; no host mutation.
$ErrorActionPreference='Continue'
Add-Type -TypeDefinition @'
namespace WingsWindows {
 public static class NodeSetup {
  public static bool Present,FailRemove;
  public static int Added,Removed;
  public static string[] Rights(string sid) {return Present?new string[]{"SeBatchLogonRight"}:new string[0];}
  public static void Right(string sid,string name,bool remove) {if(remove){if(FailRemove)throw new System.Exception("Synthetic right cleanup failure");Removed++;Present=false;}else{Added++;Present=true;}}
  public static void ValidatePath(string path) {}
 }
}
'@
if($Case -eq 'existing batch right'){[WingsWindows.NodeSetup]::Present=$true}
if($Case -eq 'right cleanup failure'){[WingsWindows.NodeSetup]::FailRemove=$true}
Import-Module $Node -Force
$m=Import-Module $Module -Force -PassThru
& $m {
 param($case)
 $script:fixtureCase=$case;$script:starts=0;$script:deleted=0
 function script:New-ScheduledTaskAction {[CmdletBinding()]param($Execute,$Argument) [pscustomobject]@{Execute=$Execute}}
 function script:New-ScheduledTaskSettingsSet {[CmdletBinding()]param($ExecutionTimeLimit,[switch]$AllowStartIfOnBatteries,[switch]$DontStopIfGoingOnBatteries) [pscustomobject]@{Limit=$ExecutionTimeLimit}}
 function script:Register-ScheduledTask {[CmdletBinding()]param($TaskName,$Action,$Settings,$User,$Password,$RunLevel) Write-Error 'Synthetic registration rejection'}
 function script:Start-ScheduledTask {[CmdletBinding()]param($TaskName) $script:starts++;throw 'Should not start an unregistered task'}
 function script:Get-ScheduledTask {[CmdletBinding()]param($TaskName)
  if($script:fixtureCase -eq 'partial task'){[pscustomobject]@{State='Ready'}}else{Write-Error 'Task absent' -ErrorId 'CmdletizationQuery_NotFound_TaskName'}
 }
 function script:Unregister-ScheduledTask {[CmdletBinding(SupportsShouldProcess=$true)]param($TaskName) $script:deleted++}
} $Case
$password=ConvertTo-SecureString 'FakeOnly1!' -AsPlainText -Force
$credential=[PSCredential]::new('.\FixtureDaemon',$password)
$plan=[pscustomobject]@{Root=$Root;DaemonSID='S-1-5-21-1-2-3-1001';Worker=(Join-Path $Root 'unused.ps1');Operation='Inspect'}
$failed=$false;try{Invoke-ServiceCredentialTask -Plan $plan -ServiceCredential $credential -Replace $false|Out-Null}catch{
 $failed=$true
 if($Case -eq 'right cleanup failure'){if($_.Exception.Message -notlike '*cleanup requires inspection*'){throw}}
 elseif($_.Exception.Message -notlike '*Synthetic registration rejection*'){throw}
}
if(-not $failed){throw 'Registration error was ignored'}
& $m {param($case)
 if($script:starts -ne 0){throw 'Registration failure continued into task start'}
 if($script:deleted -ne [int]($case -eq 'partial task')){throw 'Partial task cleanup mismatch'}
} $Case
$expected=1;if($Case -eq 'existing batch right'){$expected=0}
if([WingsWindows.NodeSetup]::Added -ne $expected){throw 'Unexpected batch grant'}
if($Case -ne 'right cleanup failure' -and [WingsWindows.NodeSetup]::Removed -ne $expected){throw 'Unexpected right cleanup'}
$lock=[IO.File]::Open((Join-Path $Root 'private\credential-provisioning.lock'),[IO.FileMode]::Open,[IO.FileAccess]::ReadWrite,[IO.FileShare]::None);$lock.Dispose()
$password.Dispose()
`, "-Module", module, "-Node", node, "-Root", root, "-Case", name)
		})
	}
}
