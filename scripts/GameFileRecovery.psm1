$ErrorActionPreference='Stop'
Import-Module (Join-Path $PSScriptRoot 'GameFiles.psm1') -Force
$script:fileModule=(Get-Command Get-GameFilePlan).Module

function Assert-RecoveryPath([string]$Path) {& $script:fileModule {param($p) Assert-GameFilePath $p} $Path}
function Set-RecoveryACL([string]$Path) {& $script:fileModule {param($p) Set-GameFileACL $p 'D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)'} $Path}
function Get-RecoveryDescriptor([string]$Path,[string]$DaemonSID,[bool]$AllowDaemon=$true) {
    Assert-RecoveryPath $Path
    $acl=Get-Acl -LiteralPath $Path
    $operator=[Security.Principal.WindowsIdentity]::GetCurrent().User.Value
    $raw=[Security.AccessControl.RawSecurityDescriptor]::new($acl.GetSecurityDescriptorBinaryForm(),0)
    if($raw.Owner.Value -notin @('S-1-5-18','S-1-5-32-544',$operator) -or -not $raw.DiscretionaryAcl -or $raw.DiscretionaryAcl.Count -eq 0){throw 'Recovery requires administrator-owned, nonempty managed ACLs.'}
    foreach($ace in $raw.DiscretionaryAcl){if($ace.AceType -ne [Security.AccessControl.AceType]::AccessAllowed){throw 'Recovery refuses unsupported or deny ACL entries.'}}
    $admins=@{};$daemonReadable=$false
    foreach($rule in $acl.GetAccessRules($true,$true,[Security.Principal.SecurityIdentifier])){
        $sid=$rule.IdentityReference.Value;$mask=[uint32]$rule.FileSystemRights
        if($sid -in @('S-1-5-18','S-1-5-32-544',$operator)){if(-not($rule.PropagationFlags -band [Security.AccessControl.PropagationFlags]::InheritOnly)){$admins[$sid]=$mask};continue}
        if(-not $AllowDaemon -or $sid -cne $DaemonSID -or ($mask -band (-bnot [uint32]0x1200a9)) -ne 0){throw 'Recovery refuses game/foreign identities or writable daemon access.'}
        if($mask){$daemonReadable=$true}
    }
    foreach($sid in @('S-1-5-18','S-1-5-32-544')){if(-not $admins.ContainsKey($sid) -or ($admins[$sid] -band 0x1f01ff) -ne 0x1f01ff){throw 'Recovery requires SYSTEM and Administrators full control.'}}
    [pscustomobject]@{SDDL=$acl.GetSecurityDescriptorSddlForm([Security.AccessControl.AccessControlSections]'Owner,Group,Access');DaemonReadable=$daemonReadable}
}
function Get-RecoveryNode([string]$InstallationRoot) {
    $service=& $script:fileModule {param($root) Get-GamePermissionService $root} $InstallationRoot
    $config=Get-Content -LiteralPath $service.Configuration -Raw|ConvertFrom-Json;$games=@()
    foreach($entry in $config.profiles.PSObject.Properties){
        if(-not $entry.Value.run_as -or $entry.Value.run_as.credential_target -cne ('WingsWindows/'+$entry.Name.ToLowerInvariant())){throw 'Every existing profile requires a UUID-bound game identity.'}
        $account=& $script:fileModule {param($s) Get-GameFileAccount $s} $entry.Value.run_as.sid;$sid=$account.SID.Value
        if($sid -cne $entry.Value.run_as.sid -or $sid -eq $service.DaemonSID -or $sid -in $games){throw 'Existing game identities must be canonical and distinct.'};$games+=$sid
    }
    $bin=Join-Path $service.Root 'bin';$parent=Join-Path $bin 'games';$private=Join-Path $service.Root 'private';$quarantine=Join-Path $private 'code-quarantine'
    foreach($path in @($service.Root,$bin)){
        Assert-RecoveryPath $path
        & $script:fileModule {param($p,$g,$s) Assert-GameFileACL $p $g $s} $path $games $service.DaemonSID
    }
    Assert-RecoveryPath $parent
    if(Test-Path -LiteralPath $parent){if(-not(Test-Path -LiteralPath $parent -PathType Container)){throw 'Code parent must be a directory.'};& $script:fileModule {param($p,$g,$s) Assert-GameFileACL $p $g $s} $parent $games $service.DaemonSID}
    Get-RecoveryDescriptor $private $service.DaemonSID|Out-Null
    Assert-RecoveryPath $quarantine
    if(Test-Path -LiteralPath $quarantine){if(-not(Test-Path -LiteralPath $quarantine -PathType Container)){throw 'Quarantine parent must be a directory.'};Get-RecoveryDescriptor $quarantine $service.DaemonSID $false|Out-Null}
    [pscustomobject]@{Root=$service.Root;Configuration=$service.Configuration;Executable=$service.Executable;DaemonSID=$service.DaemonSID;Parent=$parent;Quarantine=$quarantine;Config=$config}
}
function Close-RecoveryPlan($Plan) {foreach($entry in $Plan.Entries){if($entry.Stream){$entry.Stream.Dispose()}}}
function Get-RecoveryPlan($Node,[string]$StageName) {
    if($StageName -cnotmatch '\A\.stage-[0-9a-f]{32}\z'){throw 'Select an exact .stage-<32 lowercase hex> directory name.'}
    $stage=Join-Path $Node.Parent $StageName;$destination=Join-Path $Node.Quarantine $StageName
    Assert-RecoveryPath $stage
    if(-not(Test-Path -LiteralPath $stage -PathType Container)){throw 'Selected staging directory does not exist.'}
    Assert-RecoveryPath $destination;if(Test-Path -LiteralPath $destination){throw 'Quarantine destination already exists; recovery never overwrites it.'}
    $configuredPaths=@($Node.Config.data_directory,$Node.Config.console_logs.directory,$Node.Config.tls.cert,$Node.Config.tls.key)
    foreach($entry in $Node.Config.profiles.PSObject.Properties){$configuredPaths+=@($entry.Value.executable)+@($entry.Value.requirements.required_files)+@($entry.Value.installer.executable)+@($entry.Value.installer.requirements.required_files)}
    foreach($path in $configuredPaths){
        if(-not $path){continue};$canonical=[IO.Path]::GetFullPath($path).TrimEnd('\')
        if($canonical -ieq $stage -or $canonical.StartsWith(($stage+'\'),[StringComparison]::OrdinalIgnoreCase)){throw 'A configured path references this staging tree; resolve that binding before quarantine.'}
    }
    $entries=[Collections.Generic.List[object]]::new();$queue=[Collections.Generic.Queue[object]]::new();$queue.Enqueue([pscustomobject]@{Path=$stage;Relative='';Depth=0})
    $records=[Collections.Generic.List[string]]::new();$files=0;$bytes=[long]0;$readable=0
    try{
        while($queue.Count){
            $item=$queue.Dequeue()
            if($entries.Count -ge 10000 -or $item.Depth -gt 16){throw 'Staging inspection exceeds entry/depth bounds.'}
            $descriptor=Get-RecoveryDescriptor $item.Path $Node.DaemonSID
            if($descriptor.DaemonReadable){$readable++}
            $directory=Test-Path -LiteralPath $item.Path -PathType Container
            $stream=$null;$length=[long]0;$hash=''
            if(-not $directory){
                $stream=& $script:fileModule {param($p) Open-GameFileSource $p} $item.Path
                $entry=[pscustomobject]@{Path=$item.Path;Stream=$stream;Directory=$false};$entries.Add($entry)
                $files++;$length=$stream.Length;$bytes+=$length
                if($files -gt 1000 -or $bytes -gt 8589934592){throw 'Staging inspection exceeds file/byte bounds.'}
                $hash=& $script:fileModule {param($s) Get-GameFileHash $s} $stream
            }else{
                $entries.Add([pscustomobject]@{Path=$item.Path;Stream=$null;Directory=$true})
                foreach($child in Get-ChildItem -LiteralPath $item.Path -Force){
                    if($entries.Count+$queue.Count -ge 10000){throw 'Staging inspection exceeds entry bounds.'}
                    $relative=$child.Name;if($item.Relative){$relative=$item.Relative+'\'+$child.Name}
                    $queue.Enqueue([pscustomobject]@{Path=$child.FullName;Relative=$relative;Depth=($item.Depth+1)})
                }
            }
            $path64=[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($item.Relative))
            $acl64=[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($descriptor.SDDL))
            $records.Add(([int]$directory).ToString()+':'+$path64+':'+$length+':'+$hash+':'+$acl64)
        }
        $records.Sort([StringComparer]::Ordinal)
        $sha=[Security.Cryptography.SHA256]::Create()
        try{$digest=([BitConverter]::ToString($sha.ComputeHash([Text.Encoding]::UTF8.GetBytes(($records -join "`n"))))).Replace('-','').ToLowerInvariant()}finally{$sha.Dispose()}
        [pscustomobject]@{Node=$Node;StageName=$StageName;Stage=$stage;Destination=$destination;Entries=$entries;Fingerprint=$digest;Files=$files;Bytes=$bytes;DaemonReadable=$readable}
    }catch{foreach($entry in $entries){if($entry.Stream){$entry.Stream.Dispose()}};throw}
}
function Get-RecoverySummary($Plan) {
    [pscustomobject]@{stage_name=$Plan.StageName;state='unverified_staging';fingerprint=$Plan.Fingerprint;files=$Plan.Files;bytes=$Plan.Bytes;entries=$Plan.Entries.Count;daemon_readable_entries=$Plan.DaemonReadable}
}
function Get-RecoveryStageNames($Node) {
    if(-not(Test-Path -LiteralPath $Node.Parent)){return}
    $names=[Collections.Generic.List[string]]::new()
    foreach($entry in Get-ChildItem -LiteralPath $Node.Parent -Force -Filter '.stage-*'){if($names.Count -ge 100){throw 'More than 100 staging entries; inspect the code parent manually.'};$names.Add($entry.Name)}
    $names.Sort([StringComparer]::Ordinal);foreach($name in $names){$name}
}
function Move-RecoveryDirectory([string]$Source,[string]$Destination) {[IO.Directory]::Move($Source,$Destination)}
function Move-RecoveryPlan($Plan,[string]$ExpectedFingerprint) {
    if($ExpectedFingerprint -notmatch '\A[0-9a-fA-F]{64}\z' -or $Plan.Fingerprint -cne $ExpectedFingerprint.ToLowerInvariant()){throw 'Staging fingerprint changed; inspect it again before quarantine.'}
    $node=Get-RecoveryNode $Plan.Node.Root
    if($node.DaemonSID -cne $Plan.Node.DaemonSID){throw 'Service identity changed after inspection.'}
    $fresh=$null
    try{
        $fresh=Get-RecoveryPlan $node $Plan.StageName
        if($fresh.Fingerprint -cne $ExpectedFingerprint.ToLowerInvariant()){throw 'Staging fingerprint changed; inspect it again before quarantine.'}
        if(-not(Test-Path -LiteralPath $node.Quarantine)){[IO.Directory]::CreateDirectory($node.Quarantine)|Out-Null;Set-RecoveryACL $node.Quarantine}
        # Seal the selected stage before moving. On failure, retain its bytes
        # and any more restrictive ACLs for explicit inspection; never delete.
        foreach($entry in $fresh.Entries){Set-RecoveryACL $entry.Path}
        Close-RecoveryPlan $fresh;Close-RecoveryPlan $Plan
        Assert-RecoveryPath $fresh.Stage;Assert-RecoveryPath $fresh.Destination
        Move-RecoveryDirectory $fresh.Stage $fresh.Destination
        [pscustomobject]@{stage_name=$fresh.StageName;state='quarantined';inspected_fingerprint=$fresh.Fingerprint;files=$fresh.Files;bytes=$fresh.Bytes;destination=$fresh.Destination}
    }finally{if($fresh){Close-RecoveryPlan $fresh}}
}
Export-ModuleMember -Function Get-RecoveryNode,Get-RecoveryPlan,Get-RecoverySummary,Get-RecoveryStageNames,Close-RecoveryPlan,Move-RecoveryPlan,Open-GamePermissionLock
