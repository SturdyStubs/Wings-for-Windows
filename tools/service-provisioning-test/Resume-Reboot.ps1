#Requires -RunAsAdministrator
param([string]$Root)
$ErrorActionPreference='Stop';$ProgressPreference='SilentlyContinue'
if($Root -notmatch '^C:\\WingsWindowsTesting\\scm-([0-9a-f]{8})$'){throw 'Invalid disposable reboot root'}
$suffix=$Matches[1];$secret=Join-Path $Root 'private\reboot\state.json'
$state=Get-Content -LiteralPath $secret -Raw|ConvertFrom-Json
if($state.root -ne $Root -or $state.mock_task -ne ('WingsRebootMock_'+$suffix) -or $state.resume_task -ne ('WingsRebootResume_'+$suffix)){throw 'Reboot state identity mismatch'}
$gameName='WwScmG_'+$suffix;$daemonName='WwScmD_'+$suffix
foreach($entry in @(@{Name=$gameName;SID=$state.game_sid},@{Name=$daemonName;SID=$state.daemon_sid})){if((Get-LocalUser -Name $entry.Name).SID.Value -ne $entry.SID){throw 'Refusing unexpected account'}}
$service=Get-CimInstance Win32_Service -Filter "Name='WingsWindowsNative'"
$expected='"'+$Root+'\bin\wings-windows.exe" --config "'+$Root+'\private\config.json"'
if($service.PathName -ne $expected -or (Get-FileHash -LiteralPath (Join-Path $Root 'bin\wings-windows.exe')).Hash -ne $state.daemon_sha256){throw 'Refusing unexpected service executable'}
Import-Module (Join-Path $Root 'bin\scripts\NodeSetup.psm1') -Force;Initialize-NodeSetupNative
$config=Get-Content -LiteralPath (Join-Path $Root 'private\config.json') -Raw|ConvertFrom-Json
$base='http://'+$config.listen+'/api/servers/'+$state.server_id
function API([string]$Path=''){Invoke-RestMethod -Uri ($base+$Path) -Headers @{Authorization=('Bearer '+$config.token)} -TimeoutSec 3}
function Power([string]$Action){Invoke-RestMethod -Uri ($base+'/power') -Headers @{Authorization=('Bearer '+$config.token)} -Method Post -ContentType application/json -Body (@{action=$Action}|ConvertTo-Json) -TimeoutSec 5|Out-Null}
function Wait-State([string]$Expected){$until=[DateTime]::UtcNow.AddSeconds(25);do{try{if((API).state -eq $Expected){return}}catch{};Start-Sleep -Milliseconds 100}while([DateTime]::UtcNow -lt $until);throw 'State timeout'}
$report=[ordered]@{version=$state.version;source_revision=$state.source_revision;pre_boot_utc=$state.boot_utc;post_boot_utc=$null;passed=$false;checks=[ordered]@{};error=$null;cleanup=[ordered]@{}}
$credentialRemoved=$false;$stage='boot';$credential=$null
try{
 $boot=(Get-CimInstance Win32_OperatingSystem).LastBootUpTime.ToUniversalTime();$report.post_boot_utc=$boot.ToString('o')
 if($boot -le [DateTime]::Parse($state.boot_utc)){throw 'Host did not reboot'};$report.checks.actual_boot_changed='passed'
 $stage='automatic_service';$until=[DateTime]::UtcNow.AddSeconds(180)
 do{try{$server=API;break}catch{Start-Sleep -Seconds 2}}while([DateTime]::UtcNow -lt $until)
 if(-not $server){throw 'Automatic service unavailable'}
 $service=Get-CimInstance Win32_Service -Filter "Name='WingsWindowsNative'"
 $owner=Invoke-CimMethod -InputObject (Get-CimInstance Win32_Process -Filter ('ProcessId='+$service.ProcessId)) -MethodName GetOwnerSid
 if($owner.Sid -ne $state.daemon_sid -or $service.StartMode -ne 'Auto'){throw 'Service identity/start mode mismatch'};$report.checks.delayed_automatic_nonadmin_service='passed'
 if($state.expect_restore){
  Wait-State 'running';$server=API
  if(-not $server.native_restore.loaded_running -or -not $server.native_restore.desired_running){throw 'Running intent not restored'}
  $report.checks.running_game_restored_after_boot='passed'
 }else{
 if($server.state -ne 'offline' -or $server.native_recovery.pending){throw 'Unexpected automatic game/timer restoration'}
 foreach($process in Get-CimInstance Win32_Process -Filter "Name='wings-windows-demo.exe'"){$owner=Invoke-CimMethod -InputObject $process -MethodName GetOwnerSid;if($owner.Sid -eq $state.game_sid){throw 'Game started automatically'}}
 $report.checks.games_remain_offline_after_boot='passed'
 }
 $stage='persistent_history'
 if('ECHO:'+$state.marker -notin (API '/logs').data -or $server.native_console_logs.restored_lines -le 0){throw 'Console history not restored'};$report.checks.console_history_after_boot='passed'
 if($server.native_recovery.attempts_in_window -ne $state.expected_attempts -or $server.native_recovery.restored_attempts -ne ($state.expected_attempts-[int][bool]$state.expect_restore) -or -not $server.native_recovery.persistent_history){throw 'Budget not restored'};$report.checks.retry_budget_after_boot='passed'
 $stage='cold_game_credential';if(-not $state.expect_restore){Power 'start';Wait-State 'running'}
 $owned=@();foreach($process in Get-CimInstance Win32_Process -Filter "Name='wings-windows-demo.exe'"){$owner=Invoke-CimMethod -InputObject $process -MethodName GetOwnerSid;if($owner.Sid -eq $state.game_sid){$owned+=$process.ProcessId}}
 if($owned.Count -ne 2){throw 'Cold credential did not launch separate root/child'};$report.checks.cold_profile_credential_game_launch='passed'
 Power 'stop';Wait-State 'offline';foreach($id in $owned){if(Get-Process -Id $id -ErrorAction SilentlyContinue){throw 'Game tree survived stop'}};$report.checks.post_boot_game_tree_stop='passed'
 $report.passed=$true
}catch{$report.error='Reboot fixture failed at '+$stage}
finally{
 try{
  Stop-Service WingsWindowsNative -ErrorAction SilentlyContinue
  $s=Get-Service WingsWindowsNative;try{$s.WaitForStatus('Stopped',[TimeSpan]::FromSeconds(20))}finally{$s.Dispose()}
  Add-Type -AssemblyName System.Security
  $bytes=[Security.Cryptography.ProtectedData]::Unprotect([Convert]::FromBase64String($state.protected_password),$null,[Security.Cryptography.DataProtectionScope]::LocalMachine)
  try{$password=ConvertTo-SecureString ([Text.Encoding]::UTF8.GetString($bytes)) -AsPlainText -Force}finally{[Array]::Clear($bytes,0,$bytes.Length)}
  $credential=[PSCredential]::new($state.service_user,$password)
  $helper=Join-Path $Root 'bin\scripts\Set-ServiceGameCredential.ps1'
  $removed=& $helper -InstallationRoot $Root -ServerUUID $state.server_id -ServiceCredential $credential -Remove
  if(-not $removed.removed){throw 'Credential not removed'}
  $status=& $helper -InstallationRoot $Root -ServerUUID $state.server_id -ServiceCredential $credential -Inspect
  if($status.present){throw 'Credential remains'}
  $credentialRemoved=$true;$report.cleanup.credential_removed=$true
  & "$env:SystemRoot\System32\sc.exe" delete WingsWindowsNative|Out-Null;if($LASTEXITCODE -ne 0){throw 'Service deletion failed'};$report.cleanup.service_removed=$true
 }catch{$report.cleanup.credential_or_service_error=$true;$report.passed=$false}
 foreach($task in @($state.mock_task,$state.resume_task)){try{if($task -eq $state.mock_task){Stop-ScheduledTask -TaskName $task -ErrorAction SilentlyContinue};Unregister-ScheduledTask -TaskName $task -Confirm:$false -ErrorAction Stop}catch{$report.cleanup.task_error=$true;$report.passed=$false}}
 foreach($process in Get-CimInstance Win32_Process -Filter "Name='mock-panel.exe'"){if($process.ExecutablePath -eq (Join-Path $state.staging 'mock-panel.exe') -and $process.CommandLine.Contains($Root)){Stop-Process -Id $process.ProcessId -ErrorAction SilentlyContinue}}
 $report.cleanup.mock_and_reboot_tasks_removed=(-not (Get-ScheduledTask -TaskName $state.mock_task -ErrorAction SilentlyContinue) -and -not (Get-ScheduledTask -TaskName $state.resume_task -ErrorAction SilentlyContinue))
 if($credentialRemoved -and -not (Get-Service WingsWindowsNative -ErrorAction SilentlyContinue)){
  try{
   & (Join-Path $Root 'bin\Register-EventSource.ps1') -Remove|Out-Null;$report.cleanup.event_source_removed=$true
   foreach($sid in @($state.daemon_sid,$state.game_sid)){foreach($right in @([WingsWindows.NodeSetup]::Rights($sid))){[WingsWindows.NodeSetup]::Right($sid,$right,$true)}}
   Remove-LocalUser -Name $daemonName;Remove-LocalUser -Name $gameName;$report.cleanup.accounts_and_rights_removed=$true
   Remove-Item -LiteralPath $secret;$report.cleanup.encrypted_cleanup_state_removed=$true
  }catch{$report.cleanup.account_or_state_error=$true;$report.passed=$false}
 }
 foreach($required in @('credential_removed','service_removed','mock_and_reboot_tasks_removed','event_source_removed','accounts_and_rights_removed','encrypted_cleanup_state_removed')){if(-not $report.cleanup[$required]){$report.passed=$false}}
 if($credential){$credential.Password.Dispose()}
 [IO.File]::WriteAllText((Join-Path $Root 'reboot-report.json'),($report|ConvertTo-Json -Depth 8),[Text.UTF8Encoding]::new($false))
}
