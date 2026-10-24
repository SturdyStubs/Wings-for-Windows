# Cached disk measurement

Alpha 51 replaces the constant `disk_bytes: 0` in approved server utilization with a cached measurement of logical unnamed file-stream sizes below that server's data root. The same value appears in `GET /api/servers`, `GET /api/servers/{uuid}` and WebSocket `stats` events. Pending/unapproved servers still have no accessible data-root measurement. `GET /api/system` advertises `disk_usage_cached`.

The nested `utilization.native_metrics.disk` status includes `bytes`, `files`, `directories` (root excluded), `available`, `scanning`, `stale`, `measured_at`, `last_attempt`, `error`, `scope: data_root_logical_files` and `quota_enforced: false`. `native_metrics.disk_available` means a successful sample exists, including a stale last-good sample; consult `stale`/`error` for freshness. Before the first successful sample, bytes remain zero and available is false. After a successful empty scan, zero with available true is meaningful.

## Scheduling and limits

Stats requests return immediately from memory and may schedule a background scan; they never synchronously walk disk. At most two scans run per manager/node process, and one per approved server. A successful sample is eligible for refresh after 30 seconds; failures and capacity refusal retry after five seconds. Refresh happens only while stats are requested, not on a periodic scan of every idle server. A successful sample may become stale without a refresh while no client polls. Caches reset on daemon restart.

Each scan permits 100,000 child entries, 32 directory levels below the root, 64 root/ancestor handles, bounded 256-entry directory batches and a cooperative ten-second context. Cancellation/time bounds are checked between native operations; a blocked Windows disk call can outlive the deadline. Manager shutdown cancels work and waits up to two seconds per active server before continuing. Interrupted/incomplete scans never publish a partial total; they preserve the last good sample with generic stale/error status, or remain unavailable when no sample exists. No pathnames or raw OS errors are returned.

## Filesystem semantics

Only fixed local drive roots with canonical absolute paths are supported. Root/ancestor and active descendant directories are held without delete sharing; native handle-relative child opens refuse reparse traversal and observed reparse attributes. Regular files with multiple hardlinks are refused rather than counted ambiguously. Missing/unreadable/changing entries or exceeded bounds make the whole sample unavailable/stale. Directory handles permit ordinary writes, but can briefly block directory moves/deletion during a scan. File reads request metadata only and permit read/write/delete sharing; ordinary and exclusive data-writer sharing fixtures are covered.

Measure all regular file entries including hidden files, egg-denylisted names and API staging files. Denylists hide user content, not storage consumption. Directory metadata/NTFS allocation, alternate streams, protected game code outside the data root, console history outside it and other host data are excluded. Sparse/compressed files report logical sizes, which can exceed physical allocation. No content is read or hashed. File/directory counts describe the successful enumeration, not an atomic filesystem snapshot: concurrent writes may change sizes before or after each metadata read. Paths are not a malicious same-account isolation boundary.

## Limits and next work

This is measurement, not quota enforcement. It observes completed HTTP/archive/installer/game writes under the data root when refreshed, but does not reserve space, reject over-quota writes or stop games. The panel's disk allocation is not enforced. A game can fill the volume between samples. Broader accounting, allocated space, alternate streams, quotas across all writers, SFTP and hostile/load certification remain open under M4 issue #31.

The clean release passes 1,360 local cases/subtests, zero skips and vet; Server 2022 passes 14 native, 19 scanner/cache and one API/console-stats case. Six live disk groups and extraction/upload/WSS regressions pass; see [exact evidence](evidence/server2022-alpha51-disk-usage.md). The opt-in developer probe creates a random owned directory, adds exactly 9 MiB plus three bytes in binary/hidden/empty files, waits for the public HTTPS cached total to advance, deletes only known files/empty directory and verifies the original baseline returns. Browser dashboard rendering and a real game installation are separate live gates.
