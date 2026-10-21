# Server 2022 alpha 21 service diagnostics

Validation date: October 21, 2026 (America/Phoenix). The exact clean-source package was transferred over fingerprint-pinned SSH; the native JSON/full log and service report were retrieved. Reboot tests remain deferred and no reboot was performed.

| Field | Result |
| --- | --- |
| Host | Windows Server 2022 Standard Evaluation x64, build 20348; PowerShell 5.1.20348.558 |
| Clean package source | `56a95e95398c446748d7a672f891935267838acd` |
| ZIP SHA256 | `5690933d0aaeba8e3d5f94b1bd3935edf89885e15aa36306ed91205ee3b8e334` |
| Runtime-test SHA256 | `2f3218d6621b07a78a09aacbf4badffe923154e0233a4e218499028560acb09f` |
| Demo SHA256 | `37ae1e3381b65d221d8c0d4fdb85f1ce0ffe960590c5285f14ac487451d835ab` |
| Local suite | 317 cases/subtests, zero skipped; vet and clean release build passed |
| Packaged Server 2022 baseline | 14 cases/subtests, zero failed/skipped; both runner arguments omitted |
| Native baseline UTC | 2026-10-21T14:13:40.7807881Z through 14:13:43.3798551Z |
| SCM/credential/recovery/logging/events fixture | All 29 checks passed |

[Sanitized JSON results](server2022-alpha21-service-events-results.json) include every service check, executable/archive identity and independent cleanup audit. The six setup/credential scripts and new Event Log registration script match the immutable archive bytes. The mock panel was rebuilt for the current schema. The [developer fixture](../../tools/service-provisioning-test/README.md) ran with `-TestCrashRecovery -TestConsoleLogs -TestServiceEvents`.

## New service checks

1. Begin with no existing source. `Register-EventSource.ps1 -WhatIf` leaves the registry unchanged; registration creates the managed Application source and a second invocation preserves it.
2. Source removal refuses an existing service registration, even while stopped.
3. Under the actual dedicated non-administrator SCM identity, observe Information events 100 (starting), 101 (API listener ready), 102 (stop requested) and 103 (stopped). The fixture still exercises the complete separate-game-account workflow and actual service stop/restart.
4. After stopping the service and removing its disposable game credential, replace only the fixture configuration with malformed JSON containing a known secret sentinel. Starting the service fails and reaches Stopped. Error event 200 includes `stage=configuration` while excluding the sentinel, node token and fixture root path. Restore the original configuration in a finally block.

All previous 25 SCM/credential/recovery/logging checks also pass, including rollback, public credential lifecycle, actual owner identities, game console, crash budget/cancellation, private game-log access denial, bounded rotation and history restored after SCM restart. The test uses a harmless demo and loopback mock panel; it is not a real panel/game deployment.

Local injected-handler tests cover nonfatal missing/failing event sinks, failure-stage allowlisting, omission of raw error details, truthful startup status, current-state interrogation, stop/shutdown/startup cancellation and closed control channels. Real listener fixtures reject invalid config, failed panel sync, missing TLS and occupied ports without announcing readiness, then verify bound HTTP and an actual self-signed HTTPS request. Local packaged negative/default-invocation checks also pass without Go on PATH.

## Cleanup and retained evidence

The fixture removes its exact credential, service registration, accounts/rights, mock and Event Log source. An independent read-only audit finds zero matching users, explicit rights, processes, credential tasks, services, mocks or source keys. One profile remains loaded and is retained; no forced unload/delete occurs. Earlier fixture files/profiles and the alpha 19 disposable-credential limitation remain as [previously recorded](server2022-alpha19-recovery.md).

After removing the source, the independent audit still reads all five historical event IDs from the Application log. `Get-WinEvent -FilterHashtable` resolves registered providers and rejects this now-unregistered source; an XML XPath provider-name filter reads the retained records without reinstalling it. No Application events were cleared.

## Remaining gates

Lifecycle events are best-effort diagnostics, not a durable game/recovery ledger, interrupted-operation journal or abrupt-crash guarantee. Event Log retention/access are controlled by Windows. Broader daemon diagnostics, service auto-restart, persistent state, certificate renewal/upgrade/CI, cold boot/profile lifecycle, sustained multi-server load, actual panel/TLS/game compatibility, WFP traffic and hostile isolation remain open. M3/M6 and their broad tracking issues stay open. No reboot test is counted as passed.
