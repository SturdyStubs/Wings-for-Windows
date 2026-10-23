# Create a dedicated game account

Alpha 37 adds `scripts/New-ServiceGameAccount.ps1` for the account-creation step on an existing managed node. Run it as an administrator in **Windows PowerShell 5.1** with `WingsWindowsNative` stopped. It requires the exact registered managed layout and dedicated daemon identity. A new server's full UUID must have neither a local profile nor a data path yet; account names already present are always refused, never adopted or reset.

```powershell
Stop-Service WingsWindowsNative
$root = 'C:\WingsWindows'
$server = 'YOUR-FULL-SERVER-UUID'
$account = 'WingsGameRust01'
$command = "$root\bin\scripts\New-ServiceGameAccount.ps1"
& $command -InstallationRoot $root -ServerUUID $server -AccountName $account -WhatIf
$password = Read-Host 'New game account password' -AsSecureString
try {
    $created = & $command -InstallationRoot $root -ServerUUID $server -AccountName $account -Password $password
    $created | ConvertTo-Json -Depth 4
    # Keep this SecureString only as long as needed for credential provisioning.
} finally {
    $password.Dispose()
}
```

If `-Password` is omitted for a real creation, the command prompts securely and disposes its own SecureString afterwards. WhatIf validates inputs, the stopped service, configuration and collision guards without prompting, creating an account, granting a right or writing configuration/data. Account names are limited to 1–20 ASCII letters, digits, underscores or hyphens, starting with a letter. Windows enforces the host's password policy; passwords must also fit the existing credential store's limit.

The result contains the account name, local username, SID, server UUID and `run_as` object. It contains no password. The account is enabled, has a non-expiring password and cannot change its own password. An administrator must manage rotation and update the daemon-context credential deliberately. No privileged group membership, logon right, data directory, ACL grant, credential or profile is provisioned by this command.

## Complete the handoff

1. Add your approved executable/installer profile under `profiles.FULL-UUID`, using the returned `run_as.sid` and `WingsWindows/FULL-UUID` credential target. This is an explicit operator edit; the helper does not rewrite JSON or choose software.
2. Run the [incremental permission helper](GAME-PERMISSIONS.md), first with WhatIf, then apply.
3. Use [service credential provisioning](SERVICE-CREDENTIALS.md) with the same game username/password and the registered daemon credential. Prompt for the password again if its earlier SecureString was disposed.
4. Restart the service, invoke the approved native installer and start manually. Follow the [node walkthrough](PANEL-WINDOWS-SETUP.md) and [pending-creation handoff](PENDING-CREATION.md).

This is for adding an account to an existing managed node. Initial node setup, existing account password changes, account deletion and automatic panel provisioning remain separate. The UUID comes from the operator; this offline command does not verify live panel assignment.

## Failure and maintenance limits

Creation shares the credential-provisioning lock and holds configuration against ordinary writes/replacement. It rechecks service identity, UUID/data and account collisions after acquiring the lock. Keep writers stopped during maintenance; checks are snapshots rather than hostile-writer confinement.

After Windows returns a newly created account SID, validation checks that it is a dedicated enabled local identity, different from the daemon and outside the privileged built-in groups checked by node setup. Validation failure removes **only that returned SID**. An account that already exists is never removed. If creation's outcome is uncertain or SID removal fails, inspect local accounts before retrying; there is no durable recovery journal. A successful account remains if a later, separate permission or credential step fails. Do not delete accounts still referenced by profiles, processes or stored credentials.

This remains a trusted private-node workflow. Harmless mock/demo tests do not establish real game, live panel or container-equivalent isolation compatibility.
