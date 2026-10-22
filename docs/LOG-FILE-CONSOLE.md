# Game log-file console and readiness

Alpha 31 adds one optional, locally approved UTF-8 log file per server. It supplements stdout with `[Game log]` console lines. It can also provide readiness for games that do not print a usable startup marker to stdout, including profiles using [Source RCON commands](SOURCE-RCON.md).

```json
"log_file": {
  "path": "logs/server.log",
  "readiness": true
},
"ready_pattern": "^SERVER_READY$"
```

The path is canonical, relative to that server's data root and selected by the local operator. No absolute paths, traversal, backslashes, wildcard patterns, device aliases or NTFS alternate streams are accepted. Parent directories must already exist before launch; the daemon creates neither directories nor the log. The game may create the final file after launch. Configure the game to write that file with read sharing.

`readiness: false` (the default) leaves stdout/egg readiness unchanged while forwarding file output. With `readiness: true`, **only new complete lines from the approved file** can match the existing local `ready_pattern` or egg startup matchers. Stdout is still displayed but cannot mark the server running. Matching uses the original line, before the display prefix. RCON replies never set readiness. Native exit/Job Object state remains authoritative, so a log line cannot resurrect an exited process.

## History, rotation and encoding

Immediately before native launch, preflight snapshots the existing file's end. Existing content is not replayed or matched, including an old incomplete line completed by a later append. This is independent of the daemon's optional persistent console history; restored console history never establishes readiness.

Every 200 ms, the reader opens the approved path and follows appends. File identity changes (replacement/rotation) start at byte zero and discard buffered fragments. A visible size reduction also restarts at zero. A missing final file is retried, including a gap during rotation. Each process generation starts with a fresh snapshot. Offsets are not persisted. No attempt is made to drain an arbitrarily growing log after native process exit; unterminated final lines are not published.

Only newline-terminated UTF-8 text is supported. CRLF and an initial UTF-8 BOM are accepted; characters may span reads. UTF-16/binary/NUL text is refused. Lines are limited to 16 KiB of text; each poll reads at most 64 KiB and returns at most 256 lines. Unconsumed lines remain for later polls; a bounded partial line is retained between polls. Files over 1 TiB are refused before reading. Output still goes through the existing bounded console/history queues; this is not a lossless log shipping service.

A truncate-and-regrow operation that passes the old offset between polls may be indistinguishable from an append. Use append-only logs, rotation with a new file identity, or a fresh per-deployment log when that distinction matters. Changes before launch are treated as baseline content. This polling adapter does not claim to recover overwritten/deleted bytes or prove which process wrote a line.

## Filesystem and failure behavior

Preflight pins the existing local parent directories, rejecting observed junctions/reparse points and remote drives. It retains those pins until process exit or reader failure; games cannot rename/delete those directories while active. The final file uses short-lived shared read handles so append, file rotation and truncation remain possible. Every open requires a regular disk file with exactly one hardlink and no observed reparse point. The selected path must pass the current egg denylist. Node config/TLS/code/dependency paths and their observed identities are excluded. Protected-file identity checks are a preflight snapshot, not certification against later hostile ACL, mapping, identity-reuse or filesystem races. Existing trusted/private-node and same-account restrictions remain.

An unsafe or inaccessible existing path fails launch before the executable starts. At runtime, permission/sharing errors, unsafe replacement, malformed/oversized text or denylist changes disable the reader with `native_log_file.error: "read_failed"` and a fixed safe diagnostic. Partial output from the failing poll is discarded; raw filesystem errors and file contents are not included in that diagnostic. The game is left under native power control. If it was still starting, stdout does not become an implicit readiness fallback. Fix the file and restart the game to attach a new reader.

`GET /api/servers/{uuid}` adds `native_log_file` with `enabled`, `active` and `error`. It exposes neither the local path nor raw read errors. Process exit/kill cancels polling, releases directory pins and prevents old-generation output from reaching a replacement instance. Persistent-history queues and WebSocket permissions retain their existing behavior. File content follows the same console/history visibility rules as stdout; there is no general secret scrubber for game output.

## Verification scope

Local reader/config/native lifecycle tests cover stale history, partial lines, UTF-8/BOM/CRLF, rotation, truncation, backlog budgets, observed links/protected files, cancellation, pin release, stdout exclusion, denylist refusal, error status and combined RCON/log readiness. The separate SCM fixture has an opt-in `-TestLogFile` phase using the harmless demo under its distinct game account. The clean release passes 654 local cases/subtests, 14 packaged native cases and all 59 combined Server 2022 checks, with independent cleanup. See [exact package/source/results](evidence/server2022-alpha31-log-file.md). No actual game or new reboot certification is implied.
