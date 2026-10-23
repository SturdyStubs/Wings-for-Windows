$ErrorActionPreference='Stop'
Import-Module (Join-Path $PSScriptRoot 'GamePermissions.psm1') -Force
$script:permissionModule=(Get-Command Get-GamePermissionService).Module

function Assert-GameFilePath([string]$Path) {
    $module=$script:permissionModule
    & $module {param($p) Initialize-NodeSetupNative; Assert-GamePermissionPath $p} $Path
}
function Assert-GameFileACL([string]$Path,[string[]]$Games,[string]$DaemonSID) {
    $module=$script:permissionModule
    & $module {param($p,$games,$daemon) Assert-GamePermissionACL (Get-Acl -LiteralPath $p) $games $daemon ([Security.Principal.WindowsIdentity]::GetCurrent().User.Value) 'code'} $Path $Games $DaemonSID
}
function Set-GameFileACL([string]$Path,[string]$SDDL) {
    $module=$script:permissionModule
    & $module {param($p,$s) Set-GamePermissionACL $p $s} $Path $SDDL
}
function Get-GameFileAccount([string]$SID) {
    $module=$script:permissionModule
    & $module {param($s) Get-GamePermissionAccount $s} $SID
}
function Get-GameFileHash([IO.Stream]$Stream) {
    $sha=[Security.Cryptography.SHA256]::Create()
    try{$Stream.Position=0;return ([BitConverter]::ToString($sha.ComputeHash($Stream))).Replace('-','').ToLowerInvariant()}finally{$sha.Dispose();$Stream.Position=0}
}
function Open-GameFileSource([string]$Path) {
    Assert-GameFilePath $Path
    if(-not(Test-Path -LiteralPath $Path -PathType Leaf)){throw 'Every approved source must be an existing regular file.'}
    [IO.File]::Open($Path,[IO.FileMode]::Open,[IO.FileAccess]::Read,[IO.FileShare]::Read)
}
function Close-GameFilePlan($Plan) {foreach($file in $Plan.Files){$file.Stream.Dispose()}}
function Get-GameFileNode([string]$InstallationRoot,[string]$ServerUUID) {
    if($ServerUUID -notmatch '\A[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}\z'){throw 'A full server UUID is required.'}
    $service=Get-GamePermissionService $InstallationRoot
    $id=$ServerUUID.ToLowerInvariant();$config=Get-Content -LiteralPath $service.Configuration -Raw|ConvertFrom-Json
    if($config.profiles.PSObject.Properties[$id]){throw 'Import code before approving this UUID profile; existing profiles cannot be updated by this command.'}
    $games=@()
    foreach($entry in $config.profiles.PSObject.Properties){
        if(-not $entry.Value.run_as -or $entry.Value.run_as.credential_target -cne ('WingsWindows/'+$entry.Name.ToLowerInvariant())){throw 'Every existing profile requires a UUID-bound game identity.'}
        $account=Get-GameFileAccount $entry.Value.run_as.sid;$sid=$account.SID.Value
        if($sid -cne $entry.Value.run_as.sid -or $sid -eq $service.DaemonSID -or $sid -in $games){throw 'Existing game identities must be canonical and distinct.'};$games+=$sid
    }
    $bin=Join-Path $service.Root 'bin';$parent=Join-Path $bin 'games';$target=Join-Path $parent $id
    foreach($path in @($service.Root,$bin)){Assert-GameFilePath $path;Assert-GameFileACL $path $games $service.DaemonSID}
    Assert-GameFilePath $parent
    if(Test-Path -LiteralPath $parent){
        if(-not(Test-Path -LiteralPath $parent -PathType Container)){throw 'The code parent must be a directory.'};Assert-GameFileACL $parent $games $service.DaemonSID
        if(@(Get-ChildItem -LiteralPath $parent -Force -Filter '.stage-*').Count){throw 'Unfinished code staging exists; inspect it before another import.'}
    }
    Assert-GameFilePath $target;if(Test-Path -LiteralPath $target){throw 'The destination already exists; code import never overwrites or adopts it.'}
    [pscustomobject]@{Root=$service.Root;Executable=$service.Executable;Configuration=$service.Configuration;DaemonSID=$service.DaemonSID;ServerUUID=$id;Parent=$parent;Destination=$target;Games=$games}
}
function Get-GameFilePlan([string]$InstallationRoot,[string]$ServerUUID,[string]$SourceDirectory,[string]$ManifestPath,[string]$ManifestSHA256) {
    if($ManifestSHA256 -notmatch '\A[0-9a-fA-F]{64}\z'){throw 'A pinned SHA-256 of the manifest is required.'}
    $node=Get-GameFileNode $InstallationRoot $ServerUUID
    Assert-GameFilePath $SourceDirectory;Assert-GameFilePath $ManifestPath
    $source=[IO.Path]::GetFullPath($SourceDirectory).TrimEnd('\')
    if(-not(Test-Path -LiteralPath $source -PathType Container) -or $source.Length -le 3){throw 'Use a dedicated local source directory.'}
    foreach($path in @($source,$ManifestPath)){
        if($path -eq $node.Root -or $path.StartsWith(($node.Root+'\'),[StringComparison]::OrdinalIgnoreCase)){throw 'Source and manifest must be outside the managed installation.'}
    }
    if($node.Root.StartsWith(($source+'\'),[StringComparison]::OrdinalIgnoreCase)){throw 'Source directory cannot contain the managed installation.'}
    $files=[Collections.Generic.List[object]]::new();$manifest=$null
    try{
        $manifest=Open-GameFileSource $ManifestPath
        if($manifest.Length -eq 0 -or $manifest.Length -gt 1048576){throw 'Manifest must contain 1 byte through 1 MiB.'}
        if((Get-GameFileHash $manifest) -cne $ManifestSHA256.ToLowerInvariant()){throw 'Manifest SHA-256 mismatch.'}
        $bytes=New-Object byte[] ([int]$manifest.Length);$offset=0
        while($offset -lt $bytes.Length){$n=$manifest.Read($bytes,$offset,$bytes.Length-$offset);if($n -eq 0){throw 'Incomplete manifest read.'};$offset+=$n}
        $value=([Text.UTF8Encoding]::new($false,$true)).GetString($bytes).TrimStart([char]0xfeff)|ConvertFrom-Json
        if(@($value.PSObject.Properties.Name).Count -ne 2 -or -not $value.PSObject.Properties['schema_version'] -or $value.schema_version -isnot [int] -or $value.schema_version -cne 1 -or -not $value.PSObject.Properties['files'] -or $value.files -isnot [array] -or $value.files.Count -lt 1 -or $value.files.Count -gt 1000){throw 'Expected schema_version 1 and an array of 1-1000 files, without extra fields.'}
        $seen=@{};$total=[long]0
        foreach($entry in $value.files){
            if(@($entry.PSObject.Properties.Name).Count -ne 2 -or $entry.path -isnot [string] -or $entry.sha256 -isnot [string] -or $entry.sha256 -notmatch '\A[0-9a-fA-F]{64}\z'){throw 'Each manifest file requires only path and sha256 strings.'}
            $relative=$entry.path.Replace('/','\');$parts=$relative.Split('\')
            if($parts.Count -gt 16 -or $relative.Length -gt 220){throw 'Manifest path exceeds the supported depth or length.'}
            foreach($part in $parts){if($part -cnotmatch '\A[A-Za-z0-9_][A-Za-z0-9_. -]{0,127}\z' -or $part -match '[. ]$'){throw 'Manifest paths must be ordinary relative Windows file names without aliases or traversal.'}}
            if($seen.ContainsKey($relative)){throw 'Duplicate case-insensitive manifest path.'};$seen[$relative]=$true
            $path=Join-Path $source $relative;$stream=Open-GameFileSource $path
            $file=[pscustomobject]@{Path=$relative;Stream=$stream;SHA256=$entry.sha256.ToLowerInvariant();Length=$stream.Length};$files.Add($file)
            $total+=$stream.Length;if($total -gt 8589934592){throw 'Import exceeds the 8 GiB total size limit.'}
            if((Get-GameFileHash $stream) -cne $file.SHA256){throw 'Approved source SHA-256 mismatch.'}
        }
        # Detect file/directory collisions such as a and a/b before mutation.
        foreach($file in $files){$cursor=[IO.Path]::GetDirectoryName($file.Path);while($cursor){if($seen.ContainsKey($cursor)){throw 'Manifest file/directory path collision.'};$cursor=[IO.Path]::GetDirectoryName($cursor)}}
        [pscustomobject]@{Node=$node;Files=$files;Bytes=$total;ManifestSHA256=$ManifestSHA256.ToLowerInvariant()}
    }catch{foreach($file in $files){$file.Stream.Dispose()};throw}finally{if($manifest){$manifest.Dispose()}}
}
function Copy-GameFileStream($Source,$Destination) {$Source.CopyTo($Destination,1048576);$Destination.Flush($true)}
function Publish-GameFileDirectory([string]$Stage,[string]$Destination) {[IO.Directory]::Move($Stage,$Destination)}
function Import-GameFilePlan($Plan) {
    $node=Get-GameFileNode $Plan.Node.Root $Plan.Node.ServerUUID
    if($node.DaemonSID -cne $Plan.Node.DaemonSID -or $node.Destination -cne $Plan.Node.Destination -or ($node.Games -join ',') -cne ($Plan.Node.Games -join ',')){throw 'Service or configured identities changed after planning.'}
    $base='D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)'
    $read=$base+'(A;OICI;FRFX;;;'+$node.DaemonSID+')'
    $parentCreated=$false;$stageCreated=$false;$published=$false
    $stage=Join-Path $node.Parent ('.stage-'+[guid]::NewGuid().ToString('N'))
    $directories=[Collections.Generic.List[string]]::new();$createdFiles=[Collections.Generic.List[string]]::new()
    try{
        if(-not(Test-Path -LiteralPath $node.Parent)){
            [IO.Directory]::CreateDirectory($node.Parent)|Out-Null;$parentCreated=$true
            $parentSDDL=$read;foreach($sid in $node.Games){$parentSDDL+='(A;OICI;FRFX;;;'+$sid+')'}
            Set-GameFileACL $node.Parent $parentSDDL
        }
        Assert-GameFilePath $stage;if(Test-Path -LiteralPath $stage){throw 'Staging directory collision.'}
        [IO.Directory]::CreateDirectory($stage)|Out-Null;$stageCreated=$true;Set-GameFileACL $stage $base
        foreach($file in $Plan.Files){
            $target=Join-Path $stage $file.Path;$cursor=[IO.Path]::GetDirectoryName($target);$missing=[Collections.Generic.List[string]]::new()
            while($cursor -ne $stage -and -not(Test-Path -LiteralPath $cursor)){$missing.Add($cursor);$cursor=[IO.Path]::GetDirectoryName($cursor)}
            for($i=$missing.Count-1;$i -ge 0;$i--){[IO.Directory]::CreateDirectory($missing[$i])|Out-Null;$directories.Add($missing[$i])}
            $output=[IO.File]::Open($target,[IO.FileMode]::CreateNew,[IO.FileAccess]::ReadWrite,[IO.FileShare]::None);$createdFiles.Add($target)
            try{$file.Stream.Position=0;Copy-GameFileStream $file.Stream $output;if($output.Length -ne $file.Length -or (Get-GameFileHash $output) -cne $file.SHA256){throw 'Staged copy verification failed.'}}finally{$output.Dispose()}
        }
        foreach($path in $createdFiles){Set-GameFileACL $path $read}
        for($i=$directories.Count-1;$i -ge 0;$i--){Set-GameFileACL $directories[$i] $read}
        Set-GameFileACL $stage $read
        Assert-GameFilePath $node.Destination
        Publish-GameFileDirectory $stage $node.Destination;$published=$true
        [pscustomobject]@{server_uuid=$node.ServerUUID;destination=$node.Destination;files=$Plan.Files.Count;bytes=$Plan.Bytes;manifest_sha256=$Plan.ManifestSHA256;imported=$true}
    }catch{
        $failure=$_;$errors=@()
        if(-not $published){
            foreach($path in $createdFiles){try{Assert-GameFilePath $path;[IO.File]::Delete($path)}catch{$errors+=$_.Exception.Message}}
            for($i=$directories.Count-1;$i -ge 0;$i--){try{Assert-GameFilePath $directories[$i];[IO.Directory]::Delete($directories[$i],$false)}catch{$errors+=$_.Exception.Message}}
            if($stageCreated){try{Assert-GameFilePath $stage;[IO.Directory]::Delete($stage,$false)}catch{$errors+=$_.Exception.Message}}
            if($parentCreated){try{Assert-GameFilePath $node.Parent;[IO.Directory]::Delete($node.Parent,$false)}catch{$errors+=$_.Exception.Message}}
        }
        if($errors.Count){throw 'Code import failed and staged cleanup needs inspection. No existing destination was overwritten.'}
        throw $failure
    }
}
Export-ModuleMember -Function Get-GameFilePlan,Import-GameFilePlan,Close-GameFilePlan,Open-GamePermissionLock
