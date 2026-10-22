$ErrorActionPreference = 'Stop'
function Initialize-CredentialBridge {
    $ErrorActionPreference='Stop'
    if ('WingsWindows.CredentialPipe' -as [type]) { return }
    Add-Type -TypeDefinition @'
using System;
using System.ComponentModel;
using System.IO;
using System.IO.Pipes;
using System.Runtime.InteropServices;
using System.Text;
using Microsoft.Win32.SafeHandles;
namespace WingsWindows {
 public static class CredentialPipe {
  [DllImport("kernel32.dll",SetLastError=true)] static extern bool GetNamedPipeClientProcessId(SafePipeHandle pipe,out uint pid);
  [DllImport("kernel32.dll",SetLastError=true)] static extern bool GetNamedPipeServerProcessId(SafePipeHandle pipe,out uint pid);
  public static uint Peer(PipeStream pipe,bool server) {uint pid;bool ok=server?GetNamedPipeClientProcessId(pipe.SafePipeHandle,out pid):GetNamedPipeServerProcessId(pipe.SafePipeHandle,out pid);if(!ok)throw new Win32Exception(Marshal.GetLastWin32Error());return pid;}
  static void Read(PipeStream pipe,byte[] b,int count) {
   int offset=0;DateTime deadline=DateTime.UtcNow.AddSeconds(15);
   while(offset<count){IAsyncResult pending=pipe.BeginRead(b,offset,count-offset,null,null);using(System.Threading.WaitHandle wait=pending.AsyncWaitHandle){double left=(deadline-DateTime.UtcNow).TotalMilliseconds;if(left<=0||!wait.WaitOne((int)Math.Min(15000,left)))throw new TimeoutException("Credential pipe read timed out.");int n=pipe.EndRead(pending);if(n==0)throw new EndOfStreamException();offset+=n;}}
  }
  public static string Receive(PipeStream pipe) {
   byte[] header=new byte[4];Read(pipe,header,4);int size=BitConverter.ToInt32(header,0);if(size<1||size>32768)throw new InvalidDataException("Invalid credential frame size.");byte[] body=new byte[size];try{Read(pipe,body,size);return new UTF8Encoding(false,true).GetString(body);}finally{Array.Clear(body,0,body.Length);}
  }
  public static void Send(PipeStream pipe,string text) {
   byte[] body=Encoding.UTF8.GetBytes(text);try{if(body.Length<1||body.Length>32768)throw new InvalidDataException("Invalid credential frame size.");byte[] frame=new byte[body.Length+4];try{Buffer.BlockCopy(BitConverter.GetBytes(body.Length),0,frame,0,4);Buffer.BlockCopy(body,0,frame,4,body.Length);IAsyncResult pending=pipe.BeginWrite(frame,0,frame.Length,null,null);using(System.Threading.WaitHandle wait=pending.AsyncWaitHandle){if(!wait.WaitOne(15000))throw new TimeoutException("Credential pipe write timed out.");pipe.EndWrite(pending);}}finally{Array.Clear(frame,0,frame.Length);}}finally{Array.Clear(body,0,body.Length);}
  }
 }
}
'@
}

