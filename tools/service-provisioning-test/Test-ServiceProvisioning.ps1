#Requires -RunAsAdministrator
param([string]$Staging,[string]$Package,[string]$Root,[string]$GameName,[string]$DaemonName,[string]$ServerID,[switch]$TestCrashRecovery,[switch]$TestConsoleLogs,[switch]$TestServiceEvents,[switch]$TestPersistentRecovery,[switch]$TestEggText,[switch]$TestEggJSON,[switch]$TestEggProperties,[switch]$TestEggINI,[switch]$TestEggYAML,[switch]$TestEggXML,[switch]$PrepareReboot,[switch]$TestRestoreRunning,[switch]$TestSourceRCON,[switch]$TestLogFile,[switch]$TestRCONCredential,[switch]$TestNativeInstaller,[switch]$TestPendingCreation,[switch]$TestInstallState,[switch]$TestGamePermissions)
$ErrorActionPreference='Stop';$ProgressPreference='SilentlyContinue'
if($TestGamePermissions -and (-not $TestInstallState -or $PrepareReboot)){throw 'Game permissions fixture requires install-state phase and cannot prepare reboot'}
if($TestInstallState -and (-not $TestPendingCreation -or $PrepareReboot)){throw 'Install state fixture requires pending creation phase and cannot prepare reboot'}
if($TestPendingCreation -and (-not $TestNativeInstaller -or $PrepareReboot)){throw 'Pending creation fixture requires native installer phase and cannot prepare reboot'}
if($TestNativeInstaller -and (-not $TestRCONCredential -or $PrepareReboot)){throw 'Native installer fixture requires full credential fixture and cannot prepare reboot'}
if($TestRestoreRunning -and -not $TestPersistentRecovery){throw 'Restore fixture requires persistent recovery'}
if($TestSourceRCON -and $PrepareReboot){throw 'RCON fixture is separate from reboot preparation'}
if($TestRCONCredential -and (-not $TestSourceRCON -or $PrepareReboot)){throw 'RCON credential fixture requires RCON and cannot prepare a reboot'}
if($TestLogFile -and (-not $TestSourceRCON -or $PrepareReboot)){throw 'Log-file fixture requires RCON phase and cannot prepare a reboot'}
if($PrepareReboot -and (-not $TestPersistentRecovery -or -not $TestServiceEvents)){throw 'Reboot fixture requires persistent recovery and service events'}
if($TestEggXML -and -not $TestEggYAML){throw 'XML fixture requires TestEggYAML for six-parser validation'}
if($TestEggYAML -and -not $TestEggINI){throw 'YAML fixture requires TestEggINI for five-parser validation'}
if($TestEggINI -and -not $TestEggProperties){throw 'INI fixture requires TestEggProperties for four-parser validation'}
if($TestEggProperties -and -not $TestEggJSON){throw 'Properties fixture requires TestEggJSON for three-parser validation'}
if($TestEggJSON -and -not $TestEggText){throw 'JSON fixture requires TestEggText for mixed-batch validation'}
if($TestPersistentRecovery -and (-not $TestCrashRecovery -or -not $TestConsoleLogs)){throw 'Persistent recovery fixture requires TestCrashRecovery and TestConsoleLogs'}
if($Root -notmatch '^C:\\WingsWindowsTesting\\scm-([0-9a-f]{8})$'){throw 'Use a fresh C:\WingsWindowsTesting\scm-<eight-hex> fixture root'}
$suffix=$Matches[1]
if($GameName -ne ('WwScmG_'+$suffix) -or $DaemonName -ne ('WwScmD_'+$suffix)){throw 'Fixture account names must match the root suffix'}
if($ServerID -notmatch '^[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}$'){throw 'Use a full random server UUID'}
$report=[ordered]@{passed=$false;checks=[ordered]@{};error=$null;cleanup=[ordered]@{}}
$gameCreated=$false;$serviceCreated=$false;$credentialSeeded=$false;$mock=$null;$daemonSid=$null;$gameSid=$null
$extraCredentialSeeded=$false;$extraGameCreated=$false;$extraCleanupFailed=$false;$extraOriginalConfiguration=$null;$extraOriginalACLs=@{};$extraSID=$null;$extraID=$null
$rebootPrepared=$false;$rconSeeded=$false;$eventSourceCreated=$false;$eventStarted=Get-Date;$eventKey='HKLM:\SYSTEM\CurrentControlSet\Services\EventLog\Application\WingsWindowsNative'
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
 foreach($file in @('GamePermissions.psm1','Set-ServiceGamePermissions.ps1')){Copy-Item -LiteralPath (Join-Path $Package ('scripts\'+$file)) -Destination (Join-Path $Root ('bin\scripts\'+$file))}
 foreach($file in @('Store-RCONCredential.ps1','Set-ServiceRCONCredential.ps1')){Copy-Item -LiteralPath (Join-Path $Package ('scripts\'+$file)) -Destination (Join-Path $Root ('bin\scripts\'+$file))}
 $first=[Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback,0);$second=[Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback,0);$first.Start();$second.Start();$panelPort=$first.LocalEndpoint.Port;$nodePort=$second.LocalEndpoint.Port;$first.Stop();$second.Stop()
 $b=New-Object byte[] 32;$rng=[Security.Cryptography.RandomNumberGenerator]::Create();try{$rng.GetBytes($b);$nodeToken=[Convert]::ToBase64String($b)}finally{$rng.Dispose()}
 $profiles=@{};$profiles[$ServerID]=[ordered]@{schema_version=1;executable=(Join-Path $Root 'bin\wings-windows-demo.exe');arguments=@('spawn');allowed_environment=@();stop_command='stop';ready_pattern='^SERVER_READY$';memory_mb=128;run_as=@{sid=$gameSid;credential_target=('WingsWindows/'+$ServerID)}}
 if($TestCrashRecovery){$profiles[$ServerID].crash_recovery=@{enabled=$true;max_restarts=2;window_seconds=60;delay_seconds=2}}
 $config=[ordered]@{listen=('127.0.0.1:'+$nodePort);panel_url=('http://127.0.0.1:'+$panelPort);node_uuid=[guid]::NewGuid().ToString();token_id='scm-fixture';token=$nodeToken;data_directory=(Join-Path $Root 'data');allow_insecure_loopback=$true;tls=@{cert='';key=''};profiles=$profiles}
 if($TestConsoleLogs){$config.console_logs=@{directory=(Join-Path $Root 'logs\console');max_file_bytes=1048576;max_files=2};$profiles[$ServerID].allowed_environment=@('LOG_PROBE_PATH')}
 if($TestPersistentRecovery){$profiles[$ServerID].crash_recovery.persist_history=$true;$profiles[$ServerID].crash_recovery.window_seconds=3600;$profiles[$ServerID].allowed_environment+=@('RECOVERY_PROBE_PATH')}
 if($TestRestoreRunning){$profiles[$ServerID].restore_running=$true;$profiles[$ServerID].allowed_environment+=@('INTENT_PROBE_PATH')}
 if($TestEggText){$profiles[$ServerID].use_egg_process_configuration=$true;$profiles[$ServerID].egg_config_files=@('server.cfg')}
 if($TestEggJSON){$profiles[$ServerID].egg_config_files+=@('server.json')}
 if($TestEggProperties){$profiles[$ServerID].egg_config_files+=@('server.properties')}
 if($TestEggINI){$profiles[$ServerID].egg_config_files+=@('server.ini')}
 if($TestEggYAML){$profiles[$ServerID].egg_config_files+=@('server.yml')}
 if($TestEggXML){$profiles[$ServerID].egg_config_files+=@('server.xml')}
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
 if($TestEggJSON){
  $eggJsonPath=Join-Path $Root ('data\'+$ServerID+'\server.json')
  [IO.File]::WriteAllText($eggJsonPath,'{"invalid":',[Text.UTF8Encoding]::new($false))
  $textBefore=[IO.File]::ReadAllText($eggPath)
  Set-Power 'start'
  $until=[DateTime]::UtcNow.AddSeconds(5);$failed=$false
  do{$failed=(@((Get-NodeAPI ('/api/servers/'+$ServerID+'/logs')).data|Where-Object {$_ -like '*egg file 2 preflight failed*'}).Count -gt 0);if(-not $failed){Start-Sleep -Milliseconds 50}}while(-not $failed -and [DateTime]::UtcNow -lt $until)
  if(-not $failed -or (Get-NodeAPI ('/api/servers/'+$ServerID)).state -ne 'offline' -or [IO.File]::ReadAllText($eggPath) -ne $textBefore){throw 'Malformed JSON did not prevent launch and preserve earlier text target'}
  $report.checks.egg_json_malformed_batch_preserves_text='passed'
  [IO.File]::WriteAllText($eggJsonPath,'{"network":{"port":1,"host":"127.0.0.1"},"worlds":[{"mode":"old"},{"mode":"stay"}],"untouched":"original","precise":9007199254740993}',[Text.UTF8Encoding]::new($false))
  $eggJsonACL=(Get-Acl -LiteralPath $eggJsonPath).Sddl
 }
 if($TestEggProperties){
  $eggPropertiesPath=Join-Path $Root ('data\'+$ServerID+'\server.properties')
  [IO.File]::WriteAllText($eggPropertiesPath,'bad=\u00xz',[Text.UTF8Encoding]::new($false))
  $textBefore=[IO.File]::ReadAllText($eggPath);$jsonBefore=[IO.File]::ReadAllText($eggJsonPath)
  Set-Power 'start'
  $until=[DateTime]::UtcNow.AddSeconds(5);$failed=$false
  do{$failed=(@((Get-NodeAPI ('/api/servers/'+$ServerID+'/logs')).data|Where-Object {$_ -like '*egg file 3 preflight failed*'}).Count -gt 0);if(-not $failed){Start-Sleep -Milliseconds 50}}while(-not $failed -and [DateTime]::UtcNow -lt $until)
  if(-not $failed -or (Get-NodeAPI ('/api/servers/'+$ServerID)).state -ne 'offline' -or [IO.File]::ReadAllText($eggPath) -ne $textBefore -or [IO.File]::ReadAllText($eggJsonPath) -ne $jsonBefore){throw 'Malformed properties did not preserve earlier batch targets'}
  $report.checks.egg_properties_malformed_batch_preserves_targets='passed'
  $propertiesInput=(@('# retained header','server-port=1','name=old','mode=stay','continued=first\','   second','literal=${USERNAME}','untouched=keep') -join "`r`n")+"`r`n"
  [IO.File]::WriteAllText($eggPropertiesPath,$propertiesInput,[Text.UTF8Encoding]::new($false))
  $eggPropertiesACL=(Get-Acl -LiteralPath $eggPropertiesPath).Sddl
 }
 if($TestEggINI){
  $eggIniPath=Join-Path $Root ('data\'+$ServerID+'\server.ini')
  [IO.File]::WriteAllText($eggIniPath,'[invalid',[Text.UTF8Encoding]::new($false))
  $textBefore=[IO.File]::ReadAllText($eggPath);$jsonBefore=[IO.File]::ReadAllText($eggJsonPath);$propertiesBefore=[IO.File]::ReadAllText($eggPropertiesPath)
  Set-Power 'start'
  $until=[DateTime]::UtcNow.AddSeconds(5);$failed=$false
  do{$failed=(@((Get-NodeAPI ('/api/servers/'+$ServerID+'/logs')).data|Where-Object {$_ -like '*egg file 4 preflight failed*'}).Count -gt 0);if(-not $failed){Start-Sleep -Milliseconds 50}}while(-not $failed -and [DateTime]::UtcNow -lt $until)
  if(-not $failed -or (Get-NodeAPI ('/api/servers/'+$ServerID)).state -ne 'offline' -or [IO.File]::ReadAllText($eggPath) -ne $textBefore -or [IO.File]::ReadAllText($eggJsonPath) -ne $jsonBefore -or [IO.File]::ReadAllText($eggPropertiesPath) -ne $propertiesBefore){throw 'Malformed INI did not preserve all earlier batch targets'}
  $report.checks.egg_ini_malformed_batch_preserves_targets='passed'
  $iniInput=(@('; normalized comment','port=1','literal=${USERNAME}','path=C:\Games\','[network]','host=127.0.0.1','[parent]','shared=parent','[parent.child]','keep=yes') -join "`r`n")+"`r`n"
  [IO.File]::WriteAllText($eggIniPath,$iniInput,[Text.UTF8Encoding]::new($false))
  $eggIniACL=(Get-Acl -LiteralPath $eggIniPath).Sddl
 }
 if($TestEggYAML){
  $eggYamlPath=Join-Path $Root ('data\'+$ServerID+'\server.yml')
  [IO.File]::WriteAllText($eggYamlPath,'invalid: [',[Text.UTF8Encoding]::new($false))
  $earlierPaths=@($eggPath,$eggJsonPath,$eggPropertiesPath,$eggIniPath)
  $earlierContents=@($earlierPaths|ForEach-Object{[IO.File]::ReadAllText($_)})
  Set-Power 'start'
  $until=[DateTime]::UtcNow.AddSeconds(5);$failed=$false
  do{$failed=(@((Get-NodeAPI ('/api/servers/'+$ServerID+'/logs')).data|Where-Object {$_ -like '*egg file 5 preflight failed*'}).Count -gt 0);if(-not $failed){Start-Sleep -Milliseconds 50}}while(-not $failed -and [DateTime]::UtcNow -lt $until)
  if(-not $failed -or (Get-NodeAPI ('/api/servers/'+$ServerID)).state -ne 'offline'){throw 'Malformed YAML did not prevent launch'}
  for($i=0;$i -lt $earlierPaths.Count;$i++){if([IO.File]::ReadAllText($earlierPaths[$i]) -ne $earlierContents[$i]){throw 'Malformed YAML changed an earlier batch target'}}
  $report.checks.egg_yaml_malformed_batch_preserves_targets='passed'
  $yamlInput=(@('# normalized comment','network: {port: 1, host: 127.0.0.1}','worlds: [{mode: old}, {mode: stay}]','precise: 9007199254740993','fraction: 1.2300e+3','literal: ${USERNAME}','name: old','untouched: keep') -join "`r`n")+"`r`n"
  [IO.File]::WriteAllText($eggYamlPath,$yamlInput,[Text.UTF8Encoding]::new($false))
  $eggYamlACL=(Get-Acl -LiteralPath $eggYamlPath).Sddl
 }
 if($TestEggXML){
  $eggXmlPath=Join-Path $Root ('data\'+$ServerID+'\server.xml')
  [IO.File]::WriteAllText($eggXmlPath,'<invalid>',[Text.UTF8Encoding]::new($false))
  $earlierPaths=@($eggPath,$eggJsonPath,$eggPropertiesPath,$eggIniPath,$eggYamlPath)
  $earlierContents=@($earlierPaths|ForEach-Object{[IO.File]::ReadAllText($_)})
  Set-Power 'start'
  $until=[DateTime]::UtcNow.AddSeconds(5);$failed=$false
  do{$failed=(@((Get-NodeAPI ('/api/servers/'+$ServerID+'/logs')).data|Where-Object {$_ -like '*egg file 6 preflight failed*'}).Count -gt 0);if(-not $failed){Start-Sleep -Milliseconds 50}}while(-not $failed -and [DateTime]::UtcNow -lt $until)
  if(-not $failed -or (Get-NodeAPI ('/api/servers/'+$ServerID)).state -ne 'offline'){throw 'Malformed XML did not prevent launch'}
  for($i=0;$i -lt $earlierPaths.Count;$i++){if([IO.File]::ReadAllText($earlierPaths[$i]) -ne $earlierContents[$i]){throw 'Malformed XML changed an earlier batch target'}}
  $report.checks.egg_xml_malformed_batch_preserves_targets='passed'
  [IO.File]::WriteAllText($eggXmlPath,'<server keep="yes"><port>1</port><name>old</name><worlds><world id="one"><mode>old</mode></world><world id="two"><mode>stay</mode></world></worlds></server>',[Text.UTF8Encoding]::new($false))
  $eggXmlACL=(Get-Acl -LiteralPath $eggXmlPath).Sddl
 }
 Set-Power 'start';Wait-State 'running';$report.checks.separate_game_start='passed'
 if($TestEggXML){
  $xmlText=[IO.File]::ReadAllText($eggXmlPath);$xmlConfig=[xml]$xmlText
  if($xmlConfig.server.port -ne '28015' -or $xmlConfig.server.name -ne ('Caf'+[char]0xe9+' & <safe>') -or $xmlConfig.server.keep -ne 'yes' -or $xmlConfig.server.literal -ne '${USERNAME}' -or $xmlConfig.server.worlds.world.Count -ne 2 -or @($xmlConfig.server.worlds.world|Where-Object{$_.mode -ne 'new' -or $_.enabled -ne 'true'}).Count -ne 0 -or $xmlText -notmatch '&amp; &lt;safe&gt;' -or (Get-Acl -LiteralPath $eggXmlPath).Sddl -ne $eggXmlACL){throw 'SCM XML text/attributes/escaping/literal/Unicode content or ACL differs'}
  $report.checks.egg_xml_daemon_rewrite_text_attributes_acl='passed'
 }
 if($TestEggYAML){
  $expectedYAML=(@('"added": "${USERNAME}"','"fraction": 1.2300e+3','"literal": "${USERNAME}"',('"name": "Caf'+[char]0xe9+'"'),'"network":','  "enabled": true','  "host": "127.0.0.1"','  "port": 28015','"precise": 9007199254740993','"untouched": "keep"','"worlds":','  - "mode": "new"','  - "mode": "stay"') -join "`n")+"`n"
  if([IO.File]::ReadAllText($eggYamlPath) -ne $expectedYAML -or (Get-Acl -LiteralPath $eggYamlPath).Sddl -ne $eggYamlACL){throw 'SCM YAML typed/precise/conditional/literal/Unicode content or ACL differs'}
  $report.checks.egg_yaml_daemon_rewrite_types_precision_acl='passed'
 }
 if($TestEggINI){
  $expectedINI=(@('port=28015','literal=${USERNAME}','path=`C:\Games\`','added=${USERNAME}','[network]','host=127.0.0.1','enabled=true','note=`  ;# keep  `','[parent]','shared=parent','[parent.child]','keep=yes','shared=child-only','[/Script.Game.Settings]',('Server.Name=Caf'+[char]0xe9)) -join "`n")+"`n"
  if([IO.File]::ReadAllText($eggIniPath) -ne $expectedINI -or (Get-Acl -LiteralPath $eggIniPath).Sddl -ne $eggIniACL){throw 'SCM INI sections/Unicode/literals/paths/quotes or ACL differs'}
  $report.checks.egg_ini_daemon_rewrite_sections_literals_acl='passed'
 }
 if($TestEggProperties){
  $expectedProperties=(@('# retained header','server-port=28015','name=Windows Caf\u00e9 \ud83d\ude00','mode=stay','continued=firstsecond','literal=${USERNAME}','untouched=keep','added=${USERNAME}','enabled=true') -join "`n")+"`n"
  if([IO.File]::ReadAllText($eggPropertiesPath) -ne $expectedProperties -or (Get-Acl -LiteralPath $eggPropertiesPath).Sddl -ne $eggPropertiesACL){throw 'SCM properties escaping/continuation/literal environment/ACL differs'}
  $report.checks.egg_properties_daemon_rewrite_escapes_literals_acl='passed'
 }

 if($TestEggJSON){
  $jsonText=[IO.File]::ReadAllText($eggJsonPath);$jsonConfig=$jsonText|ConvertFrom-Json
  if($jsonConfig.network.port -is [string] -or $jsonConfig.network.port -ne 28015 -or $jsonConfig.network.enabled -isnot [bool] -or -not $jsonConfig.network.enabled -or $jsonConfig.network.host -ne '127.0.0.1' -or $jsonConfig.worlds[0].mode -ne 'new' -or $jsonConfig.worlds[1].mode -ne 'stay' -or $jsonConfig.name -ne ('Caf'+[char]0xe9) -or $jsonConfig.untouched -ne 'original' -or $jsonText -notmatch '"precise": 9007199254740993' -or (Get-Acl -LiteralPath $eggJsonPath).Sddl -ne $eggJsonACL){throw 'SCM JSON rewrite type/value/precision or ACL differs'}
  $report.checks.egg_json_daemon_rewrite_types_precision_acl='passed'
 }

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
 if($TestRestoreRunning){
  Invoke-RestMethod -Uri ('http://127.0.0.1:'+$nodePort+'/api/servers/'+$ServerID+'/commands') -Headers @{Authorization=('Bearer '+$nodeToken)} -Method Post -ContentType application/json -Body '{"commands":["probe-intent-access"]}'|Out-Null
  $until=[DateTime]::UtcNow.AddSeconds(5);$denied=$false
  do{$denied='INTENT_PROBE_DENIED' -in (Get-NodeAPI ('/api/servers/'+$ServerID+'/logs')).data;if(-not $denied){Start-Sleep -Milliseconds 50}}while(-not $denied -and [DateTime]::UtcNow -lt $until)
  if(-not $denied){throw 'Game token read persisted intent'};$report.checks.intent_state_game_read_denied='passed'
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
 if($TestRestoreRunning){
  Stop-Service WingsWindowsNative;(Get-Service WingsWindowsNative).WaitForStatus('Stopped',[TimeSpan]::FromSeconds(15))
  $edited=Get-Content -LiteralPath $configuration -Raw|ConvertFrom-Json
  $edited.profiles.$ServerID.crash_recovery.max_restarts=5
  [IO.File]::WriteAllText($configuration,($edited|ConvertTo-Json -Depth 30),[Text.UTF8Encoding]::new($false))
  Start-Service WingsWindowsNative;Wait-State 'offline';Set-Power 'start';Wait-State 'running'
  Stop-Service WingsWindowsNative;(Get-Service WingsWindowsNative).WaitForStatus('Stopped',[TimeSpan]::FromSeconds(15))
  Start-Service WingsWindowsNative;Wait-State 'running'
  $restored=Get-NodeAPI ('/api/servers/'+$ServerID)
  if(-not $restored.native_restore.loaded_running -or -not $restored.native_restore.desired_running -or $restored.native_recovery.attempts_in_window -ne 3){throw 'Persisted running intent did not restore with a reservation'}
  $report.checks.running_intent_restores_after_scm_restart='passed'
  Set-Power 'stop';Wait-State 'offline';Stop-Service WingsWindowsNative;(Get-Service WingsWindowsNative).WaitForStatus('Stopped',[TimeSpan]::FromSeconds(15))
  Start-Service WingsWindowsNative;Wait-State 'offline';Start-Sleep -Milliseconds 2300
  $stopped=Get-NodeAPI ('/api/servers/'+$ServerID)
  if($stopped.state -ne 'offline' -or $stopped.native_restore.desired_running -or $stopped.native_restore.loaded_running -or $stopped.native_recovery.attempts_in_window -ne 3){throw 'Manual stop was not durable across restart'}
  $report.checks.stopped_intent_survives_scm_restart='passed'
  Stop-Service WingsWindowsNative;(Get-Service WingsWindowsNative).WaitForStatus('Stopped',[TimeSpan]::FromSeconds(15))
  $intentPath=Join-Path $Root ('logs\console\'+$ServerID+'\intent.0.json');$intentBytes=[IO.File]::ReadAllBytes($intentPath)
  try{[IO.File]::WriteAllText($intentPath,'CORRUPT_INTENT');try{Start-Service WingsWindowsNative -ErrorAction Stop}catch{};(Get-Service WingsWindowsNative).WaitForStatus('Stopped',[TimeSpan]::FromSeconds(15));$report.checks.corrupt_intent_refuses_service_start='passed'}finally{[IO.File]::WriteAllBytes($intentPath,$intentBytes);$intentBytes=$null}
  Start-Service WingsWindowsNative;Wait-State 'offline'
 }
 if($TestSourceRCON){
  Stop-Service WingsWindowsNative;(Get-Service WingsWindowsNative).WaitForStatus('Stopped',[TimeSpan]::FromSeconds(15))
  $rconListener=[Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback,0);$rconListener.Start();$rconPort=$rconListener.LocalEndpoint.Port;$rconListener.Stop()
  $edited=Get-Content -LiteralPath $configuration -Raw|ConvertFrom-Json
  $rconProfile=$edited.profiles.$ServerID
  $rconProfile.arguments=@('rcon',('127.0.0.1:'+$rconPort))
  $rconProfile|Add-Member -NotePropertyName source_rcon -NotePropertyValue @{address=('127.0.0.1:'+$rconPort);password_environment='WINGS_RCON_FIXTURE';timeout_seconds=2}
  [IO.File]::WriteAllText($configuration,($edited|ConvertTo-Json -Depth 30),[Text.UTF8Encoding]::new($false))
  $serviceKey='HKLM:\SYSTEM\CurrentControlSet\Services\WingsWindowsNative'
  # Public fixed password belongs only to the harmless loopback demo fixture.
  New-ItemProperty -LiteralPath $serviceKey -Name Environment -PropertyType MultiString -Value @('WINGS_RCON_FIXTURE=fixture-rcon-only') -Force|Out-Null
  function Send-RCONFixture([string]$Command){Invoke-RestMethod -Uri ('http://127.0.0.1:'+$nodePort+'/api/servers/'+$ServerID+'/commands') -Headers @{Authorization=('Bearer '+$nodeToken)} -Method Post -ContentType application/json -Body (@{commands=@($Command)}|ConvertTo-Json) -TimeoutSec 5|Out-Null}
  Start-Service WingsWindowsNative;Wait-State 'offline';Set-Power 'start';Wait-State 'running'
  Send-RCONFixture 'SCM_RCON_TEST'
  if('[RCON] ECHO:SCM_RCON_TEST' -notin (Get-NodeAPI ('/api/servers/'+$ServerID+'/logs')).data){throw 'RCON response absent'}
  $report.checks.source_rcon_panel_command_response='passed'
  Send-RCONFixture 'echo-secret'
  $rconLogs=(Get-NodeAPI ('/api/servers/'+$ServerID+'/logs')).data
  if('[RCON] [REDACTED]' -notin $rconLogs -or ($rconLogs -join "`n").Contains('fixture-rcon-only')){throw 'RCON reply secret not redacted'}
  $report.checks.source_rcon_response_secret_redacted='passed'
  $rconGamePids=@();foreach($process in Get-CimInstance Win32_Process -Filter "Name='wings-windows-demo.exe'"){$owner=Invoke-CimMethod -InputObject $process -MethodName GetOwnerSid;if($owner.Sid -eq $gameSid){$rconGamePids+=$process.ProcessId}}
  if($rconGamePids.Count -ne 2){throw 'RCON root/child identities differ'}
  $report.checks.source_rcon_distinct_game_tree_identity='passed'
  Set-Power 'stop';Wait-State 'offline'
  $until=[DateTime]::UtcNow.AddSeconds(3);do{$rconLogs=(Get-NodeAPI ('/api/servers/'+$ServerID+'/logs')).data;if('RCON_COMMAND:stop' -in $rconLogs){break};Start-Sleep -Milliseconds 50}while([DateTime]::UtcNow -lt $until)
  if('RCON_COMMAND:stop' -notin $rconLogs){throw 'Stop did not reach RCON fixture'}
  foreach($id in $rconGamePids){if(Get-Process -Id $id -ErrorAction SilentlyContinue){throw 'RCON stop left child alive'}}
  $report.checks.source_rcon_graceful_stop_kills_tree='passed'
  Stop-Service WingsWindowsNative;(Get-Service WingsWindowsNative).WaitForStatus('Stopped',[TimeSpan]::FromSeconds(15))
  New-ItemProperty -LiteralPath $serviceKey -Name Environment -PropertyType MultiString -Value @('WINGS_RCON_FIXTURE=wrong-fixture-password') -Force|Out-Null
  Start-Service WingsWindowsNative;Wait-State 'offline';Set-Power 'start';Wait-State 'running'
  $authRejected=$false;try{Send-RCONFixture 'UNAUTHENTICATED_RCON_COMMAND'}catch{$authRejected=$true}
  if(-not $authRejected -or 'RCON_COMMAND:UNAUTHENTICATED_RCON_COMMAND' -in (Get-NodeAPI ('/api/servers/'+$ServerID+'/logs')).data){throw 'RCON authentication rejection failed'}
  Set-Power 'kill';Wait-State 'offline'
  $report.checks.source_rcon_bad_password_rejected='passed'
 }
 if($TestLogFile){
  Stop-Service WingsWindowsNative;(Get-Service WingsWindowsNative).WaitForStatus('Stopped',[TimeSpan]::FromSeconds(15))
  $edited=Get-Content -LiteralPath $configuration -Raw|ConvertFrom-Json
  $edited.profiles.$ServerID|Add-Member -NotePropertyName log_file -NotePropertyValue @{path='server.log';readiness=$true}
  [IO.File]::WriteAllText($configuration,($edited|ConvertTo-Json -Depth 30),[Text.UTF8Encoding]::new($false))
  New-ItemProperty -LiteralPath $serviceKey -Name Environment -PropertyType MultiString -Value @('WINGS_RCON_FIXTURE=fixture-rcon-only') -Force|Out-Null
  $serverRoot=Join-Path (Join-Path $Root 'data') $ServerID
  [IO.File]::WriteAllText((Join-Path $serverRoot 'server.log'),"SERVER_READY`nSTALE_LOG_MUST_NOT_APPEAR`n",[Text.UTF8Encoding]::new($false))
  function Wait-GameLog([string]$Line){$until=[DateTime]::UtcNow.AddSeconds(5);do{if($Line -in (Get-NodeAPI ('/api/servers/'+$ServerID+'/logs')).data){return};Start-Sleep -Milliseconds 100}while([DateTime]::UtcNow -lt $until);throw 'Expected game log line absent'}
  Start-Service WingsWindowsNative;Wait-State 'offline';Set-Power 'start';Wait-State 'starting';Start-Sleep -Milliseconds 350
  $before=Get-NodeAPI ('/api/servers/'+$ServerID)
  if($before.state -ne 'starting' -or -not $before.native_log_file.active -or '[Game log] STALE_LOG_MUST_NOT_APPEAR' -in (Get-NodeAPI ('/api/servers/'+$ServerID+'/logs')).data){throw 'Stale file/stdout established log readiness'}
  $report.checks.log_file_stale_history_and_stdout_ignored='passed'
  Send-RCONFixture 'log-ready';Wait-State 'running';Wait-GameLog '[Game log] SERVER_READY'
  Send-RCONFixture 'log-message';Wait-GameLog '[Game log] FILE_MESSAGE'
  $report.checks.log_file_readiness_and_console_via_rcon='passed'
  Send-RCONFixture 'log-rotate';Wait-GameLog '[Game log] ROTATED_MESSAGE'
  Send-RCONFixture 'log-truncate';Wait-GameLog '[Game log] RESET'
  $report.checks.log_file_rotation_and_truncation='passed'
  Set-Power 'kill';Wait-State 'offline'
  # Verify the exact disposable server directory can move after reader shutdown.
  $probe=$serverRoot+'-pin-probe';$allowed=[IO.Path]::GetFullPath((Join-Path $Root 'data'))+'\'
  foreach($candidate in @($serverRoot,$probe)){if(-not [IO.Path]::GetFullPath($candidate).StartsWith($allowed,[StringComparison]::OrdinalIgnoreCase)){throw 'Pin probe escaped fixture data'}}
  $moved=$false;$until=[DateTime]::UtcNow.AddSeconds(3)
  do{try{[IO.Directory]::Move($serverRoot,$probe);$moved=$true}catch{Start-Sleep -Milliseconds 100}}while(-not $moved -and [DateTime]::UtcNow -lt $until)
  if(-not $moved){throw 'Log reader left directory pinned after kill'}
  [IO.Directory]::Move($probe,$serverRoot)
  $report.checks.log_file_kill_releases_directory_pins='passed'
  Set-Power 'start';Wait-State 'starting';Send-RCONFixture 'log-ready';Wait-State 'running'
  Send-RCONFixture 'log-malformed'
  $until=[DateTime]::UtcNow.AddSeconds(5);do{$after=Get-NodeAPI ('/api/servers/'+$ServerID);if(-not $after.native_log_file.active){break};Start-Sleep -Milliseconds 100}while([DateTime]::UtcNow -lt $until)
  if($after.state -ne 'running' -or $after.native_log_file.active -or $after.native_log_file.error -ne 'read_failed'){throw 'Malformed log failure status incorrect'}
  if(((Get-NodeAPI ('/api/servers/'+$ServerID+'/logs')).data -join "`n").Contains([string][char]0)){throw 'Malformed text reached console'}
  Set-Power 'kill';Wait-State 'offline';$report.checks.log_file_malformed_text_stops_reader_not_game='passed'
 }
 if($TestRCONCredential){
  Stop-Service WingsWindowsNative;(Get-Service WingsWindowsNative).WaitForStatus('Stopped',[TimeSpan]::FromSeconds(15))
  $edited=Get-Content -LiteralPath $configuration -Raw|ConvertFrom-Json
  $edited.profiles.$ServerID.PSObject.Properties.Remove('log_file')
  $edited.profiles.$ServerID.source_rcon.password_environment=''
  $edited.profiles.$ServerID.source_rcon|Add-Member -NotePropertyName password_credential_target -NotePropertyValue ('WingsWindowsRCON/'+$ServerID)
  [IO.File]::WriteAllText($configuration,($edited|ConvertTo-Json -Depth 30),[Text.UTF8Encoding]::new($false))
  Remove-ItemProperty -LiteralPath $serviceKey -Name Environment -ErrorAction SilentlyContinue
  $rconCommand=Join-Path $Root 'bin\scripts\Set-ServiceRCONCredential.ps1'
  $rconOptions=@{InstallationRoot=$Root;ServerUUID=$ServerID;ServiceCredential=$serviceCredential}
  $rconPassword=ConvertTo-SecureString 'fixture-rcon-only' -AsPlainText -Force
  $rconSeeded=$true # Conservatively inspect/remove even if a store response is interrupted.
  try{
   $rejected=$false;try{& $rconCommand @rconOptions -Password $rconPassword -Replace|Out-Null}catch{$rejected=$true}
   if(-not $rejected){throw 'RCON replacement created missing target'}
   $stored=& $rconCommand @rconOptions -Password $rconPassword;$rconSeeded=$true
   $metadata=& $rconCommand @rconOptions -Inspect
   if(-not $metadata.present -or $metadata.target -cne ('WingsWindowsRCON/'+$ServerID) -or ($metadata|ConvertTo-Json).Contains('fixture-rcon-only')){throw 'RCON metadata invalid'}
   $rejected=$false;try{& $rconCommand @rconOptions -Password $rconPassword|Out-Null}catch{$rejected=$true}
   if(-not $rejected){throw 'RCON overwrite allowed'}
   $report.checks.rcon_vault_store_inspect_overwrite_rules='passed'
   $wrongOptions=@{InstallationRoot=$Root;ServerUUID=[guid]::NewGuid().ToString();ServiceCredential=$serviceCredential}
   $rejected=$false;try{& $rconCommand @wrongOptions -Inspect|Out-Null}catch{$rejected=$true}
   if(-not $rejected){throw 'Unconfigured RCON target accepted'}
   $report.checks.rcon_vault_unconfigured_server_rejected='passed'
   Start-Service WingsWindowsNative;Wait-State 'offline';Set-Power 'start';Wait-State 'running'
   Send-RCONFixture 'VAULT_RCON_COMMAND';Send-RCONFixture 'echo-secret'
   $vaultLogs=(Get-NodeAPI ('/api/servers/'+$ServerID+'/logs')).data
   if('[RCON] ECHO:VAULT_RCON_COMMAND' -notin $vaultLogs -or ($vaultLogs -join "`n").Contains('fixture-rcon-only')){throw 'Vault RCON command or redaction failed'}
   $report.checks.rcon_vault_service_authentication_without_environment='passed'
   Send-RCONFixture ('probe-rcon-credential '+$ServerID)
   $until=[DateTime]::UtcNow.AddSeconds(5);do{$vaultLogs=(Get-NodeAPI ('/api/servers/'+$ServerID+'/logs')).data;if('RCON_CREDENTIAL_UNAVAILABLE' -in $vaultLogs){break};Start-Sleep -Milliseconds 100}while([DateTime]::UtcNow -lt $until)
   if('RCON_CREDENTIAL_UNAVAILABLE' -notin $vaultLogs -or 'RCON_CREDENTIAL_PRESENT' -in $vaultLogs){throw 'Game context could resolve daemon credential'}
   $report.checks.rcon_vault_unavailable_to_distinct_game_context='passed'
   Set-Power 'kill';Wait-State 'offline';Stop-Service WingsWindowsNative
   $wrongSecret=ConvertTo-SecureString 'wrong-fixture-password' -AsPlainText -Force
   try{& $rconCommand @rconOptions -Password $wrongSecret -Replace|Out-Null}finally{$wrongSecret.Dispose()}
   Start-Service WingsWindowsNative;Wait-State 'offline';Set-Power 'start';Wait-State 'running'
   $rejected=$false;try{Send-RCONFixture 'BAD_VAULT_SECRET'}catch{$rejected=$true};if(-not $rejected){throw 'Rotated wrong secret authenticated'}
   Set-Power 'kill';Wait-State 'offline';Stop-Service WingsWindowsNative
   & $rconCommand @rconOptions -Password $rconPassword -Replace|Out-Null
   $report.checks.rcon_vault_deliberate_rotation='passed'
   $removed=& $rconCommand @rconOptions -Remove;$status=& $rconCommand @rconOptions -Inspect
   if(-not $removed.removed -or $status.present){throw 'RCON credential removal not confirmed'}
   $rconSeeded=$false;$report.cleanup.rcon_credential_removed=$true
   $gameMetadata=& (Join-Path $Root 'bin\scripts\Set-ServiceGameCredential.ps1') @rconOptions -Inspect
   if(-not $gameMetadata.present){throw 'RCON operations removed game credential'}
   $report.checks.rcon_vault_removal_preserves_game_credential='passed'
   New-ItemProperty -LiteralPath $serviceKey -Name Environment -PropertyType MultiString -Value @('WINGS_RCON_FIXTURE=fixture-rcon-only') -Force|Out-Null
   Start-Service WingsWindowsNative;Wait-State 'offline';Set-Power 'start';Wait-State 'running'
   $rejected=$false;try{Send-RCONFixture 'MISSING_VAULT_SECRET'}catch{$rejected=$true};if(-not $rejected){throw 'Missing vault reference fell back to environment'}
   Set-Power 'kill';Wait-State 'offline';$report.checks.rcon_vault_missing_secret_has_no_environment_fallback='passed'
  }finally{$rconPassword.Dispose()}
 }

 if($TestNativeInstaller){
  $callbackFile=Join-Path $Root 'private\install-callbacks.jsonl'
  function Set-InstallerMode([string]$Mode,[int]$Timeout=10){
   Stop-Service WingsWindowsNative;(Get-Service WingsWindowsNative).WaitForStatus('Stopped',[TimeSpan]::FromSeconds(15))
   $edited=Get-Content -LiteralPath $configuration -Raw|ConvertFrom-Json
   $edited.profiles.$ServerID.PSObject.Properties.Remove('source_rcon')
   $edited.profiles.$ServerID.arguments=@('spawn')
   $recipe=@{executable=(Join-Path $Root 'bin\wings-windows-demo.exe');arguments=@('install',$Mode);timeout_seconds=$Timeout;memory_mb=128}
   $edited.profiles.$ServerID|Add-Member -NotePropertyName installer -NotePropertyValue $recipe -Force
   [IO.File]::WriteAllText($configuration,($edited|ConvertTo-Json -Depth 30),[Text.UTF8Encoding]::new($false))
   Start-Service WingsWindowsNative;Wait-State 'offline'
  }
  function Start-NativeInstall([string]$Action){Invoke-RestMethod -Uri ('http://127.0.0.1:'+$nodePort+'/api/servers/'+$ServerID+'/'+$Action) -Headers @{Authorization=('Bearer '+$nodeToken)} -Method Post -TimeoutSec 5|Out-Null}
  function Wait-Install([string]$Stage){
   $until=[DateTime]::UtcNow.AddSeconds(15)
   do{$state=Get-NodeAPI ('/api/servers/'+$ServerID);if(-not $state.native_install.active){if($state.native_install.stage -ne $Stage -or $state.native_install.callback_error){throw ('Unexpected installer result '+($state.native_install|ConvertTo-Json -Compress))};return $state};Start-Sleep -Milliseconds 100}while([DateTime]::UtcNow -lt $until)
   throw 'Installer completion timeout'
  }
  function Assert-InstallCallback([bool]$Success,[bool]$Reinstall){
   $records=@(Get-Content -LiteralPath $callbackFile|ForEach-Object{$_|ConvertFrom-Json})
   $last=$records[-1]
   if($last.successful -ne $Success -or $last.reinstall -ne $Reinstall){throw 'Wrong panel callback result'}
  }
  Set-InstallerMode 'success';Start-NativeInstall 'install';$state=Wait-Install 'complete'
  if($state.state -ne 'offline' -or -not $state.native_install.successful -or [IO.File]::ReadAllText((Join-Path $Root ('data\'+$ServerID+'\install.marker'))) -ne 'installed'){throw 'Approved installation marker/state invalid'}
  Assert-InstallCallback $true $false
  $report.checks.native_install_distinct_identity_marker_callback='passed'
  Set-Power 'start';Wait-State 'running'
  Start-NativeInstall 'reinstall';$state=Wait-Install 'complete';Assert-InstallCallback $true $true
  if($state.state -ne 'offline'){throw 'Reinstall left game running'}
  $report.checks.native_reinstall_stops_game_and_sets_callback_flag='passed'
  Set-InstallerMode 'fail';Start-NativeInstall 'install';$state=Wait-Install 'failed';Assert-InstallCallback $false $false
  if($state.native_install.successful){throw 'Nonzero installer reported success'}
  $report.checks.native_install_nonzero_exit_failure_callback='passed'
  Set-InstallerMode 'hang';Start-NativeInstall 'install'
  $until=[DateTime]::UtcNow.AddSeconds(5)
  do{$lines=(Get-NodeAPI ('/api/servers/'+$ServerID+'/logs')).data;$childLine=@($lines|Where-Object{$_ -match '^\[Installer\] INSTALL_CHILD:\d+$'})|Select-Object -Last 1;if($childLine){$childPID=[int]($childLine.Split(':')[-1]);$child=Get-CimInstance Win32_Process -Filter ('ProcessId='+$childPID) -ErrorAction Stop;if($child){break}};Start-Sleep -Milliseconds 100}while([DateTime]::UtcNow -lt $until)
  if(-not $child){throw 'Installer child missing'}
  $owner=Invoke-CimMethod -InputObject $child -MethodName GetOwnerSid
  if($owner.Sid -ne $gameSid){throw 'Installer child is not the configured game identity'}
  $report.checks.native_install_child_uses_distinct_game_sid='passed'
  $rejected=$false;try{Start-NativeInstall 'reinstall'}catch{if([int]$_.Exception.Response.StatusCode -eq 409){$rejected=$true}else{throw}}
  if(-not $rejected){throw 'Concurrent reinstall allowed'}
  $rejected=$false;try{Set-Power 'start'}catch{if([int]$_.Exception.Response.StatusCode -eq 409){$rejected=$true}else{throw}}
  if(-not $rejected){throw 'Concurrent power action allowed'}
  $report.checks.native_install_rejects_concurrent_power_and_install='passed'
  Set-Power 'kill';$state=Wait-Install 'cancelled';Assert-InstallCallback $false $false
  if(Get-Process -Id $childPID -ErrorAction SilentlyContinue){throw 'Installer child survived kill'}
  $report.checks.native_install_kill_cancels_and_cleans_job_tree='passed'
  Set-InstallerMode 'hang' 1;Start-NativeInstall 'install';$state=Wait-Install 'timed_out';Assert-InstallCallback $false $false
  $report.checks.native_install_timeout_failure_callback='passed'
  Set-InstallerMode 'success'
 }

 if($TestPendingCreation){
  $creationOriginalConfig=[IO.File]::ReadAllText($configuration)
  try{
   Stop-Service WingsWindowsNative
   $edited=$creationOriginalConfig|ConvertFrom-Json
   $edited|Add-Member -NotePropertyName allow_pending_creation -NotePropertyValue $true -Force
   [IO.File]::WriteAllText($configuration,($edited|ConvertTo-Json -Depth 30),[Text.UTF8Encoding]::new($false))
   Start-Service WingsWindowsNative;Wait-State 'offline'
   $pendingID=[guid]::NewGuid().ToString()
   [IO.File]::WriteAllText((Join-Path $Root 'private\pending-uuid.txt'),$pendingID)
   $pendingRoot=Join-Path $Root ('data\'+$pendingID)
   $callbackCount=@(Get-Content -LiteralPath $callbackFile).Count
   function New-PendingRecord([bool]$AutoStart=$false){Invoke-RestMethod -Uri ('http://127.0.0.1:'+$nodePort+'/api/servers') -Headers @{Authorization=('Bearer '+$nodeToken)} -Method Post -ContentType application/json -Body (@{uuid=$pendingID;start_on_completion=$AutoStart;executable='unapproved.exe'}|ConvertTo-Json) -TimeoutSec 12|Out-Null}
   New-PendingRecord
   $pending=Get-NodeAPI ('/api/servers/'+$pendingID)
   if($pending.native_creation.stage -ne 'awaiting_profile' -or $pending.native_creation.execution_approved -or (Test-Path -LiteralPath $pendingRoot)){throw 'Creation did not remain pending without data/execution'}
   $report.checks.pending_creation_accepts_assigned_record_without_execution='passed'
   New-PendingRecord
   $listed=@(Get-NodeAPI '/api/servers'|Where-Object{$_.configuration.uuid -eq $pendingID})
   if($listed.Count -ne 1){throw 'Pending replay duplicated record'}
   $report.checks.pending_creation_idempotent_replay_and_listing='passed'
   $rejected=$false;try{New-PendingRecord $true}catch{if([int]$_.Exception.Response.StatusCode -eq 422){$rejected=$true}else{throw}}
   if(-not $rejected){throw 'Automatic start silently accepted'}
   $rejected=$false;try{Invoke-RestMethod -Uri ('http://127.0.0.1:'+$nodePort+'/api/servers/'+$pendingID+'/install') -Headers @{Authorization=('Bearer '+$nodeToken)} -Method Post -TimeoutSec 3|Out-Null}catch{if([int]$_.Exception.Response.StatusCode -eq 409){$rejected=$true}else{throw}}
   if(-not $rejected){throw 'Pending server installation accepted'}
   $report.checks.pending_creation_blocks_install_and_automatic_start='passed'
   Stop-Service WingsWindowsNative;Start-Service WingsWindowsNative;Wait-State 'offline'
   $pending=Get-NodeAPI ('/api/servers/'+$pendingID)
   if($pending.native_creation.stage -ne 'awaiting_profile' -or (Test-Path -LiteralPath $pendingRoot)){throw 'Pending discovery did not survive daemon restart'}
   $report.checks.pending_creation_reconstructed_from_panel_after_restart='passed'
   Stop-Service WingsWindowsNative
   $edited.profiles=[pscustomobject]@{}
   [IO.File]::WriteAllText($configuration,($edited|ConvertTo-Json -Depth 30),[Text.UTF8Encoding]::new($false))
   Start-Service WingsWindowsNative
   $until=[DateTime]::UtcNow.AddSeconds(12);do{try{$pending=Get-NodeAPI ('/api/servers/'+$ServerID);if($pending.native_creation.stage -eq 'awaiting_profile'){break}}catch{};Start-Sleep -Milliseconds 100}while([DateTime]::UtcNow -lt $until)
   if($pending.configuration.uuid -ne $ServerID -or $pending.native_creation.stage -ne 'awaiting_profile'){throw 'Zero-profile bootstrap failed'}
   if(@(Get-Content -LiteralPath $callbackFile).Count -ne $callbackCount){throw 'Pending bootstrap sent installation callback'}
   $report.checks.pending_creation_zero_profile_bootstrap_has_no_install_callback='passed'
  }finally{
   Stop-Service WingsWindowsNative -ErrorAction SilentlyContinue
   [IO.File]::WriteAllText($configuration,$creationOriginalConfig,[Text.UTF8Encoding]::new($false))
   Start-Service WingsWindowsNative;Wait-State 'offline'
  }
  $approved=Get-NodeAPI ('/api/servers/'+$ServerID)
  if($approved.native_creation -or -not $approved.native_install.enabled){throw 'Operator profile restore did not approve execution'}
  Start-NativeInstall 'reinstall';$approved=Wait-Install 'complete';Assert-InstallCallback $true $true
  $report.checks.pending_creation_local_profile_handoff_to_native_reinstall='passed'
 }

 if($TestInstallState){
  Stop-Service WingsWindowsNative
  $edited=Get-Content -LiteralPath $configuration -Raw|ConvertFrom-Json
  $edited.profiles.$ServerID.installer|Add-Member -NotePropertyName persist_state -NotePropertyValue $true -Force
  $edited.profiles.$ServerID.allowed_environment+=@('INSTALL_STATE_PROBE_PATH')
  [IO.File]::WriteAllText($configuration,($edited|ConvertTo-Json -Depth 30),[Text.UTF8Encoding]::new($false))
  $rejectFile=Join-Path $Root 'private\reject-install-callback'
  [IO.File]::WriteAllText($rejectFile,'fixture rejects before recording callback')
  $acceptedCount=@(Get-Content -LiteralPath $callbackFile).Count
  Start-Service WingsWindowsNative;Wait-State 'offline';Start-NativeInstall 'install'
  $until=[DateTime]::UtcNow.AddSeconds(12);do{$native=(Get-NodeAPI ('/api/servers/'+$ServerID)).native_install;if(-not $native.active -and $native.attempt_id){break};Start-Sleep -Milliseconds 100}while([DateTime]::UtcNow -lt $until)
  if($native.receipt_phase -ne 'result' -or -not $native.pending_notification -or -not $native.successful -or $native.callback_error -ne 'notification_failed'){throw 'Failed callback result was not persisted'}
  $attempt=[string]$native.attempt_id
  $report.checks.install_state_failed_callback_preserves_success_receipt='passed'
  $rejected=$false;try{Start-NativeInstall 'reinstall'}catch{if([int]$_.Exception.Response.StatusCode -eq 409){$rejected=$true}else{throw}}
  if(-not $rejected){throw 'Pending receipt overwritten by reinstall'}
  $report.checks.install_state_pending_result_blocks_another_installer='passed'
  $marker=Join-Path $Root ('data\'+$ServerID+'\install.marker')
  [IO.File]::WriteAllText($marker,'NOT_RERUN')
  Stop-Service WingsWindowsNative;Remove-Item -LiteralPath $rejectFile
  Start-Service WingsWindowsNative;Wait-State 'offline'
  $native=(Get-NodeAPI ('/api/servers/'+$ServerID)).native_install
  if($native.attempt_id -ne $attempt -or -not $native.pending_notification -or @(Get-Content -LiteralPath $callbackFile).Count -ne $acceptedCount){throw 'Restart lost result or automatically notified'}
  $report.checks.install_state_restart_retains_result_without_auto_retry='passed'
  function Reconcile-Install([string]$Attempt,[string]$Action){Invoke-RestMethod -Uri ('http://127.0.0.1:'+$nodePort+'/api/servers/'+$ServerID+'/install/reconcile') -Headers @{Authorization=('Bearer '+$nodeToken)} -Method Post -ContentType application/json -Body (@{attempt_id=$Attempt;action=$Action}|ConvertTo-Json) -TimeoutSec 8|Out-Null}
  $rejected=$false;try{Reconcile-Install ([guid]::NewGuid().ToString()) 'notify'}catch{if([int]$_.Exception.Response.StatusCode -eq 409){$rejected=$true}else{throw}}
  if(-not $rejected){throw 'Stale attempt accepted'}
  Reconcile-Install $attempt 'notify';Assert-InstallCallback $true $false
  if([IO.File]::ReadAllText($marker) -ne 'NOT_RERUN'){throw 'Notification reran installer'}
  $rejected=$false;try{Reconcile-Install $attempt 'notify'}catch{if([int]$_.Exception.Response.StatusCode -eq 409){$rejected=$true}else{throw}}
  if(-not $rejected -or @(Get-Content -LiteralPath $callbackFile).Count -ne ($acceptedCount+1)){throw 'Acknowledged notification repeated'}
  $report.checks.install_state_explicit_retry_no_rerun_stale_and_duplicate_gates='passed'
  Stop-Service WingsWindowsNative;Start-Service WingsWindowsNative;Wait-State 'offline'
  $native=(Get-NodeAPI ('/api/servers/'+$ServerID)).native_install
  if($native.receipt_phase -ne 'acknowledged' -or $native.pending_notification){throw 'Acknowledgement not restored'}
  $report.checks.install_state_acknowledgement_survives_restart='passed'
  Set-Power 'start';Wait-State 'running'
  Invoke-RestMethod -Uri ('http://127.0.0.1:'+$nodePort+'/api/servers/'+$ServerID+'/commands') -Headers @{Authorization=('Bearer '+$nodeToken)} -Method Post -ContentType application/json -Body '{"commands":["probe-install-access"]}' -TimeoutSec 5|Out-Null
  $until=[DateTime]::UtcNow.AddSeconds(5);do{$lines=(Get-NodeAPI ('/api/servers/'+$ServerID+'/logs')).data;if('INSTALL_STATE_PROBE_DENIED' -in $lines){break};Start-Sleep -Milliseconds 100}while([DateTime]::UtcNow -lt $until)
  if('INSTALL_STATE_PROBE_DENIED' -notin $lines){throw 'Game context could read private installation receipt'}
  Set-Power 'kill';Wait-State 'offline'
  $report.checks.install_state_distinct_game_read_denied='passed'
  Stop-Service WingsWindowsNative
  $edited.profiles.$ServerID.installer.arguments=@('install','hang');$edited.profiles.$ServerID.installer.timeout_seconds=120
  [IO.File]::WriteAllText($configuration,($edited|ConvertTo-Json -Depth 30),[Text.UTF8Encoding]::new($false))
  Start-Service WingsWindowsNative;Wait-State 'offline';Start-NativeInstall 'install'
  $child=$null;$until=[DateTime]::UtcNow.AddSeconds(5)
  do{$native=(Get-NodeAPI ('/api/servers/'+$ServerID)).native_install;$lines=(Get-NodeAPI ('/api/servers/'+$ServerID+'/logs')).data;$childLine=@($lines|Where-Object{$_ -match '^\[Installer\] INSTALL_CHILD:\d+$'})|Select-Object -Last 1;if($childLine){$childPID=[int]($childLine.Split(':')[-1]);$child=Get-CimInstance Win32_Process -Filter ('ProcessId='+$childPID) -ErrorAction Stop};if($native.receipt_phase -eq 'running' -and $child){break};Start-Sleep -Milliseconds 100}while([DateTime]::UtcNow -lt $until)
  if(-not $child -or $native.receipt_phase -ne 'running'){throw 'Hanging persistent installer not launched'}
  $interruptedAttempt=[string]$native.attempt_id
  $daemonPID=(Get-CimInstance Win32_Service -Filter "Name='WingsWindowsNative'").ProcessId
  Stop-Process -Id $daemonPID -Force
  (Get-Service WingsWindowsNative).WaitForStatus('Stopped',[TimeSpan]::FromSeconds(15))
  $until=[DateTime]::UtcNow.AddSeconds(3);do{if(-not (Get-Process -Id $childPID -ErrorAction SilentlyContinue)){break};Start-Sleep -Milliseconds 100}while([DateTime]::UtcNow -lt $until)
  if(Get-Process -Id $childPID -ErrorAction SilentlyContinue){throw 'Installer child survived daemon termination'}
  $report.checks.install_state_daemon_termination_kills_installer_tree='passed'
  Start-Service WingsWindowsNative;Wait-State 'offline'
  $native=(Get-NodeAPI ('/api/servers/'+$ServerID)).native_install
  if($native.stage -ne 'interrupted' -or $native.attempt_id -ne $interruptedAttempt){throw 'Interrupted receipt not recovered'}
  $rejected=$false;try{Set-Power 'start'}catch{if([int]$_.Exception.Response.StatusCode -eq 409){$rejected=$true}else{throw}}
  if(-not $rejected){throw 'Interrupted installation allowed game start'}
  $rejected=$false;try{Reconcile-Install $interruptedAttempt 'notify'}catch{if([int]$_.Exception.Response.StatusCode -eq 409){$rejected=$true}else{throw}}
  if(-not $rejected){throw 'Unknown outcome notified as completed result'}
  $report.checks.install_state_interrupted_outcome_blocks_start_and_notification='passed'
  [IO.File]::WriteAllText($marker,'INTERRUPTED_NOT_RERUN')
  Reconcile-Install $interruptedAttempt 'report_failed';Assert-InstallCallback $false $false
  if([IO.File]::ReadAllText($marker) -ne 'INTERRUPTED_NOT_RERUN'){throw 'Failure resolution reran installer'}
  $report.checks.install_state_explicit_interruption_failure_without_execution='passed'
  Stop-Service WingsWindowsNative
  $slots=@(0,1|ForEach-Object{$path=Join-Path $Root ('logs\console\'+$ServerID+'\install.'+$_+'.json');$value=Get-Content -LiteralPath $path -Raw|ConvertFrom-Json;[pscustomobject]@{Path=$path;Sequence=$value.payload.sequence}})
  $latest=$slots|Sort-Object Sequence -Descending|Select-Object -First 1
  $saved=[IO.File]::ReadAllBytes($latest.Path)
  try{[IO.File]::WriteAllText($latest.Path,'CORRUPT_INSTALL_RECEIPT');try{Start-Service WingsWindowsNative -ErrorAction Stop}catch{};(Get-Service WingsWindowsNative).WaitForStatus('Stopped',[TimeSpan]::FromSeconds(15))}finally{[IO.File]::WriteAllBytes($latest.Path,$saved)}
  Start-Service WingsWindowsNative;Wait-State 'offline'
  $report.checks.install_state_corruption_refuses_start_then_releases_handles='passed'
 }

 if($TestGamePermissions){
  Stop-Service WingsWindowsNative
  $extraOriginalConfiguration=[IO.File]::ReadAllText($configuration)
  $extraID=[guid]::NewGuid().ToString();$extraName='WwAdd_'+$suffix;$extraPassword=New-TestPassword
  if(Get-LocalUser -Name $extraName -ErrorAction SilentlyContinue){throw 'Extra fixture account exists'}
  New-LocalUser -Name $extraName -Password $extraPassword -Description 'Disposable incremental game identity'|Out-Null;$extraGameCreated=$true
  $extraSID=(Get-LocalUser -Name $extraName).SID.Value;$report.extra_identity_sid=$extraSID
  $edited=$extraOriginalConfiguration|ConvertFrom-Json
  $profile=($edited.profiles.$ServerID|ConvertTo-Json -Depth 30)|ConvertFrom-Json
  $profile.run_as.sid=$extraSID;$profile.run_as.credential_target='WingsWindows/'+$extraID
  $profile.arguments=@('spawn');$profile.use_egg_process_configuration=$false;$profile.egg_config_files=@();$profile.restore_running=$false
  $profile.allowed_environment=@('OTHER_DATA_PROBE_PATH');$profile.installer.arguments=@('install','success');$profile.installer.timeout_seconds=10
  $edited.profiles|Add-Member -NotePropertyName $extraID -NotePropertyValue $profile
  [IO.File]::WriteAllText($configuration,($edited|ConvertTo-Json -Depth 30),[Text.UTF8Encoding]::new($false))
  [IO.File]::WriteAllText((Join-Path $Root 'private\pending-uuid.txt'),$extraID)
  $permissions=Join-Path $Root 'bin\scripts\Set-ServiceGamePermissions.ps1'
  $extraOptions=@{InstallationRoot=$Root;ServerUUID=$extraID;ServiceCredential=$serviceCredential}
  foreach($p in @((Get-Item -LiteralPath $Root),(Get-Item -LiteralPath (Join-Path $Root 'data')),(Get-Item -LiteralPath (Join-Path $Root 'bin')))+@(Get-ChildItem -LiteralPath (Join-Path $Root 'bin') -Recurse -Force)){$extraOriginalACLs[$p.FullName]=(Get-Acl -LiteralPath $p.FullName).GetSecurityDescriptorSddlForm([Security.AccessControl.AccessControlSections]::Access)}
  $unchanged=@{};foreach($p in @((Get-Item -LiteralPath (Join-Path $Root ('data\'+$ServerID))))+@(Get-ChildItem -LiteralPath (Join-Path $Root ('data\'+$ServerID)) -Recurse -Force)+@((Get-Item -LiteralPath (Join-Path $Root 'logs')))+@(Get-ChildItem -LiteralPath (Join-Path $Root 'logs') -Recurse -Force)){$unchanged[$p.FullName]=@{ACL=(Get-Acl -LiteralPath $p.FullName).Sddl;Hash=$null};if(-not $p.PSIsContainer){$unchanged[$p.FullName].Hash=(Get-FileHash -LiteralPath $p.FullName -Algorithm SHA256).Hash}}
  & $permissions -InstallationRoot $Root -ServerUUID $extraID -WhatIf
  if(Test-Path -LiteralPath (Join-Path $Root ('data\'+$extraID))){throw 'WhatIf created server data'}
  if('SeBatchLogonRight' -in @([WingsWindows.NodeSetup]::Rights($extraSID))){throw 'WhatIf granted a real right'}
  foreach($p in $extraOriginalACLs.Keys){if((Get-Acl -LiteralPath $p).GetSecurityDescriptorSddlForm([Security.AccessControl.AccessControlSections]::Access) -cne $extraOriginalACLs[$p]){throw 'WhatIf changed shared code ACL'}}
  $report.checks.game_permissions_whatif_preserves_paths_and_rights='passed'
  $prepared=& $permissions -InstallationRoot $Root -ServerUUID $extraID
  if(-not $prepared.prepared -or -not $prepared.batch_right_added -or 'SeBatchLogonRight' -notin @([WingsWindows.NodeSetup]::Rights($extraSID))){throw 'New identity not prepared'}
  foreach($p in $unchanged.Keys){if((Get-Acl -LiteralPath $p).Sddl -cne $unchanged[$p].ACL){throw 'Existing game/log ACL changed'};if($unchanged[$p].Hash -and (Get-FileHash -LiteralPath $p -Algorithm SHA256).Hash -ne $unchanged[$p].Hash){throw 'Existing game/log content changed'}}
  $report.checks.game_permissions_existing_populated_data_and_logs_preserved='passed'
  $again=& $permissions -InstallationRoot $Root -ServerUUID $extraID
  if(-not $again.prepared -or $again.batch_right_added){throw 'Permission replay changed a preexisting right'}
  $report.checks.game_permissions_empty_target_replay_preserves_batch_right='passed'
  $extraCredentialSeeded=$true
  & (Join-Path $Root 'bin\scripts\Set-ServiceGameCredential.ps1') @extraOptions -GameCredential ([PSCredential]::new(('.\'+$extraName),$extraPassword))|Out-Null
  Start-Service WingsWindowsNative;Wait-State 'offline'
  $extraURI='http://127.0.0.1:'+$nodePort+'/api/servers/'+$extraID
  Invoke-RestMethod -Uri ($extraURI+'/install') -Headers @{Authorization=('Bearer '+$nodeToken)} -Method Post -TimeoutSec 5|Out-Null
  $until=[DateTime]::UtcNow.AddSeconds(15);do{$extra=Get-NodeAPI ('/api/servers/'+$extraID);if(-not $extra.native_install.active -and $extra.native_install.receipt_phase -eq 'acknowledged'){break};Start-Sleep -Milliseconds 100}while([DateTime]::UtcNow -lt $until)
  if(-not $extra.native_install.successful -or $extra.native_install.receipt_phase -ne 'acknowledged' -or -not (Test-Path -LiteralPath (Join-Path $Root ('data\'+$extraID+'\install.marker')))){throw 'New identity setup did not finish'}
  $report.checks.game_permissions_new_uuid_distinct_account_installer='passed'
  Invoke-RestMethod -Uri ($extraURI+'/power') -Headers @{Authorization=('Bearer '+$nodeToken)} -Method Post -ContentType application/json -Body '{"action":"start"}' -TimeoutSec 5|Out-Null
  $until=[DateTime]::UtcNow.AddSeconds(12);do{$extra=Get-NodeAPI ('/api/servers/'+$extraID);if($extra.state -eq 'running'){break};Start-Sleep -Milliseconds 100}while([DateTime]::UtcNow -lt $until)
  if($extra.state -ne 'running'){throw 'New UUID did not run'}
  $owned=@();foreach($process in Get-CimInstance Win32_Process -Filter "Name='wings-windows-demo.exe'"){$owner=Invoke-CimMethod -InputObject $process -MethodName GetOwnerSid;if($owner.Sid -eq $extraSID){$owned+=@($process.ProcessId)}}
  if($owned.Count -lt 2){throw 'New game tree did not use the new SID'}
  $report.checks.game_permissions_new_uuid_game_tree_uses_new_sid='passed'
  Invoke-RestMethod -Uri ($extraURI+'/commands') -Headers @{Authorization=('Bearer '+$nodeToken)} -Method Post -ContentType application/json -Body '{"commands":["probe-other-data-access"]}' -TimeoutSec 5|Out-Null
  $until=[DateTime]::UtcNow.AddSeconds(5);do{$lines=(Get-NodeAPI ('/api/servers/'+$extraID+'/logs')).data;if('OTHER_DATA_PROBE_DENIED' -in $lines){break};Start-Sleep -Milliseconds 100}while([DateTime]::UtcNow -lt $until)
  if('OTHER_DATA_PROBE_DENIED' -notin $lines){throw 'New identity can read existing game data'}
  $report.checks.game_permissions_other_server_data_read_denied='passed'
  Invoke-RestMethod -Uri ($extraURI+'/power') -Headers @{Authorization=('Bearer '+$nodeToken)} -Method Post -ContentType application/json -Body '{"action":"kill"}' -TimeoutSec 5|Out-Null
  $until=[DateTime]::UtcNow.AddSeconds(5);do{$alive=@($owned|Where-Object{Get-Process -Id $_ -ErrorAction SilentlyContinue});if(-not $alive.Count){break};Start-Sleep -Milliseconds 100}while([DateTime]::UtcNow -lt $until)
  if($alive.Count){throw 'New game tree survived kill'}
  Stop-Service WingsWindowsNative
  $rejected=$false;try{& $permissions -InstallationRoot $Root -ServerUUID $extraID|Out-Null}catch{$rejected=$true}
  if(-not $rejected){throw 'Populated target accepted for permission rewrite'}
  $report.checks.game_permissions_populated_target_rejected_after_tree_cleanup='passed'
 }
 if($PrepareReboot){
  Set-Power 'start';Wait-State 'running'
  Invoke-RestMethod -Uri ('http://127.0.0.1:'+$nodePort+'/api/servers/'+$ServerID+'/commands') -Headers @{Authorization=('Bearer '+$nodeToken)} -Method Post -ContentType application/json -Body (@{commands=@('REBOOT_MARKER_'+$suffix)}|ConvertTo-Json)|Out-Null
  $until=[DateTime]::UtcNow.AddSeconds(5);do{if(('ECHO:REBOOT_MARKER_'+$suffix) -in (Get-NodeAPI ('/api/servers/'+$ServerID+'/logs')).data){break};Start-Sleep -Milliseconds 100}while([DateTime]::UtcNow -lt $until)
  if(-not $TestRestoreRunning){Set-Power 'stop';Wait-State 'offline'}
  & (Join-Path $Staging 'Prepare-Reboot.ps1') -Root $Root -Staging $Staging -Package $Package -ServerID $ServerID -DaemonSID $daemonSid -GameSID $gameSid -ServiceCredential $serviceCredential
  $rebootPrepared=$true;$report.reboot_prepared=$true;throw 'WINGS_REBOOT_PREPARED'
 }
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
} catch {if($rebootPrepared -and $_.Exception.Message -eq 'WINGS_REBOOT_PREPARED'){$report.passed=$true}else{$report.error=$_.Exception.Message}}
finally {
 if(-not $rebootPrepared){

 if($extraGameCreated){
  try{
   Stop-Service WingsWindowsNative -ErrorAction SilentlyContinue
   if($extraCredentialSeeded){$options=@{InstallationRoot=$Root;ServerUUID=$extraID;ServiceCredential=$serviceCredential};& (Join-Path $Root 'bin\scripts\Set-ServiceGameCredential.ps1') @options -Remove|Out-Null;$metadata=& (Join-Path $Root 'bin\scripts\Set-ServiceGameCredential.ps1') @options -Inspect;if($metadata.present){throw 'Extra credential remains'};$extraCredentialSeeded=$false}
   foreach($path in $extraOriginalACLs.Keys){$acl=[Security.AccessControl.FileSecurity]::new();if(Test-Path -LiteralPath $path -PathType Container){$acl=[Security.AccessControl.DirectorySecurity]::new()};$acl.SetSecurityDescriptorSddlForm($extraOriginalACLs[$path],[Security.AccessControl.AccessControlSections]::Access);if($acl -is [Security.AccessControl.DirectorySecurity]){[IO.Directory]::SetAccessControl($path,$acl)}else{[IO.File]::SetAccessControl($path,$acl)}}
   if($extraOriginalConfiguration){[IO.File]::WriteAllText($configuration,$extraOriginalConfiguration,[Text.UTF8Encoding]::new($false))}
   if('SeBatchLogonRight' -in @([WingsWindows.NodeSetup]::Rights($extraSID))){[WingsWindows.NodeSetup]::Right($extraSID,'SeBatchLogonRight',$true)}
   Remove-LocalUser -Name $extraName;$extraGameCreated=$false
   $report.cleanup.extra_game_credential_account_right_and_code_acls_restored=$true
  }catch{$extraCleanupFailed=$true;$report.cleanup.extra_game_cleanup_error=$_.Exception.Message;$report.passed=$false}
 }
 if($rconSeeded){try{Stop-Service WingsWindowsNative -ErrorAction Stop;$removed=& $rconCommand @rconOptions -Remove;$status=& $rconCommand @rconOptions -Inspect;if($status.present){throw 'RCON credential remains'};$rconSeeded=$false;$report.cleanup.rcon_credential_removed=$true}catch{$report.cleanup.rcon_credential_cleanup_failed=$true;$report.passed=$false}}
 if($credentialSeeded -and $daemonSid){try{Stop-Service WingsWindowsNative -ErrorAction Stop;Invoke-Bootstrap -Cleanup;$credentialSeeded=$false;$report.cleanup.credential_removed=$true}catch{$report.cleanup.credential_error=$_.Exception.Message;$report.passed=$false}}
 if($serviceCreated -and -not $credentialSeeded -and -not $rconSeeded -and -not $extraCredentialSeeded -and -not $extraCleanupFailed){try{Stop-Service WingsWindowsNative -ErrorAction SilentlyContinue;$s=Get-Service WingsWindowsNative;try{$s.WaitForStatus('Stopped',[TimeSpan]::FromSeconds(10))}finally{$s.Dispose()}; & "$env:SystemRoot\System32\sc.exe" delete WingsWindowsNative|Out-Null;$report.cleanup.service_removed=($LASTEXITCODE -eq 0)}catch{$report.cleanup.service_error=$_.Exception.Message;$report.passed=$false}}
 if($mock){Stop-Process -Id $mock.Id -ErrorAction SilentlyContinue;$mock.Dispose();$report.cleanup.mock_stopped=$true}
 if($eventSourceCreated){try{& $eventScript -Remove;$report.cleanup.event_source_removed=(-not (Test-Path -LiteralPath $eventKey))}catch{$report.cleanup.event_source_error=$_.Exception.Message;$report.passed=$false}}
 if($extraCredentialSeeded -or $extraCleanupFailed -or $rconSeeded -or $credentialSeeded -or (Get-Service WingsWindowsNative -ErrorAction SilentlyContinue)){$report.passed=$false;$report.cleanup.state_retained='Service or credential cleanup incomplete; accounts and rights retained for inspection'}else{
 foreach($entry in @(@{SID=$daemonSid;Names=@('SeServiceLogonRight','SeAssignPrimaryTokenPrivilege','SeIncreaseQuotaPrivilege','SeBatchLogonRight')},@{SID=$gameSid;Names=@('SeBatchLogonRight')})){
  if($entry.SID){foreach($right in $entry.Names){if($right -in @([WingsWindows.NodeSetup]::Rights($entry.SID))){[WingsWindows.NodeSetup]::Right($entry.SID,$right,$true)}}}
 }
 if(Get-LocalUser -Name $DaemonName -ErrorAction SilentlyContinue){Remove-LocalUser -Name $DaemonName};if($gameCreated){Remove-LocalUser -Name $GameName}
 $report.cleanup.accounts_and_rights_removed=$true
 }
 }
 if($extraPassword){$extraPassword.Dispose()};if($gamePassword){$gamePassword.Dispose()};if($daemonPassword){$daemonPassword.Dispose()}
}
$json=$report|ConvertTo-Json -Depth 10
if(Test-Path -LiteralPath $Root){[IO.File]::WriteAllText((Join-Path $Root 'scm-report.json'),$json,[Text.UTF8Encoding]::new($false))}
Write-Output $json
