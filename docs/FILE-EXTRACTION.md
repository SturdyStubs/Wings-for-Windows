# Bounded tar.gz extraction

Alpha 48 implements the existing panel/Wings Extract action for a single gzip-compressed tar archive:

```http
POST /api/servers/SERVER_UUID/files/decompress
Authorization: Bearer NODE_TOKEN
Content-Type: application/json

{"root":"/target","file":"bundle.tar.gz"}
```

`root` must already exist under an approved server's data root. The archive path is relative to that root; extracted paths are also relative to it. Success returns 204. The input is detected as gzip/tar rather than trusted by extension. Pending profiles have no file access. The independent fixture follows reviewed Wings `postServerDecompressFiles` and panel `app/Services/Daemon/FilesClient.php::decompressFile`.

**Existing top-level destinations are refused**, including existing empty directories, files, links and the archive itself if an entry would replace it. Use a destination containing no colliding names. Extraction does not merge trees or overwrite files; this intentionally differs from broader upstream behavior. Archives made by the native Compress action round-trip into a fresh destination.

## Validation before publication

- At most 101 MiB compressed input, 100 MiB aggregate file data, 102 MiB total decoded tar stream (including headers/padding), 256 returned archive headers and 256 final paths including implicit directories. Independent limits can reject a combination of otherwise valid entries.
- Regular files and directories only. Refuse symbolic/hard links, device/FIFO/global metadata entries and sparse records. Do not apply archive ownership, ACLs, alternate streams, modes, read-only flags or timestamps; new objects inherit destination permissions. Modification times need not match the archive.
- Validate raw entry names as relative Windows-valid UTF-8 paths, with bounded depth/component lengths. Refuse absolute paths, traversal, backslashes, ADS, devices, trailing-dot/space aliases, repeated separators and `./` aliases. Directory headers may have one trailing slash. Case-insensitive duplicate files, inconsistent parent casing, repeated explicit directory headers and file/directory conflicts fail.
- Check requested/canonical source, destination, ancestor and temporary-file denylist policy. Refuse linked/multi-hardlink archive sources and observed reparse parents. Hold a native read-sharing-only archive handle against ordinary conflicting writes/deletion.
- Validate the complete single gzip member, including CRC/length/truncation, tar end markers and bounded zero padding, before publication. Refuse concatenated gzip members, compressed trailing bytes and nonzero trailing tar payload. Go's tar/gzip parsers have their own bounded metadata allocations before subset checks; no fixed 64 KiB total-memory claim is made.

File payloads stream through 64 KiB chunks into exclusive random `.wings-extract-*.tmp` files in the existing destination root. No final directories/files are created while parsing. A malformed later header or bad checksum removes unpublished stages rather than publishing earlier valid payloads.

## Publication and partial outcomes

After complete validation, recheck all final policies/top-level collisions and the source. Publish parents before children. Native handle-relative directory creation refuses existing names; held new-directory handles prevent ordinary moves/deletion while children are published. To support Server 2022 while keeping new directories pinned without delete sharing, files are created exclusively at their final names and streamed from their validated stages, flushed and held exclusively until the operation returns. Existing names are never replaced. A new filename may be visible while its data is copied, but the exclusive handle blocks ordinary reads/writes; failure removes the exact owned incomplete output. Every destination parent and staged file is checked again before use. An unexpected late-created child is preserved rather than overwritten.

Publication is **not transactional**. A later I/O, collision, policy, cancellation or shutdown failure may leave earlier newly created files or empty directories. Already published entries are retained; do not attempt recursive rollback where a game may have added data. Reload the listing after failure. Unpublished stages are deleted by their exact owned handles. Cleanup failure returns 500 and can retain a temporary file. Crash/reboot recovery is not durable; interrupted staging or incomplete new final files can remain. Validated stages and outputs can temporarily consume roughly twice the expanded file data (up to 200 MiB, excluding the source archive); no space reservation is made.

Creation and extraction share two global archive slots and one operation per server. A cooperative 60-second context checks parsing, data chunks and publication; response-write timing permits the operation plus a short reply interval. Native I/O can still block independently. This is not an atomic filesystem snapshot or hostile same-account isolation boundary.

Typical statuses: 401 bad node bearer; 404 unavailable archive/parent; 400 malformed format/checksum/truncation; 403 restricted source/destination; 408 cancellation/deadline; 409 destination/per-server conflict; 413 size/count limits; 422 unsupported/unsafe entries or trailing payload; 423 conflicting archive writer; 503 capacity/shutdown; 507 storage exhaustion.

## Verification and remaining work

The focused suite covers the panel contract, native create/extract round trip, Unicode/binary/empty/implicit directories, exact 100 MiB and 256-path boundaries, overflow, unsafe paths and metadata types, corruption/truncation/trailing members, expansion and decoded-stream limits, links, collisions, dynamic policy, nonempty-stage cancellation/shutdown cleanup, pinned new directories and partial publication. Full clean build and Server 2022 validation are pending; the opt-in [live probe](../tools/live-extract-test/README.md) uses known owned fixture data and nonrecursive cleanup.

Plain tar, ZIP/RAR/7z, merge/overwrite, sparse/link extraction, metadata restoration, quotas/reservations, remote fetch, progress/cancel UI, durable recovery and actual panel browser Extract certification remain open. The separate Nginx large-upload body limit does not affect this small JSON request; it still affects uploading large archives through public HTTPS. The private/trusted-node limitation remains.
