#Requires -RunAsAdministrator
[CmdletBinding(DefaultParameterSetName='Store')]
param(
    [Parameter(Mandatory=$true)][string]$InstallationRoot,
    [Parameter(Mandatory=$true)][string]$ServerUUID,
    [Parameter(Mandatory=$true)][PSCredential]$ServiceCredential,
    [Parameter(Mandatory=$true,ParameterSetName='Store')][PSCredential]$GameCredential,
    [Parameter(ParameterSetName='Store')][switch]$Replace,
    [Parameter(Mandatory=$true,ParameterSetName='Inspect')][switch]$Inspect,
    [Parameter(Mandatory=$true,ParameterSetName='Remove')][switch]$Remove
)
$ErrorActionPreference='Stop'
Import-Module (Join-Path $PSScriptRoot 'NodeSetup.psm1') -Force
Import-Module (Join-Path $PSScriptRoot 'CredentialBridge.psm1') -Force
$plan=Get-ServiceCredentialPlan -InstallationRoot $InstallationRoot -ServerUUID $ServerUUID -ServiceCredential $ServiceCredential -GameCredential $GameCredential -Operation $PSCmdlet.ParameterSetName
Invoke-ServiceCredentialTask -Plan $plan -ServiceCredential $ServiceCredential -GameCredential $GameCredential -Replace $Replace.IsPresent
