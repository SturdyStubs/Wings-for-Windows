$ErrorActionPreference='Stop'
Import-Module (Join-Path $PSScriptRoot 'NodeSetup.psm1') -Force

function Assert-GamePermissionPath([string]$Path) {
    $module=(Get-Command Initialize-NodeSetupNative).Module
    & $module {param($p) Assert-NodeSetupPath $p} $Path
}
function Get-GamePermissionAccount([string]$SID) {
    $user=Get-LocalUser -SID ([Security.Principal.SecurityIdentifier]::new($SID)) -ErrorAction Stop
    Get-NodeLocalAccount ('.\'+$user.Name)
}
function Get-GamePermissionRights([string]$SID) { [WingsWindows.NodeSetup]::Rights($SID) }
function Set-GamePermissionBatchRight([string]$SID,[bool]$Remove) { [WingsWindows.NodeSetup]::Right($SID,'SeBatchLogonRight',$Remove) }
function Set-GamePermissionACL([string]$Path,[string]$SDDL) {
    Assert-GamePermissionPath $Path
    if(Test-Path -LiteralPath $Path -PathType Container){$acl=[Security.AccessControl.DirectorySecurity]::new()}else{$acl=[Security.AccessControl.FileSecurity]::new()}
    $acl.SetSecurityDescriptorSddlForm($SDDL,[Security.AccessControl.AccessControlSections]::Access)
    if($acl -is [Security.AccessControl.DirectorySecurity]){[IO.Directory]::SetAccessControl($Path,$acl)}else{[IO.File]::SetAccessControl($Path,$acl)}
}
function Open-GamePermissionLock([string]$Root) {
    $path=Join-Path $Root 'private\credential-provisioning.lock'
    Assert-GamePermissionPath $path
    $lock=[IO.File]::Open($path,[IO.FileMode]::OpenOrCreate,[IO.FileAccess]::ReadWrite,[IO.FileShare]::None)
    if($lock.Length -ne 0){$lock.Dispose();throw 'Invalid provisioning lock file.'}
    return $lock
}
function Get-GamePermissionService([string]$InstallationRoot) {
    Initialize-NodeSetupNative
    Assert-GamePermissionPath $InstallationRoot
    $root=[IO.Path]::GetFullPath($InstallationRoot).TrimEnd('\')
    $exe=Join-Path $root 'bin\wings-windows.exe';$config=Join-Path $root 'private\config.json'
    $service=Get-CimInstance Win32_Service -Filter "Name='WingsWindowsNative'" -ErrorAction Stop
    if(-not $service -or $service.State -ne 'Stopped'){throw 'Stop WingsWindowsNative before preparing a game identity.'}
    if($service.PathName -cne ('"'+$exe+'" --config "'+$config+'"')){throw 'Service registration must match the managed layout exactly.'}
    $name=[string]$service.StartName
    if($name.StartsWith('.\')){$name=$env:COMPUTERNAME+$name.Substring(1)}
    $sid=([Security.Principal.NTAccount]::new($name)).Translate([Security.Principal.SecurityIdentifier]).Value
    $account=Get-GamePermissionAccount $sid
    [pscustomobject]@{Root=$root;Executable=$exe;Configuration=$config;DaemonSID=$account.SID.Value}
}

# Accept only the existing managed ACL shape. This is a bounded snapshot for a
# stopped trusted node, not effective-token or adversarial filesystem analysis.
function Assert-GamePermissionACL($ACL,[string[]]$Games,[string]$DaemonSID,[string]$OperatorSID,[string]$Kind) {
    $raw=[Security.AccessControl.RawSecurityDescriptor]::new($ACL.GetSecurityDescriptorBinaryForm(),0)
    if(-not $raw.DiscretionaryAcl -or $raw.DiscretionaryAcl.Count -eq 0){throw 'Managed paths require a nonempty DACL.'}
    if($raw.Owner.Value -notin @('S-1-5-18','S-1-5-32-544',$DaemonSID,$OperatorSID)){throw 'Unsupported owner on managed path.'}
    foreach($ace in $raw.DiscretionaryAcl){if($ace.AceType -ne [Security.AccessControl.AceType]::AccessAllowed){throw 'Unsupported ACL entry on managed path.'}}
    $admins=@{}
    foreach($rule in $ACL.GetAccessRules($true,$true,[Security.Principal.SecurityIdentifier])) {
        $sid=$rule.IdentityReference.Value;$mask=[uint32]$rule.FileSystemRights
        if($sid -in @('S-1-5-18','S-1-5-32-544',$OperatorSID)){$admins[$sid]=$mask;continue}
        $allowed=[uint32]0x1200a9 # read/execute/synchronize
        if($sid -eq $DaemonSID){if($Kind -in @('logs','data')){$allowed=0x1301bf}}
        elseif($sid -notin $Games -or $Kind -in @('private','logs')){throw 'Unexpected identity has managed-path access.'}
        if(($mask -band (-bnot $allowed)) -ne 0){throw 'Managed access exceeds the supported permission mask.'}
    }
    foreach($sid in @('S-1-5-18','S-1-5-32-544')){if(-not $admins.ContainsKey($sid) -or ($admins[$sid] -band 0x1f01ff) -ne 0x1f01ff){throw 'Managed paths must retain SYSTEM and Administrators full control.'}}
}

function Get-GamePermissionPlan([string]$InstallationRoot,[string]$ServerUUID,[string]$DaemonSID) {
    Initialize-NodeSetupNative
    if($ServerUUID -notmatch '\A[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}\z'){throw 'A full server UUID is required.'}
    if($DaemonSID -notmatch '^S-1-5-21-(\d+-){3}\d+$'){throw 'A canonical dedicated daemon SID is required.'}
    Assert-GamePermissionPath $InstallationRoot
    $root=[IO.Path]::GetFullPath($InstallationRoot).TrimEnd('\');if($root.Length -le 3){throw 'Use a dedicated installation root.'}
    $bin=Join-Path $root 'bin';$private=Join-Path $root 'private';$data=Join-Path $root 'data';$logs=Join-Path $root 'logs'
    foreach($path in @($bin,$private,$data,$logs)){Assert-GamePermissionPath $path;if(-not(Test-Path -LiteralPath $path -PathType Container)){throw 'Managed bin/private/data/logs directories must exist.'}}
    foreach($item in Get-ChildItem -LiteralPath $root -Force){if(-not $item.PSIsContainer -or $item.Name -notin @('bin','private','data','logs')){throw 'Unexpected installation root entry.'}}
    $configuration=Join-Path $private 'config.json';Assert-GamePermissionPath $configuration
    $config=Get-Content -LiteralPath $configuration -Raw|ConvertFrom-Json
    if($config.data_directory.TrimEnd('\') -ne $data){throw 'Managed data directory mismatch.'}
    if($config.console_logs -and $config.console_logs.directory -ne (Join-Path $logs 'console')){throw 'Managed console log directory mismatch.'}
    $id=$ServerUUID.ToLowerInvariant();$entry=$config.profiles.PSObject.Properties[$id]
    if(-not $entry -or -not $entry.Value.run_as){throw 'Approve this UUID with a distinct run_as profile first.'}
    $games=@();$selected=$entry.Value
    foreach($profileEntry in $config.profiles.PSObject.Properties){
        $profile=$profileEntry.Value
        if(-not $profile.run_as -or $profile.run_as.credential_target -cne ('WingsWindows/'+$profileEntry.Name.ToLowerInvariant())){throw 'All profiles require UUID-bound game identities.'}
        $account=Get-GamePermissionAccount ([string]$profile.run_as.sid);$sid=$account.SID.Value
        if($sid -cne $profile.run_as.sid -or $sid -eq $DaemonSID -or $sid -in $games){throw 'Game SIDs must be canonical, distinct and different from the daemon.'}
        $games+=$sid
    }
    $gameSID=[string]$selected.run_as.sid
    if($selected.network_policy.mode -and $selected.network_policy.mode -ne 'unmanaged'){throw 'This helper does not provision WFP permissions.'}
    foreach($path in @($selected.executable)+@($selected.requirements.required_files)+@($selected.installer.executable)+@($selected.installer.requirements.required_files)){
        if(-not $path){continue};Assert-GamePermissionPath $path
        if(-not $path.StartsWith(($bin+'\'),[StringComparison]::OrdinalIgnoreCase) -or -not(Test-Path -LiteralPath $path -PathType Leaf)){throw 'Approved code and dependencies must already exist under bin.'}
    }
    $target=Join-Path $data $id;Assert-GamePermissionPath $target
    if(Test-Path -LiteralPath $target){
        if(-not(Test-Path -LiteralPath $target -PathType Container)){throw 'Game data root must be a directory.'}
        foreach($child in Get-ChildItem -LiteralPath $target -Force){
            Assert-GamePermissionPath $child.FullName
            if(-not $child.PSIsContainer -or $child.Name -ne '.tmp' -or @(Get-ChildItem -LiteralPath $child.FullName -Force).Count){throw 'This helper prepares only empty new server data.'}
        }
    }
    $operator=[Security.Principal.WindowsIdentity]::GetCurrent().User.Value
    $code=[Collections.Generic.List[string]]::new();$code.Add($bin)
    $paths=[Collections.Generic.List[object]]::new()
    foreach($item in @(@{Path=$root;Kind='code'},@{Path=$data;Kind='data'},@{Path=$bin;Kind='code'},@{Path=$private;Kind='private'},@{Path=$logs;Kind='logs'})){$paths.Add([pscustomobject]$item)}
    $queue=[Collections.Generic.Queue[object]]::new()
    foreach($item in @(@{Path=$bin;Kind='code'},@{Path=$private;Kind='private'},@{Path=$logs;Kind='logs'})){$queue.Enqueue([pscustomobject]$item)}
    while($queue.Count){
        $parent=$queue.Dequeue()
        foreach($child in Get-ChildItem -LiteralPath $parent.Path -Force){
            Assert-GamePermissionPath $child.FullName
            if($paths.Count -ge 10000){throw 'Permission planning exceeds 10000 entries.'}
            $item=[pscustomobject]@{Path=$child.FullName;Kind=$parent.Kind};$paths.Add($item)
            if($parent.Kind -eq 'code'){$code.Add($child.FullName)}
            if($child.PSIsContainer){$queue.Enqueue($item)}
        }
    }
    $rules=[Collections.Generic.List[object]]::new()
    foreach($item in $paths){
        $acl=Get-Acl -LiteralPath $item.Path
        Assert-GamePermissionACL $acl $games $DaemonSID $operator $item.Kind
        if($item.Path -eq $root -or $item.Path -eq $data -or $item.Path -in $code){
            $before=$acl.GetSecurityDescriptorSddlForm([Security.AccessControl.AccessControlSections]::Access)
            # Non-inheriting grants avoid propagating into existing server data.
            $acl.AddAccessRule([Security.AccessControl.FileSystemAccessRule]::new([Security.Principal.SecurityIdentifier]::new($gameSID),[Security.AccessControl.FileSystemRights]0x1200a9,[Security.AccessControl.AccessControlType]::Allow))
            $rules.Add([pscustomobject]@{Path=$item.Path;SDDL=$acl.GetSecurityDescriptorSddlForm([Security.AccessControl.AccessControlSections]::Access);Before=$before})
        }
    }
    $rights=@(Get-GamePermissionRights $gameSID)
    if('SeDenyBatchLogonRight' -in $rights){throw 'Game account has an explicit deny-batch-logon right.'}
    $sddl='D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;0x1301bf;;;'+$DaemonSID+')(A;OICI;0x1301bf;;;'+$gameSID+')'
    foreach($path in @($target,(Join-Path $target '.tmp'))){$rules.Add([pscustomobject]@{Path=$path;SDDL=$sddl;Before=$null})}
    [pscustomobject]@{Root=$root;ServerUUID=$id;GameSID=$gameSID;DaemonSID=$DaemonSID;Rules=$rules;AddBatch=('SeBatchLogonRight' -notin $rights)}
}

function Enable-GamePermissionPlan($Plan) {
    $state=[pscustomobject]@{ACLs=[Collections.Generic.List[object]]::new();Rights=[Collections.Generic.List[object]]::new();Directories=[Collections.Generic.List[string]]::new()}
    try{
        foreach($rule in $Plan.Rules){
            Assert-GamePermissionPath $rule.Path
            if($rule.Before -and (Get-Acl -LiteralPath $rule.Path).GetSecurityDescriptorSddlForm([Security.AccessControl.AccessControlSections]::Access) -cne $rule.Before){throw 'Permissions changed after planning.'}
        }
        $rights=@(Get-GamePermissionRights $Plan.GameSID)
        if('SeDenyBatchLogonRight' -in $rights){throw 'Game account has an explicit deny-batch-logon right.'}
        if('SeBatchLogonRight' -notin $rights){$state.Rights.Add([pscustomobject]@{SID=$Plan.GameSID;Name='SeBatchLogonRight'});Set-GamePermissionBatchRight $Plan.GameSID $false}
        foreach($rule in $Plan.Rules){
            Assert-GamePermissionPath $rule.Path
            if(-not(Test-Path -LiteralPath $rule.Path)){[IO.Directory]::CreateDirectory($rule.Path)|Out-Null;$state.Directories.Add($rule.Path)}
            $before=(Get-Acl -LiteralPath $rule.Path).GetSecurityDescriptorSddlForm([Security.AccessControl.AccessControlSections]::Access)
            $state.ACLs.Add([pscustomobject]@{Path=$rule.Path;SDDL=$before})
            Set-GamePermissionACL $rule.Path $rule.SDDL
        }
    }catch{
        $failure=$_;$errors=@()
        for($i=$state.ACLs.Count-1;$i -ge 0;$i--){try{Set-GamePermissionACL $state.ACLs[$i].Path $state.ACLs[$i].SDDL}catch{$errors+=$_.Exception.Message}}
        if($state.Rights.Count){try{Set-GamePermissionBatchRight $Plan.GameSID $true}catch{$errors+=$_.Exception.Message}}
        if($errors.Count){throw ('Permission preparation failed; rollback needs inspection: '+($errors -join '; '))}
        throw $failure
    }
    # New empty directories are retained on failure; no data is deleted.
    [pscustomobject]@{server_uuid=$Plan.ServerUUID;game_sid=$Plan.GameSID;prepared=$true;paths=$Plan.Rules.Count;batch_right_added=($state.Rights.Count -gt 0)}
}

Export-ModuleMember -Function Get-GamePermissionService,Get-GamePermissionPlan,Enable-GamePermissionPlan,Open-GamePermissionLock
