# Server 2022 alpha 47 archive creation validation

Recorded October 23, 2026 (America/Phoenix). This verifies bounded tar.gz creation, not extraction or complete archive/remote-fetch parity.

| Field | Result |
| --- | --- |
| Clean package source | `befa2ee7fa1a60f9319932c92da735fedbf3f48f` |
| ZIP SHA256 | `7fea51c9414354f5d9a6d8a47d84d28132151f5412500e12088c3a3011471802` |
| Deployed daemon SHA256 | `06d12c3bfae3efb82d404cbe5ddba638189dd00aff4fa73c381976c64665ece9` |
| Local suite | 1,117 cases/subtests, zero skipped; vet and clean release pass |
| Packaged native Server 2022 baseline | 14 cases/subtests, zero failed/skipped |
| Portable compress API suite | 38 cases/subtests, zero failed/skipped |
| Live archive groups | 11 pass through retained non-administrator service and public HTTPS |
| Live source binary | 9,437,184 random bytes |
| Public downloaded archive | 9,440,621 bytes; exact archive and contained binary hashes match |
| Live upload regression | All 10 existing bounded checks pass; public large upload remains 413 |
| Live WSS regression | All 10 synthetic origin/token/permission/revocation checks pass |
| Host | Windows Server 2022 Standard Evaluation, build 20348; PowerShell 5.1.20348.558 |

[Sanitized results](server2022-alpha47-archive-creation-results.json) bind the clean source/ZIP, daemon, packaged native/demo and separate portable API executable, request-contract fixture, live probe, timestamps and cleanup. The package was transferred over fingerprint-pinned SSH and hash checked before extraction. Its default host runner and the focused portable API suite pass on the VM without Go installed. Developer probes are separate from the runtime ZIP; no GitHub binary release is implied.

## Regression coverage

The independently sourced `root`/`files` request matches reviewed Wings `postServerCompressFiles` / `CompressFiles` and the panel `FilesClient::compressFiles`. The 38 focused cases/subtests cover metadata and tar round trips, Unicode/nested/empty entries, 9 MiB streaming, exact aggregate 100 MiB input and one-byte overflow using sparse test sources, invalid/overlapping/duplicate/root selections, reserved Windows names, denied descendants/output/staging, actual junctions and multi-hardlinks, conflicting writers, entry/output/capacity bounds, nonempty-stage cancellation/shutdown cleanup, and source-handle pinning against ordinary writes/renames. A policy refresh through `Server.Sync` during staging refuses publication and cleans up.

The full local suite reruns prior runtime/protocol checks. The native baseline and compress tests run from the exact staged package/developer test executable on Server 2022. The older combined mock SCM fixture was not rerun. Extraction and malicious archive parsing are not exercised because that route is still unsupported.

## Retained deployment

The demo was stopped gracefully for the runtime-only alpha 46-to-47 upgrade. Protected operator-vault backups remain. The new executable hash matches the clean ZIP, executable SDDL is unchanged, and the native configuration remains byte-identical. Existing identities, credentials, profile, data and supervised tunnel remain. The demo resumes running; operator-managed update acknowledgement returns 200/applied false, and outgoing real-panel sync returns 204.

## Live creation, download and cleanup

The opt-in [archive probe](../../tools/live-archive-test/README.md) runs on the node and reads protected configuration locally without displaying it. Without its owned-fixture flag it exits 2 before reading configuration. It creates a fresh exclusive `archive-probe-*` directory, known files and empty directories. Compression uses the node bearer over trusted public HTTPS. Download uses a fresh operator-minted synthetic file JWT; neither route follows redirects or bypasses certificate verification.

Eleven groups pass:

1. Public compression returns the expected archive file metadata.
2. A scoped HTTPS download decompresses and parses as the expected tar stream.
3. Unicode binary, empty file and directory entries have the expected relative names/types.
4. The downloaded archive bytes match the created file's SHA256, and its 9 MiB contained binary matches the source SHA256.
5. A second compression request produces a distinct archive and preserves the first archive's bytes.
6. Traversal is refused.
7. Parent/descendant overlapping selection is refused.
8. A real conflicting source writer causes refusal rather than a moving-content archive.
9. The original source binary remains intact.
10. No temporary archive remains after successful/refused requests.
11. Only known files, validated returned archive leaf names and the exclusive empty fixture directories are removed through `os.Root`, without recursive cleanup.

Upload and WSS regressions then pass against alpha 47. A separate read-only audit finds zero archive/upload probe roots, earlier marker/file-manager data still present, one retained profile, service running as `.\WingsRuntime`, and service/demo/tunnel Running. This batch does not add a new direct distinct-game-account archive-read check or a real game.

## Open gates

The public 9 MiB upload regression still returns proxy 413. Archive creation uses a small JSON request and the archive download succeeds; this does not fix the separate upload body-size gate. The existing dedicated HTTPS Nginx `client_max_body_size 101m;` change and retest remain pending.

Actual panel browser Compress, browser-issued download links, real subuser removal, extraction, malicious-archive extraction checks, guarded remote fetch, progress/cancel UI, quotas/reservations, durable crash-stage recovery, an atomic directory snapshot, larger/live maximum-bound operations and broader hostile isolation remain open. Creation serializes portable modes, not Windows ACLs or alternate streams. No new reboot or complete panel/game/service certification is claimed.

[Archive/remote-fetch issue #29](https://github.com/SturdyStubs/Wings-for-Windows/issues/29) and M4 remain open. No new accepted task is split out in this batch; milestone counts stay **M4 4/9**, with 44 deliverables overall. HTTP inventory is **24/38 with partial behavior**, not a completion percentage. See [protocol/limits](../FILE-ARCHIVES.md).
