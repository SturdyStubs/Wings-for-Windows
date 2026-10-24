# Bounded archive extraction

Alpha 48 implements the existing panel/Wings Extract action for a single gzip-compressed tar archive. Alpha 50 also accepts a bounded ZIP subset; alpha 52 accepts plain TAR through the same endpoint:

```http
POST /api/servers/SERVER_UUID/files/decompress
Authorization: Bearer NODE_TOKEN
Content-Type: application/json

{"root":"/target","file":"bundle.tar.gz"}
```

`root` must already exist under an approved server's data root. The archive path is relative to that root; extracted paths are also relative to it. Success returns 204. The input is detected as plain TAR, gzip/tar or ZIP rather than trusted by extension. Pending profiles have no file access. The independent fixture follows reviewed Wings `postServerDecompressFiles` and panel `app/Services/Daemon/FilesClient.php::decompressFile`.

**Existing top-level destinations are refused**, including existing empty directories, files, links and the archive itself if an entry would replace it. Use a destination containing no colliding names. Extraction does not merge trees or overwrite files; this intentionally differs from broader upstream behavior. Archives made by the native Compress action round-trip into a fresh destination.

## Validation before publication

- At most 101 MiB source input, 100 MiB aggregate file data, 102 MiB total decoded tar stream (including headers/padding), 256 returned archive headers and 256 final paths including implicit directories. Independent limits can reject a combination of otherwise valid entries.
- Regular files and directories only. Refuse symbolic/hard links, device/FIFO/global metadata entries and sparse records. Do not apply archive ownership, ACLs, alternate streams, modes, read-only flags or timestamps; new objects inherit destination permissions. Modification times need not match the archive.
- Validate raw entry names as relative Windows-valid UTF-8 paths, with bounded depth/component lengths. Refuse absolute paths, traversal, backslashes, ADS, devices, trailing-dot/space aliases, repeated separators and `./` aliases. Directory headers may have one trailing slash. Case-insensitive duplicate files, inconsistent parent casing, repeated explicit directory headers and file/directory conflicts fail.
- Check requested/canonical source, destination, ancestor and temporary-file denylist policy. Refuse linked/multi-hardlink archive sources and observed reparse parents. Hold a native read-sharing-only archive handle against ordinary conflicting writes/deletion.
- Validate the complete single gzip member, including CRC/length/truncation, tar end markers and bounded zero padding, before publication. Refuse concatenated gzip members, compressed trailing bytes and nonzero trailing tar payload. Go's tar/gzip parsers have their own bounded metadata allocations before subset checks; no fixed 64 KiB total-memory claim is made.

File payloads stream through 64 KiB chunks into exclusive random `.wings-extract-*.tmp` files in the existing destination root. No final directories/files are created while parsing. A malformed later header or bad checksum removes unpublished stages rather than publishing earlier valid payloads.

## Plain TAR (alpha 52)

Uncompressed TAR uses the same TAR parser/header checks, implicit-parent/path limits, source handles, staging and publication as tar.gz. Input is parsed by content; the filename suffix is not trusted. Validate all headers and data lengths, require end markers and allow only bounded zero padding afterward. Refuse concatenated archives/nonzero trailing bytes, malformed header checksums, truncated bodies/footers, unsafe Windows paths, links, sparse/special entries, duplicate aliases and collisions before final publication. Empty archives are refused.

The source cap is 101 MiB, aggregate regular-file data 100 MiB, decoded stream 102 MiB and 256 headers/final paths including implicit parents. Unlike tar.gz or ZIP, plain TAR does not carry a file-payload CRC; valid headers/framing cannot detect arbitrary payload bit changes. Known fixture hashes/content are verified in tests. No new format selection parameter is needed in the panel.

## ZIP subset (alpha 50)

Single-disk ZIP with Store or Deflate payloads, UTF-8/ASCII names, optional comments and ordinary 32-bit data descriptors is supported. A 1 MiB central-directory cap and at most 256 central entries are checked before Go ZIP parser allocation; the shared 256-final-path bound also counts implicit parents. Compressed input remains capped at 101 MiB and expanded file data at 100 MiB aggregate. The 102 MiB decoded-tar-stream limit applies only to tar.gz. ZIP fields cannot increase these limits.

Local headers must match central names, methods and flags; local records and descriptors must occupy contiguous nonoverlapping ranges from byte zero to the central directory. The central directory must immediately precede the final end record and its declared comment. Refuse multi-disk/ZIP64 archives, encryption, unknown flags/methods, self-extracting prefixes, gaps, overlapping records, trailing payload and inconsistent headers/descriptors. Unix special-file modes are refused; archive permissions/read-only flags are ignored. Every file is read to its declared size and CRC-checked before any final output is published; empty directory streams are validated too. Non-UTF-8 legacy names are refused. This deliberately bounded subset does not accept every otherwise legal ZIP.

