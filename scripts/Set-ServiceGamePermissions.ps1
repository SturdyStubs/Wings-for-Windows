#Requires -RunAsAdministrator
[CmdletBinding(SupportsShouldProcess=$true)]
param([Parameter(Mandatory=$true)][string]$InstallationRoot,[Parameter(Mandatory=$true)][string]$ServerUUID)
$ErrorActionPreference='Stop'
if($PSVersionTable.PSEdition -ne 'Desktop' -or $PSVersionTable.PSVersion.Major -ne 5){throw 'Run this command with elevated Windows PowerShell 5.1.'}
Import-Module (Join-Path $PSScriptRoot 'GamePermissions.psm1') -Force
$service=Get-GamePermissionService $InstallationRoot
$lock=$null
try{
    $configuration=[IO.File]::Open($service.Configuration,[IO.FileMode]::Open,[IO.FileAccess]::Read,[IO.FileShare]::Read)
    try{
        & $service.Executable --config $service.Configuration --check | Out-Null
        if($LASTEXITCODE -ne 0){throw 'Configuration validation failed.'}
        & $service.Executable --config $service.Configuration --check-profiles | Out-Null
        if($LASTEXITCODE -ne 0){throw 'Code/dependency preflight failed.'}
        $plan=Get-GamePermissionPlan $service.Root $ServerUUID $service.DaemonSID
        if($PSCmdlet.ShouldProcess($ServerUUID,'Prepare new game read/execute, private data ACLs and batch logon right')){
            $lock=Open-GamePermissionLock $service.Root
            $current=Get-GamePermissionService $service.Root
            if($current.DaemonSID -ne $plan.DaemonSID){throw 'Service identity changed after planning.'}
            Enable-GamePermissionPlan $plan
        }
    }finally{$configuration.Dispose()}
}finally{if($lock){$lock.Dispose()}}
