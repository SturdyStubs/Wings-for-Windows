# Internal worker. Invoke through Set-ServiceGameCredential.ps1, never directly.
param([Parameter(Mandatory=$true)][string]$PipeName,[Parameter(Mandatory=$true)][string]$ExpectedSID,[Parameter(Mandatory=$true)][uint32]$ParentProcessID)
$ErrorActionPreference='Stop';$pipe=$null;$credential=$null
try {
    Import-Module (Join-Path $PSScriptRoot 'CredentialBridge.psm1') -Force
    Initialize-CredentialBridge
    $sid=[Security.Principal.WindowsIdentity]::GetCurrent().User.Value
    if($sid -ne $ExpectedSID){throw 'Worker identity mismatch'}
    $pipe=[IO.Pipes.NamedPipeClientStream]::new('.',$PipeName,[IO.Pipes.PipeDirection]::InOut,[IO.Pipes.PipeOptions]::Asynchronous)
    $pipe.Connect(15000)
    if([WingsWindows.CredentialPipe]::Peer($pipe,$false) -ne $ParentProcessID){throw 'Worker parent mismatch'}
    [WingsWindows.CredentialPipe]::Send($pipe,(@{sid=$sid}|ConvertTo-Json -Compress))
    $request=[WingsWindows.CredentialPipe]::Receive($pipe)|ConvertFrom-Json
    if($request.server -notmatch '\A[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}\z'){throw 'Invalid target'}
    $exe=Join-Path (Split-Path -Parent $PSScriptRoot) 'wings-windows.exe'
    $result=@{ok=$false;metadata=$null}
    try {
        switch -CaseSensitive ($request.operation) {
            'Store' {
                $secure=ConvertTo-SecureString $request.password -AsPlainText -Force;$request.password=$null
                try{$credential=[PSCredential]::new($request.username,$secure);& (Join-Path $PSScriptRoot 'Store-GameCredential.ps1') -ServerUUID $request.server -Credential $credential -Replace:([bool]$request.replace)|Out-Null}finally{$secure.Dispose();$credential=$null}
                $output=& $exe --credential-status $request.server
            }
            'Inspect' {$output=& $exe --credential-status $request.server}
            'Remove' {$output=& $exe --remove-credential $request.server}
            default {throw 'Invalid operation'}
        }
        if($LASTEXITCODE -ne 0){throw 'Credential CLI failed'}
        $result.metadata=($output -join "`n")|ConvertFrom-Json;$result.ok=$true
    } catch { $result=@{ok=$false;metadata=$null} }
    [WingsWindows.CredentialPipe]::Send($pipe,($result|ConvertTo-Json -Depth 5 -Compress))
} catch { exit 1 }
finally {if($request){$request.password=$null};if($pipe){$pipe.Dispose()}}
