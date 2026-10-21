#Requires -RunAsAdministrator
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$Executable,
    [Parameter(Mandatory = $true)][string]$Configuration,
    [Parameter(Mandatory = $true)][PSCredential]$Credential,
    [string]$InstallationRoot,
    [switch]$CreateLocalAccount,
    [switch]$ConfigurePermissions,
    [switch]$Start
)
$ErrorActionPreference = 'Stop'
$executablePath = (Resolve-Path -LiteralPath $Executable).Path
$configurationPath = (Resolve-Path -LiteralPath $Configuration).Path
if (Get-Service -Name 'WingsWindowsNative' -ErrorAction SilentlyContinue) {
    throw 'WingsWindowsNative already exists. Stop it and update its executable/configuration explicitly.'
}
Import-Module (Join-Path $PSScriptRoot 'NodeSetup.psm1') -Force
if ($CreateLocalAccount -and -not $ConfigurePermissions) { throw 'CreateLocalAccount requires ConfigurePermissions.' }
if ($ConfigurePermissions -and [string]::IsNullOrWhiteSpace($InstallationRoot)) { throw 'ConfigurePermissions requires InstallationRoot.' }
if ($Credential.UserName -notmatch '^\.\\([^\\/@\x00\r\n]+)$') { throw 'Use .\local-account for a dedicated service identity.' }
$accountName = $Matches[1]
if ($Credential.Password.Length -eq 0) { throw 'A nonempty service password is required.' }
& $executablePath --config $configurationPath --check
if ($LASTEXITCODE -ne 0) { throw 'Configuration validation failed.' }
$created = $false; $registered = $false; $setupState = $null
try {
    if ($CreateLocalAccount) {
        if (Get-LocalUser -Name $accountName -ErrorAction SilentlyContinue) { throw 'CreateLocalAccount refuses an existing account.' }
        New-LocalUser -Name $accountName -Password $Credential.Password -Description 'Wings Windows Native service identity' | Out-Null
        $created = $true
    }
    $account = Get-NodeLocalAccount $Credential.UserName
    if ($ConfigurePermissions) {
        $plan = Get-NodePermissionPlan -InstallationRoot $InstallationRoot -Executable $executablePath -Configuration $configurationPath -DaemonSID $account.SID.Value
        $setupState = Enable-NodeInstallation $plan
    }
    $binaryPath = '"{0}" --config "{1}"' -f $executablePath, $configurationPath
    New-Service -Name 'WingsWindowsNative' -DisplayName 'Wings Windows Native (Experimental)' `
        -Description 'Native Windows game-server runtime compatible with a subset of the Wings API.' `
        -BinaryPathName $binaryPath -StartupType Manual -Credential $Credential | Out-Null
    $registered = $true
    if ($Start) { Start-Service -Name 'WingsWindowsNative' }
    if ($ConfigurePermissions) {
        Write-Output 'Service registered with dedicated-account rights and initial node ACLs. Provision game credentials as this service identity before launching games. WFP permissions are not configured.'
    } else {
        Write-Output 'Service registered. Configure account ACLs and Log on as a service before starting it.'
    }
} catch {
    $original = $_; $cleanup = @()
    $serviceRemoved = -not $registered
    if ($registered) {
        try {
            $service = Get-Service -Name 'WingsWindowsNative'
            try {if ($service.Status -ne 'Stopped') {Stop-Service -Name $service.Name; $service.WaitForStatus('Stopped',[TimeSpan]::FromSeconds(15))}} finally {$service.Dispose()}
            & "$env:SystemRoot\System32\sc.exe" delete WingsWindowsNative | Out-Null
            if ($LASTEXITCODE -ne 0) { throw 'Failed to remove the new service registration.' }
            $serviceRemoved = $true
        } catch { $cleanup += $_.Exception.Message }
    }
    if (-not $serviceRemoved) { throw ('Installation failed: '+$original.Exception.Message+'; service cleanup failed, so account/rights/ACLs were retained for inspection: '+($cleanup -join '; ')) }
    if ($setupState) {try {Undo-NodeInstallation $setupState} catch {$cleanup += $_.Exception.Message}}
    if ($created) {try {Remove-LocalUser -Name $accountName} catch {$cleanup += $_.Exception.Message}}
    if ($cleanup.Count -gt 0) {throw ('Installation failed: '+$original.Exception.Message+'; rollback needs inspection: '+($cleanup -join '; '))}
    throw $original
}
