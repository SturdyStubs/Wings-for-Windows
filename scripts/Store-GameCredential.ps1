[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][ValidatePattern('\A[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\z')][string]$ServerUUID,
    [Parameter(Mandatory = $true)][PSCredential]$Credential,
    [switch]$Replace
)
$ErrorActionPreference = 'Stop'
if ($Credential.UserName -notmatch '\A\.\\[^\\/@\x00\r\n]+\z') { throw 'Use .\local-game-account as the credential username.' }
if ($Credential.Password.Length -eq 0) { throw 'Empty game passwords are not supported.' }
if ($Credential.Password.Length -gt 2560) { throw 'Credential password exceeds the 5120-byte storage limit.' }
if (-not ('WingsWindows.CredentialWriterV2' -as [type])) {
    Add-Type -TypeDefinition @'
using System;
using System.ComponentModel;
using System.Runtime.InteropServices;
namespace WingsWindows {
    public static class CredentialWriterV2 {
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
        [DllImport("advapi32.dll", EntryPoint="CredReadW", CharSet=CharSet.Unicode, SetLastError=true)]
        private static extern bool CredRead(string target, uint type, uint flags, out IntPtr value);
        [DllImport("advapi32.dll")]
        private static extern void CredFree(IntPtr value);
        public static void Save(string target, string user, IntPtr password, int bytes, bool replace) {
            IntPtr existing;
            if (CredRead(target, 1, 0, out existing)) {
                try {
                    var old = (Credential)Marshal.PtrToStructure(existing, typeof(Credential));
                    try {
                        if (!replace) throw new InvalidOperationException("Credential already exists; use -Replace for deliberate rotation.");
                        if (!String.Equals(old.UserName, user, StringComparison.OrdinalIgnoreCase)) throw new InvalidOperationException("Rotation cannot change the game account; remove the old credential and update the pinned SID explicitly.");
                    } finally {
                        if (old.Blob != IntPtr.Zero && old.BlobSize <= 5120) for (int i=0; i<old.BlobSize; i++) Marshal.WriteByte(old.Blob, i, 0);
                    }
                } finally { CredFree(existing); }
            } else {
                int error = Marshal.GetLastWin32Error();
                if (error != 1168) throw new Win32Exception(error);
                if (replace) throw new InvalidOperationException("No credential exists to rotate; omit -Replace to create it.");
            }
            var value = new Credential {Type=1, TargetName=target, UserName=user, Blob=password, BlobSize=(uint)bytes, Persist=2};
            if (!CredWrite(ref value, 0)) throw new Win32Exception(Marshal.GetLastWin32Error());
        }
    }
}
'@
}
$passwordBuffer = [Runtime.InteropServices.Marshal]::SecureStringToCoTaskMemUnicode($Credential.Password)
try {
    [WingsWindows.CredentialWriterV2]::Save(('WingsWindows/' + $ServerUUID.ToLowerInvariant()), $Credential.UserName, $passwordBuffer, ($Credential.Password.Length * 2), $Replace.IsPresent)
} finally {
    [Runtime.InteropServices.Marshal]::ZeroFreeCoTaskMemUnicode($passwordBuffer)
}
Write-Output 'Game credential stored in the current Windows account credential set. No game account or ACL was changed.'
