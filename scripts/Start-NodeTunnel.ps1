[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)][ValidatePattern('^[A-Za-z0-9][A-Za-z0-9.-]*$')][string]$HostName,
    [Parameter(Mandatory=$true)][ValidatePattern('^[A-Za-z0-9_-]+$')][string]$UserName,
    [Parameter(Mandatory=$true)][string]$IdentityFile,
    [Parameter(Mandatory=$true)][string]$KnownHostsFile,
    [ValidateRange(1,65535)][int]$SSHPort=22,
    [ValidateRange(1,65535)][int]$RemotePort=18080,
    [ValidateRange(1,65535)][int]$LocalPort=8080
)
$ErrorActionPreference='Stop'
$ssh=Join-Path $env:SystemRoot 'System32\OpenSSH\ssh.exe'
foreach($file in @($ssh,$IdentityFile,$KnownHostsFile)){
    if(-not (Test-Path -LiteralPath $file -PathType Leaf)){throw 'A required tunnel executable or pinned credential file is missing'}
}
# Run in a dedicated protected directory. Do not use game-writable paths.
# No SSH config or agent is inherited; unknown/changed hosts fail closed.
$arguments=@('-F','NUL','-NT','-p',[string]$SSHPort,'-i',$IdentityFile,
    '-o','IdentitiesOnly=yes','-o','IdentityAgent=none','-o','BatchMode=yes',
    '-o','StrictHostKeyChecking=yes','-o',('UserKnownHostsFile='+$KnownHostsFile),
    '-o','ExitOnForwardFailure=yes','-o','ConnectTimeout=10',
    '-o','ServerAliveInterval=15','-o','ServerAliveCountMax=3',
    '-R',('127.0.0.1:{0}:127.0.0.1:{1}' -f $RemotePort,$LocalPort),
    ($UserName+'@'+$HostName))
while($true){
    # Native stderr must not terminate the supervisor on Windows PowerShell 5.1.
    # The supervisor intentionally keeps no unbounded connection log.
    $previous=$ErrorActionPreference
    try{$ErrorActionPreference='Continue';& $ssh @arguments 2>&1|Out-Null}
    finally{$ErrorActionPreference=$previous}
    Start-Sleep -Seconds 10
}
