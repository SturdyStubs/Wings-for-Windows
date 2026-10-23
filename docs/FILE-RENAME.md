# Renaming and moving files from the panel

Alpha 41 adds `PUT /api/servers/{uuid}/files/rename`, using the existing panel **Rename** and **Move** actions. It returns an empty 204 on success:

```json
{
  "root": "/config",
  "files": [
    {"from": "server.cfg", "to": "plugins/server.cfg"},
    {"from": "old-name.cfg", "to": "new-name.cfg"}
  ]
}
```

Paths are relative to `root`. `../` can move to a parent directory while remaining inside the server data root; escaping the root is rejected. Leading forward slashes remain server-relative to the supplied root. Case-only renames and unchanged-name requests are supported. Missing sources are ignored, matching the upstream router's retry behavior.

## Supported scope and bounds

Files and directories can move only within the locally registered server data tree. **Destination parents must already exist**: create them with the directory-creation action first. Unlike upstream's broader filesystem behavior, this implementation does not automatically create missing destination parents. It does not move between servers, volumes or protected game-code storage.

Requests use the existing node bearer; panel/subuser file permissions remain the panel's responsibility. Limit: 1–100 entries and 64 KiB JSON. Each source/destination has a 4,096-byte raw combined bound, at most 64 input components and at most 32 normalized components, with 255 UTF-16 units per name. Windows-invalid names, drive/ADS/backslash syntax, trailing dots/spaces and device aliases are rejected before mutation.

Chains, swaps, duplicate sources/destinations and overlapping parent/child entries in one batch are rejected. Perform dependent operations separately. Requested and resolved canonical paths are checked so aliases cannot introduce a known overlap. Directory scans share a 10,000-entry batch budget and a 32-level relative-depth bound. Unsupported descendant names/types, links/reparse points and files with multiple hardlinks are refused.

## Existing files and permissions

All entries are preflighted before the first move. Both source and destination names are checked against the current egg denylist, including directory descendants at their old and proposed new locations. The source is held by a Windows handle that blocks ordinary deletion/replacement during the operation. The native move uses a destination-directory handle and **does not replace an existing destination**, including one created after preflight. A case-only destination is allowed only when it is the same existing file.

Moves preserve the object and its contents, owner and DACL; they do not grant new principals, apply Unix chmod/chown or re-inherit destination ACLs. The managed per-server identity must retain the necessary Windows delete/move permissions. Locked/read-only files can fail; the daemon does not forcibly unlock them or clear attributes. Source pins can temporarily block conflicting ordinary opens.

## Errors, cancellation and partial completion

- 400: malformed JSON or an existing destination.
- 401/404: unauthorized bearer or unknown local server.
- 403: denylist, links/reparse points or multi-hardlink files.
- 408: cancelled traversal or execution.
- 409: another rename batch is active for this server.
- 413: JSON or directory scan bounds exceeded.
- 422: invalid/overlapping paths, missing destination parents, unsupported types or Windows access/sharing/I/O errors.
- 500: server root cannot be opened.

One rename batch per server can run through this API at a time. Other filesystem operations and game writes do not participate in that lock. **A batch is not a transaction.** A later I/O error, late collision or cancellation can leave earlier moves completed. There is no destructive rollback or durable rename journal. Inspect the current listing before retrying; a disconnected client may have missed success.

Descendant and policy checks are bounded snapshots. Native no-replace protects destination contents, but concurrent ACL, mount/namespace, hardlink or game changes remain outside the full isolation guarantee. The node remains limited to trusted private hosting. This is a partial route implementation, not certification of a live panel/game workflow. Copy, delete, scoped upload/download, SFTP and disk enforcement remain unfinished.

## Contract references

Compared with the pinned Wings `747bc43` router/UnixFS rename implementation and the local Panel 2.0.0-beta.1 `FilesClient`, file model and move dialog. The fixture is `internal/api/testdata/upstream-rename.json`.

The Windows implementation follows [FILE_RENAME_INFORMATION](https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/ntifs/ns-ntifs-_file_rename_information) with replacement disabled and the handle-relative approach in Go 1.26's Windows `Renameat`. Local and Server 2022 checks, rather than the documentation alone, establish this release's verified scope.
