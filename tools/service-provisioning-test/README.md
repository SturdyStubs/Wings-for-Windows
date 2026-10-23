# Developer-only service provisioning fixture

Alpha 41 adds `-TestFileRename`, requiring `-TestFileDirectories`, for seven more checks: bearer rejection, file batch/content preservation, case-only/no-op behavior, directory moves with unchanged DACLs, actual distinct-game-account reads/writes after the move, whole-batch collision preservation, and descendant denylist/traversal rejection. The focused basic-plus-directories-plus-rename fixture has 30 checks. Rebuild the matching demo/mock and use an immutable package. No reboot or live panel/game certification.

Alpha 40 adds independent `-TestFileDirectories` (incompatible with reboot preparation). It adds five checks to either the basic or combined suite: bearer rejection without mutation, nested create/replay/list/write under the non-administrator SCM daemon, denylist preflight without stray parents, invalid/file-collision preservation, and actual distinct-game-account reads/writes through inherited directory DACLs. Rebuild the mock and use the matching alpha 40 demo. This is a loopback mock panel fixture, not live panel certification.

`-TestRCONCredential` adds seven checks after Source RCON: service-account store/inspect/overwrite rules, unconfigured-server rejection, actual vault authentication, inability of the distinct game context to resolve the daemon entry, deliberate replacement, removal preserving the game credential, and no environment fallback. Its extra cleanup flag confirms removal. Failed/uncertain provisioning retains conservative cleanup responsibility before service/account deletion. This cannot prepare a reboot.

`-TestLogFile` adds five checks after `-TestSourceRCON`: stale/stdout readiness exclusion, RCON-driven log readiness/console, rotation/truncation, kill releasing directory pins and safe malformed-text failure while the game remains controllable. The directory move probe validates both absolute paths stay inside the exact disposable data root. This phase cannot prepare a reboot and does not certify a real game.

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

`-TestNativeInstaller` follows the complete RCON-credential fixture. It exercises approved setup/reinstall callbacks, failure, distinct game-account child identity, concurrent action rejection, kill/tree cleanup and timeout using only the packaged harmless demo and loopback panel. No real egg import or game certification is implied.

`-TestPendingCreation` follows native installers: a new mock-assigned UUID stays pending without a data directory, idempotent replay/listing, automatic-start/install rejection, restart discovery, zero-profile bootstrap without callbacks and local-profile restoration handing off to native reinstall. The new UUID is not given a new Windows account/ACL; the final handoff reuses the original approved fixture UUID. Actual panel creation and fresh-account provisioning remain unverified.

`-TestInstallState` follows pending creation: rejects a mock callback, retains success through restart, retries it without changing a marker, checks stale/duplicate gates and game-account read denial, forcibly terminates the daemon during a hanging installer, verifies child cleanup and interrupted-start blocking, reports failure without execution, then tests corrupt receipt startup refusal and handle release. It does not reboot or certify a real panel/game.

`-TestGamePermissions` requires `-TestInstallState` and cannot prepare reboot. It adds seven checks (96 combined): WhatIf, populated older data/log preservation, empty-target replay, a fresh UUID with a third dedicated local account, installer and game-tree identity, cross-server read denial, and populated-target rejection. It invokes the immutable package’s public permission command. Cleanup removes its credential, added batch right and account, restores shared code ACLs and the original configuration, and retains data/logs and any loaded profiles. Identity/rights/process cleanup needs independent auditing of all three SIDs. No real panel or game is certified.

`-TestGameAccount` requires `-TestGamePermissions`. It replaces manual third-account creation with the immutable package’s public command and adds five checks (101 combined): WhatIf without a password, shared operation-lock refusal, account flags/SID/profile output with no configuration/right mutation, existing-name refusal and existing-profile UUID refusal. Later setup/game launch validates that account’s password and identity. Existing three-identity cleanup/audit requirements remain.

`-TestGameFiles` requires `-TestGameAccount`. It adds six checks (107 combined): import WhatIf, rejected manifest/source hash mismatches, verified protected copy, existing-destination refusal and actual game-token dependency write denial. The new UUID’s installer and root/child game processes run the imported executable. Code and source fixtures remain retained for inspection; existing three-identity cleanup applies. No real distribution, game or live panel is certified.

`-TestStageRecovery` requires `-TestGameFiles`. It adds seven checks (114 combined) using a **simulated interrupted-copy tree**, not an actual process crash: inspection/fingerprint, blocked import, WhatIf, maintenance-lock refusal, changed-fingerprint refusal, byte-preserving administrator-only quarantine and refused replay. The ordinary verified import/permission/credential/setup/game handoff then proceeds. Quarantine is retained; no file deletion or real crash/reboot certification is implied.
