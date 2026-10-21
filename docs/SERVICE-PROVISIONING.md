# Initial service account and permissions

Run installation from elevated Windows PowerShell 5.1. Elevation lets the installer configure the host; the daemon should run as a dedicated non-administrator account. Game processes use separate local accounts via `run_as`.

This is initial setup for a disposable trusted node, not an upgrade/migration tool or a complete isolation boundary. The managed mode refuses existing service registrations and populated server data/logs. Keep other writers stopped throughout setup.

## Prepare the owned layout

Create a dedicated directory with only these children:

```text
C:\WingsWindows\
  bin\wings-windows.exe
  bin\scripts\Install-Service.ps1
  bin\scripts\NodeSetup.psm1
  bin\... game executables, declared dependencies and package files
  private\config.json
  private\... TLS certificate/key if configured
  data\
  logs\                         optional, initially empty
```

Extract the release into `bin`. Create `private` and `data`; prepare your real configuration in `private`. Remove any filled example configuration from the shared `bin` tree. The entire `bin` tree is readable/executable by every configured game account: **never put credentials, private keys or other secrets there**.

Set `data_directory` to exactly `C:\WingsWindows\data`. Configure each profile with a full UUID and a distinct enabled non-built-in local game account in `run_as`, including its pinned SID and per-server credential reference. Create game accounts and obtain their passwords using your administrative account workflow before running this installer. It does not create game accounts or reset their passwords.

All executable and declared dependency files must already exist under `bin`. Configured TLS certificate/key files must exist under `private`. Initial data may contain only configured UUID directories, empty apart from an optional empty `.tmp`. The planner rejects observed reparse points, multi-hardlink files and ambiguous paths, and limits code/private enumeration to 10,000 entries. Managed setup currently accepts only an absent or `unmanaged` network policy; it does not grant WFP permissions.

## Install with explicit provisioning

Use a new dedicated local service-account name and a password that satisfies local policy. The prompt keeps the password out of command history:

```powershell
$serviceCredential = Get-Credential '.\WingsRuntime'
C:\WingsWindows\bin\scripts\Install-Service.ps1 `
  -Executable C:\WingsWindows\bin\wings-windows.exe `
  -Configuration C:\WingsWindows\private\config.json `
  -Credential $serviceCredential `
  -InstallationRoot C:\WingsWindows `
  -CreateLocalAccount -ConfigurePermissions
```

Omit `-CreateLocalAccount` to use an existing dedicated local service account. The installer rejects privileged built-in group memberships and disabled/built-in identities. This is not a complete audit of custom group privileges or domain policy. Without `-ConfigurePermissions`, the script only validates configuration and registers an already prepared account; it does not configure its rights/ACLs.

Managed mode grants these **specific rights**, preserving existing assignments:

| Identity | Rights |
| --- | --- |
| Daemon | `SeServiceLogonRight`, `SeAssignPrimaryTokenPrivilege`, `SeIncreaseQuotaPrivilege` |
| Each game | `SeBatchLogonRight` |

It does not add either identity to Administrators, grant interactive/RDP logon or change Windows Firewall. Explicit deny-service/batch rights on the account fail setup. Group/domain deny policy can still prevent logon and must be checked separately. Service and game password expiration, rotation and recovery remain operator responsibilities.

Managed DACLs disable inheritance. Administrators and SYSTEM retain full control; the daemon gets read/execute on code, read on private files and modify on data/logs. Games get read/execute on shared code and data-parent traversal, plus modify only on their own UUID directory and `.tmp`; they get no private/log access. This replaces DACLs on the owned layout, so use it only for initial setup.

## Provision game credentials, then start

The service is registered with **manual startup** and is not started by default. Each game password still must be stored in Credential Manager **as the daemon account**, using the [service credential command](SERVICE-CREDENTIALS.md), or [Store-GameCredential.ps1](IDENTITIES.md) in an appropriate local password-backed provisioning context. Running that helper as Administrator stores the credential for Administrator, not for the daemon. Key-authenticated SSH may lack a usable credential set (observed error 1312); it is insufficient for this step on the tested VM.

Alpha 18 provides [Set-ServiceGameCredential.ps1](SERVICE-CREDENTIALS.md) to perform this step from an elevated operator session. It uses the service credential to create a temporary password-backed task, passes the game credential through a restricted in-memory pipe, and removes the task and any added batch right afterward. The service must remain stopped during the operation. Game-account creation, password changes and cold-profile/reboot recovery remain separate work.

Run identity diagnostics in the daemon's actual context, then validate start, console, stop and restart with the demo before a real game. Once credentials are ready:

```powershell
Start-Service WingsWindowsNative
Get-Service WingsWindowsNative
```

`-Start` is available for a fully prepared account, but a successful service start alone does not prove panel readiness or game credential availability. Host reboot, cold credential/profile initialization, real trusted-TLS panel/game use, persistent service logging and automatic recovery remain open. Keep manual startup until those checks pass.

## Failure and recovery

Configuration and layout checks precede rights/ACL changes. On failure, the script attempts to remove its new service registration, restore all snapshotted DACLs, remove only rights it added and delete only the service account it created. If stopping/deleting the service fails, account/rights/ACL state is retained for inspection. Cleanup errors are surfaced alongside the original error.

New empty directories remain after rollback. No server data is recursively deleted. There is no durable transaction journal: a killed installer or machine failure requires inspection. Path validation is a bounded snapshot for trusted setup, not an atomic defense against concurrent filesystem mutation. Do not run it while untrusted writers can modify the installation tree.

Uninstalling with `sc.exe delete WingsWindowsNative` removes only service registration; account/rights/credential/profile cleanup is a separate administrative step. Never delete a loaded profile or unrelated account to make a test pass.

## Evidence and remaining limits

Server 2022 build 20348 testing of the new installer with alpha 16 runtime binaries passed actual SCM start, non-administrator daemon identity, separate game identity, console, complete demo job-tree termination on service stop and restart/start/stop. An injected registration failure restored the setup changes. The final alpha 17 package passed all 11 checks, including expanded start-failure/full-DACL rollback. See [sanitized evidence](evidence/server2022-alpha17-service.md).

These tests use a local mock panel and harmless demo, with unmanaged networking. They do not certify real panel/TLS/game, WFP traffic isolation, reboot/profile recovery or mutually untrusted hosting. Follow [ROADMAP.md](../ROADMAP.md) for remaining gates.

For optional persistent game console history, configure console_logs.directory as InstallationRoot\logs\console. The existing protected logs ACL is inherited; the daemon creates console/UUID children after installation. See [retention and failure behavior](CONSOLE-LOGS.md). Initial managed setup still requires an empty logs directory.
