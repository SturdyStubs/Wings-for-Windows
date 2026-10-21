# Provision credentials for a managed service

`Set-ServiceGameCredential.ps1` runs from an elevated Windows PowerShell operator session and manages one configured game's credential **as the registered daemon account**. You do not need to log in interactively as that account or write your own bootstrap task. It supports create, metadata inspection, explicit same-account replacement and removal.

The service must already be registered with the managed `bin/private/data/logs` layout and **stopped**. Create the separate non-administrator game account first and configure its full UUID, `run_as.sid` and exact `WingsWindows/<lowercase-UUID>` credential target. The helper checks the actual registered daemon SID/path and the supplied game account's pinned SID. It does not create game users or change their Windows passwords.

## Create, inspect, replace and remove

Run the helper from the new package installed under `bin\scripts`; keep the worker/module and store helper beside it. During maintenance:

```powershell
Stop-Service WingsWindowsNative
$serviceCredential = Get-Credential '.\WingsRuntime'
$gameCredential = Get-Credential '.\WingsGameRust'
$credentialOptions = @{
  InstallationRoot = 'C:\WingsWindows'
  ServerUUID = '11111111-1111-4111-8111-111111111111'
  ServiceCredential = $serviceCredential
}
$helper = 'C:\WingsWindows\bin\scripts\Set-ServiceGameCredential.ps1'
& $helper @credentialOptions -GameCredential $gameCredential
& $helper @credentialOptions -Inspect
```

Creation refuses an existing credential. Inspection returns only the CLI metadata (`target`, `present`, and when present `username`, `last_written`, `persistence`). It does not log on the game account; presence does not establish that the password works. Validate an actual demo/game start through the service afterward.

To update the stored credential deliberately, obtain the intended current game password and use `-Replace`:

```powershell
$gameCredential = Get-Credential '.\WingsGameRust'
& $helper @credentialOptions -GameCredential $gameCredential -Replace
```

Replacement requires an existing target with the same account name. It does **not** reset the Windows account password. Coordinate any actual password change separately, then update the credential and test a real start. Already running processes retain their tokens. Changing identity requires explicit retirement/reconfiguration, rather than replacing it through this command.

To retire the credential while its matching profile is still configured and the service is stopped:

```powershell
& $helper @credentialOptions -Remove
& $helper @credentialOptions -Inspect
```

Removal is exact-target and idempotent. Remove or update the retired profile separately. No account, game file or unrelated credential is deleted. Start the daemon only after the intended credentials/configuration are ready.

## What happens under the hood

The helper temporarily grants the dedicated daemon `SeBatchLogonRight` only if it is absent, then creates an on-demand, limited, password-backed Windows scheduled task. Windows Task Scheduler receives the service password through its credential API; the helper unregisters the task afterward. It does not embed that password in task arguments or a script/config file.

The task runs the packaged internal worker as the daemon. A random duplex named pipe grants access only to the daemon, Administrators and SYSTEM. The operator checks the connected process's actual owner SID before sending the game credential; the worker checks its own SID and the pipe server's OS-reported process ID. Messages are length-framed, limited to 32 KiB and have bounded I/O waits. Only credential metadata returns over the pipe.

The game password is transferred in memory, not through a plaintext file, command-line argument or report. Some managed strings necessarily remain subject to normal .NET garbage collection; this is not protection against administrator debugging or memory dumps. Do not enable custom transcription/instrumentation that captures secret variable values. Game accounts receive no interactive/RDP access.

A no-content exclusive `private\credential-provisioning.lock` serializes helper operations for the installation. It remains on disk, contains no password and is not a transaction journal. Do not run another credential writer, alter account policy or start the service concurrently. Deny-batch rights assigned through groups/domain policy can still prevent task logon.

## Failure, cleanup and deployment limits

Task Scheduler/CIM errors terminate the operation immediately. Registration failure can partially create a task, so cleanup checks the exact random task name even if registration reported an error. Normal completion removes the temporary task and only the batch right this invocation added. Existing rights remain. The task has a one-minute execution limit; connection and pipe waits are bounded. If cleanup cannot confirm the task stopped, the helper reports the task name and retains the added right for inspection. A killed operator process or VM crash may leave temporary task/right state; review it before retrying.

A failure after sending the request can leave a changed credential even if no success response arrived. There is no automatic credential rollback: inspect with `-Inspect` and deliberately choose the next action. Cleanup failures are surfaced even when the credential operation succeeded. The helper does not force-unload or delete a Windows profile. The tested VM retains initialized profiles as loaded after their processes exit.

This command automates the password-backed provisioning context. It does not certify cold boot/profile recovery, service auto-start, full account/password lifecycle, real panel/TLS/game compatibility or WFP isolation. The [roadmap](../ROADMAP.md) and [verification](VERIFICATION.md) track these gates. Lower-level current-account operations remain available in [credential lifecycle](CREDENTIAL-LIFECYCLE.md).

Windows documents the logon-session dependency of [CredWrite](https://learn.microsoft.com/en-us/windows/win32/api/wincred/nf-wincred-credwritew); an Administrator SSH session's credential store is not a substitute for the daemon account's store.
