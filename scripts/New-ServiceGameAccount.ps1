#Requires -RunAsAdministrator
[CmdletBinding(SupportsShouldProcess=$true)]
param(
    [Parameter(Mandatory=$true)][string]$InstallationRoot,
    [Parameter(Mandatory=$true)][string]$ServerUUID,
    [Parameter(Mandatory=$true)][string]$AccountName,
    [Security.SecureString]$Password
)
$ErrorActionPreference='Stop'
if($PSVersionTable.PSEdition -ne 'Desktop' -or $PSVersionTable.PSVersion.Major -ne 5){throw 'Run this command with elevated Windows PowerShell 5.1.'}
Import-Module (Join-Path $PSScriptRoot 'GameAccounts.psm1') -Force
$plan=Get-ServiceGameAccountPlan $InstallationRoot $ServerUUID $AccountName
$configuration=[IO.File]::Open($plan.Configuration,[IO.FileMode]::Open,[IO.FileAccess]::Read,[IO.FileShare]::Read)
$lock=$null
$ownedPassword=$false
try{
    & $plan.Executable --config $plan.Configuration --check | Out-Null
    if($LASTEXITCODE -ne 0){throw 'Configuration validation failed.'}
    if($PSCmdlet.ShouldProcess($AccountName,('Create dedicated local game account for '+$plan.ServerUUID))){
        if(-not $Password){$Password=Read-Host 'New game account password' -AsSecureString;$ownedPassword=$true}
        if($Password.Length -eq 0 -or $Password.Length -gt 1280){throw 'Provide a nonempty password within the credential storage limit.'}
        $lock=Open-GamePermissionLock $plan.Root
        New-ServiceGameAccount $plan $Password
    }
}finally{if($lock){$lock.Dispose()};$configuration.Dispose();if($ownedPassword -and $Password){$Password.Dispose()}}
