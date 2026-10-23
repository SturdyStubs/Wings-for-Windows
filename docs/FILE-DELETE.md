# Bounded Windows file and directory deletion

Alpha 44 supports the panel/upstream contract:

```http
POST /api/servers/{uuid}/files/delete
Authorization: Bearer NODE_TOKEN
Content-Type: application/json

{"root":"/config","files":["old.cfg","unused-folder"]}
```

Success returns 204 without a body. Files and directory trees are permanently deleted; there is no recycle bin, quarantine, restore, journal or transactional rollback. This is file-manager deletion, not deletion of a server record, account or entire server root.

The independent fixture follows upstream `router/router_server_files.go::postServerDeleteFiles` at reviewed revision `747bc4393daf5d95fdf61d76382f5e23749cffe9` and panel `app/Services/Daemon/FilesClient.php::deleteFiles`. Existing node authentication and manual-profile approval apply.

## Bounds and preflight

Requests contain 1–100 targets and one JSON object up to 64 KiB. Paths use forward slashes, at most 4,096 bytes/32 components with Windows-valid names. Traversal, root deletion, devices, ADS, backslashes, trailing-dot/space aliases, duplicate/overlapping selections and resolved alias overlaps are refused. Missing entries and missing parents are ignored for replay, provided the requested path passes validation/policy.

The entire batch is scanned before the first delete, with a **1,024 total file/directory entry** budget and bounded depth. Every requested/canonical ancestor and descendant is checked against current egg denylist policy. Observed junctions, symbolic links, other reparse points, multi-hardlink files and unsupported entry types are rejected. They are not silently unlinked. Existing parent paths must be ordinary checked directories.

Each existing entry is opened relative to a checked parent with DELETE/read-data/read-attributes access and no delete sharing. This pins the exact object against ordinary replacement and makes ordinary conflicting open handles fail before mutation. Directory contents are read through the pinned directory handle. One delete batch per server is accepted; other file operations may still run and can conflict.

## Mutation and partial outcomes

After full preflight, current handle-derived canonical paths and policies are rechecked. Children are marked for native deletion and closed before parents. The daemon never uses recursive path deletion or a second directory scan to chase new children. If a new unscanned child appears after preparation, deleting the now-nonempty parent fails and preserves that child. Earlier scanned children may already be gone.

Late I/O, readonly attributes, permission changes, policy refresh, cancellation, disconnect or a crash can leave earlier deletions completed. Preflight is not rollback, and there is no durable acknowledgement journal. Do not blindly retry an uncertain result without refreshing the listing. The daemon does not change read-only attributes, take ownership, bypass sharing checks, force-close another process's files or remove a running game account/profile.

Typical responses: malformed JSON 400; invalid/count/overlap paths 422; denied paths/links/access 403; conflicting delete batch or nonempty late directory 409; ordinary sharing locks 423; scan budget 413; cancellation 408. Later-stage errors explicitly say earlier deletions may have completed. Generic errors do not echo private paths or request values. Some readonly/native errors remain a generic 422. Non-Windows builds have no destructive fallback.

## Verification and limits

Focused tests cover the independent panel fixture, nested/empty/Unicode/case names, missing replay, body/path/count/overlap/auth/method guards, whole-batch source/ancestor/descendant denylist refusal, real junctions and hardlinks, exact entry budget and overflow, deep trees, locked/read-only files, source replacement refusal, live policy refresh, preflight and mid-batch cancellation, delete-lock contention and a late unscanned child. Source/destination test data is created in dedicated temporary roots.

These are bounded NTFS checks for the project's trusted private-node model. Held object handles, path snapshots and policy checks are not proof against every hostile namespace/ACL race, writable memory mapping, filesystem type, alias or privileged process. They do not provide a process/network sandbox. Browser file actions and real-game/operator deletion workflows require separate live validation; server-record deletion, SFTP, disk quotas and full file-management parity remain open.
