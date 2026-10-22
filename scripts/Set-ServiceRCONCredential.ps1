#Requires -RunAsAdministrator
[CmdletBinding(DefaultParameterSetName='Store')]
param(
 [Parameter(Mandatory=$true)][string]$InstallationRoot,
 [Parameter(Mandatory=$true)][string]$ServerUUID,
 [Parameter(Mandatory=$true)][PSCredential]$ServiceCredential,
 [Parameter(Mandatory=$true,ParameterSetName='Store')][Security.SecureString]$Password,
 [Parameter(ParameterSetName='Store')][switch]$Replace,
 [Parameter(Mandatory=$true,ParameterSetName='Inspect')][switch]$Inspect,
 [Parameter(Mandatory=$true,ParameterSetName='Remove')][switch]$Remove
)
$ErrorActionPreference='Stop'
Import-Module (Join-Path $PSScriptRoot 'NodeSetup.psm1') -Force
Import-Module (Join-Path $PSScriptRoot 'CredentialBridge.psm1') -Force
$secret=$null
if($PSCmdlet.ParameterSetName -eq 'Store'){$secret=[PSCredential]::new('.\WingsRCONSecret-v1',$Password)}
$plan=Get-ServiceCredentialPlan -InstallationRoot $InstallationRoot -ServerUUID $ServerUUID -ServiceCredential $ServiceCredential -GameCredential $secret -Operation $PSCmdlet.ParameterSetName -Purpose RCON
Invoke-ServiceCredentialTask -Plan $plan -ServiceCredential $ServiceCredential -GameCredential $secret -Replace $Replace.IsPresent
