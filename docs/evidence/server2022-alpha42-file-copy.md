# Server 2022 alpha 42 file-copy validation

Validation date: October 22, 2026. The immutable clean-source package was transferred over fingerprint-pinned SSH and its SHA-256 checked before extraction.

| Check | Result |
| --- | --- |
| Host | Microsoft Windows Server 2022 Standard Evaluation, build 20348; PowerShell 5.1.20348.558 |
| Clean package source | `79a0195dc931d60553e207faed66f62bb383d20a` |
| ZIP SHA256 | `123f8cb7739ecc8311ca08e4b69e2e198a119f309ad3ab47fe33f1249233864e` |
| Local suite | 931 cases/subtests, zero skipped; vet and clean release build passed |
| Packaged native VM baseline | 14 cases/subtests, zero failed/skipped; default runner invocation |
| Native baseline UTC | 2026-10-23T06:25:21.7279993Z through 2026-10-23T06:25:24.2473927Z |
| Portable file API suite | 98 cases/subtests: 38 directory, 34 rename and 26 copy; zero failed/skipped |
| Managed-service fixture | 35 checks: 18 baseline, five directory, seven rename and five copy; four cleanup flags true |
| Independent cleanup | Zero fixture accounts, explicit rights, owned processes, credential tasks, service, mock or managed Event Log source |

[Sanitized JSON results](server2022-alpha42-file-copy-results.json) bind source, package, executable and fixture hashes to the checks. Credentials, account SIDs and private paths are excluded. The developer API-test executable and contract sidecars are separate from the runtime ZIP.

## Exercised copy behavior

The 26 new cases/subtests exercise the upstream request contract, extension and compound-extension naming, Unicode and dotfiles, successive numbered copies, empty and 9 MiB binary files, malformed/auth/method/path/type guards, source/destination/parent/stage denylist checks, junctions/hardlinks, all 51 occupied candidates, sparse oversize refusal, ordinary source writer exclusion, post-stage cancellation cleanup, concurrent request refusal, exclusive-stage ownership, short/growing streams, disk-full mapping and late-collision no-replace publication. The earlier 72 directory/rename cases are rerun.

Five new service checks verify anonymous rejection, content-preserving numbered copies, denylist/path/type refusal, real distinct-game-account read/write access to daemon-created copies, and no temporary-stage residue. All 35 focused basic/directory/rename/copy checks pass under the non-administrator service. This uses a harmless executable and loopback mock panel.

## Cleanup and limits

The fixture confirms credential removal, service deletion, mock termination and account/right removal. A separate read-only inspection confirms zero remaining accounts, explicit LSA rights, owned processes, credential tasks, service registrations, mock processes or managed Event Log sources. **1 profile remains retained, 1 loaded**; no forced profile unload or deletion was attempted. Fixture data and logs remain for inspection.

No live panel, real game, public TLS workflow, Windows 2025 host, new reboot, WFP traffic, SFTP or disk-quota enforcement was tested. The older combined 114-check suite was not rerun. Copy handles primary bytes of regular files up to 1 GiB into the same directory. New files inherit the parent DACL; source ACLs, alternate streams and metadata are not cloned. Cancellation and ordinary I/O errors attempt owned-stage cleanup; crashes can retain stages without a durable journal. Bounded policy/namespace snapshots and sharing pins do not establish complete hostile-process or filesystem isolation. HTTP inventory is **19/38 with partial behavior**; milestones and live gates remain open. See [API/operator scope](../FILE-COPY.md).
