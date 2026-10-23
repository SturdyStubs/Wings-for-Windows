# Add one game identity to an existing node

Alpha 36 adds `scripts/Set-ServiceGamePermissions.ps1` for an existing managed installation with populated data and logs. It prepares a **new, empty server directory** without rerunning initial service setup. Run it as an administrator in **Windows PowerShell 5.1** during a maintenance window with `WingsWindowsNative` stopped.

First create a dedicated, enabled, non-administrator local game account with the [account helper](GAME-ACCOUNTS.md), before adding its UUID profile, or provision it manually. Add an approved profile keyed by the full server UUID, with that account's canonical `run_as.sid` and `credential_target` equal to `WingsWindows/FULL-UUID`. All configured game identities must be distinct from each other and the daemon. Provision the approved executable, installer and required files under the managed `bin` directory before proceeding. For a new UUID, the [verified file importer](GAME-FILES.md) can do this before adding its profile. Leftover `.stage-*` code blocks permission preparation until inspected.

```powershell
Stop-Service WingsWindowsNative
$root = 'C:\WingsWindows'
$server = 'YOUR-FULL-SERVER-UUID'
& "$root\bin\scripts\Set-ServiceGamePermissions.ps1" -InstallationRoot $root -ServerUUID $server -WhatIf
& "$root\bin\scripts\Set-ServiceGamePermissions.ps1" -InstallationRoot $root -ServerUUID $server
```

`-WhatIf` validates the service registration, configuration, code requirements, accounts, paths and existing ACLs without changing permissions, rights or directories. The real command:

- Adds non-inheriting read/execute grants to the installation root, data parent and existing code tree. Grants on these shared parents do not propagate into other server data.
- Creates the selected server's directory and `.tmp` if absent, with protected SYSTEM/Administrators full control and daemon/selected-game modify access.
- Grants that game account `SeBatchLogonRight` if absent, preserving preexisting rights. An explicit deny-batch right aborts preparation.

After preparation, use [service credential provisioning](SERVICE-CREDENTIALS.md) to store the new game's credential under the daemon identity. Account creation, password management and credential storage are separate operations; alpha 37 supplies an [operator account-creation command](GAME-ACCOUNTS.md). Start the service, invoke panel **Reinstall** for the [approved native setup recipe](NATIVE-INSTALLERS.md), then start the server manually. See [pending creation](PENDING-CREATION.md) for the UUID approval handoff.

## Requirements and limits

The command requires the exact registered managed `bin/private/data/logs` layout and a dedicated non-administrator daemon account. It refuses unexpected ACL identities, unsupported/deny ACEs, unsafe write masks, nonprivileged owners, links, multi-hardlink files and paths outside the managed code tree. Planning scans at most 10,000 combined managed entries; it does not recursively rewrite other servers' data. Keep every writer stopped during maintenance. These checks are bounded filesystem snapshots, not protection against hostile concurrent administrators or complete effective-token isolation.

The selected data directory must be absent or empty, apart from an empty `.tmp`. A populated target is rejected. This command is not an ACL repair, migration, account removal or binary update tool. New files added to `bin` later need an explicitly reviewed provisioning/permission update; grants on existing directories do not automatically include future children. WFP permission setup is unsupported here.

Application shares the credential-provisioning lock and holds configuration against ordinary writes/replacement. It checks saved shared ACLs again before changing them. On failure it restores snapshotted DACLs and removes only a batch right it added. Newly created empty directories remain; it never recursively deletes data. Rollback errors require operator inspection. There is no durable rollback journal or automatic recovery after power loss.

This remains an experimental trusted-node workflow. Passing a harmless fixture does not certify live panel provisioning, a real Windows game, unrestricted Linux eggs or container-equivalent isolation.
