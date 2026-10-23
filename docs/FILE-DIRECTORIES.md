# Creating directories from the panel

Alpha 40 implements `POST /api/servers/{uuid}/files/create-directory` with the existing Wings request and empty 204 success response:

```json
{"name":"nested/folder","path":"/configs/"}
```

The example creates `configs/nested/folder` under that server's data root, including missing parents. Existing directories succeed without changing their ACLs or contents. A regular file anywhere in the requested directory path returns 400, matching the upstream ENOTDIR case. The panel's existing **Create directory** action can use this route without an extension. Real-panel verification remains open.

## Windows behavior

Only the existing node bearer authorizes this endpoint. Panel-side file permissions and subuser authorization remain the panel's responsibility; this is not a new direct browser or signed-upload endpoint. Unknown/unapproved server UUIDs cannot select arbitrary host roots. The directory is opened relative to the locally registered server data root.

All components are validated before creating anything. Requests reject traversal, empty interior components, drive/UNC/backslash/ADS syntax, trailing dots/spaces, control characters, Windows-invalid characters and reserved device names, including COM/LPT superscript-digit aliases. Leading/trailing forward slashes are normalized as server-relative syntax. A name is required; path may be empty or `/`.

Limits are 64 KiB of JSON, 4,096 UTF-8 bytes in the combined normalized path, 32 components and 255 UTF-16 units per component. These are request bounds, not a guarantee that every underlying filesystem/Windows volume supports every accepted length. Unicode and a path longer than MAX_PATH are covered by local Windows tests.

The current case-insensitive egg denylist is checked for every requested component before mutation and against opened directory paths during traversal. Existing links/reparse points are refused, including aliases back into the same server. Directory handles provide confinement; these checks do not provide process isolation or a complete hostile filesystem-race defense.

New directories inherit the server tree's Windows DACL. The daemon does not grant new principals, rewrite existing ACLs or translate Unix ownership/mode changes. The distinct game identity therefore needs inheritable read/write/traverse permissions on its server tree; managed-node permission provisioning supplies these. Profiles using other ACL arrangements must provide the corresponding permissions themselves.

## Errors and retries

- 400: malformed JSON or an existing non-directory path component.
- 401/404: missing/invalid bearer or unknown local server.
- 403: egg denylist or an observed link/reparse entry.
- 408: request cancelled during traversal; already created parents may remain.
- 413: JSON exceeds 64 KiB.
- 422: invalid/bounded Windows path, filesystem access/sharing error or unsupported creation.
- 500: server root cannot be opened.

A denied or invalid request is preflighted before creating parents. A later I/O error, cancellation, policy change or external filesystem mutation can leave earlier new directories in place. There is no recursive rollback: deleting them could destroy files a game wrote concurrently. Inspect and retry; successful repeat requests preserve existing directories. A disconnected caller may have missed a successful response.

Rename, copy, delete, permissions editing, scoped upload/download, archives, remote fetch, disk enforcement and SFTP remain separate unfinished work. This endpoint increases the partial upstream HTTP inventory to **17 of 38**; it does not complete M4 or certify a real panel/game deployment.
