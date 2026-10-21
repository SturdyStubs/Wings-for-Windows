#Requires -RunAsAdministrator
param([string]$Root,[string]$Staging,[string]$Package,[string]$ServerID,[string]$DaemonSID,[string]$GameSID,[PSCredential]$ServiceCredential)
$ErrorActionPreference='Stop'
if($Root -notmatch '^C:\\WingsWindowsTesting\\scm-([0-9a-f]{8})$'){throw 'Invalid disposable reboot root'}
$suffix=$Matches[1];$mockTask='WingsRebootMock_'+$suffix;$resumeTask='WingsRebootResume_'+$suffix
foreach($name in @($mockTask,$resumeTask)){if(Get-ScheduledTask -TaskName $name -ErrorAction SilentlyContinue){throw 'Reboot task exists'}}
$secret=Join-Path $Root 'private\reboot';if(Test-Path -LiteralPath $secret){throw 'Reboot state exists'}
$service=Get-CimInstance Win32_Service -Filter "Name='WingsWindowsNative'"
if($service.PathName -ne ('"'+$Root+'\bin\wings-windows.exe" --config "'+$Root+'\private\config.json"')){throw 'Unexpected service path'}
$created=@();$success=$false
try {
 New-Item -ItemType Directory -Path $secret|Out-Null
 $acl=[Security.AccessControl.DirectorySecurity]::new();$acl.SetAccessRuleProtection($true,$false)
 foreach($sid in @('S-1-5-18','S-1-5-32-544')){$rule=[Security.AccessControl.FileSystemAccessRule]::new([Security.Principal.SecurityIdentifier]::new($sid),'FullControl','ContainerInherit,ObjectInherit','None','Allow');$acl.AddAccessRule($rule)}
 Set-Acl -LiteralPath $secret -AclObject $acl
 Add-Type -AssemblyName System.Security
 $pointer=[Runtime.InteropServices.Marshal]::SecureStringToGlobalAllocUnicode($ServiceCredential.Password)
 try{
  $plain=[Runtime.InteropServices.Marshal]::PtrToStringUni($pointer);$bytes=[Text.Encoding]::UTF8.GetBytes($plain)
  try{$protected=[Security.Cryptography.ProtectedData]::Protect($bytes,$null,[Security.Cryptography.DataProtectionScope]::LocalMachine)}finally{[Array]::Clear($bytes,0,$bytes.Length);$plain=$null}
 }finally{[Runtime.InteropServices.Marshal]::ZeroFreeGlobalAllocUnicode($pointer)}
 $build=Get-Content -LiteralPath (Join-Path $Package 'BUILD-INFO.json') -Raw|ConvertFrom-Json
 $state=@{schema_version=1;root=$Root;staging=$Staging;server_id=$ServerID;daemon_sid=$DaemonSID;game_sid=$GameSID;service_user=$ServiceCredential.UserName;protected_password=[Convert]::ToBase64String($protected);boot_utc=(Get-CimInstance Win32_OperatingSystem).LastBootUpTime.ToUniversalTime().ToString('o');version=$build.version;source_revision=$build.source_revision;daemon_sha256=(Get-FileHash -LiteralPath (Join-Path $Root 'bin\wings-windows.exe')).Hash;mock_task=$mockTask;resume_task=$resumeTask;marker=('REBOOT_MARKER_'+$suffix)}
 [IO.File]::WriteAllText((Join-Path $secret 'state.json'),($state|ConvertTo-Json),[Text.UTF8Encoding]::new($false))
 Copy-Item -LiteralPath (Join-Path $Package 'scripts\Register-EventSource.ps1') -Destination (Join-Path $Root 'bin\Register-EventSource.ps1')
 Copy-Item -LiteralPath (Join-Path $Staging 'Resume-Reboot.ps1') -Destination (Join-Path $Root 'bin\Resume-Reboot.ps1')
 $principal=New-ScheduledTaskPrincipal -UserId 'SYSTEM' -LogonType ServiceAccount -RunLevel Highest
 $settings=New-ScheduledTaskSettingsSet -Hidden -ExecutionTimeLimit (New-TimeSpan -Minutes 15) -StartWhenAvailable
 $trigger=New-ScheduledTaskTrigger -AtStartup
 $action=New-ScheduledTaskAction -Execute (Join-Path $Staging 'mock-panel.exe') -Argument ('"'+$Root+'\private\config.json"')
 # The mock deliberately remains running until the bounded resume task cleans it.
 $mockSettings=New-ScheduledTaskSettingsSet -Hidden -ExecutionTimeLimit ([TimeSpan]::Zero) -StartWhenAvailable
 Register-ScheduledTask -TaskName $mockTask -Action $action -Trigger $trigger -Principal $principal -Settings $mockSettings|Out-Null;$created+=$mockTask
 $resumeTrigger=New-ScheduledTaskTrigger -AtStartup;$resumeTrigger.Delay='PT3M'
 $action=New-ScheduledTaskAction -Execute "$env:SystemRoot\System32\WindowsPowerShell\v1.0\powershell.exe" -Argument ('-NoProfile -NonInteractive -ExecutionPolicy Bypass -File "'+$Root+'\bin\Resume-Reboot.ps1" -Root "'+$Root+'"')
 Register-ScheduledTask -TaskName $resumeTask -Action $action -Trigger $resumeTrigger -Principal $principal -Settings $settings|Out-Null;$created+=$resumeTask
 & "$env:SystemRoot\System32\sc.exe" config WingsWindowsNative start= delayed-auto|Out-Null
 if($LASTEXITCODE -ne 0){throw 'Delayed service setup failed'}
 $success=$true
}finally{
 if(-not $success){foreach($name in $created){Unregister-ScheduledTask -TaskName $name -Confirm:$false -ErrorAction SilentlyContinue};Set-Service WingsWindowsNative -StartupType Manual;Remove-Item -LiteralPath (Join-Path $secret 'state.json') -ErrorAction SilentlyContinue}
}
