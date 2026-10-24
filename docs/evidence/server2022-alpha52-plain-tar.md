# Server 2022 alpha 52 plain TAR extraction verification

Observed October 24, 2026 UTC (October 23 America/Phoenix). Plain TAR now uses the same guarded Extract action as tar.gz and ZIP, including a real panel-signed upload and Client API extraction. [Machine-readable results](server2022-alpha52-plain-tar-results.json).

## Clean artifact and tests

- Immutable source/probe revision: `a2488ab5ab6c96e5b1ced4bcf3efa6d6c77ba7d7`, source dirty false.
- Package: `wings-windows-native-0.1.0-alpha.52-windows-amd64.zip`.
- Package SHA256: `97cc75cbe25fabcfc7998326e61bcff7fe512a5fe1f06a1e3d1092289421274b`.
- Daemon SHA256: `833832cb294419f77af98cb618a4fe79a5b04bf76210bd14d4999ac75b875f55`.
- Clean Windows build: **1,394 local cases/subtests**, zero skipped, vet and default-invocation host-runner smoke pass.
- Exact package on Server 2022 build 20348: **14 native baseline** and **150 portable extraction** cases/subtests, zero failures/skips. The reviewed upstream root/file fixture is staged with the portable executable. Client probe source/executable hashes are recorded in JSON.

## Implemented scope

The existing upstream/panel `POST files/decompress` root/file request supports plain TAR without a new parameter. Format is selected from content, not suffix. Uncompressed TAR shares the tar.gz reader, validation, owned stages and publication logic. Detect gzip before constructing its decoder; otherwise parse TAR headers directly (ZIP retains its independent preflight).

Require valid TAR headers, complete payload lengths/end markers and bounded zero padding before publication. Refuse concatenated/nonzero trailing archives, truncation, unsafe Windows paths, links/sparse/special entries, aliases/duplicates and collisions. Source cap 101 MiB, aggregate files 100 MiB, decoded TAR 102 MiB and 256 headers/final paths including implicit parents. As before, final publication creates only new destinations and can retain earlier entries after late failure.

Plain TAR has no file-payload CRC. Header/framing validation cannot detect arbitrary payload bit changes; fixtures verify expected contents and hashes. Extracted executable bytes are not approved or run. Existing guarded source handles, parent/path policy, cancellation, staging cleanup and concurrency bounds remain unchanged.

Focused cases add a 9 MiB Unicode/binary plain TAR round trip with a non-TAR filename, empty/implicit directories, repeat preservation, header corruption, truncated payload/footer, concatenated/trailing data, empty archives, traversal/link/alias/conflict refusal, exact 100 MiB and overflow, header/path count bounds and nonempty-stage cancellation cleanup. The old deliberate plain-TAR refusal case is replaced by these positive/negative cases; tar.gz and ZIP regression coverage stays passing.

## Live retained-service and actual panel checks

Runtime-only alpha51-to52 upgrade preserves config bytes and executable SDDL with protected vault backups, retains one approved profile and restores the previously running demo. Public system/capability checks pass; operator-managed update remains 200/applied false without mutation; outgoing real-panel sync returns 204.

The [real Client probe](../../tools/live-client-test/README.md) passes **19 groups** using a temporary user-provided Client key held only in memory. Actual panel-issued WSS auth/stats, Client and WSS command echoes, Client stop/start, file create/edit/read/rename/copy, tar.gz compress/move/extract, signed 64 KiB binary upload/download with integrity/replay refusal, and known-file cleanup all pass. The new group requests a second real panel upload link, uploads a plain TAR, extracts it through Client API and reads the expected `plain.txt` content.

**Sixteen tar.gz/ZIP live groups** and **ten each upload/WSS regressions** also pass. Those regression helpers use operator master requests/synthetic JWTs and remain distinguished from real Client/panel-issued-token checks. Public 9 MiB uploads still return proxy 413; retained-service large uploads and public small uploads pass.

The Client helper deletes only known fixture files and validates empty child directories. On-node cleanup independently validates the exact random fixture and types, then uses nonrecursive `Directory.Delete(path, false)` for only three empty directories. Final audit: zero client/disk/extract/upload/pull fixture roots and active pulls, one profile, prior marker/file-manager data intact, native config unchanged/protected, exact daemon hash, demo/service/tunnel Running, unsuspended. No keys, node tokens, JWTs, signed URLs or raw responses are saved.

## Remaining gates

Merge/overwrite, RAR/7z/other formats, ZIP creation, quotas/reservations, durable recovery, browser rendering, actual subuser removal and real-game certification remain open. Earlier real Application API lifecycle evidence is [alpha51](server2022-alpha51-real-panel-apis.md), not relabeled as rerun on this daemon. All milestones remain open and counts stay unchanged (M4 4/9, 44 deliverables); this advances broad issue #29 without closing it. Route inventory remains 28/38 with partial semantics. [Protocol and limits](../FILE-EXTRACTION.md).
