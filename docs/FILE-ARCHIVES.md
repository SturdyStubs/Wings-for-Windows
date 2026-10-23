# Bounded tar.gz archive creation

Alpha 47 adds the existing panel/Wings Compress action:

```http
POST /api/servers/SERVER_UUID/files/compress
Authorization: Bearer NODE_TOKEN
Content-Type: application/json

{"root":"/config","files":["settings.json","nested-folder"]}
```

All selected paths are relative to `root`. Empty root or `/` means the approved server's data root. Select regular files or directories; nested selections retain their relative paths in the archive. The output is a new `archive-DATE-RANDOM.tar.gz` in `root`. HTTP 200 returns the normal file object (`name`, `mode`, `mode_bits`, `size`, `file`, `directory`, `symlink`, `mime`, `created`, `modified`) with MIME `application/tar+gzip`. It can be downloaded using the existing scoped download route. There is no upload body, so the pending public upload-size proxy limit does not affect this operation.

The independent request fixture follows reviewed upstream `router/router_server_files.go::postServerCompressFiles`, `server/filesystem/compress.go::CompressFiles`, and panel `app/Services/Daemon/FilesClient.php::compressFiles` / `app/Actions/Files/CompressFiles.php`. Node master authentication and an approved local profile remain required; no automatic access to pending profiles is added.

## Bounds and refusal behavior

- 1–100 selected paths, at most **256 total entries including directories**, **100 MiB aggregate uncompressed file data**, and **101 MiB compressed output**. No partial archive is published when a bound fails.
- Two concurrent creation jobs globally and one per server. Excess capacity returns 503; another archive on the same server returns 409.
- A cooperative 60-second operation context with checks during traversal, reads, writes and publication. The response write deadline allows that work plus a short response interval. Native I/O can still block independently; this is not a hard process-kill guarantee.
- Existing destination root/parents only; bounded Windows-valid UTF-8 names and directory depth. Refuse traversal, drive/UNC/ADS/device aliases, root selections, duplicate or overlapping selections, and invalid nested names. Selecting a parent already includes its scanned descendants.
- Requested and canonical ancestor/entry/output denylist checks. Restricted descendants fail the entire request rather than silently producing an incomplete permitted-only archive. Observed links/reparse entries and multi-hardlink regular files are refused.
- A bounded preflight holds native source handles with read sharing only, blocking ordinary conflicting writes, deletion and renaming of pinned files. Existing conflicting writers return 423. Source identity and current policy are rechecked between chunks and before publication.

Directories are enumerated in bounded batches. This is not an atomic filesystem snapshot: a game can add children while enumeration proceeds, and children arriving after a directory was scanned are not guaranteed inclusion. File contents that pass admission are read from the retained handles. The private/trusted-node limitation and broader adversarial isolation gate remain.

## Format, publication and cleanup

Go's tar/gzip writers stream bytes without reading the whole input into memory. Tar/PAX handles Unicode and supported long paths; entries are relative regular files/directories only. Modification times are preserved. Portable modes are 0644 for files and 0755 for directories. Windows ACLs, alternate streams, extended attributes, host usernames/groups and executable identity are not serialized or restored. No symlink/hardlink records are generated.

Creation writes into an exclusive random `.wings-archive-*.tmp` in the checked destination parent, with inherited Windows permissions. After tar/gzip completion and flush, final source/policy/parent checks precede native handle-relative no-replace publication. A fresh random final filename prevents normal repeat collisions; a late collision still cannot overwrite an existing object.

An ordinary error, cancellation, shutdown or policy change removes the unpublished temporary file by its exact owned handle. No pathname-recursive cleanup is used. Cleanup failure returns 500 and can leave the owned stage. A daemon/host crash can leave temporary archives; there is no durable recovery journal. Once publication succeeds, a lost HTTP response can still leave a valid archive. Retrying produces a separate new archive.

Typical statuses: 401 missing/bad node bearer; 404 missing source/parent; 403 restricted or linked source; 408 cancellation/deadline; 409 per-server conflict/destination collision; 413 entry/size bounds; 422 invalid selections; 423 locked/unavailable source; 503 capacity/shutdown; 507 storage exhaustion.

## Remaining work and verification

Archive extraction is still unsupported (501), including malicious-archive validation. ZIP/RAR/7z creation, remote pull, progress events, durable stage recovery, compression tuning, storage quotas/reservations and actual browser Compress certification remain open. A 100 MiB input bound does not enforce the panel's disk allocation. Master-authenticated source-level/API tests do not prove a real browser session.

The focused suite covers the independent request fixture, metadata and tar round trips, Unicode/nested/empty/large files, exact aggregate size and one-byte-overflow, malformed selections, links/hardlinks, existing writers, entry/output limits, dynamic policy, nonempty stage cancellation/shutdown cleanup, pinning and capacity. The clean build passes 1,117 local cases/subtests with zero skipped and vet; Server 2022 passes 14 native and 38 portable compress cases, eleven live archive checks and ten each upload/WSS regressions. Public HTTPS creates and downloads a 9 MiB random binary archive with matching hashes. Actual browser Compress remains unverified. See [exact evidence](evidence/server2022-alpha47-archive-creation.md). The opt-in [live archive probe](../tools/live-archive-test/README.md) uses a fresh owned directory and explicit nonrecursive cleanup.