function Get-ServiceCredentialPlan([string]$InstallationRoot,[string]$ServerUUID,[PSCredential]$ServiceCredential,[PSCredential]$GameCredential,[string]$Operation,[ValidateSet('Game','RCON')][string]$Purpose='Game') {
    $ErrorActionPreference='Stop'
    Initialize-NodeSetupNative
    if($ServerUUID -notmatch '\A[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}\z'){throw 'A full server UUID is required.'}
    if($Operation -notin @('Store','Inspect','Remove')){throw 'Unsupported credential operation.'}
    $account=Get-NodeLocalAccount $ServiceCredential.UserName
    if($ServiceCredential.Password.Length -eq 0){throw 'A service password is required.'}
    $service=Get-CimInstance Win32_Service -Filter "Name='WingsWindowsNative'" -ErrorAction Stop
    if(-not $service -or $service.State -ne 'Stopped'){throw 'Stop WingsWindowsNative before credential provisioning.'}
    $startName=$service.StartName
    if($startName.StartsWith('.\')){$startName=$env:COMPUTERNAME+$startName.Substring(1)}
    $registeredSID=([Security.Principal.NTAccount]::new($startName)).Translate([Security.Principal.SecurityIdentifier]).Value
    if($registeredSID -ne $account.SID.Value){throw 'Service credential does not match the registered service identity.'}
    $root=[IO.Path]::GetFullPath($InstallationRoot).TrimEnd('\')
    $exe=Join-Path $root 'bin\wings-windows.exe';$config=Join-Path $root 'private\config.json'
    if($service.PathName -cne ('"'+$exe+'" --config "'+$config+'"')){throw 'Service registration must match the managed installation layout exactly.'}
    $worker=Join-Path $root 'bin\scripts\CredentialTask.ps1'
    $nodeModule=(Get-Command Initialize-NodeSetupNative).Module
    if(-not $nodeModule){throw 'Import NodeSetup before credential planning.'}
    foreach($path in @($root,$exe,$config,$worker,(Join-Path $root 'bin\scripts\CredentialBridge.psm1'),(Join-Path $root 'bin\scripts\Store-GameCredential.ps1'))){& $nodeModule {param($p) Assert-NodeSetupPath $p} $path;if(-not(Test-Path -LiteralPath $path)){throw 'Managed credential setup files are missing.'}}
    $configuration=Get-Content -LiteralPath $config -Raw | ConvertFrom-Json
    $id=$ServerUUID.ToLowerInvariant();$profile=$configuration.profiles.PSObject.Properties[$id].Value
    if(-not $profile.run_as -or $profile.run_as.credential_target -cne ('WingsWindows/'+$id)){throw 'Configure the server run_as SID and exact credential target first.'}
    if($profile.run_as.sid -eq $account.SID.Value){throw 'Game identity must differ from the daemon.'}
    if($Purpose -eq 'RCON'){
        if(-not $profile.source_rcon -or $profile.source_rcon.password_credential_target -cne ('WingsWindowsRCON/'+$id) -or $profile.source_rcon.password_environment){throw 'Configure the exact server RCON credential target without an environment reference first.'}
        $store=Join-Path $root 'bin\scripts\Store-RCONCredential.ps1'
        & $nodeModule {param($p) Assert-NodeSetupPath $p} $store
        if(-not(Test-Path -LiteralPath $store)){throw 'Managed RCON credential setup file is missing.'}
    }
    if($Operation -eq 'Store'){
        if(-not $GameCredential -or $GameCredential.Password.Length -eq 0 -or $GameCredential.Password.Length -gt 1280){throw 'A nonempty game credential within the storage limit is required.'}
        if($Purpose -eq 'Game'){
            $game=Get-NodeLocalAccount $GameCredential.UserName
            if($game.SID.Value -ne $profile.run_as.sid){throw 'Game credential does not match the configured pinned SID.'}
        }elseif($GameCredential.UserName -cne '.\WingsRCONSecret-v1'){throw 'Invalid RCON secret format.'}
    }
    return [pscustomobject]@{Root=$root;Executable=$exe;Configuration=$config;Worker=$worker;ServerUUID=$id;DaemonSID=$account.SID.Value;Operation=$Operation;Purpose=$Purpose}
}

function Invoke-ServiceCredentialTask($Plan,[PSCredential]$ServiceCredential,[PSCredential]$GameCredential,[bool]$Replace) {
    $ErrorActionPreference='Stop'
    Initialize-NodeSetupNative;Initialize-CredentialBridge
    $nodeModule=(Get-Command Initialize-NodeSetupNative).Module
    $lockPath=Join-Path $Plan.Root 'private\credential-provisioning.lock'
    & $nodeModule {param($p) Assert-NodeSetupPath $p} $lockPath
    # Serialize task/temporary-right ownership across this installation. The
    # empty lock contains no credentials and stays in the protected private tree.
    $gate=[IO.File]::Open($lockPath,[IO.FileMode]::OpenOrCreate,[IO.FileAccess]::ReadWrite,[IO.FileShare]::None)
    try {
    $rights=@([WingsWindows.NodeSetup]::Rights($Plan.DaemonSID))
    if('SeDenyBatchLogonRight' -in $rights){throw 'The daemon identity has an explicit deny-batch-logon right.'}
    $added=$false;$attempted=$false;$pipe=$null;$task='WingsCredential_'+[guid]::NewGuid().ToString('N');$result=$null;$failure=$null;$cleanup=@()
    try {
        if('SeBatchLogonRight' -notin $rights){[WingsWindows.NodeSetup]::Right($Plan.DaemonSID,'SeBatchLogonRight',$false);$added=$true}
        $name='WingsCredential_'+[guid]::NewGuid().ToString('N')
        $security=[IO.Pipes.PipeSecurity]::new();$security.SetAccessRuleProtection($true,$false)
        foreach($sid in @('S-1-5-18','S-1-5-32-544',$Plan.DaemonSID)){$security.AddAccessRule([IO.Pipes.PipeAccessRule]::new([Security.Principal.SecurityIdentifier]::new($sid),[IO.Pipes.PipeAccessRights]::ReadWrite,[Security.AccessControl.AccessControlType]::Allow))}
        $pipe=[IO.Pipes.NamedPipeServerStream]::new($name,[IO.Pipes.PipeDirection]::InOut,1,[IO.Pipes.PipeTransmissionMode]::Byte,[IO.Pipes.PipeOptions]::Asynchronous,4096,4096,$security)
        $arguments='-NoProfile -NonInteractive -WindowStyle Hidden -ExecutionPolicy Bypass -File "'+$Plan.Worker+'" -PipeName "'+$name+'" -ExpectedSID "'+$Plan.DaemonSID+'" -ParentProcessID '+$PID
        $action=New-ScheduledTaskAction -Execute "$env:SystemRoot\System32\WindowsPowerShell\v1.0\powershell.exe" -Argument $arguments -ErrorAction Stop
        $settings=New-ScheduledTaskSettingsSet -ExecutionTimeLimit ([TimeSpan]::FromMinutes(1)) -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -ErrorAction Stop
        $user=$env:COMPUTERNAME+'\'+$ServiceCredential.UserName.Substring(2)
        $attempted=$true
        Register-ScheduledTask -TaskName $task -Action $action -Settings $settings -User $user -Password $ServiceCredential.GetNetworkCredential().Password -RunLevel Limited -ErrorAction Stop | Out-Null
        $pending=$pipe.BeginWaitForConnection($null,$null)
        try{Start-ScheduledTask -TaskName $task -ErrorAction Stop;if(-not $pending.AsyncWaitHandle.WaitOne(20000)){throw 'Credential worker did not connect; check password and batch-logon policy.'};$pipe.EndWaitForConnection($pending)}finally{$pending.AsyncWaitHandle.Dispose()}
        $hello=[WingsWindows.CredentialPipe]::Receive($pipe)|ConvertFrom-Json
        $peer=[WingsWindows.CredentialPipe]::Peer($pipe,$true)
        $process=Get-CimInstance Win32_Process -Filter ('ProcessId='+$peer) -ErrorAction Stop
        $owner=Invoke-CimMethod -InputObject $process -MethodName GetOwnerSid -ErrorAction Stop
        if($owner.Sid -ne $Plan.DaemonSID -or $hello.sid -ne $Plan.DaemonSID){throw 'Credential worker identity mismatch.'}
        $purpose='Game';if($Plan.Purpose){$purpose=$Plan.Purpose};if($purpose -notin @('Game','RCON')){throw 'Invalid credential purpose.'}
        $request=[ordered]@{operation=$Plan.Operation;purpose=$purpose;server=$Plan.ServerUUID;replace=$Replace;username=$null;password=$null}
        if($Plan.Operation -eq 'Store'){$request.username=$GameCredential.UserName;$request.password=$GameCredential.GetNetworkCredential().Password}
        try{[WingsWindows.CredentialPipe]::Send($pipe,($request|ConvertTo-Json -Compress))}finally{$request.password=$null}
        $result=[WingsWindows.CredentialPipe]::Receive($pipe)|ConvertFrom-Json
        if(-not $result.ok){throw 'Credential operation failed in the daemon context. Check existing target/rotation rules; inspect before retrying.'}
    } catch {$failure=$_}
    finally {
        if($pipe){$pipe.Dispose()}
        $stopped=$true
        $ownedTask=$null
        if($attempted){try{$ownedTask=Get-ScheduledTask -TaskName $task -ErrorAction Stop}catch{
            if($_.FullyQualifiedErrorId -notlike 'CmdletizationQuery_NotFound_TaskName*'){$stopped=$false;$cleanup+=$_.Exception.Message}
        }}
        # Registration can partially create a task before returning an error.
        # Inspect the exact random name even when Register-ScheduledTask threw.
        if($ownedTask){$stopped=$false;try{
            $deadline=[DateTime]::UtcNow.AddSeconds(5)
            while((Get-ScheduledTask -TaskName $task -ErrorAction Stop).State -eq 'Running' -and [DateTime]::UtcNow -lt $deadline){Start-Sleep -Milliseconds 100}
            if((Get-ScheduledTask -TaskName $task -ErrorAction Stop).State -eq 'Running'){Stop-ScheduledTask -TaskName $task -ErrorAction Stop;Start-Sleep -Milliseconds 300}
            if((Get-ScheduledTask -TaskName $task -ErrorAction Stop).State -eq 'Running'){$stopped=$false;throw ('Credential task still running: '+$task)}
            $stopped=$true
            Unregister-ScheduledTask -TaskName $task -Confirm:$false -ErrorAction Stop
        }catch{$cleanup+=$_.Exception.Message}}
        if($added -and $stopped){try{[WingsWindows.NodeSetup]::Right($Plan.DaemonSID,'SeBatchLogonRight',$true)}catch{$cleanup+=$_.Exception.Message}}
        if(-not $stopped){$cleanup+='Temporary batch right retained for task inspection.'}
    }
    if($cleanup.Count){throw ('Credential provisioning cleanup requires inspection: '+($cleanup -join '; ')+'. Credential may have changed; inspect before retrying.')}
    if($failure){throw ('Credential provisioning failed: '+$failure.Exception.Message+' Credential may have changed; inspect before retrying.')}
    return $result.metadata
    } finally {$gate.Dispose()}
}
Export-ModuleMember -Function Initialize-CredentialBridge,Get-ServiceCredentialPlan,Invoke-ServiceCredentialTask
