# Developer-only service provisioning fixture

Run only on a disposable, checkpointed Windows Server VM as Administrator. This is privileged test infrastructure, not a production setup/bootstrap tool. It deliberately creates temporary local users, grants their rights, replaces DACLs in a fresh owned fixture tree and registers the real `WingsWindowsNative` service. It refuses an existing service or matching account/root. Never point it at real node configuration or a populated installation.

The loopback mock panel serves only the fixture server; the packaged daemon/demo perform the real SCM and separate-account lifecycle. The fixture invokes the packaged Set-ServiceGameCredential.ps1 command, exercising its temporary password-backed task, restricted pipe, metadata/overwrite/replacement/removal behavior and task/right cleanup. It also checks wrong service passwords, operation locking and preservation of preexisting batch rights.

Build the mock in this repository with Windows amd64 Go:

```powershell
go build -o C:\FixtureStaging\mock-panel.exe ./tools/service-provisioning-test
```

Put `mock-panel.exe` and `Test-ServiceProvisioning.ps1` in a new staging directory. Copy the tested package's `Install-Service.ps1`, `NodeSetup.psm1`, `Store-GameCredential.ps1`, `Set-ServiceGameCredential.ps1`, `CredentialBridge.psm1` and `CredentialTask.ps1` from `scripts` into that same staging directory. Keep the verified, extracted release separate. Example elevated invocation (use a fresh random suffix/UUID each time):

```powershell
$suffix = ([guid]::NewGuid().ToString('N')).Substring(0,8)
C:\FixtureStaging\Test-ServiceProvisioning.ps1 `
  -Staging C:\FixtureStaging `
  -Package C:\ExtractedRelease `
  -Root ("C:\WingsWindowsTesting\scm-"+$suffix) `
  -GameName ('WwScmG_'+$suffix) -DaemonName ('WwScmD_'+$suffix) `
  -ServerID ([guid]::NewGuid().ToString())
```

Inspect the returned JSON and `<Root>\scm-report.json`: require `passed=true`, all eighteen checks passed, and every cleanup flag true. The script reports failures in JSON; do not interpret PowerShell process exit 0 alone as success. Independently audit temporary users/rights/service/tasks/processes after execution. On a residual service it retains account/rights state for inspection. Cleanup is best effort and not crash-safe; no force-unload/profile deletion is attempted. Retained files/profile state require separate review after stopping all owned processes.

The historical alpha 17 eleven-check fixture is retained in commit 3deb1b6. The current fixture exercises the public credential helper and eighteen checks; final package results are tracked in [verification](../../docs/VERIFICATION.md). Cleanup-failure recovery is not fully live verified. The tool is not a supported operator utility in the release package. Real panel/TLS/game, reboot/cold-profile recovery, WFP and hostile isolation remain separate gates.

Pass `-TestCrashRecovery` with an alpha 19 package to add four actual game-crash/recovery checks (22 total). This enables a two-attempt/60-second policy in the disposable profile and verifies old job-tree cleanup, manual kill cancellation, budget exhaustion and in-memory budget reset on service restart. It never reboots the host.

Pass `-TestConsoleLogs` with an alpha 20 package to add three logging checks: actual game-token read denial, controlled rotation/file limits and history restored after SCM restart. Combined with `-TestCrashRecovery`, the fixture has 25 checks. Controlled fill uses 36 batches of 16 commands, keeping each JSON request below the API's 64 KiB limit while exceeding the two-file retention budget. Rebuild the mock panel from current source before each new config-schema test. The probe reads only its operator-approved LOG_PROBE_PATH environment value and returns access status without file content.

Pass `-TestServiceEvents` with alpha 21 to add four checks (29 combined): source registration/idempotence/WhatIf, rejection of removal with an existing service, actual lifecycle entries under the non-administrator SCM identity, and a deliberately malformed-config startup failure with safe Event Log contents. It refuses an existing source and removes its own source during cleanup without clearing historical events. No reboot is performed.

Pass `-TestPersistentRecovery` with alpha 22 alongside all three previous flags for 33 combined checks. It requires TestCrashRecovery and TestConsoleLogs, uses a one-hour window, checks actual game-token budget-file denial, preserves the consumed budget across SCM restart and forced termination of the exact fixture daemon, denies further automatic attempts and rejects a deliberately corrupt snapshot during startup. Snapshot bytes are restored in finally. The legacy in-memory reset assertion is replaced by persistent preservation; games remain offline until manual start. No host reboot is performed.

Pass `-TestEggText` with alpha 23 for two additional checks (35 with all previous flags): a missing approved text target blocks launch, then the non-administrator SCM daemon rewrites a UTF-8 file using fresh mock-panel process configuration and preserves its full SDDL. No host reboot occurs. The mock must be rebuilt for egg_config_files.