ZIP and tar.gz use the same source handles, path/denylist checks, flat owned stages, exact-handle cleanup and publication code. Extracting executable bytes does not approve or execute them. ZIP creation, other compression methods, merge/overwrite and durable recovery remain open.

## Publication and partial outcomes

After complete validation, recheck all final policies/top-level collisions and the source. Publish parents before children. Native handle-relative directory creation refuses existing names; held new-directory handles prevent ordinary moves/deletion while children are published. To support Server 2022 while keeping new directories pinned without delete sharing, files are created exclusively at their final names and streamed from their validated stages, flushed and held exclusively until the operation returns. Existing names are never replaced. A new filename may be visible while its data is copied, but the exclusive handle blocks ordinary reads/writes; failure removes the exact owned incomplete output. Every destination parent and staged file is checked again before use. An unexpected late-created child is preserved rather than overwritten.

Publication is **not transactional**. A later I/O, collision, policy, cancellation or shutdown failure may leave earlier newly created files or empty directories. Already published entries are retained; do not attempt recursive rollback where a game may have added data. Reload the listing after failure. Unpublished stages are deleted by their exact owned handles. Cleanup failure returns 500 and can retain a temporary file. Crash/reboot recovery is not durable; interrupted staging or incomplete new final files can remain. Validated stages and outputs can temporarily consume roughly twice the expanded file data (up to 200 MiB, excluding the source archive); no space reservation is made.

Creation and extraction share two global archive slots and one operation per server. A cooperative 60-second context checks parsing, data chunks and publication; response-write timing permits the operation plus a short reply interval. Native I/O can still block independently. This is not an atomic filesystem snapshot or hostile same-account isolation boundary.

Typical statuses: 401 bad node bearer; 404 unavailable archive/parent; 400 malformed format/checksum/truncation; 403 restricted source/destination; 408 cancellation/deadline; 409 destination/per-server conflict; 413 size/count limits; 422 unsupported/unsafe entries or trailing payload; 423 conflicting archive writer; 503 capacity/shutdown; 507 storage exhaustion.

## Verification and remaining work

The focused suite covers the panel contract, native create/extract round trip, Unicode/binary/empty/implicit directories, exact 100 MiB and 256-path boundaries, overflow, unsafe paths and metadata types, corruption/truncation/trailing members, expansion and decoded-stream limits, links, collisions, dynamic policy, nonempty-stage cancellation/shutdown cleanup, pinned new directories and partial publication. The corrected clean release passes 1,187 local cases/subtests, zero skips and vet; Server 2022 passes 14 native, 70 portable extraction and thirteen live create/extract/cleanup groups, plus ten each upload/WSS regressions. Public HTTPS extracts a 9 MiB binary with matching SHA256 and refuses malicious inputs. The initial Server 2022 sharing issue was fixed before deployment; see [exact evidence](evidence/server2022-alpha48-archive-extraction.md). The opt-in [live probe](../tools/live-extract-test/README.md) uses known owned fixture data and nonrecursive cleanup.

RAR/7z, ZIP64/encrypted ZIP/ZIP creation, merge/overwrite, sparse/link extraction, metadata restoration, quotas/reservations, broader remote-fetch policies, progress/cancel UI, durable recovery and actual panel browser Extract certification remain open. The separate Nginx large-upload body limit does not affect this small JSON request; it still affects uploading large archives through public HTTPS. The private/trusted-node limitation remains.


Alpha 49 adds opt-in [guarded HTTPS remote-file pulls](REMOTE-PULLS.md); downloads do not automatically extract or execute content.

Alpha 50 verification: 1,340 local cases/subtests, zero skips and vet; 14 native and 133 portable extraction cases on Server 2022; sixteen live tar.gz/ZIP groups through public HTTPS, including both formats with 9 MiB binary integrity, and ten each upload/WSS regressions. Initial live helper encoding/cleanup failure and corrected rerun are retained in [alpha 50 evidence](evidence/server2022-alpha50-zip-extraction.md); runtime code is unchanged by that instrumentation correction.

Alpha 52 verification: 1,394 local cases/subtests (zero skipped), vet and clean release; 14 native / 150 portable extraction cases on Server 2022. Nineteen real Client/panel-issued-token groups pass, including plain TAR signed upload/extraction, with tar.gz/ZIP/upload/WSS regressions and independent nonrecursive cleanup. [Exact evidence](evidence/server2022-alpha52-plain-tar.md).
