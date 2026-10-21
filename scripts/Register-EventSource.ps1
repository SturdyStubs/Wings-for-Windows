#Requires -RunAsAdministrator
[CmdletBinding(SupportsShouldProcess=$true)]
param([switch]$Remove)
$ErrorActionPreference='Stop'
$source='WingsWindowsNative'
$path='HKLM:\SYSTEM\CurrentControlSet\Services\EventLog\Application\'+$source
$messageFile=Join-Path $env:SystemRoot 'System32\EventCreate.exe'
if(Test-Path -LiteralPath $path){
    $existing=Get-ItemProperty -LiteralPath $path
    if($existing.WingsWindowsManaged -ne 1 -or $existing.EventMessageFile -ne $messageFile -or $existing.TypesSupported -ne 7){throw 'Existing Event Log source is not this managed registration; refusing to change it.'}
    if(-not $Remove){Write-Output 'Event source already registered.';return}
    if(Get-Service -Name $source -ErrorAction SilentlyContinue){throw 'Remove the service registration before removing its event source.'}
    if($PSCmdlet.ShouldProcess($source,'Remove managed Application Event Log source')){
        Remove-Item -LiteralPath $path
        Write-Output 'Event source removed. Historical Application events are retained.'
    }
    return
}
if($Remove){Write-Output 'Event source already absent.';return}
if([Diagnostics.EventLog]::SourceExists($source)){throw 'Source exists in another Event Log; refusing to change it.'}
if(-not $PSCmdlet.ShouldProcess($source,'Register Application Event Log source')){return}
$created=$false
try {
    # CreateSubKey alone would silently open an existing key. Use New-Item without Force.
    New-Item -Path $path -ErrorAction Stop | Out-Null
    $created=$true
    New-ItemProperty -LiteralPath $path -Name EventMessageFile -Value $messageFile -PropertyType String | Out-Null
    New-ItemProperty -LiteralPath $path -Name TypesSupported -Value 7 -PropertyType DWord | Out-Null
    New-ItemProperty -LiteralPath $path -Name WingsWindowsManaged -Value 1 -PropertyType DWord | Out-Null
    Write-Output 'Event source registered. Start or restart WingsWindowsNative to enable lifecycle events.'
} catch {
    if($created){Remove-Item -LiteralPath $path -ErrorAction Stop}
    throw
}
