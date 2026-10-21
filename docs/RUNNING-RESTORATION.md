# Opt-in running intent restoration

Alpha 29 adds local profile `restore_running: true`. It is disabled by default and requires private `console_logs` plus enabled `crash_recovery` with `persist_history: true`. Unchanged profiles still start offline after daemon restart/reboot.

```json
"restore_running": true,
"crash_recovery": {
  "enabled": true,
  "persist_history": true,
  "max_restarts": 3,
  "window_seconds": 3600,
  "delay_seconds": 5
}
```

## Power intent and startup

Executed start/restart actions durably record running intent before trying to launch. Stop/kill actions record stopped intent; normal clean game exits also clear intent when their write completes. Node shutdown kills the game tree while preserving its running intent. The record is desired state, not a PID, process snapshot or proof that a launch succeeded. A failed start can retain running intent; issue stop/kill and verify storage status to cancel it. Commands accepted into the asynchronous power queue are not durable until executed.

After initial panel sync, TLS/listener setup and service readiness, the manager considers loaded running intents once. It uses the configured recovery delay and permits at most two concurrent startup-restoration actions. Every automatic launch reserves and syncs one attempt in the existing persistent restart budget before fresh panel sync, profile/egg file checks, credentials, ACL guards and native process creation. Budget exhaustion, suspension, corruption, storage errors or explicit power cancellation prevent automatic launch. A failed automatic attempt requires manual intervention; it is not an unbounded retry loop. Normal crash recovery retains its existing policy.

Explicit power actions received before startup replay suppress that replay for the server. Stops/kills cancel pending restores. Kill cancels launch and kills the current game before waiting for intent storage, so a slow write cannot keep the game tree alive. Durable confirmation can still wait on OS file I/O; a failed/unconfirmed stop intent must be inspected before restarting the daemon. The API exposes safe error status and console messages rather than silently declaring the write durable.

A clean exit's intent write is drained during normal node shutdown. Abrupt failure before a write is committed can retain the previous intent; the intent record and process exit are not a single transaction. This starts a new process, not reattachment to a surviving process or restoration of console pipes/PIDs. Installation/provisioning, backups and transfers are not resumed.

## Private storage and failure behavior

The server's existing private console directory contains `.intent.lock` and two bounded `intent.0.json` / `intent.1.json` snapshots. The single writer pins directories/files, requires regular single-link files, refuses conflicting writers and syncs sequence/checksum/UUID-bound records in place. Each snapshot is capped at 4 KiB. Both slots initialize to stopped before running intent may be recorded. This prevents a missing latest stop slot from appearing to be a legitimate first running snapshot. New or wholly absent state defaults to stopped.

Any malformed, mismatched, inconsistent, non-initial missing slot or torn snapshot fails daemon startup rather than falling back to an older running value. Failed writes are sticky until the store is reopened; no launch is permitted from an unconfirmed intent update. Checksums detect accidental corruption, not malicious administrator edits. For separate identities, keep the private directory inaccessible to game accounts; the fixture verifies that denial. Same-account games share the daemon identity and cannot be denied its access. The existing trusted-node and filesystem/ACL/mapping-race limitations still apply.

Do not hand-edit one slot, remove just one file or copy snapshots between UUIDs. To deliberately reset lost intent to stopped, stop the daemon and remove both snapshots together only after inspecting/backing them up. Re-enabling the option reuses retained intent: stop the server and confirm stopped intent before disabling it if you do not want a later re-enable to use an older running intent. While disabled, power changes do not update these files. No cross-file atomicity with the separate recovery-budget snapshots is claimed; automatic reservations remain conservative.

## Status and operator controls

`GET /api/servers/{uuid}` includes `native_restore` with `enabled`, `desired_running`, `loaded_running` and `pending`, plus `storage_error: "intent_write_failed"` on failure. `loaded_running` is the intent read at this daemon's startup, not a successful-launch flag. `pending` identifies a scheduled startup restore; use the ordinary server state and `native_recovery` for actual state and remaining budget. No new panel endpoint is required.

Automatic service startup is still an operator deployment choice; the installer continues to default to Manual. The disposable reboot fixture uses delayed automatic start and a loopback mock. This is not certification of immediate-start dependency races, real panel/TLS/game behavior, password rotation, power loss or full isolation.

## Verification

Local checks cover durable transitions, exclusivity/link/corruption refusal, deletion of a latest stopped slot, stale exit writes, slow-write emergency kill, readiness gating, fresh suspension checks, budget limits, explicit cancellation, clean exits and manual stops across manager restarts. The service fixture adds actual game-token intent-file denial, SCM running/stopped restoration and corrupt-intent refusal. The exact release passes 584 local cases/subtests, 14 packaged Server 2022 cases, 49 combined service checks and seven actual post-boot checks, including automatic distinct-account launch. Pinned SSH reconnect and unattended/independent cleanup pass. See [exact package/source/VM evidence](evidence/server2022-alpha29-running-restoration.md).
