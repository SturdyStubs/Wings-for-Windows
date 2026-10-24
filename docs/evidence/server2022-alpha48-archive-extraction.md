# Server 2022 alpha 48 extraction validation

Recorded October 23, 2026 (America/Phoenix; final VM observations are October 24 UTC). This verifies bounded single-member tar.gz extraction into new destinations, not full archive parity.

| Field | Result |
| --- | --- |
| Corrected clean package source | `29dc6152d8b530c646d9e8631d2a8d10cb6995ce` |
| ZIP SHA256 | `b2af8e905c3f94aac14a4d451c610951d51a0775ac34888f9dad4503a89cf85d` |
| Deployed daemon SHA256 | `1f3756fe024803adc8c84574ca30a3a5f3d189644c4c69d9f12bf952652a1ef4` |
| Local suite | 1,187 cases/subtests, zero skipped; vet and clean release pass |
| Packaged native Server 2022 baseline | 14 cases/subtests, zero failed/skipped |
| Final portable extraction API suite | 70 cases/subtests, zero failed/skipped |
| Live extraction/cleanup groups | 13 pass through public HTTPS and retained non-administrator service |
| Live source / compressed archive | 9,437,184 / 9,440,617 bytes; extracted binary SHA256 matches source |
| Upload / WSS regressions | Ten checks each pass; public 9 MiB upload still returns proxy 413 |
| Host | Windows Server 2022 Standard Evaluation, build 20348; PowerShell 5.1.20348.558 |

[Sanitized results](server2022-alpha48-archive-extraction-results.json) bind source/package, daemon/runtime/demo/API executables, request fixture and live probe hashes, UTC timestamps and independent cleanup. The final package was fingerprint-pinned SSH transferred, hash checked before extraction and tested by its default host runner. Portable developer tests and live probes are separate from the ZIP. No GitHub binary release is implied.

## Preliminary failures and correction

The initial clean package (`7a6c665`, SHA256 `bf35480b8cd7fe7be2bb481e594308d4b5a412d4d9d6fe7051e29c43e0647c1a`) passed local tests but **failed five portable Server 2022 assertions** (62 passed). Nested-file publication using the existing native rename helper failed with a sharing violation when the new parent was held without delete sharing. The local Windows host admitted that operation; Server 2022 did not. The service upgrade was withheld and alpha 47 remained running.

The corrected implementation keeps new directories pinned without delete sharing and exclusively creates each final file, streams from a validated read/write stage, flushes it, and removes an owned incomplete output on failure. It does not require the incompatible child rename behavior. New filenames can be visible while copying, with exclusive handles blocking ordinary data access. This uses additional bounded temporary storage and is not atomic batch publication.

Additional cleanup tests initially attempted directory enumeration while the native parent handle was held; that observer itself conflicted with sharing. A clean-build attempt failed those three test/subtest results and emitted no replacement package. The observer was changed to exact-file metadata lookup. The corrected product first passed all 67 preliminary portable checks, then all 70 including incomplete-output cancellation/policy cleanup. A new clean full build and final packaged native/portable checks pass before deployment. Preliminary results and original package/source binding are retained in the sanitized results; failures are not relabeled as successful verification.

## Coverage and deployment

The 70 focused cases/subtests exercise the reviewed `root`/`file` contract, native Compress-to-Extract round trip, Unicode/nested/binary/empty entries, implicit parents, exact 100 MiB and 256-path boundaries, compressed/expanded/decoded/header/parent limits, traversal/Windows aliases, duplicates and file/directory conflicts, links/special/global records, malformed/truncated tar/gzip, CRC errors, missing end markers, trailing members/data, existing destinations, junction targets, denylist/source locks, dynamic policy, nonempty-stage cancellation/shutdown cleanup, directory pinning, late child collisions, retained partial directories and exact incomplete-output cleanup. All prior tests rerun in the full local suite. The older combined mock SCM fixture was not rerun.

The retained service was upgraded from alpha 47 only after final VM checks passed. The demo was stopped gracefully, protected rollback backups retained, executable hash matched, exact SDDL preserved and native configuration kept byte-identical. This is a runtime-only update; existing identities, credentials, profile, data and tunnel remain. The demo resumes running; operator-managed acknowledgement returns 200/applied false and outgoing real-panel sync returns 204.

## Live checks and cleanup

The [opt-in probe](../../tools/live-extract-test/README.md) reads protected credentials only on the node and uses master-authenticated trusted HTTPS without redirect following or TLS bypass. Missing its owned-fixture flag, it exits 2 before reading configuration. A fresh exclusive fixture contains known source/target paths and a retained sentinel. Thirteen groups pass:

1. Existing public archive creation still works.
2. Public extraction returns 204.
3. A Unicode-named 9 MiB random binary extracts with matching SHA256.
4. Empty file/directory and implicit parents are correct.
5. Repeating extraction refuses collisions and preserves extracted data.
6. Traversal is refused before earlier valid payloads are published.
7. Links are refused before publication.
8. Bad gzip checksum is refused before publication.
9. The sentinel outside the extraction destination remains intact, with no escaped output.
10. A conflicting archive writer is refused.
11. The original source binary remains intact.
12. No temporary extraction file remains.
13. Known files and empty owned directories are removed nonrecursively through `os.Root`; unexpected content would be retained and fail cleanup.

Traversal payloads, if a regression occurred, would escape only into another location in the same owned test fixture; no unrelated user files are selected. A separate read-only audit verifies zero extraction/upload probe roots, prior install marker/file-manager data present, one retained profile, service running as `.\WingsRuntime`, and service/demo/tunnel Running. Ten upload and ten synthetic WSS regression checks pass afterward.

## Remaining limits

No actual panel browser Extract, direct distinct-game-account output read, real game, new reboot or full hostile isolation certification is claimed. Existing trees are not merged or overwritten. Plain tar/ZIP/RAR/7z, metadata restoration, remote fetch, progress UI, quotas/reservations, durable recovery and broader adversarial cases remain open. Late failures can retain earlier new entries; crashes can retain temporary files or incomplete final files. Stages plus output can consume roughly 200 MiB excluding the source archive.

Public large uploads still need the dedicated HTTPS Nginx body-limit change/retest; the small JSON Extract request works independently. [Issue #29](https://github.com/SturdyStubs/Wings-for-Windows/issues/29) and all milestones remain open. No new accepted task is split out: M4 remains 4/9, with 44 deliverables overall. HTTP inventory is 25/38 with partial semantics, not a completion percentage. See [operator scope](../FILE-EXTRACTION.md).
