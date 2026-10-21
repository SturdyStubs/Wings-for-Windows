# Server 2022 alpha 20 persistent console logs

Validation date: October 21, 2026 (America/Phoenix). The agent transferred the checksum-verified package over fingerprint-pinned SSH and retrieved the complete native report/log and service fixture report. No reboot was performed; reboot tests remain deferred by operator instruction.

| Field | Result |
| --- | --- |
| Host | Windows Server 2022 Standard Evaluation x64, build 20348; PowerShell 5.1.20348.558 |
| Clean package source | `da2ae2940c450375e670b767f12dce18bcbebf36` |
| ZIP SHA256 | `fd6094bebb494572eeb6d4cfb2dad7ab7cfee7fa4f3248f1ffe5ce249ec8434d` |
| Runtime-test SHA256 | `2f3218d6621b07a78a09aacbf4badffe923154e0233a4e218499028560acb09f` |
| Demo SHA256 | `d2a5ec5b315f84a175283e686318427d0aef30c5095df7ddb9962f69fd9b7c77` |
| Local suite | 294 cases/subtests, zero skipped; vet and clean release build passed |
| Packaged VM baseline | 14 cases/subtests, zero failed/skipped; both runner arguments omitted |
| Native baseline UTC | 2026-10-21T13:53:48.0515250Z through 13:53:50.6342854Z |
| SCM/credential/recovery/logging fixture | All 25 checks passed |

The six uploaded setup/credential scripts match the archive bytes. The mock panel was rebuilt from current source. The retained [developer fixture](../../tools/service-provisioning-test/README.md) runs with both `-TestCrashRecovery` and `-TestConsoleLogs`. The [sanitized JSON results](server2022-alpha20-console-results.json) retain every check and the independent cleanup audit. Package source/checksum remain unchanged by this documentation and fixture follow-up.

## New logging checks

The harmless demo runs under a distinct game account; the daemon runs under a dedicated non-administrator SCM account. Managed setup uses the private `logs\console` directory with two retained files, each limited to 1 MiB.

1. Send `probe-log-access`. Under the actual game token, opening its own active log returns permission denied. The probe emits only access status, never log contents.
2. Fill the log with 36 batches of 16 commands, waiting for persisted counters to drain between batches. The generated output exceeds 2 MiB. Confirm two retained JSONL files, each at most 1 MiB, with no reported storage error or dropped records.
3. Write a restart marker, stop the SCM service and confirm its complete game tree exits. Restart the service and confirm nonzero restored-history count and the marker in the unchanged `/logs` API. The game remains offline until manually started; the in-memory crash budget resets as documented.

The previous 22 SCM/credential/recovery checks also pass: installer registration/start rollback, rights/ACLs, bad-password cleanup, operation locks, preserved preexisting rights, public credential lifecycle, actual account ownership, console, crash restart/manual cancellation/budget, service stop/restart and game stop. This uses a loopback mock panel, not a real panel installation.

Local coverage additionally exercises malformed/torn records, truncation/invalid UTF-8, hardlinks/junctions, duplicate writers, concurrent close/enqueue, slow-worker overflow, bounded shutdown and storage failures. Local packaged checks cover archive metadata/hashes, omitted PowerShell defaults without Go and negative runner/config cases. The local verifier needed its inherited PowerShell 7 module path removed for Windows PowerShell 5.1 children; the direct VM command already passed. A test-only helper covers this environment boundary without changing global settings.

## Fixture correction and cleanup

The first logging fixture tried 32 long commands in one request and correctly received HTTP 413 from the existing 64 KiB API limit. It had already passed game-token read denial and earlier recovery checks. Reduce the fixture to 16 commands per batch, doubling the number of batches to preserve total output. The full rerun passes all 25 checks against the same runtime artifact; no API limit was relaxed.

Both alpha 20 runs removed their exact credential, temporary accounts/rights/service and mock. An independent read-only audit across both runs found zero matching users, explicit rights, owned processes, credential tasks, service registrations or mock processes. One profile from each run remains marked loaded and is retained; no forced unload, deletion or reboot was attempted. Previous fixture files/profiles and the earlier alpha 19 disposable-credential cleanup limitation remain as [previously documented](server2022-alpha19-recovery.md).

## Remaining gates

Persistent console history does not persist game state, recovery timers/budgets or interrupted operations. Daemon startup-error logging/Event Log integration, daemon/host crash recovery, cold boot/profile lifecycle, real panel/TLS/game validation, actual WFP traffic and hostile isolation remain open. Slow disks can drop records; rotation/close Sync is not a power-loss guarantee. The feature is opt-in. M3, M6 and their broad tracking issues remain open.
