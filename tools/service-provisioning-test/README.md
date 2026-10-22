# Developer-only service provisioning fixture

`-TestSourceRCON` adds a separate phase after the normal checks: a loopback Source TCP demo, daemon-only environment password reference, API command/reply, redaction, distinct game identity, graceful tree stop and bad-password rejection. Its fixed demo password is public test data. This mode cannot be combined with `-PrepareReboot`; it does not certify a real game or private service environment provisioning.

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

Pass `-TestEggJSON` with alpha 24 alongside TestEggText for two additional checks (37 with all flags): malformed JSON blocks launch without changing an earlier text target, then actual SCM rewriting preserves typed values, untouched large-number precision, conditional wildcard behavior, Unicode and full SDDL. Rebuild the mock for both approved target entries. No host reboot occurs.

Pass `-TestEggProperties` with alpha 25 alongside TestEggJSON for two additional checks (39 with all flags): malformed third-file Unicode prevents launch and preserves earlier text/JSON, then actual SCM edits preserve header/literal environment references, decode CRLF continuations, encode Unicode/surrogate pairs safely, honor literal conditions and retain full SDDL. Rebuild the mock for all three targets. No host reboot occurs.

Pass `-TestEggINI` with alpha 26 alongside TestEggProperties for two additional checks (41 with all flags): malformed fourth-file INI prevents launch and preserves earlier text/JSON/properties, then actual SCM edits verify default/bracketed/dotted sections, exact child ownership, Unicode, literal environment references, Windows paths, quoted values and full SDDL. Rebuild the mock for all four targets. No host reboot occurs.

Pass `-TestEggYAML` with alpha 27 alongside TestEggINI for two additional checks (43 with all flags): malformed fifth-file YAML prevents launch and preserves earlier text/JSON/properties/INI, then actual SCM edits verify typed values, large integer and decimal-exponent spelling, conditional wildcard updates, Unicode, literal environment references and full SDDL. Rebuild the mock for all five targets. No host reboot occurs.

Pass `-TestEggXML` with alpha 28 alongside TestEggYAML for two additional checks (45 with all flags): malformed sixth-file XML preserves all earlier files, then actual SCM edits verify text/attributes, escaping, Unicode, literal references and full SDDL. Rebuild the mock for all six targets.

For a separate disposable reboot run, pass `-PrepareReboot` with persistent recovery/events enabled and stage `Prepare-Reboot.ps1` and `Resume-Reboot.ps1`. This deliberately retains its own resources, prepares delayed startup/resume tasks and records `reboot_prepared`; it does not itself reboot. Follow [reboot validation](../../docs/REBOOT-VALIDATION.md). Normal runs still clean up immediately.

Alpha 29 `-TestRestoreRunning` requires persistent recovery and adds four checks (49 with all previous flags): actual game-token intent-file denial, running restoration across SCM restart, manual stop across SCM restart, and corrupt-intent startup refusal. The fixture increases only its disposable restart allowance from two to five for these additional startup reservations. With `-PrepareReboot`, it leaves running intent and expects automatic post-boot distinct-account launch; 44 preparation checks are a repeated subset of the combined checks.
