[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][ValidatePattern('^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$')][string]$ServerUUID,
    [Parameter(Mandatory = $true)][PSCredential]$Credential
)
$ErrorActionPreference = 'Stop'
if ($Credential.UserName -notmatch '^\.\\[^\\/@]+$') { throw 'Use .\local-game-account as the credential username.' }
if ($Credential.Password.Length -eq 0) { throw 'Empty game passwords are not supported.' }
if (-not ('WingsWindows.CredentialWriter' -as [type])) {
    Add-Type -TypeDefinition @'
using System;
using System.ComponentModel;
using System.Runtime.InteropServices;
namespace WingsWindows {
    public static class CredentialWriter {
        [StructLayout(LayoutKind.Sequential, CharSet=CharSet.Unicode)]
        private struct Credential {
            public uint Flags, Type;
            public string TargetName, Comment;
            public System.Runtime.InteropServices.ComTypes.FILETIME LastWritten;
            public uint BlobSize;
            public IntPtr Blob;
            public uint Persist, AttributeCount;
            public IntPtr Attributes;
            public string TargetAlias, UserName;
        }
        [DllImport("advapi32.dll", EntryPoint="CredWriteW", CharSet=CharSet.Unicode, SetLastError=true)]
        private static extern bool CredWrite(ref Credential value, uint flags);
        public static void Save(string target, string user, IntPtr password, int bytes) {
            var value = new Credential {Type=1, TargetName=target, UserName=user, Blob=password, BlobSize=(uint)bytes, Persist=2};
            if (!CredWrite(ref value, 0)) throw new Win32Exception(Marshal.GetLastWin32Error());
        }
    }
}
'@
}
$passwordBuffer = [Runtime.InteropServices.Marshal]::SecureStringToCoTaskMemUnicode($Credential.Password)
try {
    [WingsWindows.CredentialWriter]::Save(('WingsWindows/' + $ServerUUID.ToLowerInvariant()), $Credential.UserName, $passwordBuffer, ($Credential.Password.Length * 2))
} finally {
    [Runtime.InteropServices.Marshal]::ZeroFreeCoTaskMemUnicode($passwordBuffer)
}
Write-Output 'Game credential stored in the current Windows account credential set. No game account or ACL was changed.'
