# Server 2022 alpha 50 ZIP extraction verification

Observed October 24, 2026 UTC (October 23 in America/Phoenix). This adds a bounded ZIP subset to the existing panel Extract endpoint; it does not certify a real game or browser workflow. Machine-readable [results](server2022-alpha50-zip-extraction-results.json) preserve exact bindings and the initial helper failure.

## Source and clean artifact

- Runtime source: `0f428edf289837a9a0c7718dbf68eb42066c82a9`; source dirty: false.
- ZIP: `wings-windows-native-0.1.0-alpha.50-windows-amd64.zip`; SHA256 `8df86521e4450666aa894f34c5930711231fea50142bacf6309fff4ba40d1fbb`.
- Daemon SHA256: `51aa8bc19e3eeec078d158a03ef3826977522a20eb33e2ec7dae4d629915a35c`.
- Go build runs the complete suite, vet, and the default-invocation portable host runner: **1,340 local cases/subtests, zero skipped**; portable native runner **14**.
- The exact package passes **14 native** and **133 portable extraction** cases/subtests on Windows Server 2022 build 20348, zero failures/skips. This includes the existing 70 tar.gz checks and 63 ZIP checks. The upstream root/file request fixture is staged beside the portable tests.

## Implemented and tested scope

ZIP Store/Deflate payloads use existing source/path/denylist checks, random exclusive stages and handle-relative no-overwrite publication. Structure is checked before Go ZIP parser allocation: at most 256 entries and 1 MiB central metadata, matching local/central headers and descriptors, contiguous nonoverlapping records and no ambiguous alternate end metadata. ZIP64, encryption, prefixes, special files, unsafe Windows names and unsupported flags/methods are refused. Size/CRC validation completes before final publication. Source limit is 101 MiB; aggregate data is 100 MiB; implicit parents count toward 256 final paths. Existing time/concurrency limits apply.

Focused tests cover Store/Deflate, descriptor and raw stored forms, comments, Unicode/binary/empty entries, ignored read-only metadata, exact limits and overflow, malformed structure/CRC/truncation, conflicting paths, alternate end/ZIP64 metadata and nonempty-stage cancellation/guard-failure cleanup. Existing tar.gz framing, malicious inputs, dynamic policy, sharing/pinning, publication and cleanup tests remain passing.

## Retained service and public HTTPS

Runtime-only upgrade from alpha 49 preserves native configuration bytes and executable SDDL, checks the existing profile and retains one approved demo. Incoming authenticated HTTPS succeeds, operator-managed update remains 200/applied false with no config change, and outgoing real-panel sync returns 204. Protected backups are retained in the operator vault. Config protected DACL is observed; vault backup ACLs are not used as a source-ACL equality baseline.

**Sixteen live extraction groups** pass through the retained non-administrator service and public HTTPS, including tar.gz creation/extraction, both formats' SHA256-verified 9 MiB Unicode binary integrity, repeat preservation, ZIP traversal refusal before publication, tar checksum/link/path refusal, writer locking and exact owned cleanup. **Ten upload** and **ten WSS** regression checks pass. These are operator API/synthetic JWT checks, not panel browser actions.

The first live ZIP fixture had an accidental question mark in its generated filename after PowerShell-to-Python text encoding. The runtime correctly returned 422. The helper also omitted its new ZIP filenames from cleanup. Only developer instrumentation changed: explicit Go Unicode escapes, known-file cleanup entries and a sanitized status field. The known leftover archive and empty fixture directories were removed without recursion; the corrected helper passes with runtime code unchanged. Its separate executable/source hashes are recorded above in the JSON.

Independent audit confirms zero extract/upload/pull fixture roots, zero active pulls, prior install marker/file-manager data intact, one retained profile, remote-pull opt-in retained, config bytes unchanged/protected DACL, and service/demo/tunnel Running. No new reboot or direct game-account output-read claim.

## Remaining work and progress

[Issue #29](https://github.com/SturdyStubs/Wings-for-Windows/issues/29) remains open for broader archive/fetch semantics, progress UI and validation. New destinations only; no merge/overwrite, ZIP creation, other formats, quotas/reservations or durable recovery. Partial publication can retain earlier entries, and crash recovery is not transactional. Downloaded/extracted executable bytes are not approved or run automatically. See [operator limits](../FILE-EXTRACTION.md).

Actual browser Extract, real game installation and hostile isolation certification remain open. Public 9 MiB uploads still return proxy 413, separately from successful retained-service large uploads/public small uploads. No new accepted deliverable is split out: M4 remains 4/9, 44 deliverables overall and all six milestones open. HTTP inventory stays 28/38 with partial semantics, not a completion percentage.
