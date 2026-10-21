$ErrorActionPreference = 'Stop'

function Initialize-NodeSetupNative {
    if ('WingsWindows.NodeSetup' -as [type]) { return }
    Add-Type -TypeDefinition @'
using System;
using System.Collections.Generic;
using System.ComponentModel;
using System.Runtime.InteropServices;
using System.Security.Principal;
using Microsoft.Win32.SafeHandles;
namespace WingsWindows {
 public static class NodeSetup {
  [StructLayout(LayoutKind.Sequential)] struct OA { public uint Length; public IntPtr Root,Object; public uint Attributes; public IntPtr Descriptor,Quality; }
  [StructLayout(LayoutKind.Sequential)] struct US { public ushort Length,MaximumLength; public IntPtr Buffer; }
  [StructLayout(LayoutKind.Sequential)] struct FI { public uint Attributes; public System.Runtime.InteropServices.ComTypes.FILETIME Created,Accessed,Written; public uint Volume,SizeHigh,SizeLow,Links,IndexHigh,IndexLow; }
  [DllImport("advapi32.dll")] static extern uint LsaOpenPolicy(IntPtr system, ref OA a, uint access, out IntPtr h);
  [DllImport("advapi32.dll")] static extern uint LsaAddAccountRights(IntPtr h, IntPtr sid, US[] rights, uint count);
  [DllImport("advapi32.dll")] static extern uint LsaRemoveAccountRights(IntPtr h, IntPtr sid, [MarshalAs(UnmanagedType.U1)] bool all, US[] rights, uint count);
  [DllImport("advapi32.dll")] static extern uint LsaEnumerateAccountRights(IntPtr h, IntPtr sid, out IntPtr rights, out uint count);
  [DllImport("advapi32.dll")] static extern uint LsaNtStatusToWinError(uint status);
  [DllImport("advapi32.dll")] static extern uint LsaFreeMemory(IntPtr p);
  [DllImport("advapi32.dll")] static extern uint LsaClose(IntPtr h);
  [DllImport("kernel32.dll",CharSet=CharSet.Unicode,SetLastError=true)] static extern SafeFileHandle CreateFile(string name,uint access,uint share,IntPtr security,uint disposition,uint flags,IntPtr template);
  [DllImport("kernel32.dll",SetLastError=true)] static extern bool GetFileInformationByHandle(SafeFileHandle h,out FI info);
  static void Check(uint status) { if(status!=0)throw new Win32Exception((int)LsaNtStatusToWinError(status)); }
  static IntPtr Open() { OA a=new OA();a.Length=(uint)Marshal.SizeOf(typeof(OA));IntPtr h;Check(LsaOpenPolicy(IntPtr.Zero,ref a,0x810,out h));return h; }
  static IntPtr Sid(string text) {SecurityIdentifier s=new SecurityIdentifier(text);byte[] b=new byte[s.BinaryLength];s.GetBinaryForm(b,0);IntPtr p=Marshal.AllocHGlobal(b.Length);Marshal.Copy(b,0,p,b.Length);return p;}
  public static string[] Rights(string text) {
   IntPtr h=IntPtr.Zero,s=IntPtr.Zero,p=IntPtr.Zero;
   try {h=Open();s=Sid(text);uint count;uint status=LsaEnumerateAccountRights(h,s,out p,out count);if(LsaNtStatusToWinError(status)==2)return new string[0];Check(status);List<string> names=new List<string>();int size=Marshal.SizeOf(typeof(US));for(int i=0;i<count;i++){US u=(US)Marshal.PtrToStructure(IntPtr.Add(p,i*size),typeof(US));names.Add(Marshal.PtrToStringUni(u.Buffer,u.Length/2));}return names.ToArray();}
   finally {if(p!=IntPtr.Zero)LsaFreeMemory(p);if(s!=IntPtr.Zero)Marshal.FreeHGlobal(s);if(h!=IntPtr.Zero)LsaClose(h);}
  }
  public static void Right(string text,string name,bool remove) {
   IntPtr h=IntPtr.Zero,s=IntPtr.Zero,b=IntPtr.Zero;
   try {h=Open();s=Sid(text);b=Marshal.StringToHGlobalUni(name);US u=new US();u.Length=(ushort)(name.Length*2);u.MaximumLength=(ushort)(u.Length+2);u.Buffer=b;US[] r=new US[]{u};Check(remove?LsaRemoveAccountRights(h,s,false,r,1):LsaAddAccountRights(h,s,r,1));}
   finally {if(b!=IntPtr.Zero)Marshal.FreeHGlobal(b);if(s!=IntPtr.Zero)Marshal.FreeHGlobal(s);if(h!=IntPtr.Zero)LsaClose(h);}
  }
  public static void ValidatePath(string path) {
   using(SafeFileHandle h=CreateFile(path,0x80000000,3,IntPtr.Zero,3,0x02200000,IntPtr.Zero)) {
    if(h.IsInvalid)throw new Win32Exception(Marshal.GetLastWin32Error());FI i;if(!GetFileInformationByHandle(h,out i))throw new Win32Exception(Marshal.GetLastWin32Error());
    if((i.Attributes&0x400)!=0)throw new InvalidOperationException("Reparse points are unsupported in installation paths.");
    if((i.Attributes&0x10)==0&&i.Links!=1)throw new InvalidOperationException("Installation files must have one hardlink.");
   }
  }
 }
}
'@
}

