# Server 2022 alpha 18 credential provisioning evidence

Validation date: October 20, 2026 (America/Phoenix). The native baseline ran at 2026-10-21T05:05:28.2782487Z through 05:05:30.6698437Z; the service fixture followed. The agent used fingerprint-verified SSH, transferred the checksum-verified ZIP and retrieved full baseline JSON/log and the service report. Full SIDs, passwords, tokens and access details are omitted.

| Field | Result |
| --- | --- |
| Host | Windows Server 2022 Standard Evaluation x64, build 20348 |
| PowerShell | 5.1.20348.558 |
| Clean source | `856b0e604503a7fbe9389d433626d4495ba1cdce` |
| ZIP SHA256 | `59e8a92745c262dd947d9e27a14f1cd8d27e87cefcd0ecbf492d1a639c688ad8` |
| Runtime-test SHA256 | `2f3218d6621b07a78a09aacbf4badffe923154e0233a4e218499028560acb09f` |
| Demo SHA256 | `3a4d6e9c7bc0af16de3a77421e5dfb0cc431b7f99cda7fd1835cf0455b2a61a3` |
| Development checks | 231 cases/subtests, zero skipped; vet and clean release build passed |
| Packaged VM native baseline | 14 cases/subtests, zero failed/skipped; both runner arguments omitted |
| VM service/credential fixture | 18 checks passed |

The six setup/credential scripts and modules uploaded for the fixture were byte-for-byte equal to the release ZIP. The retained [developer fixture](../../tools/service-provisioning-test/README.md) uses the public packaged helper; no separate credential-bootstrap worker was substituted. Sanitized [machine-readable results](server2022-alpha18-credential-results.json) retain each check and cleanup outcome.

## Actual credential and service behavior

The fixture first checks registration/start failure rollback and successful managed account/rights/ACL setup. It then invokes Set-ServiceGameCredential.ps1 while SCM is stopped:

- An incorrect service password fails, with no helper task or added daemon batch right remaining.
- An already-held provisioning lock rejects another invocation.
- A preexisting daemon batch right survives helper completion; the fixture removes that explicitly added test right itself.
- Create and inspect return the exact target's metadata from the dedicated daemon account.
- Accidental overwrite is rejected; a different game account/pinned SID is rejected.
- Explicit same-account replacement and inspection succeed. This uses the existing game password; it is not an actual Windows-account password-reset test.
- Exact-target removal and subsequent missing-status inspection succeed at teardown.

After public credential creation, actual SCM launch runs as the non-administrator daemon and launches the separate non-administrator game account. The real API accepts a console command and returns its echo. Both demo root/child SIDs match the game user; SCM stop terminates both. Service restart followed by game start/stop succeeds using the provisioned credential.

## Failure found during development

An unpublished candidate exposed Task Scheduler/CIM errors continuing after a bad-password registration attempt, leaving fixture state for inspection. The retained implementation now sets a local stop preference, explicitly requests terminating CIM/task errors and checks its exact random task name after attempted registration, including partial creation. Four injected registration/cleanup subcases plus their parent test cover absent and partial tasks, preserved preexisting rights and surfaced right-cleanup failure without host mutations.

The agent inspected and removed the failed fixture's exact task, stopped-service registration, temporary accounts and rights before retrying. No unrelated account, task or service was removed. The final checksum/revision above identifies the rebuilt candidate that passed all eighteen checks; the earlier unpublished ZIP is not the validated artifact.

## Cleanup and remaining gates

The helper removed its credential in the daemon context; the fixture removed temporary accounts/rights/tasks/service/mock process. An independent read-only audit confirmed zero matching temporary accounts (including failed development fixtures), owned processes, provisioning tasks, service registrations, mock processes and final-identity explicit rights. One final-fixture daemon profile remains loaded; it was deliberately retained. No forced unload, host reboot or shared-service restart occurred. Fixture files and prior loaded profiles remain on the disposable VM.

This verifies the supported operator credential command in an initialized password-backed profile and an actual SCM demo workflow with a loopback mock panel. It does not certify host reboot/cold-profile availability, crash-safe cleanup, profile teardown, actual account-password rotation, real panel/TLS/game compatibility, WFP traffic/crash behavior, hostile filesystem/loader races or sustained multi-server hosting. M2/M6 and their broad deliverable issues remain open.
