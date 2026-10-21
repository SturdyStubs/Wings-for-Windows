# Persistent server console history

Alpha 20 adds opt-in disk storage for game console output and Windows node lifecycle/recovery messages. Recent lines reload into the existing console history API after an SCM service restart. This persists **logs only**: games remain offline until started, and crash-recovery attempt history is still in memory. Reboot validation remains deferred.

## Enable and set retention

Add this top-level object to the node configuration and restart the daemon during maintenance:

```json
"console_logs": {
  "directory": "C:\WingsWindows\logs\console",
  "max_file_bytes": 5242880,
  "max_files": 4
}
```

Use normal JSON Windows escaping as above. Omit the object to disable disk logging. Defaults are 5 MiB per file and four files per server (one active plus three rotated), so up to 20 MiB per UUID. Accepted bounds are 1..64 MiB per file and 1..8 files. The directory must be canonical and local, outside the server data tree; it cannot contain node configuration, TLS files, executables or declared dependencies. UNC, mapped network drives, ambiguous components and observed reparse points are unsupported.

Managed installation requires exactly `InstallationRoot\logs\console`; its protected `logs` ACL is inherited by the new console/UUID directories. Create the initial managed layout with an empty `logs` directory and let the daemon create the console children. Outside managed setup, configure directory ACLs administratively: the daemon needs modify, while game accounts must have no read/write access. The normal run_as launch guard audits the shared log tree recursively and rejects exposed logs. Legacy games sharing the daemon account can access its logs; this feature does not isolate them.

## Files and restart behavior

Each full server UUID owns its directory containing:

```text
console.jsonl       active file
console.1.jsonl     most recent rotated file
console.2.jsonl     older rotation, if configured
.writer.lock       empty writer-ownership file
```

JSONL records contain a UTC `time` and `line`. Encoding preserves embedded control characters as JSON escapes. Files are not plain-text concatenation of untrusted console lines. An exclusive writer lease rejects two daemon instances writing the same UUID. Directory handles prevent observed path replacement; regular-file/single-hardlink checks reject observed file aliases before writes. These guards do not establish safety against concurrent administrative ACL edits or writable mappings.

The worker rotates before a write would exceed the file limit, discarding the oldest owned segment. Removing older configured rotations also applies on open when reducing `max_files`. Files belonging to retired UUIDs and unrelated filenames are not deleted automatically. Retention is a per-UUID bound on owned files, not a node-wide disk quota or protection against game writes elsewhere. Existing files larger than a newly configured limit cause a startup error; archive/migrate them before lowering that limit.

On startup, history loading reads at most the last 1 MiB from each retained file, then keeps at most 1,000 lines/1 MiB of decoded text. Complete malformed records are skipped and counted. A partial final active record is discarded before new records are appended, bounded to a 1 MiB tail. No unbounded corrupt-tail truncation is attempted. Use this directory only for the daemon's owned files.

## Slow disks, errors and durability

A 64-record queue separates disk work from the game stdout reader and console subscribers. Each persisted line is limited to 16 KiB plus a truncation marker; live console output keeps its existing limits and is not shortened by this disk policy. Invalid UTF-8 is normalized for persisted JSON. Flooding a slow disk can overflow the queue: those disk records are dropped and counted while the live console remains available.

Transient sharing conflicts during rotation receive a bounded one-second retry. A persistent write/rotation error disables further disk writes for that store until daemon restart; queued and subsequent disk records are counted as dropped. Existing memory history/console delivery continues. Opening an invalid/locked initial store instead fails node synchronization, with proper cleanup of stores already opened.

`GET /api/servers/<UUID>` includes `native_console_logs`: `enabled`, `persisted` (current-process writes), `restored_lines`, `dropped`, `truncated`, `corrupt_records`, and a generic `error` code on storage failure. It exposes no host path or password. The existing `/logs` response remains unchanged and includes restored lines for that server only.

Graceful shutdown waits up to two seconds for console reader/exit callbacks, then up to two seconds for that server's queued writer. Rotation and successful writer close call file Sync. A timeout/error is reported; shutdown does not wait forever for unresponsive storage. A timed-out worker can retain its handles until completion/process exit. This is not a power-loss durability promise, durable event queue, daemon startup-error log or Windows Event Log integration.

Console output may itself contain game-generated secrets. Storage access is administrative; it does not redact arbitrary game output. Review dropped/error counts before treating the disk history as complete evidence.

## Validation scope

Local fixtures cover restart history, rotation/size/file-count bounds, writer ownership, hardlinks/junctions, corrupt/torn records, invalid UTF-8, record truncation, slow-worker overflow, bounded shutdown, write/close/locked-rotation failures and concurrent enqueue/close. Config and managed-layout tests reject misplaced stores, and identity rules protect logs from game access. The exact alpha 20 package passes 14 native cases and 25 SCM/credential/recovery/logging checks on Server 2022: actual game-token log read denial, two-file/1 MiB rotation limits and restored history after SCM restart. See [evidence and cleanup limits](evidence/server2022-alpha20-console.md). Reboot tests remain deferred.

Opt-in [persistent recovery budgets](RECOVERY-STATE.md) use three separate files in the same private UUID directory. Console rotation/retention does not remove or reconstruct them. Budget snapshots add at most 8 KiB per opted-in UUID plus an empty lease file.
