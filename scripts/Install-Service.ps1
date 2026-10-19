#Requires -RunAsAdministrator
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$Executable,
    [Parameter(Mandatory = $true)][string]$Configuration,
    [Parameter(Mandatory = $true)][PSCredential]$Credential,
    [switch]$Start
)
$ErrorActionPreference = 'Stop'
$executablePath = (Resolve-Path -LiteralPath $Executable).Path
$configurationPath = (Resolve-Path -LiteralPath $Configuration).Path
if (Get-Service -Name 'WingsWindowsNative' -ErrorAction SilentlyContinue) {
    throw 'WingsWindowsNative already exists. Stop it and update its executable/configuration explicitly.'
}
if ($Credential.UserName -match '^(LocalSystem|NT AUTHORITY\\SYSTEM|\.\\SYSTEM)$') {
    throw 'Use a dedicated unprivileged service account.'
}
& $executablePath --config $configurationPath --check
if ($LASTEXITCODE -ne 0) { throw 'Configuration validation failed.' }
$binaryPath = '"{0}" --config "{1}"' -f $executablePath, $configurationPath
New-Service -Name 'WingsWindowsNative' -DisplayName 'Wings Windows Native (Experimental)' `
    -Description 'Native Windows game-server runtime compatible with a subset of the Wings API.' `
    -BinaryPathName $binaryPath -StartupType Manual -Credential $Credential
if ($Start) { Start-Service -Name 'WingsWindowsNative' }
Write-Output 'Service registered. Configure account ACLs and Log on as a service before starting it.'
