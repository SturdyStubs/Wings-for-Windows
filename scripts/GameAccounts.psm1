$ErrorActionPreference='Stop'
Import-Module (Join-Path $PSScriptRoot 'GamePermissions.psm1') -Force

function Get-GameAccountByName([string]$Name) { Get-LocalUser -Name $Name -ErrorAction SilentlyContinue }
function New-GameAccountUser([string]$Name,[Security.SecureString]$Password,[string]$UUID) {
    # New-LocalUser limits Description to 48 characters; retain the full UUID.
    New-LocalUser -Name $Name -Password $Password -Description ('Wings game '+$UUID) -PasswordNeverExpires -UserMayNotChangePassword -ErrorAction Stop
}
function Remove-GameAccountUser([string]$SID) { Remove-LocalUser -SID ([Security.Principal.SecurityIdentifier]::new($SID)) -ErrorAction Stop }
function Assert-GameAccountUser([string]$Name) {
    $module=(Get-Command Get-GamePermissionService).Module
    & $module {param($n) Get-NodeLocalAccount ('.\'+$n)} $Name
}
function Get-ServiceGameAccountPlan([string]$InstallationRoot,[string]$ServerUUID,[string]$AccountName) {
    if($ServerUUID -notmatch '\A[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}\z'){throw 'A full server UUID is required.'}
    if($AccountName -notmatch '\A[A-Za-z][A-Za-z0-9_-]{0,19}\z'){throw 'Use a local account name of 1-20 ASCII letters, digits, underscores or hyphens, starting with a letter.'}
    $service=Get-GamePermissionService $InstallationRoot
    $id=$ServerUUID.ToLowerInvariant()
    $config=Get-Content -LiteralPath $service.Configuration -Raw|ConvertFrom-Json
    if($config.data_directory.TrimEnd('\') -ne (Join-Path $service.Root 'data')){throw 'Managed data directory mismatch.'}
    foreach($entry in $config.profiles.PSObject.Properties){if($entry.Name -ieq $id){throw 'This UUID already has a local profile; account creation will not replace it.'}}
    if(Test-Path -LiteralPath (Join-Path $service.Root ('data\'+$id))){throw 'This UUID already has a data path; inspect it before account creation.'}
    if(Get-GameAccountByName $AccountName){throw 'The local account already exists; it will not be adopted or reset.'}
    [pscustomobject]@{Root=$service.Root;Configuration=$service.Configuration;Executable=$service.Executable;DaemonSID=$service.DaemonSID;ServerUUID=$id;AccountName=$AccountName}
}
function New-ServiceGameAccount($Plan,[Security.SecureString]$Password) {
    if(-not $Password -or $Password.Length -eq 0 -or $Password.Length -gt 1280){throw 'Provide a nonempty SecureString password within the credential storage limit.'}
    # Revalidate after the caller acquires the shared maintenance lock. No
    # password conversion, JSON serialization or native argument forwarding.
    $current=Get-ServiceGameAccountPlan $Plan.Root $Plan.ServerUUID $Plan.AccountName
    if($current.DaemonSID -ne $Plan.DaemonSID -or $current.Configuration -cne $Plan.Configuration){throw 'Service identity or configuration binding changed after planning.'}
    $created=$null
    try{
        $created=New-GameAccountUser $Plan.AccountName $Password $Plan.ServerUUID
        if(-not $created -or -not $created.SID -or $created.Name -ine $Plan.AccountName){throw 'Account creation did not return the expected identity; inspect local accounts.'}
        $checked=Assert-GameAccountUser $Plan.AccountName
        if($checked.SID.Value -cne $created.SID.Value -or $checked.SID.Value -eq $Plan.DaemonSID){throw 'Created game identity did not pass dedicated-account validation.'}
        [pscustomobject]@{server_uuid=$Plan.ServerUUID;account_name=$created.Name;username=('.\'+$created.Name);sid=$checked.SID.Value;run_as=[pscustomobject]@{sid=$checked.SID.Value;credential_target=('WingsWindows/'+$Plan.ServerUUID)}}
    }catch{
        $failure=$_
        # Delete by the returned SID only. An existing account/name collision
        # is never adopted; an uncertain create outcome requires inspection.
        if($created -and $created.SID){
            try{Remove-GameAccountUser $created.SID.Value}catch{throw 'Game account validation failed and rollback could not remove the created SID. Inspect local accounts; no permissions or credentials were provisioned.'}
        }
        throw $failure
    }
}
Export-ModuleMember -Function Get-ServiceGameAccountPlan,New-ServiceGameAccount,Open-GamePermissionLock
