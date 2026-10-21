# Server 2022 alpha 23 approved egg text rewriting

Validation date: October 21, 2026 (America/Phoenix). The exact clean-source archive was transferred over fingerprint-pinned SSH and native JSON/full log and service results were retrieved. Reboot tests remain deferred; Windows was not rebooted. The existing fixture still tests forced termination of its exact disposable daemon as a separate process test.

| Field | Result |
| --- | --- |
| Host | Windows Server 2022 Standard Evaluation x64, build 20348; PowerShell 5.1.20348.558 |
| Clean package source | `e8b6b7c7e209b2ff11700d8a434752f02aaf9d3b` |
| ZIP SHA256 | `5c8ed6caf39deb06864591bd3850aa399afc0fb08316f2644cc1a727ff70ec76` |
| Runtime-test SHA256 | `2f3218d6621b07a78a09aacbf4badffe923154e0233a4e218499028560acb09f` |
| Demo SHA256 | `16939622ee4ac69d7fe6f93264bff112681c6c5b23ca29acb8b5d33845561851` |
| Local suite | 394 cases/subtests, zero skipped; vet and clean release build passed |
| Packaged Server 2022 baseline | 14 cases/subtests, zero failed/skipped; both runner arguments omitted |
| Native baseline UTC | 2026-10-21T16:18:42.4889177Z through 16:18:45.0671546Z |
| Combined service/persistence/text fixture | All 35 checks passed |

[Sanitized JSON results](server2022-alpha23-egg-text-results.json) retain every check and the independent cleanup audit. Seven setup/credential/event-source scripts match the immutable archive bytes. The mock was rebuilt for the new profile field. The [developer fixture](../../tools/service-provisioning-test/README.md) ran with all previous flags plus `-TestEggText`.

## New text checks

The disposable profile enables egg process configuration and approves exactly server.cfg. The loopback mock panel supplies text-prefix replacement entries and supported readiness/stop settings. The daemon uses a dedicated non-administrator SCM identity; the harmless demo uses its separate game identity.

1. With the approved file absent, request game start. Observe the preflight failure in console history and the server remaining offline. No target file is created by the parser.
2. Create the disposable UTF-8 target with CRLF lines, an unmatched comment, old name/port values and inherited game-directory permissions. Snapshot its full SDDL. Start the game, wait for running, and verify the comment is retained, configured name includes Café, port is replaced, output uses LF and the full SDDL remains identical.

The previous 33 checks remain passing with this parser enabled, including installer rollback, credential lifecycle, actual identities, console, separate-account private-file denial, crash/retry budgets, persistent logs, SCM restart, forced daemon termination, corrupt-budget rejection and secret-safe service events. This uses a mock panel and harmless fixture, not a real Windows game or panel deployment.

Local tests additionally validate exact authorization, prefix/overlap order, scalar/legacy forms, malformed/unsupported rules, paths and limits, UTF-8/BOM/NUL handling, cancellation, all-target preflight before any write, protected node/code identities, hardlink/junction/lock rejection and released directory handles. An actual native start fails with a missing required target and succeeds after the file is provisioned and rewritten. Initial harness checks exposed only test-infrastructure issues: the lock test needed to close its own fixture handle before checking directory rename, and an ignored upstream reference download was renamed from .go to .txt so Go did not treat it as a build package. No runtime limit or safety check was relaxed. The final clean build and packaged negative/default-runner checks pass.

## Cleanup and limits

The fixture removes its exact game credential, temporary accounts/rights, service, mock and Event Log source. Independent read-only inspection finds zero matching users, explicit rights, processes, credential tasks, service, mock or source keys. Historical events remain. One loaded profile is retained without forced unload/deletion. Earlier fixture files/profiles and the [alpha 19 disposable-credential limitation](server2022-alpha19-recovery.md) remain as previously documented.

Only the plain-text file parser is implemented in this batch. JSON, YAML, INI, properties, XML, unresolved node templates and conditional text rules remain unsupported. Every target is rendered before writes begin, but writes are in place: I/O failure/cancellation can leave partial edits, and later native preflight failure does not undo edits. There is no atomic batch, rollback journal or hostile filesystem/ACL/mapping-race certification. Installer/interrupted-operation workflows, full egg compatibility, real panel/TLS/game, WFP traffic, broader isolation and reboot/profile recovery remain open. The M3 parser issue and milestone stay open.
