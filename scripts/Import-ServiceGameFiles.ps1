#Requires -RunAsAdministrator
[CmdletBinding(SupportsShouldProcess=$true)]
param(
    [Parameter(Mandatory=$true)][string]$InstallationRoot,
    [Parameter(Mandatory=$true)][string]$ServerUUID,
    [Parameter(Mandatory=$true)][string]$SourceDirectory,
    [Parameter(Mandatory=$true)][string]$ManifestPath,
    [Parameter(Mandatory=$true)][string]$ManifestSHA256
)
$ErrorActionPreference='Stop'
if($PSVersionTable.PSEdition -ne 'Desktop' -or $PSVersionTable.PSVersion.Major -ne 5){throw 'Run this command with elevated Windows PowerShell 5.1.'}
Import-Module (Join-Path $PSScriptRoot 'GameFiles.psm1') -Force
$plan=$null;$lock=$null;$configuration=$null
try{
    $plan=Get-GameFilePlan $InstallationRoot $ServerUUID $SourceDirectory $ManifestPath $ManifestSHA256
    $configuration=[IO.File]::Open($plan.Node.Configuration,[IO.FileMode]::Open,[IO.FileAccess]::Read,[IO.FileShare]::Read)
    & $plan.Node.Executable --config $plan.Node.Configuration --check | Out-Null
    if($LASTEXITCODE -ne 0){throw 'Configuration validation failed.'}
    if($PSCmdlet.ShouldProcess($plan.Node.Destination,'Import SHA-256 verified new game code directory')){
        $lock=Open-GamePermissionLock $plan.Node.Root
        Import-GameFilePlan $plan
    }
}finally{if($lock){$lock.Dispose()};if($configuration){$configuration.Dispose()};if($plan){Close-GameFilePlan $plan}}
