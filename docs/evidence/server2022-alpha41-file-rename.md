# Server 2022 alpha 41 file rename/move validation

Validation date: October 22, 2026. The immutable clean-source package was transferred over fingerprint-pinned SSH and its SHA-256 checked before extraction. The default packaged native runner, portable directory/rename API suite, non-administrator SCM fixture and independent cleanup audit pass.

| Field | Result |
| --- | --- |
| Host | Microsoft Windows Server 2022 Standard Evaluation, build 20348; PowerShell 5.1.20348.558 |
| Clean package source | `467c17fa7c09aeb5827dabbfe219b683a44253dd` |
| ZIP SHA256 | `2c68d35c8136efd9cdc1d7aa8d26481d4bed9866d81cd2ddc74a3b9f1ee13312` |
| Runtime-test SHA256 | `428f07aab4df4bb585932c92c4424841c8bdec66dcd0a221d6289151212a9251` |
| Demo SHA256 | `d2f76a8aaca0a5c74dcb8d0eb6a224d47b2b55ff6100ee113c70d123c71835bd` |
| Portable API-test SHA256 | `580f2ea0f5c9416e473b432e2fe0706416841aaa7c67d56bce0fc73a4a641c3d` |
| Local suite | 905 cases/subtests, zero skipped; vet and clean release build passed |
| Packaged native VM baseline | 14 cases/subtests, zero failed/skipped; default runner invocation |
| Native baseline UTC | 2026-10-23T05:32:45.3206518Z through 2026-10-23T05:32:47.8663971Z |
| Portable file API suite | All 72 cases/subtests pass: 38 directory cases and 34 rename cases, zero failed/skipped |
| Managed-service fixture | All 30 checks pass: 18 basic, five directory and seven rename checks; four cleanup flags true |
| Independent cleanup | No remaining fixture accounts, explicit rights, owned processes, credential tasks, service, mock or managed Event Log source |

[Sanitized JSON results](server2022-alpha41-file-rename-results.json) record every service check, source/package/fixture hashes and cleanup counts. Node configuration, account SIDs, passwords and private raw output are excluded. The portable API executable and JSON contract sidecars are developer evidence, separate from the runtime ZIP.

## Exercised rename behavior

The 34 rename cases/subtests run without Go on Server 2022. They cover the independently sourced panel/upstream request, two-file batches, missing sources, relative parent moves within the root, case-only names/no-op, directory descendants and Unicode, whole-batch invalid/collision/overlap preflight, source/destination/descendant denylist checks, real junctions/hardlinks, JSON/method/auth/size handling, request and scan bounds, cancellation, ordinary source replacement refusal and a destination created after preparation. The native test also verifies a one-character destination. The earlier 38 directory cases are rerun in the same portable suite.

The seven new SCM checks verify:

1. Anonymous rename returns 401 without changing the source.
2. Two files written by the daemon and distinct game account are renamed with contents preserved.
3. A case-only change is visible in listing and unchanged-name replay succeeds.
4. A non-empty directory moves with its tested directory/file SDDL unchanged.
5. The actual distinct game identity reads the moved daemon-written file and writes a new file inside the moved directory; the API reads it back.
6. An existing destination refuses the entire preflighted batch, preserving its earlier source and destination bytes.
7. A directory move whose new descendant location is denylisted, and a traversal request, are refused with the source intact.

All final checks in this batch pass. The alpha 39 combined 114-check fixture and reboot tests were not rerun; this batch uses the focused 30-check basic/directory/rename SCM fixture.

## Cleanup and limits

The service fixture confirms credential removal, service deletion, mock termination and account/right removal. A separate read-only inspection verifies zero remaining test accounts, explicit LSA rights, owned processes, credential tasks, service registrations, mock processes or managed Event Log sources. **1 profile remains retained, 1 loaded**; no forced profile unload/deletion was attempted. Fixture data, moved folders/files and logs remain for inspection. Earlier fixture retention limits remain unchanged.

No real panel, real game, public TLS workflow, Windows 2025 host, new reboot, WFP traffic, SFTP or disk-quota enforcement was tested. Destination parents must already exist; dependent/overlapping moves are refused. Later I/O/cancellation/late collision can leave earlier moves completed without rollback or a durable journal. Bounded policy/tree snapshots and source pins do not establish full hostile-filesystem or process isolation. HTTP inventory is now **18/38 with partial behavior**; M3/M4 and the live gates remain open. See [API/operator scope](../FILE-RENAME.md).
