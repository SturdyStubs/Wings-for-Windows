# Copying files from the panel

Alpha 42 adds the existing `POST /api/servers/{uuid}/files/copy` contract. Send `{"location":"/config/source.txt"}`; success is an empty 204. The panel's existing Copy action can use it without an extension. Only regular files are supported; directories and missing sources return 404.

Copies stay in the source directory. Names follow the pinned upstream code: `source copy.txt`, `source copy 1.txt` through `source copy 50.txt`. Compound `.tar.gz` extensions stay together, and dotfiles follow the same extension rule (`.env` becomes ` copy.env`). Existing files, directories, case aliases and reparse entries occupy a candidate name and are never overwritten. Unlike upstream's timestamp fallback, exhaustion of all 51 names returns 409; no Windows-invalid timestamp name is generated.

## Bounds and path policy

The existing node bearer and locally registered server root remain required. JSON is capped at 64 KiB. Source paths use the strict directory-creation validator: 4,096 UTF-8 bytes, 32 components and 255 UTF-16 units per component; traversal, drive/backslash/ADS syntax, device aliases and ambiguous Windows names are rejected. Generated copy names must fit too. Parent directories must already exist.

Both requested/resolved source paths and candidate destinations are checked against the current egg denylist. Observed directory links/reparse points and multi-hardlink source files are refused. An internal staging name blocked by the denylist also refuses the operation. A locked/writable-open source can fail; there is no sharing fallback.

**Sources are limited to 1 GiB per copy.** Bytes stream in 64 KiB chunks, including binary data; the text contents endpoint's separate 8 MiB limit does not restrict the copy itself. This is a per-operation bound, **not panel disk-quota enforcement or space reservation**. Disk accounting/enforcement remains unfinished. Disk-full errors are reported as 507; other Windows I/O/sharing errors return 422.

## Staging, publication and failure

The source handle permits ordinary reads while refusing ordinary writes/deletion during the copy. A new exclusive, non-shared `.wings-copy-<random>.tmp` sibling receives the bytes. A new stage inherits the directory DACL; it cannot be ordinarily opened by other readers/writers while this handle is held. The daemon verifies the expected length/EOF, flushes it, rechecks the parent/policy and publishes through the native handle-relative no-replace move. Late destination collisions advance to the next bounded candidate without recopying or replacing existing contents.

The copy is a new object with inherited parent permissions and the daemon's ownership, not a clone of the source's owner, DACL, timestamps, attributes, alternate streams or extended metadata. Its primary byte stream is copied. The distinct managed game identity must have the corresponding inherited access.

Before publication, ordinary failures/cancellation attempt deletion of only the owned stage **by handle**, followed by closing that handle. The original and existing destinations are never removed. Cleanup failure reports 500 and can leave an owned temporary file. A daemon/host crash can also retain a stage; there is no durable copy journal or automatic orphan sweep. Inspect retained stages separately rather than deleting arbitrary matching names.

One copy per server runs through this API at a time (409 for another copy). Other writes/renames/game operations do not share that lock. Cancellation is checked between stream chunks and before publication; it cannot instantly interrupt a blocked Windows I/O or flush. A caller that disconnects after publication may miss success; retrying can create another numbered copy.

Source sharing and policy/parent checks do not establish a hostile filesystem/mapping/ACL race guarantee. The node remains restricted to trusted private hosting. Real-panel/game, quota and full isolation gates remain open.

## Verification references

Contract compared directly with pinned Wings `747bc43` `postServerCopyFile`, `Filesystem.Copy/findCopySuffix` and Panel 2.0.0-beta.1 `FilesClient.copyFile`. Fixture: `internal/api/testdata/upstream-copy.json`. Local and Server 2022 evidence distinguishes this bounded implementation from full upstream file parity.
