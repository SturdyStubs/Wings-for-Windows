# Server 2022 alpha 51 disk measurement verification

Observed October 24, 2026 UTC (October 23 in America/Phoenix). This verifies bounded cached logical data-root usage through the retained non-administrator daemon and public HTTPS. It does not verify quota enforcement or browser dashboard rendering. Exact bindings/results: [JSON evidence](server2022-alpha51-disk-usage-results.json).

## Clean source and package

- Immutable runtime/probe source: `e1d29fb79025f69a873d7a8c75a90b85f4cd4473`; source dirty false.
- Package: `wings-windows-native-0.1.0-alpha.51-windows-amd64.zip`.
- Package SHA256: `838db2abc190a028da306810626173245c40e2f7a580ce24328543abb9d4c771`.
- Daemon SHA256: `0e70a483a7a2b5a4302418caa1252421d00ad89a0fb2b59147868b52be789180`.
- Clean build: **1,360 local cases/subtests**, zero skipped; vet and default-invocation host-runner smoke pass.
- Exact package on Windows Server 2022 build 20348: **14 native baseline**, **19 portable scanner/cache** cases/subtests and **one portable API/console-stats integration** case; zero failures/skips. Portable executable hashes are recorded in JSON.

## Scope exercised

The new meter reports logical file-stream bytes and file/directory counts from approved data roots. Stats requests schedule work and return cached snapshots. Availability, stale/scanning states, timestamps and sanitized failure status distinguish a legitimate zero from unavailable/incomplete data. A failed scan discards its partial total and preserves the last good sample. No quota enforcement is advertised.

Tests exercise binary/hidden/empty files, changed lengths, cache reuse/refresh, last-good retention after failure, sanitized errors, worker-capacity admission, cancellation and bounded shutdown waits, exact entry/depth bounds and overflow, missing/relative roots, real NTFS hardlinks and nonprivileged mount-point junctions at entry/root/ancestor positions, 8 GiB sparse logical size, attribute-only reads with ordinary/exclusive data writers, and pinned directory move refusal/write allowance/handle release. API responses and console `StatsJSON` agree on the measured value.

## Upgrade and live checks

Runtime-only upgrade preserves native configuration bytes and executable SDDL with protected operator-vault backups. Approved demo is gracefully stopped and restored. Authenticated public system/update checks pass, operator-managed updates remain 200/applied false without config changes, and outgoing real-panel sync returns 204. No new configuration fields or opt-in changes are needed; the earlier remote-pull opt-in is retained.

The [opt-in live probe](../../tools/live-disk-test/README.md) performs **six successful check groups** using the actual production cache interval. It obtains a successful baseline, creates only a random owned directory with a 9 MiB ordinary file, a three-byte hidden file and an empty file, and waits for a later fresh sample. Both public `disk_bytes` and native status agree on exactly **9,437,187 extra bytes**, three files and one directory. Deleting only those known files and the empty directory restores the original bytes/counts after refresh. Fixture cleanup is verified. Credentials stay on the node; no config/token/response-body secrets are published.

**Sixteen live tar.gz/ZIP extraction**, **ten upload** and **ten WSS** regression groups pass afterward. The separate public 9 MiB upload still returns proxy 413; the retained service's large-file integrity and public small uploads pass. These probes use operator master requests/synthetic JWTs rather than actual panel browser sessions.

Independent audit finds zero disk/extract/upload/pull fixture roots and zero active pulls, one retained profile, earlier install marker/file-manager data present, protected config DACL and unchanged bytes, disk measurement available with quota flag false, and service/demo/tunnel Running. No actual game/reboot or direct game-account read is claimed.

## Remaining work

Logical data-root sizes exclude NTFS allocation, alternate streams, metadata overhead and protected code/logs outside the root. Scans are not atomic snapshots, can fail on linked/inaccessible/changing entries, and can briefly block directory moves/deletion. Limits: two workers, 100,000 entries, 32 child-directory levels, 64 root/ancestor handles, ten-second cooperative deadline; native I/O can exceed cancellation/shutdown waits. Samples refresh on stats demand after 30 seconds, with five-second failure/capacity backoff.

This advances [M4 issue #31](https://github.com/SturdyStubs/Wings-for-Windows/issues/31) but leaves it open for complete accounting, reservations and quota enforcement across all writers. SFTP, browser validation and hostile/load certification remain open. All six milestones remain open; M4 4/9 and 44 deliverables are unchanged. No route was added: inventory stays 28/38 with partial semantics. See [operator scope](../DISK-USAGE.md).
