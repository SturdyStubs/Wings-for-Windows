# Server 2022 alpha 22 persistent recovery budgets

Validation date: October 21, 2026 (America/Phoenix). The checksum-verified clean-source package was transferred over fingerprint-pinned SSH and the native JSON/full log and service report were retrieved. Host reboot tests remain deferred; Windows was not rebooted. The fixture deliberately terminated only its exact daemon process as a separate process-crash test.

| Field | Result |
| --- | --- |
| Host | Windows Server 2022 Standard Evaluation x64, build 20348; PowerShell 5.1.20348.558 |
| Clean package source | `4203f22e920da4941b78e6068eb75a937f72644b` |
| ZIP SHA256 | `ef41729502817a86e8c687abd50c0f747adce90ac83480f9a6e3d0907a902b81` |
| Runtime-test SHA256 | `2f3218d6621b07a78a09aacbf4badffe923154e0233a4e218499028560acb09f` |
| Demo SHA256 | `77ef7f8ae7c75b48889647afba9e968a07d5c5f698c2e278024129c4ea828b12` |
| Local suite | 345 cases/subtests, zero skipped; vet and clean release build passed |
| Packaged Server 2022 baseline | 14 cases/subtests, zero failed/skipped; both runner arguments omitted |
| Native baseline UTC | 2026-10-21T15:46:49.8168942Z through 15:46:52.4220084Z |
| SCM/credential/recovery/logging/events/persistence fixture | All 33 checks passed |

[Sanitized JSON results](server2022-alpha22-recovery-state-results.json) retain every check and the independent cleanup audit. Seven setup/credential/event-source scripts match the archive bytes. The mock panel was rebuilt from current source. The [developer fixture](../../tools/service-provisioning-test/README.md) ran with `-TestCrashRecovery -TestConsoleLogs -TestServiceEvents -TestPersistentRecovery`.

## Persistent-state checks

The disposable profile enables a two-attempt, one-hour recovery window with a two-second delay and persist_history. Console/budget files use the existing private logs directory. The daemon runs as a dedicated non-administrator SCM account, and the harmless demo uses a distinct game account provisioned by the public credential helper.

1. The actual game token receives permission denied when opening recovery.1.json, returning only access status. The existing console-log read-denial test also passes independently.
2. Consume both automatic slots using actual game crashes. Stop/restart the service and verify two restored reservations, no pending recovery and an offline game. This replaces the legacy in-memory reset assertion for this opted-in run.
3. Start manually, crash again and wait beyond the retry delay. The restored exhausted budget leaves the game offline and creates no extra automatic attempt. Manual start remains available.
4. Confirm the registered service process executable matches the exact disposable fixture binary, terminate that PID, wait for SCM Stopped and restart the service. Both consumed reservations remain, with no pending timer or automatic game startup. This tests abrupt daemon termination after completed writes, not host power loss or interruption at every write instruction.
5. After stopping the service and removing its disposable game credential, save one snapshot's bytes in memory, write known corruption, and attempt service startup. SCM reaches Stopped instead of Running. Restore the original bytes in finally. No older snapshot is silently accepted to reset the budget.

The combined run has 33 checks: the previous 29, replacing its in-memory reset check with persistent preservation and adding four checks for game-file denial, exhaustion after restoration, forced daemon termination and corrupt-state refusal. Existing installer rollback, credential lifecycle, actual identity/job-tree controls, crash cancellation, bounded logs/history and secret-safe Event Log tests remain passing. This uses a loopback mock panel and harmless demo, not a real panel/game deployment.

Local coverage adds repeated save/reopen, independent writer ownership, malformed/oversize/unknown/checksum/UUID/time/count/sequence rejection, hardlinks, sticky storage errors and bounded close; manager restoration/expiry/future-clock handling; failed-sync lease cleanup; and blocked-write tests proving reservation precedes launch, emergency kill can acquire server state and cancellation prevents a later launch. A write-failure fixture also verifies manual start while automatic recovery remains disabled. Full-suite packaged metadata/default-runner/negative checks pass without Go on PATH.

## Cleanup and limits

The fixture removes its exact game credential, temporary users/rights, service, mock and Event Log source. Independent read-only inspection finds zero matching users, explicit rights, processes, credential tasks, service, mock or source keys. Historical lifecycle/error events remain. One loaded profile is retained without force unload/deletion. Earlier fixture files/profiles and the [alpha 19 disposable-credential limitation](server2022-alpha19-recovery.md) remain as previously documented.

Persistence is opt-in and depends on the private console log store. Reservations use a conservative automatic-start deadline, can remain counted for up to an extra 30 seconds, and are not a full crash ledger. Snapshot Sync does not establish power-loss/storage-hardware durability; initial corruption fails closed. Slow OS writes can delay ordinary power actions while emergency kill remains available. No game/timer auto-restore, interrupted-provisioning journal, cold boot/profile recovery, real panel/TLS/game, actual WFP traffic or hostile-isolation result is claimed. M3/M6 and broad tracking issues remain open.
