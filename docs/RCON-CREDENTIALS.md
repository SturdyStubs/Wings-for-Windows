# RCON passwords in Windows Credential Manager

Alpha 32 adds an alternative to daemon environment passwords. Each RCON-enabled profile can reference one generic credential in the **daemon Windows account's** credential set:

```json
"source_rcon": {
  "address": "127.0.0.1:27015",
  "password_credential_target": "WingsWindowsRCON/11111111-1111-4111-8111-111111111111",
  "timeout_seconds": 3
}
```

Replace the UUID with that profile's server UUID. Choose exactly one of `password_credential_target` or the existing `password_environment`; neither automatic fallback nor a reference to another server/account namespace is permitted. Node JSON stores only the reference. The daemon reads it afresh for each command, so removal/rotation affects subsequent commands. Configuration preflight and metadata inspection do not authenticate against the game; an entry's presence is not proof of a working password.

The `WingsWindowsRCON/<UUID>` namespace is separate from `WingsWindows/<UUID>` game-account logon credentials. RCON store/remove operations never create users, change game passwords, edit ACLs or log on using the RCON password. A fixed format marker occupies the credential username field; it is not a Windows account to create. Passwords are nonempty single values bounded by both the 4086-byte RCON UTF-8 budget and Windows storage: at most 1280 UTF-16 code units (2560 bytes). Invalid text/format fails closed. The store refuses accidental overwrite; `-Replace` requires an existing matching-format entry.

## Managed service deployment

From an elevated PowerShell session, stop the game and service, configure the exact reference above, and run the packaged helper. Use prompts rather than literal password text:

```powershell
$root = 'C:\WingsWindows'
$server = '11111111-1111-4111-8111-111111111111'
$service = Get-Credential '.\WingsDaemon'
$secret = Read-Host 'Game RCON password' -AsSecureString
try {
    & "$root\bin\scripts\Set-ServiceRCONCredential.ps1" `
        -InstallationRoot $root -ServerUUID $server `
        -ServiceCredential $service -Password $secret
} finally { $secret.Dispose() }
```

Configure the game's matching password separately using that game's documented method. This helper does not rewrite game config or prove the game's listener belongs to the expected process.

The service must be stopped, registered to the supplied dedicated local daemon account, and match the managed installation paths exactly. The configured profile must retain the managed separate `run_as` reference and exact RCON target. Inspection/removal require that configured reference; remove the entry before deleting the profile. The helper reuses the existing serialized credential worker: random temporary scheduled task under the daemon SID, bounded local named pipe with protected DACL and peer process/SID checks, temporary batch-right ownership and verified cleanup. The RCON secret is carried in that pipe, not task arguments, node JSON, environment variables or temporary plaintext files.

Use the same helper with `-Inspect` for metadata, `-Password $secret -Replace` for deliberate rotation, or `-Remove` for removal. Those are mutually exclusive parameter sets. Rotation does not change the game's own configured password. Stop the game before service restart if automatic running restoration is enabled. A helper failure may occur after a store succeeded; inspect before retrying. If worker/task/right cleanup fails, preserve the service account and inspect the reported state rather than assuming completion.

## Interactive daemon account

When running directly under the intended daemon account, use `scripts\Store-RCONCredential.ps1 -ServerUUID $server -Password $secret` (and `-Replace` for rotation). Running that command as Administrator stores it in Administrator's credential set, not in another service account. For metadata and removal under the current account:

```powershell
.\wings-windows.exe --rcon-credential-status $server
.\wings-windows.exe --remove-rcon-credential $server
```

These CLI operations require a full UUID and cannot combine with another CLI operation. They do not load node configuration, contact the panel, authenticate RCON, enumerate other entries or start/stop games. Metadata contains target, presence, timestamp and persistence, never the password. Removing a missing entry is idempotent. The game-account credential CLI remains separate.

## Boundaries and verification

Windows `local_machine` persistence here means subsequent logon sessions of the **same user on this computer**, not a credential shared with every local user. See [Microsoft's persistence definition](https://learn.microsoft.com/en-us/windows/win32/api/wincred/ns-wincred-credentialw). Use distinct daemon/game identities. Same-account games, administrators, compromised daemon code and plaintext in process memory are outside the isolation guarantee. Windows account/profile/password lifecycle and backup behavior still need deployment-specific validation. Network RCON remains loopback-only and unencrypted.

Exact password matches in assembled RCON replies are still redacted before publication; arbitrary stdout, log files, commands and encoded secret variants are not generally scrubbed. Missing/invalid credentials fail explicitly with no environment fallback. Existing transport limits, cancellation and no-retry behavior remain unchanged.

Tests cover format/namespace rules, Unicode, guarded create/replace/remove, metadata, malformed blobs, missing-secret behavior and live native RCON rotation. The combined service fixture adds `-TestRCONCredential` for actual daemon-account provisioning, separate-game lookup failure, namespace preservation and cleanup. The clean release passes 672 local cases/subtests, 14 native cases and all 66 combined Server 2022 checks, with independent cleanup. See [exact package/source/results](evidence/server2022-alpha32-rcon-credentials.md). No real-game or new reboot certification is implied.