function Get-NodeLocalAccount([string]$Name) {
    if ($Name -notmatch '^\.\\([^\\/@\x00\r\n]+)$') { throw 'Use .\local-account for the service credential.' }
    $account = Get-LocalUser -Name $Matches[1] -ErrorAction Stop
    if (-not $account.Enabled -or $account.SID.Value -notmatch '^S-1-5-21-(\d+-){3}(\d+)$' -or [uint64]$Matches[2] -lt 1000) { throw 'Use an enabled dedicated non-built-in local account.' }
    foreach ($group in @('S-1-5-32-544','S-1-5-32-547','S-1-5-32-548','S-1-5-32-549','S-1-5-32-550','S-1-5-32-551')) {
        $members = @(Get-LocalGroupMember -SID ([Security.Principal.SecurityIdentifier]::new($group)) -ErrorAction SilentlyContinue)
        if ($account.SID.Value -in @($members | ForEach-Object { $_.SID.Value })) { throw 'The dedicated account must not belong to a privileged built-in group.' }
    }
    return $account
}

function Assert-NodeSetupPath([string]$Path) {
    if (-not [IO.Path]::IsPathRooted($Path) -or $Path -notmatch '^[A-Za-z]:\\' -or $Path -match '[\x00-\x1f*?<>|]' -or $Path.Substring(2).Contains(':')) { throw 'Installation paths must be absolute local paths without aliases or streams.' }
    $clean = [IO.Path]::GetFullPath($Path).TrimEnd('\')
    if ($clean -ne $Path.TrimEnd('\')) { throw 'Installation paths must be canonical.' }
    foreach ($part in $clean.Substring(3).Split('\')) { if ($part -match '[. ]$' -or $part -match '^(?i:CON|PRN|AUX|NUL|COM[1-9]|LPT[1-9])(?:\.|$)') { throw 'Ambiguous installation path component.' } }
    $cursor = $clean
    while ($cursor.Length -ge 3) {
        if (Test-Path -LiteralPath $cursor) { [WingsWindows.NodeSetup]::ValidatePath($cursor) }
        $parent = [IO.Path]::GetDirectoryName($cursor)
        if (-not $parent -or $parent -eq $cursor) { break }
        $cursor = $parent
    }
}

function Get-NodePermissionPlan([string]$InstallationRoot,[string]$Executable,[string]$Configuration,[string]$DaemonSID) {
    Initialize-NodeSetupNative
    if($DaemonSID -notmatch '^S-1-5-21-(\d+-){3}\d+$' -or ([Security.Principal.SecurityIdentifier]::new($DaemonSID)).Value -ne $DaemonSID){throw 'A canonical local daemon SID is required.'}
    Assert-NodeSetupPath $InstallationRoot
    $root = [IO.Path]::GetFullPath($InstallationRoot).TrimEnd('\')
    if ($root.Length -le 3) { throw 'InstallationRoot must be a dedicated directory, not a volume root.' }
    $bin = Join-Path $root 'bin'; $private = Join-Path $root 'private'; $data = Join-Path $root 'data'
    $logs = Join-Path $root 'logs';Assert-NodeSetupPath $logs
    foreach($item in Get-ChildItem -LiteralPath $root -Force){if(-not $item.PSIsContainer -or $item.Name -notin @('bin','private','data','logs')){throw 'InstallationRoot may contain only the owned bin/private/data/logs layout.'}}
    if((Test-Path -LiteralPath $logs) -and @(Get-ChildItem -LiteralPath $logs -Force).Count -ne 0){throw 'Managed setup requires an empty initial logs directory.'}
    foreach ($path in @($Executable,$Configuration)) { Assert-NodeSetupPath $path }
    if (-not $Executable.StartsWith(($bin+'\'),[StringComparison]::OrdinalIgnoreCase) -or -not $Configuration.StartsWith(($private+'\'),[StringComparison]::OrdinalIgnoreCase)) { throw 'Managed setup requires the executable under bin and configuration under private.' }
    $config = Get-Content -LiteralPath $Configuration -Raw | ConvertFrom-Json
    if($config.console_logs -and $config.console_logs.directory -ne (Join-Path $logs 'console')){throw 'Managed console_logs.directory must be InstallationRoot\logs\console.'}
    if ($config.data_directory.TrimEnd('\') -ne $data) { throw 'Managed setup requires data_directory = InstallationRoot\data.' }
    $gameSIDs = @();$profiles = @($config.profiles.PSObject.Properties)
    foreach ($entry in $profiles) {
        if($entry.Name -notmatch '^[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}$'){throw 'Every profile must use a full server UUID.'}
        $profile = $entry.Value
        if (-not $profile.run_as) { throw 'Managed setup requires a distinct run_as identity on every profile.' }
        $sid = [string]$profile.run_as.sid
        $game = Get-LocalUser -SID ([Security.Principal.SecurityIdentifier]::new($sid)) -ErrorAction Stop
        $checked = Get-NodeLocalAccount ('.\'+$game.Name)
        if ($checked.SID.Value -eq $DaemonSID -or $sid -in $gameSIDs) { throw 'Every game identity must be distinct from the daemon and other profiles.' }
        $gameSIDs += $sid
        if ($profile.network_policy.mode -and $profile.network_policy.mode -ne 'unmanaged') { throw 'Managed setup does not grant WFP permissions; validate that policy separately.' }
        foreach ($path in @($profile.executable)+@($profile.requirements.required_files)) {
            if (-not $path) { continue };Assert-NodeSetupPath $path
            if (-not $path.StartsWith(($bin+'\'),[StringComparison]::OrdinalIgnoreCase)) { throw 'All configured executable/dependency files must be under bin.' }
            if (-not(Test-Path -LiteralPath $path -PathType Leaf)) { throw 'Configured executable/dependency files must already exist.' }
        }
    }
    foreach ($path in @($config.tls.cert,$config.tls.key)) { if($path){Assert-NodeSetupPath $path;if(-not $path.StartsWith(($private+'\'),[StringComparison]::OrdinalIgnoreCase)){throw 'TLS material must be under private.'};if(-not(Test-Path -LiteralPath $path -PathType Leaf)){throw 'TLS files must already exist.'}} }
    $items = @()
    foreach ($directory in @($bin,$private,$data)) {
        if (-not (Test-Path -LiteralPath $directory -PathType Container)) { throw 'Create the bin/private/data layout before managed setup.' }
    }
    # Reject links before recursing. A trusted installer must still stop writers:
    # this is a bounded snapshot, not an adversarial filesystem transaction.
    $queue = [Collections.Generic.Queue[string]]::new();$queue.Enqueue($bin);$queue.Enqueue($private)
    while($queue.Count -gt 0) {
        $directory=$queue.Dequeue()
        foreach($item in Get-ChildItem -LiteralPath $directory -Force) {
            Assert-NodeSetupPath $item.FullName;$items+=$item.FullName
            if($items.Count -gt 10000){throw 'Managed setup exceeds 10000 code/private entries.'}
            if($item.PSIsContainer){$queue.Enqueue($item.FullName)}
        }
    }
    $expected = @($profiles | ForEach-Object { $_.Name })
    foreach($entry in Get-ChildItem -LiteralPath $data -Force) { if(-not $entry.PSIsContainer -or $entry.Name -notin $expected){throw 'Managed data must contain only configured empty server directories.'} }
    foreach($entry in $profiles) {
        $serverRoot=Join-Path $data $entry.Name;Assert-NodeSetupPath $serverRoot
        if(Test-Path -LiteralPath $serverRoot){foreach($child in Get-ChildItem -LiteralPath $serverRoot -Force){if(-not $child.PSIsContainer -or $child.Name -ne '.tmp'){throw 'Managed setup only supports empty initial server data.'};Assert-NodeSetupPath $child.FullName;if(@(Get-ChildItem -LiteralPath $child.FullName -Force).Count -ne 0){throw 'Initial .tmp directory must be empty.'}}}
    }
    $allGame = ($gameSIDs | ForEach-Object { '(A;OICI;FRFX;;;'+$_+')' }) -join ''
    $base='D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)'
    $read=$base+'(A;OICI;FRFX;;;'+$DaemonSID+')'+$allGame
    $privateRead=$base+'(A;OICI;FRFX;;;'+$DaemonSID+')'
    $daemonModify=$base+'(A;OICI;0x1301bf;;;'+$DaemonSID+')'
    $rules=@([pscustomobject]@{Path=$root;SDDL=$read},[pscustomobject]@{Path=$bin;SDDL=$read},[pscustomobject]@{Path=$private;SDDL=$privateRead},[pscustomobject]@{Path=$data;SDDL=$daemonModify+$allGame},[pscustomobject]@{Path=$logs;SDDL=$daemonModify})
    foreach($path in $items){$sddl=$privateRead;if(-not(Test-Path -LiteralPath $path -PathType Container)){$sddl=$base+'(A;;FR;;;'+$DaemonSID+')'};if($path.StartsWith(($bin+'\'),[StringComparison]::OrdinalIgnoreCase)){$sddl=$read};$rules+=[pscustomobject]@{Path=$path;SDDL=$sddl}}
    foreach($entry in $profiles){$sddl=$daemonModify+'(A;OICI;0x1301bf;;;'+$entry.Value.run_as.sid+')';$serverRoot=Join-Path $data $entry.Name;$rules+=[pscustomobject]@{Path=$serverRoot;SDDL=$sddl};$rules+=[pscustomobject]@{Path=(Join-Path $serverRoot '.tmp');SDDL=$sddl}}
    return [pscustomobject]@{Rules=$rules;GameSIDs=$gameSIDs;DaemonSID=$DaemonSID;Root=$root}
}

function Enable-NodeInstallation($Plan) {
    $state=[pscustomobject]@{ACLs=[Collections.Generic.List[object]]::new();Rights=[Collections.Generic.List[object]]::new();Directories=[Collections.Generic.List[string]]::new()}
    try {
        foreach($entry in @([pscustomobject]@{SID=$Plan.DaemonSID;Names=@('SeServiceLogonRight','SeAssignPrimaryTokenPrivilege','SeIncreaseQuotaPrivilege')})+@($Plan.GameSIDs | ForEach-Object {[pscustomobject]@{SID=$_;Names=@('SeBatchLogonRight')}})) {
            $existing=@([WingsWindows.NodeSetup]::Rights($entry.SID))
            if('SeDenyServiceLogonRight' -in $existing -and 'SeServiceLogonRight' -in $entry.Names){throw 'The service identity has an explicit deny-service-logon right.'}
            if('SeDenyBatchLogonRight' -in $existing -and 'SeBatchLogonRight' -in $entry.Names){throw 'A game identity has an explicit deny-batch-logon right.'}
            foreach($name in $entry.Names){if($name -notin $existing){[WingsWindows.NodeSetup]::Right($entry.SID,$name,$false);$state.Rights.Add([pscustomobject]@{SID=$entry.SID;Name=$name})}}
        }
        foreach($rule in $Plan.Rules) {
            Assert-NodeSetupPath $rule.Path
            if(-not(Test-Path -LiteralPath $rule.Path)){[IO.Directory]::CreateDirectory($rule.Path)|Out-Null;$state.Directories.Add($rule.Path)}
            $acl=Get-Acl -LiteralPath $rule.Path
            $state.ACLs.Add([pscustomobject]@{Path=$rule.Path;SDDL=$acl.GetSecurityDescriptorSddlForm([Security.AccessControl.AccessControlSections]::Access)})
        }
        foreach($rule in $Plan.Rules) {
            Assert-NodeSetupPath $rule.Path
            $acl=Get-Acl -LiteralPath $rule.Path
            $acl.SetSecurityDescriptorSddlForm($rule.SDDL,[Security.AccessControl.AccessControlSections]::Access)
            Set-Acl -LiteralPath $rule.Path -AclObject $acl
        }
        return $state
    } catch { $original=$_;try{Undo-NodeInstallation $state}catch{throw ('Setup failed: '+$original.Exception.Message+'; rollback failed: '+$_.Exception.Message)};throw $original }
}

function Undo-NodeInstallation($State) {
    $failures=@()
    for($i=$State.ACLs.Count-1;$i -ge 0;$i--){$entry=$State.ACLs[$i];try{Assert-NodeSetupPath $entry.Path;$acl=Get-Acl -LiteralPath $entry.Path;$acl.SetSecurityDescriptorSddlForm($entry.SDDL,[Security.AccessControl.AccessControlSections]::Access);Set-Acl -LiteralPath $entry.Path -AclObject $acl}catch{$failures+=$_.Exception.Message}}
    foreach($entry in $State.Rights){try{[WingsWindows.NodeSetup]::Right($entry.SID,$entry.Name,$true)}catch{$failures+=$_.Exception.Message}}
    # Keep newly created directories on failure; never recursively delete data.
    if($failures.Count -ne 0){throw ($failures -join '; ')}
}

Export-ModuleMember -Function Initialize-NodeSetupNative,Get-NodeLocalAccount,Get-NodePermissionPlan,Enable-NodeInstallation,Undo-NodeInstallation
