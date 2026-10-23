#Requires -RunAsAdministrator
[CmdletBinding(SupportsShouldProcess=$true,DefaultParameterSetName='Inspect')]
param(
    [Parameter(Mandatory=$true)][string]$InstallationRoot,
    [Parameter(ParameterSetName='Inspect')][Parameter(Mandatory=$true,ParameterSetName='Quarantine')][string]$StageName,
    [Parameter(Mandatory=$true,ParameterSetName='Quarantine')][switch]$Quarantine,
    [Parameter(Mandatory=$true,ParameterSetName='Quarantine')][string]$ExpectedFingerprint
)
$ErrorActionPreference='Stop'
if($PSVersionTable.PSEdition -ne 'Desktop' -or $PSVersionTable.PSVersion.Major -ne 5){throw 'Run this command with elevated Windows PowerShell 5.1.'}
Import-Module (Join-Path $PSScriptRoot 'GameFileRecovery.psm1') -Force
$node=Get-RecoveryNode $InstallationRoot
$configuration=[IO.File]::Open($node.Configuration,[IO.FileMode]::Open,[IO.FileAccess]::Read,[IO.FileShare]::Read)
$lock=$null
try{
    & $node.Executable --config $node.Configuration --check|Out-Null
    if($LASTEXITCODE -ne 0){throw 'Configuration validation failed.'}
    $names=@();if($StageName){$names=@($StageName)}else{$names=@(Get-RecoveryStageNames $node)}
    foreach($name in $names){
        $plan=$null
        try{
            $plan=Get-RecoveryPlan $node $name
            if($Quarantine){
                if($ExpectedFingerprint -notmatch '\A[0-9a-fA-F]{64}\z' -or $plan.Fingerprint -cne $ExpectedFingerprint.ToLowerInvariant()){throw 'Staging fingerprint changed; inspect it again before quarantine.'}
                if($PSCmdlet.ShouldProcess($plan.Stage,'Retain staged bytes in administrator-only quarantine')){
                    $lock=Open-GamePermissionLock $node.Root
                    Move-RecoveryPlan $plan $ExpectedFingerprint
                }
            }else{Get-RecoverySummary $plan}
        }finally{if($plan){Close-RecoveryPlan $plan}}
    }
}finally{if($lock){$lock.Dispose()};$configuration.Dispose()}
