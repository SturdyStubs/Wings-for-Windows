[CmdletBinding()]
param(
 [Parameter(Mandatory=$true)][ValidatePattern('\A[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}\z')][string]$ServerUUID,
 [Parameter(Mandatory=$true)][Security.SecureString]$Password,
 [switch]$Replace
)
$ErrorActionPreference='Stop'
if($Password.Length -eq 0 -or $Password.Length -gt 1280){throw 'RCON password must fit the credential storage limit.'}
if (-not ('WingsWindows.RCONCredentialWriterV1' -as [type])) {
    Add-Type -TypeDefinition @'
using System;
using System.ComponentModel;
using System.Runtime.InteropServices;
namespace WingsWindows {
    public static class RCONCredentialWriterV1 {
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
                        if (!String.Equals(old.UserName, user, StringComparison.OrdinalIgnoreCase)) throw new InvalidOperationException("Rotation cannot change the credential format; remove the old credential and update the pinned SID explicitly.");
                    } finally {
                        if (old.Blob != IntPtr.Zero && old.BlobSize <= 2560) for (int i=0; i<old.BlobSize; i++) Marshal.WriteByte(old.Blob, i, 0);
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
$passwordBuffer=[Runtime.InteropServices.Marshal]::SecureStringToCoTaskMemUnicode($Password)
$text=$null
try {
 $text=[Runtime.InteropServices.Marshal]::PtrToStringUni($passwordBuffer,$Password.Length)
 try{$valid=($text.IndexOfAny([char[]]@([char]0,[char]10,[char]13)) -lt 0 -and [Text.UTF8Encoding]::new($false,$true).GetByteCount($text) -le 4086)}catch{$valid=$false}
 if(-not $valid){throw 'RCON password must be a single UTF-8 value of at most 4086 bytes.'}
 [WingsWindows.RCONCredentialWriterV1]::Save(('WingsWindowsRCON/'+$ServerUUID.ToLowerInvariant()),'.\WingsRCONSecret-v1',$passwordBuffer,($Password.Length*2),$Replace.IsPresent)
} finally {
 $text=$null
 [Runtime.InteropServices.Marshal]::ZeroFreeCoTaskMemUnicode($passwordBuffer)
}
Write-Output 'RCON credential stored in the current Windows account credential set.'
