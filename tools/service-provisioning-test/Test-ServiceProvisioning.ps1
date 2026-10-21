#Requires -RunAsAdministrator
param([string]$Staging,[string]$Package,[string]$Root,[string]$GameName,[string]$DaemonName,[string]$ServerID,[switch]$TestCrashRecovery,[switch]$TestConsoleLogs,[switch]$TestServiceEvents,[switch]$TestPersistentRecovery,[switch]$TestEggText)
$ErrorActionPreference='Stop';$ProgressPreference='SilentlyContinue'
if($TestPersistentRecovery -and (-not $TestCrashRecovery -or -not $TestConsoleLogs)){throw 'Persistent recovery fixture requires TestCrashRecovery and TestConsoleLogs'}
if($Root -notmatch '^C:\\WingsWindowsTesting\\scm-([0-9a-f]{8})$'){throw 'Use a fresh C:\WingsWindowsTesting\scm-<eight-hex> fixture root'}
$suffix=$Matches[1]
if($GameName -ne ('WwScmG_'+$suffix) -or $DaemonName -ne ('WwScmD_'+$suffix)){throw 'Fixture account names must match the root suffix'}
if($ServerID -notmatch '^[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}$'){throw 'Use a full random server UUID'}
$report=[ordered]@{passed=$false;checks=[ordered]@{};error=$null;cleanup=[ordered]@{}}
$gameCreated=$false;$serviceCreated=$false;$credentialSeeded=$false;$mock=$null;$daemonSid=$null;$gameSid=$null
$eventSourceCreated=$false;$eventStarted=Get-Date;$eventKey='HKLM:\SYSTEM\CurrentControlSet\Services\EventLog\Application\WingsWindowsNative'
$eventScript=Join-Path $Package 'scripts\Register-EventSource.ps1'
$installer=Join-Path $Staging 'Install-Service.ps1';Import-Module (Join-Path $Staging 'NodeSetup.psm1') -Force;Initialize-NodeSetupNative
if(Get-Service WingsWindowsNative -ErrorAction SilentlyContinue){throw 'A Wings service already exists; refusing to touch it'}
foreach($name in @($GameName,$DaemonName)){if(Get-LocalUser -Name $name -ErrorAction SilentlyContinue){throw 'Test account exists'}}
if(Test-Path -LiteralPath $Root){throw 'Test root exists'}
if($TestServiceEvents -and (Test-Path -LiteralPath $eventKey)){throw 'Event source exists; refusing to touch it'}
function New-TestPassword {$b=New-Object byte[] 32;$r=[Security.Cryptography.RandomNumberGenerator]::Create();try{$r.GetBytes($b);ConvertTo-SecureString ('Aa1!'+[Convert]::ToBase64String($b)) -AsPlainText -Force}finally{[Array]::Clear($b,0,$b.Length);$r.Dispose()}}
function Invoke-Bootstrap([switch]$Cleanup) {
 $script=Join-Path $Root 'bin\scripts\Set-ServiceGameCredential.ps1'
 $options=@{InstallationRoot=$Root;ServerUUID=$ServerID;ServiceCredential=$serviceCredential}
 if($Cleanup){
  $removed=& $script @options -Remove
  if(-not $removed.removed){throw 'Expected credential removal'}
  $status=& $script @options -Inspect
  if($status.present){throw 'Credential remains after removal'}
  $report.checks.public_credential_remove_inspect='passed'
 }else{
  $wrongPassword=New-TestPassword
  try{
   $badOptions=@{InstallationRoot=$Root;ServerUUID=$ServerID;ServiceCredential=[PSCredential]::new($serviceCredential.UserName,$wrongPassword)}
   $failed=$false;try{& $script @badOptions -Inspect|Out-Null}catch{$failed=$true}
   if(-not $failed){throw 'Public helper accepted an incorrect service password'}
  }finally{$wrongPassword.Dispose()}
  if('SeBatchLogonRight' -in @([WingsWindows.NodeSetup]::Rights($daemonSid)) -or @(Get-ScheduledTask|Where-Object{$_.TaskName -like 'WingsCredential_*'}).Count){throw 'Bad service password left task/right state'}
  $report.checks.public_bad_service_password_cleanup='passed'
  $lock=[IO.File]::Open((Join-Path $Root 'private\credential-provisioning.lock'),[IO.FileMode]::OpenOrCreate,[IO.FileAccess]::ReadWrite,[IO.FileShare]::None)
  try{$failed=$false;try{& $script @options -Inspect|Out-Null}catch{$failed=$true};if(-not $failed){throw 'Concurrent helper bypassed the operation lock'}}finally{$lock.Dispose()}
  $report.checks.public_operation_lock_rejected='passed'
  [WingsWindows.NodeSetup]::Right($daemonSid,'SeBatchLogonRight',$false)
  try{& $script @options -Inspect|Out-Null;if('SeBatchLogonRight' -notin @([WingsWindows.NodeSetup]::Rights($daemonSid))){throw 'Helper removed a preexisting batch right'}}finally{[WingsWindows.NodeSetup]::Right($daemonSid,'SeBatchLogonRight',$true)}
  $report.checks.public_preexisting_batch_right_preserved='passed'
  $gameCredential=[PSCredential]::new(('.\'+$GameName),$gamePassword)
  $created=& $script @options -GameCredential $gameCredential
  if(-not $created.present -or $created.username -ne $gameCredential.UserName){throw 'Public credential creation failed'}
  $status=& $script @options -Inspect
  if(-not $status.present -or $status.username -ne $gameCredential.UserName){throw 'Public credential inspection failed'}
  $failed=$false;try{& $script @options -GameCredential $gameCredential | Out-Null}catch{$failed=$true}
  if(-not $failed){throw 'Public helper overwrote an existing target'}
  $report.checks.public_credential_overwrite_rejected='passed'
  $failed=$false;try{& $script @options -GameCredential $serviceCredential | Out-Null}catch{$failed=$true}
  if(-not $failed){throw 'Public helper accepted wrong pinned game SID'}
  $report.checks.public_credential_wrong_identity_rejected='passed'
  $replaced=& $script @options -GameCredential $gameCredential -Replace
  if(-not $replaced.present){throw 'Public same-account replacement failed'}
  $report.checks.public_credential_replace_inspect='passed'
 }
 if('SeBatchLogonRight' -in @([WingsWindows.NodeSetup]::Rights($daemonSid))){throw 'Public helper left an added batch right'}
 if(@(Get-ScheduledTask|Where-Object{$_.TaskName -like 'WingsCredential_*'}).Count){throw 'Public helper left a task'}
}
function Get-NodeAPI([string]$Path) {Invoke-RestMethod -Uri ('http://127.0.0.1:'+$nodePort+$Path) -Headers @{Authorization=('Bearer '+$nodeToken)} -TimeoutSec 3}
function Set-Power([string]$Action) {Invoke-RestMethod -Uri ('http://127.0.0.1:'+$nodePort+'/api/servers/'+$ServerID+'/power') -Headers @{Authorization=('Bearer '+$nodeToken)} -Method Post -ContentType application/json -Body (@{action=$Action}|ConvertTo-Json) -TimeoutSec 5 | Out-Null}
function Wait-State([string]$Expected) {$until=[DateTime]::UtcNow.AddSeconds(12);do{try{$state=(Get-NodeAPI ('/api/servers/'+$ServerID)).state;if($state -eq $Expected){return}}catch{};Start-Sleep -Milliseconds 100}while([DateTime]::UtcNow -lt $until);$logs=Get-NodeAPI ('/api/servers/'+$ServerID+'/logs');throw ('State timeout '+$Expected+': '+($logs|ConvertTo-Json -Compress))}
try {
 if($TestServiceEvents){
  & $eventScript -WhatIf
  if(Test-Path -LiteralPath $eventKey){throw 'WhatIf registered an Event Log source'}
  & $eventScript;$eventSourceCreated=$true
  & $eventScript
  if((Get-ItemProperty -LiteralPath $eventKey).WingsWindowsManaged -ne 1){throw 'Event source registration missing marker'}
  $report.checks.event_source_registration_idempotent_whatif='passed'
 }
 $gamePassword=New-TestPassword;$daemonPassword=New-TestPassword;$serviceCredential=[PSCredential]::new(('.\'+$DaemonName),$daemonPassword)
 New-LocalUser -Name $GameName -Password $gamePassword -Description 'Disposable SCM game account' | Out-Null;$gameCreated=$true;$gameSid=(Get-LocalUser -Name $GameName).SID.Value
 foreach($dir in @($Root,(Join-Path $Root 'bin'),(Join-Path $Root 'private'),(Join-Path $Root 'data'))){New-Item -ItemType Directory -Path $dir|Out-Null}
 foreach($file in @('wings-windows.exe','wings-windows-demo.exe')){Copy-Item -LiteralPath (Join-Path $Package $file) -Destination (Join-Path $Root ('bin\'+$file))}
 Copy-Item -LiteralPath (Join-Path $Package 'scripts\Store-GameCredential.ps1') -Destination (Join-Path $Root 'bin\Store-GameCredential.ps1')
 New-Item -ItemType Directory -Path (Join-Path $Root 'bin\scripts') | Out-Null
 foreach($file in @('Store-GameCredential.ps1','Set-ServiceGameCredential.ps1','CredentialBridge.psm1','CredentialTask.ps1','NodeSetup.psm1')){Copy-Item -LiteralPath (Join-Path $Staging $file) -Destination (Join-Path $Root ('bin\scripts\'+$file))}
 $first=[Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback,0);$second=[Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback,0);$first.Start();$second.Start();$panelPort=$first.LocalEndpoint.Port;$nodePort=$second.LocalEndpoint.Port;$first.Stop();$second.Stop()
 $b=New-Object byte[] 32;$rng=[Security.Cryptography.RandomNumberGenerator]::Create();try{$rng.GetBytes($b);$nodeToken=[Convert]::ToBase64String($b)}finally{$rng.Dispose()}
 $profiles=@{};$profiles[$ServerID]=[ordered]@{schema_version=1;executable=(Join-Path $Root 'bin\wings-windows-demo.exe');arguments=@('spawn');allowed_environment=@();stop_command='stop';ready_pattern='^SERVER_READY$';memory_mb=128;run_as=@{sid=$gameSid;credential_target=('WingsWindows/'+$ServerID)}}
 if($TestCrashRecovery){$profiles[$ServerID].crash_recovery=@{enabled=$true;max_restarts=2;window_seconds=60;delay_seconds=2}}
 $config=[ordered]@{listen=('127.0.0.1:'+$nodePort);panel_url=('http://127.0.0.1:'+$panelPort);node_uuid=[guid]::NewGuid().ToString();token_id='scm-fixture';token=$nodeToken;data_directory=(Join-Path $Root 'data');allow_insecure_loopback=$true;tls=@{cert='';key=''};profiles=$profiles}
 if($TestConsoleLogs){$config.console_logs=@{directory=(Join-Path $Root 'logs\console');max_file_bytes=1048576;max_files=2};$profiles[$ServerID].allowed_environment=@('LOG_PROBE_PATH')}
 if($TestPersistentRecovery){$profiles[$ServerID].crash_recovery.persist_history=$true;$profiles[$ServerID].crash_recovery.window_seconds=3600;$profiles[$ServerID].allowed_environment+=@('RECOVERY_PROBE_PATH')}
 if($TestEggText){$profiles[$ServerID].use_egg_process_configuration=$true;$profiles[$ServerID].egg_config_files=@('server.cfg')}
 $configuration=Join-Path $Root 'private\config.json';$config|ConvertTo-Json -Depth 10|Set-Content -LiteralPath $configuration -Encoding Ascii
 $exe=Join-Path $Root 'bin\wings-windows.exe'
 # Force a post-ACL failure once to validate compensation on real VM state.
 $originalACL=(Get-Acl -LiteralPath $Root).Sddl
 $originalFiles=@{};foreach($item in @(Get-Item -LiteralPath $Root)+@(Get-ChildItem -LiteralPath $Root -Recurse -Force)){$originalFiles[$item.FullName]=(Get-Acl -LiteralPath $item.FullName).Sddl}
 function global:New-Service {param($Name,$DisplayName,$Description,$BinaryPathName,$StartupType,$Credential) throw 'Synthetic registration failure'}
 $failed=$false
 try {& $installer -Executable $exe -Configuration $configuration -Credential $serviceCredential -InstallationRoot $Root -CreateLocalAccount -ConfigurePermissions | Out-Null}catch{if($_.Exception.Message -notlike '*Synthetic registration failure*'){throw};$failed=$true}finally{Remove-Item Function:\New-Service}
 if(-not $failed -or (Get-LocalUser -Name $DaemonName -ErrorAction SilentlyContinue) -or (Get-Service WingsWindowsNative -ErrorAction SilentlyContinue) -or (Get-Acl -LiteralPath $Root).Sddl -ne $originalACL -or @([WingsWindows.NodeSetup]::Rights($gameSid)).Count -ne 0){throw 'Installer rollback did not restore expected state'}
 foreach($path in $originalFiles.Keys){if((Get-Acl -LiteralPath $path).Sddl -ne $originalFiles[$path]){throw ('Rollback changed original DACL: '+$path)}}
 $report.checks.installer_rollback='passed'
 function global:Start-Service {param($Name) throw 'Synthetic start failure'}
 $failed=$false
 try {& $installer -Executable $exe -Configuration $configuration -Credential $serviceCredential -InstallationRoot $Root -CreateLocalAccount -ConfigurePermissions -Start | Out-Null}catch{if($_.Exception.Message -notlike '*Synthetic start failure*'){throw};$failed=$true}finally{Remove-Item Function:\Start-Service}
 if(-not $failed -or (Get-LocalUser -Name $DaemonName -ErrorAction SilentlyContinue) -or (Get-Service WingsWindowsNative -ErrorAction SilentlyContinue) -or @([WingsWindows.NodeSetup]::Rights($gameSid)).Count -ne 0){throw 'Start failure rollback did not restore expected state'}
 foreach($path in $originalFiles.Keys){if((Get-Acl -LiteralPath $path).Sddl -ne $originalFiles[$path]){throw ('Start rollback changed original DACL: '+$path)}}
 $report.checks.installer_start_failure_rollback='passed'
 & $installer -Executable $exe -Configuration $configuration -Credential $serviceCredential -InstallationRoot $Root -CreateLocalAccount -ConfigurePermissions | Out-Null;$serviceCreated=$true
 $daemonSid=(Get-LocalUser -Name $DaemonName).SID.Value
 $rights=@([WingsWindows.NodeSetup]::Rights($daemonSid));foreach($right in @('SeServiceLogonRight','SeAssignPrimaryTokenPrivilege','SeIncreaseQuotaPrivilege')){if($right -notin $rights){throw 'Required daemon right missing'}}
 if('SeBatchLogonRight' -notin @([WingsWindows.NodeSetup]::Rights($gameSid))){throw 'Required game batch right missing'}
 $report.checks.installer_account_rights_acls='passed'
 Invoke-Bootstrap;$credentialSeeded=$true;$report.checks.credential_bootstrap='passed'
 $mock=Start-Process -FilePath (Join-Path $Staging 'mock-panel.exe') -ArgumentList @($configuration) -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $Staging 'panel-out.log') -RedirectStandardError (Join-Path $Staging 'panel-error.log')
 Start-Sleep -Milliseconds 300
 Start-Service WingsWindowsNative
 Wait-State 'offline';$report.checks.scm_service_start_and_api='passed'
 $serviceInfo=Get-CimInstance Win32_Service -Filter "Name='WingsWindowsNative'"
 $owner=Invoke-CimMethod -InputObject (Get-CimInstance Win32_Process -Filter ('ProcessId='+$serviceInfo.ProcessId)) -MethodName GetOwnerSid
 if($owner.Sid -ne $daemonSid){throw 'SCM daemon SID mismatch'};$report.checks.actual_service_identity='passed'
 if($TestEggText){
  Set-Power 'start'
  $until=[DateTime]::UtcNow.AddSeconds(5);$failed=$false
  do{$failed=(@((Get-NodeAPI ('/api/servers/'+$ServerID+'/logs')).data|Where-Object {$_ -like '*egg file 1 preflight failed*'}).Count -gt 0);if(-not $failed){Start-Sleep -Milliseconds 50}}while(-not $failed -and [DateTime]::UtcNow -lt $until)
  if(-not $failed -or (Get-NodeAPI ('/api/servers/'+$ServerID)).state -ne 'offline'){throw 'Missing approved egg file did not reject startup'}
  $report.checks.egg_text_missing_file_rejects_launch='passed'
  $eggPath=Join-Path $Root ('data\'+$ServerID+'\server.cfg')
  [IO.File]::WriteAllText($eggPath,"# keep`r`nserver.name old`r`nserver.port 1`r`n",[Text.UTF8Encoding]::new($false))
  $eggACL=(Get-Acl -LiteralPath $eggPath).Sddl
 }
 Set-Power 'start';Wait-State 'running';$report.checks.separate_game_start='passed'
 if($TestEggText){
  $expected="# keep`nserver.name ""Windows Caf$([char]0xe9)""`nserver.port 28015`n"
  if([IO.File]::ReadAllText($eggPath) -ne $expected -or (Get-Acl -LiteralPath $eggPath).Sddl -ne $eggACL){throw 'SCM text rewrite content or ACL differs'}
  $report.checks.egg_text_daemon_rewrite_preserves_acl='passed'
 }

 Invoke-RestMethod -Uri ('http://127.0.0.1:'+$nodePort+'/api/servers/'+$ServerID+'/commands') -Headers @{Authorization=('Bearer '+$nodeToken)} -Method Post -ContentType application/json -Body (@{commands=@('SCM console test')}|ConvertTo-Json) | Out-Null
 $deadline=[DateTime]::UtcNow.AddSeconds(5);$echoed=$false
 do{$log=Get-NodeAPI ('/api/servers/'+$ServerID+'/logs');$echoed='ECHO:SCM console test' -in $log.data;if(-not $echoed){Start-Sleep -Milliseconds 100}}while(-not $echoed -and [DateTime]::UtcNow -lt $deadline)
 if(-not $echoed){throw 'Service console echo missing'};$report.checks.service_console='passed'
 if($TestConsoleLogs){
  Invoke-RestMethod -Uri ('http://127.0.0.1:'+$nodePort+'/api/servers/'+$ServerID+'/commands') -Headers @{Authorization=('Bearer '+$nodeToken)} -Method Post -ContentType application/json -Body '{"commands":["probe-log-access"]}'|Out-Null
  $until=[DateTime]::UtcNow.AddSeconds(5);$denied=$false
  do{$denied='LOG_PROBE_DENIED' -in (Get-NodeAPI ('/api/servers/'+$ServerID+'/logs')).data;if(-not $denied){Start-Sleep -Milliseconds 50}}while(-not $denied -and [DateTime]::UtcNow -lt $until)
  if(-not $denied){throw 'Actual game token did not report denial opening the console log'}
  $report.checks.console_log_game_read_denied='passed'
 }
 if($TestPersistentRecovery){
  Invoke-RestMethod -Uri ('http://127.0.0.1:'+$nodePort+'/api/servers/'+$ServerID+'/commands') -Headers @{Authorization=('Bearer '+$nodeToken)} -Method Post -ContentType application/json -Body '{"commands":["probe-recovery-access"]}'|Out-Null
  $until=[DateTime]::UtcNow.AddSeconds(5);$denied=$false
  do{$denied='RECOVERY_PROBE_DENIED' -in (Get-NodeAPI ('/api/servers/'+$ServerID+'/logs')).data;if(-not $denied){Start-Sleep -Milliseconds 50}}while(-not $denied -and [DateTime]::UtcNow -lt $until)
  if(-not $denied){throw 'Actual game token did not report denial opening recovery state'}
  $report.checks.recovery_state_game_read_denied='passed'
 }
 if($TestCrashRecovery){
  function Crash-Game {Invoke-RestMethod -Uri ('http://127.0.0.1:'+$nodePort+'/api/servers/'+$ServerID+'/commands') -Headers @{Authorization=('Bearer '+$nodeToken)} -Method Post -ContentType application/json -Body '{"commands":["crash"]}'|Out-Null}
  function Wait-RecoveryPending {$until=[DateTime]::UtcNow.AddSeconds(5);do{$state=Get-NodeAPI ('/api/servers/'+$ServerID);if($state.native_recovery.pending){return};Start-Sleep -Milliseconds 50}while([DateTime]::UtcNow -lt $until);throw 'Recovery was not scheduled'}
  $oldProcesses=@();foreach($p in Get-CimInstance Win32_Process -Filter "Name='wings-windows-demo.exe'"){$o=Invoke-CimMethod -InputObject $p -MethodName GetOwnerSid;if($o.Sid -eq $gameSid){$oldProcesses+=$p.ProcessId}}
  Crash-Game;Wait-RecoveryPending;Wait-State 'running'
  if((Get-NodeAPI ('/api/servers/'+$ServerID)).native_recovery.attempts_in_window -ne 1){throw 'Unexpected automatic restart count'}
  foreach($oldProcess in $oldProcesses){if(Get-Process -Id $oldProcess -ErrorAction SilentlyContinue){throw 'Old game tree survived crash recovery'}}
  $report.checks.crash_recovery_restarts_and_cleans_old_tree='passed'
  Crash-Game;Wait-RecoveryPending;Set-Power 'kill';Wait-State 'offline';Start-Sleep -Milliseconds 2300
  $state=Get-NodeAPI ('/api/servers/'+$ServerID)
  if($state.state -ne 'offline' -or $state.native_recovery.pending -or $state.native_recovery.attempts_in_window -ne 1){throw 'Manual kill failed to cancel delayed recovery'}
  $report.checks.crash_recovery_manual_kill_cancels='passed'
  Set-Power 'start';Wait-State 'running';Crash-Game;Wait-RecoveryPending;Wait-State 'running';Crash-Game;Wait-State 'offline';Start-Sleep -Milliseconds 2300
  $state=Get-NodeAPI ('/api/servers/'+$ServerID)
  if($state.state -ne 'offline' -or $state.native_recovery.pending -or $state.native_recovery.attempts_in_window -ne 2){throw 'Crash recovery limit failed'}
  $report.checks.crash_recovery_budget_stops_loop='passed'
  Set-Power 'start';Wait-State 'running'
 }
 if($TestConsoleLogs){
  $fill='LOG_FILL_'+('x'*3800)
  for($batch=0;$batch -lt 36;$batch++){
   $before=(Get-NodeAPI ('/api/servers/'+$ServerID)).native_console_logs.persisted
   Invoke-RestMethod -Uri ('http://127.0.0.1:'+$nodePort+'/api/servers/'+$ServerID+'/commands') -Headers @{Authorization=('Bearer '+$nodeToken)} -Method Post -ContentType application/json -Body (@{commands=@(1..16|ForEach-Object{$fill})}|ConvertTo-Json)|Out-Null
   $until=[DateTime]::UtcNow.AddSeconds(5)
   do{$status=(Get-NodeAPI ('/api/servers/'+$ServerID)).native_console_logs;if($status.error){throw 'Persistent console storage reported failure'};if($status.persisted -lt ($before+16)){Start-Sleep -Milliseconds 25}}while($status.persisted -lt ($before+16) -and [DateTime]::UtcNow -lt $until)
   if($status.persisted -lt ($before+16) -or $status.dropped -ne 0){throw 'Controlled console fill failed to drain without drops'}
  }
  $files=@(Get-ChildItem -LiteralPath (Join-Path $Root ('logs\console\'+$ServerID)) -Filter '*.jsonl')
  if($files.Count -ne 2 -or @($files|Where-Object{$_.Length -gt 1048576}).Count){throw 'Rotated console file limits failed'}
  $report.checks.console_log_rotation_and_size_limit='passed'
  Invoke-RestMethod -Uri ('http://127.0.0.1:'+$nodePort+'/api/servers/'+$ServerID+'/commands') -Headers @{Authorization=('Bearer '+$nodeToken)} -Method Post -ContentType application/json -Body '{"commands":["PERSISTED_RESTART_MARKER"]}'|Out-Null
 }
 $gameProcesses=@();foreach($process in Get-CimInstance Win32_Process -Filter "Name='wings-windows-demo.exe'"){$owner=Invoke-CimMethod -InputObject $process -MethodName GetOwnerSid;if($owner.Sid -eq $gameSid){$gameProcesses+=$process.ProcessId}}
 if($gameProcesses.Count -ne 2){throw 'Expected root and child with actual game SID'};$report.checks.actual_game_tree_identity='passed'
 Stop-Service WingsWindowsNative
 (Get-Service WingsWindowsNative).WaitForStatus('Stopped',[TimeSpan]::FromSeconds(10))
 foreach($processID in $gameProcesses){if(Get-Process -Id $processID -ErrorAction SilentlyContinue){throw 'Managed game survived SCM stop'}};$report.checks.scm_stop_kills_game_tree='passed'
 Start-Service WingsWindowsNative;Wait-State 'offline';if($TestConsoleLogs){$restored=(Get-NodeAPI ('/api/servers/'+$ServerID)).native_console_logs;if(-not $restored.enabled -or $restored.restored_lines -le 0 -or 'ECHO:PERSISTED_RESTART_MARKER' -notin (Get-NodeAPI ('/api/servers/'+$ServerID+'/logs')).data){throw 'Console history did not survive actual service restart'};$report.checks.console_log_history_restored_after_service_restart='passed'};if($TestCrashRecovery){
  $budget=(Get-NodeAPI ('/api/servers/'+$ServerID)).native_recovery
  if($TestPersistentRecovery){
   if($budget.attempts_in_window -ne 2 -or -not $budget.persistent_history -or $budget.restored_attempts -ne 2 -or $budget.pending){throw 'Persistent recovery budget did not survive service restart'}
   $report.checks.crash_recovery_service_restart_preserves_budget='passed'
   Set-Power 'start';Wait-State 'running';Crash-Game;Wait-State 'offline';Start-Sleep -Milliseconds 2300
   $after=(Get-NodeAPI ('/api/servers/'+$ServerID)).native_recovery
   if((Get-NodeAPI ('/api/servers/'+$ServerID)).state -ne 'offline' -or $after.pending -or $after.attempts_in_window -ne 2){throw 'Service restart bypassed consumed recovery budget'}
   $report.checks.restored_budget_blocks_extra_automatic_restart='passed'
   $daemon=Get-CimInstance Win32_Service -Filter "Name='WingsWindowsNative'"
   $owned=Get-CimInstance Win32_Process -Filter ('ProcessId='+$daemon.ProcessId)
   if($owned.ExecutablePath -ne $exe){throw 'Refusing to terminate unexpected service executable'}
   Stop-Process -Id $daemon.ProcessId -Force
   (Get-Service WingsWindowsNative).WaitForStatus('Stopped',[TimeSpan]::FromSeconds(15))
   Start-Service WingsWindowsNative;Wait-State 'offline'
   $after=(Get-NodeAPI ('/api/servers/'+$ServerID)).native_recovery
   if($after.attempts_in_window -ne 2 -or $after.restored_attempts -ne 2 -or $after.pending){throw 'Abrupt daemon termination reset recovery state'}
   $report.checks.daemon_termination_preserves_recovery_budget='passed'
  }else{if($budget.attempts_in_window -ne 0){throw 'In-memory recovery state did not reset on service restart'};$report.checks.crash_recovery_service_restart_resets_budget='passed'}
 };Set-Power 'start';Wait-State 'running';Set-Power 'stop';Wait-State 'offline';$report.checks.service_restart_and_game_stop='passed'
 Stop-Service WingsWindowsNative
 Invoke-Bootstrap -Cleanup;$credentialSeeded=$false;$report.cleanup.credential_removed=$true
 if($TestPersistentRecovery){
  $snapshot=Join-Path $Root ('logs\console\'+$ServerID+'\recovery.0.json')
  $originalSnapshot=[IO.File]::ReadAllBytes($snapshot)
  try{
   [IO.File]::WriteAllText($snapshot,'CORRUPT_RECOVERY_STATE')
   try{Start-Service WingsWindowsNative -ErrorAction Stop}catch{}
   (Get-Service WingsWindowsNative).WaitForStatus('Stopped',[TimeSpan]::FromSeconds(15))
   $report.checks.corrupt_recovery_state_refuses_service_start='passed'
  }finally{[IO.File]::WriteAllBytes($snapshot,$originalSnapshot);$originalSnapshot=$null}
 }
 if($TestServiceEvents){
  $rejected=$false;try{& $eventScript -Remove}catch{$rejected=$true};if(-not $rejected){throw 'Removed event source with existing service'}
  $report.checks.event_source_remove_existing_service_rejected='passed'
  $events=@(Get-WinEvent -FilterHashtable @{LogName='Application';ProviderName='WingsWindowsNative';StartTime=$eventStarted} -ErrorAction Stop)
  foreach($id in @(100,101,102,103)){if($id -notin $events.Id){throw ('Missing service lifecycle event '+$id)}}
  $report.checks.actual_service_lifecycle_events='passed'
  $configurationPath=Join-Path $Root 'private\config.json'
  $originalConfiguration=[IO.File]::ReadAllText($configurationPath)
  try{
   [IO.File]::WriteAllText($configurationPath,'{"token":"EVENT_SECRET_DO_NOT_LOG')
   try{Start-Service WingsWindowsNative -ErrorAction Stop}catch{}
   $failedService=Get-Service WingsWindowsNative;try{$failedService.WaitForStatus('Stopped',[TimeSpan]::FromSeconds(15))}finally{$failedService.Dispose()}
   $events=@(Get-WinEvent -FilterHashtable @{LogName='Application';ProviderName='WingsWindowsNative';Id=200;StartTime=$eventStarted} -ErrorAction Stop)
   $messages=($events|ForEach-Object{$_.Properties[0].Value}) -join "`n"
   if($messages -notmatch 'stage=configuration' -or $messages -match 'EVENT_SECRET_DO_NOT_LOG' -or $messages.Contains($Root) -or $messages.Contains($nodeToken)){throw 'Startup failure event missing safe stage or exposes raw details'}
   $report.checks.actual_startup_failure_event_secret_safe='passed'
  }finally{[IO.File]::WriteAllText($configurationPath,$originalConfiguration);$originalConfiguration=$null}
 }
 $report.passed=$true
} catch {$report.error=$_.Exception.Message}
finally {
 if($credentialSeeded -and $daemonSid){try{Stop-Service WingsWindowsNative -ErrorAction Stop;Invoke-Bootstrap -Cleanup;$credentialSeeded=$false;$report.cleanup.credential_removed=$true}catch{$report.cleanup.credential_error=$_.Exception.Message;$report.passed=$false}}
 if($serviceCreated -and -not $credentialSeeded){try{Stop-Service WingsWindowsNative -ErrorAction SilentlyContinue;$s=Get-Service WingsWindowsNative;try{$s.WaitForStatus('Stopped',[TimeSpan]::FromSeconds(10))}finally{$s.Dispose()}; & "$env:SystemRoot\System32\sc.exe" delete WingsWindowsNative|Out-Null;$report.cleanup.service_removed=($LASTEXITCODE -eq 0)}catch{$report.cleanup.service_error=$_.Exception.Message;$report.passed=$false}}
 if($mock){Stop-Process -Id $mock.Id -ErrorAction SilentlyContinue;$mock.Dispose();$report.cleanup.mock_stopped=$true}
 if($eventSourceCreated){try{& $eventScript -Remove;$report.cleanup.event_source_removed=(-not (Test-Path -LiteralPath $eventKey))}catch{$report.cleanup.event_source_error=$_.Exception.Message;$report.passed=$false}}
 if($credentialSeeded -or (Get-Service WingsWindowsNative -ErrorAction SilentlyContinue)){$report.passed=$false;$report.cleanup.state_retained='Service or credential cleanup incomplete; accounts and rights retained for inspection'}else{
 foreach($entry in @(@{SID=$daemonSid;Names=@('SeServiceLogonRight','SeAssignPrimaryTokenPrivilege','SeIncreaseQuotaPrivilege','SeBatchLogonRight')},@{SID=$gameSid;Names=@('SeBatchLogonRight')})){
  if($entry.SID){foreach($right in $entry.Names){if($right -in @([WingsWindows.NodeSetup]::Rights($entry.SID))){[WingsWindows.NodeSetup]::Right($entry.SID,$right,$true)}}}
 }
 if(Get-LocalUser -Name $DaemonName -ErrorAction SilentlyContinue){Remove-LocalUser -Name $DaemonName};if($gameCreated){Remove-LocalUser -Name $GameName}
 $report.cleanup.accounts_and_rights_removed=$true
 }
 if($gamePassword){$gamePassword.Dispose()};if($daemonPassword){$daemonPassword.Dispose()}
}
$json=$report|ConvertTo-Json -Depth 10
if(Test-Path -LiteralPath $Root){[IO.File]::WriteAllText((Join-Path $Root 'scm-report.json'),$json,[Text.UTF8Encoding]::new($false))}
Write-Output $json
