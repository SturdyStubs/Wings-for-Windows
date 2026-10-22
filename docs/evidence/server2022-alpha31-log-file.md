# Server 2022 alpha 31 game log-file validation

Validation date: October 21, 2026 (America/Phoenix; UTC records are October 22). The exact clean-source archive was transferred over fingerprint-pinned SSH to the disposable Server 2022 VM. Native logs, combined service results and an independent read-only cleanup audit were retrieved.

| Field | Result |
| --- | --- |
| Host | Windows Server 2022 Standard Evaluation x64, build 20348; PowerShell 5.1.20348.558 |
| Clean package source | `9db599ec75406add07f151abf5b7a46151ed6d9c` |
| ZIP SHA256 | `74031556800d9b00560bb15b826811f74dfa8aab407d03f377bbba102acc2597` |
| Runtime-test SHA256 | `2f3218d6621b07a78a09aacbf4badffe923154e0233a4e218499028560acb09f` |
| Demo SHA256 | `48a13d363bdb8494dd78155bda68eb0741d9b1211d5fb6666d07ea07227935c6` |
| Local suite | 654 cases/subtests, zero skipped; vet and clean release build pass |
| Packaged Server 2022 baseline | 14 cases/subtests, zero failed/skipped; default runner invocation |
| Native baseline UTC | 2026-10-22T04:07:07.4803621Z through 2026-10-22T04:07:10.0030515Z |
| Combined service fixture | All 59 checks pass; all five cleanup flags true |
| Independent cleanup audit | Pass; one retained/loaded fixture profile, no forced unload/delete |

[Sanitized JSON results](server2022-alpha31-log-file-results.json) retain every check, source/package binding, fixture hash and cleanup counts. Seven setup/credential/Event Log scripts match the immutable archive. No node configuration, credentials or cleanup state is committed.

## New log-file service checks

The existing 54-check combined fixture is followed by five new checks. The harmless demo still runs under the distinct game SID and accepts commands through the existing authorized API and Source RCON adapter:

1. Place old readiness and stale-marker lines in the approved file before launch. Enable log-only readiness and verify neither old file history nor the demo's stdout readiness marks the game running.
2. Send RCON commands that append fresh readiness and console lines from the actual game account. Verify the server becomes running and `[Game log]` output appears in the API console.
3. Rotate to a new file identity, then visibly truncate it. Verify new rotation/reset lines reach the console.
4. Kill the game and verify reader directory pins release by moving the exact disposable server directory away and back. Both absolute paths are checked to remain under that fixture's data root before moving.
5. Restart with a fresh baseline, establish readiness again, append malformed NUL text and verify safe reader failure status without taking down the running game or exposing malformed file bytes. Emergency kill still works.

Local reader tests also cover missing final files, old partial lines, split UTF-8, BOM/CRLF, partial-line retention, read/line backlog budgets, oversize/invalid text, missing parents, directory targets, hardlinks, junction parents, protected files, cancellation and handle release. Native lifecycle tests cover current egg denylist refusal, protected-node-config refusal, stdout exclusion, combined RCON/log readiness and old process output rejection.

## Cleanup and remaining scope

The independent audit finds zero matching accounts, explicit logon rights, owned processes, credential tasks, service, mock or Event Log source keys. Historical lifecycle/failure events remain. The current fixture `scm-25885956` retains one loaded game profile; no profile was force-unloaded/deleted. Earlier fixtures/profiles and the [alpha 19 disposable-credential limitation](server2022-alpha19-recovery.md) remain documented.

This proves a harmless demo/loopback mock workflow, not an actual game or live panel/TLS workflow. Polling cannot recover overwritten/deleted bytes or reliably detect truncate-and-regrow past the old offset between polls. UTF-16/binary logs, wildcard/multiple logs, missing parent creation, full filesystem-race isolation and private RCON secret provisioning remain unsupported. Protected-file identities are a preflight snapshot. Local disk I/O and log production are not a single transaction; console queues remain bounded rather than lossless.

No VM reboot was performed in this batch. [Alpha 29 reboot evidence](server2022-alpha29-running-restoration.md) remains historical rather than relabeled as alpha 31. M1/M2/M3, Windows control-event adapters and native create/install/reinstall/delete remain open. See [operator setup and limits](../LOG-FILE-CONSOLE.md).
