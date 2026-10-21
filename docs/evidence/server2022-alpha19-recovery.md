# Server 2022 alpha 19 bounded crash recovery

Validation date: October 20, 2026 (America/Phoenix). The agent transferred the verified archive over fingerprint-pinned SSH and retrieved native JSON/log and service fixture results. Reboot tests were explicitly deferred by operator instruction; the host was never rebooted.

| Field | Result |
| --- | --- |
| Host | Windows Server 2022 Standard Evaluation x64, build 20348; PowerShell 5.1.20348.558 |
| Clean package source | `6e3a6d2f216f40a610c91c51600b7b5daa4c9c5e` |
| ZIP SHA256 | `6f995f1f779515908c8d23ea6fde0914d5e61130a2627d412597d4448d78804d` |
| Runtime-test SHA256 | `2f3218d6621b07a78a09aacbf4badffe923154e0233a4e218499028560acb09f` |
| Demo SHA256 | `1cea78796c1f99ac7aefb0b4c6e09bc8e05938afabc923bd784e19e78244634a` |
| Local suite | 258 cases/subtests, zero skipped; vet and clean release build passed |
| VM native baseline | 14 cases/subtests, zero failed/skipped; both runner arguments omitted |
| Native baseline UTC | 2026-10-21T05:16:27.4813689Z through 05:16:30.2305306Z |
| SCM/credential/recovery fixture | 22 checks passed |

The uploaded six setup/credential files match the archive bytes. The mock panel was rebuilt from current source so its strict config loader recognizes crash_recovery. The [retained fixture](../../tools/service-provisioning-test/README.md) runs with `-TestCrashRecovery`. [Machine-readable results](server2022-alpha19-recovery-results.json) include every check and the independent audit.

## New recovery checks

A disposable profile enables two automatic attempts in a 60-second window with a two-second delay. The daemon runs as a dedicated non-administrator SCM account; the harmless demo root/child run as a separate game account provisioned by the public credential helper.

1. Crash the demo with exit code 23 through its console. Observe pending recovery and a fresh running process, one consumed attempt, and termination of both old game processes.
2. Crash again, observe the delay, issue manual kill, then wait beyond that delay. Confirm offline state, no pending restart and no additional consumed attempt.
3. Start manually, crash and recover using the second attempt, then crash again. Wait beyond the delay and confirm the retry budget leaves the game offline, with two consumed attempts and no timer. Manual start still works.
4. Stop/restart the SCM service, without rebooting Windows. Confirm the explicitly in-memory recovery budget resets to zero and games remain offline until a manual start. This is a scope check, not persistent recovery certification.

The previous eighteen account/credential/SCM checks also pass, including registration/start rollback, bad-password cleanup, operation locking, preserved existing rights, create/inspect/replace/remove, actual owner SIDs, console, service stop killing the complete game tree and restart/start/stop.

Local coverage additionally tests startup crash loops, clean/disabled exits, queued/manual stop/kill, suspension/shutdown cancellation, kill while a recovery launch is blocked fetching panel settings, fresh-panel suspension rejection, preflight failure and rolling attempt expiry. One early active-launch test checked lock release immediately after cancellation; it was corrected to wait for launch unwind before asserting release. The release suite passes all 258 cases/subtests.

## Harness failure and cleanup limits

The first VM fixture attempt used a stale mock binary, which rejected the new config field before service readiness. That attempt exposed an error-cleanup ordering defect in the developer fixture: it deleted service registration before trying the public credential-removal command, which requires the service to exist. Its temporary accounts were already deleted, so the disposable credential remains in that deleted account's retained loaded profile. No real account password or node credential was used. The profile was not force-deleted or unloaded.

The fixture now removes credentials before service registration and retains accounts/rights when credential/service cleanup remains incomplete. These are developer-harness corrections; the tested production ZIP source/checksum above is unchanged. The final successful run removed its exact credential, temporary accounts/rights/tasks/service and mock process. A separate audit confirms no matching accounts/processes/rights/tasks/services/mocks remain across both runs. One final-fixture profile and one failed-fixture profile remain marked loaded. Other previous fixture files/profiles also remain on this disposable VM.

## Gates still open

This proves a bounded native demo recovery workflow under SCM with initialized credential profiles and a loopback mock panel. It does not establish real panel/TLS/game compatibility, a persistent crash ledger/log store, daemon or host reboot recovery, profile teardown, interrupted-operation recovery, actual WFP traffic filtering or hostile multi-tenant isolation. The restart feature is disabled by default. M3's broad persistent-state/recovery issue stays open; M2/M6 also remain open.
