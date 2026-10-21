# Persistent automatic-restart budgets

Alpha 22 can retain each server's automatic-restart reservations across daemon/service restarts. This prevents service restarts from silently clearing a consumed budget. It does not start games automatically or resume a pending timer.

## Enable

Configure the global private [console log store](CONSOLE-LOGS.md), then opt in per local profile:

```json
"crash_recovery": {
  "enabled": true,
  "persist_history": true,
  "max_restarts": 3,
  "window_seconds": 300,
  "delay_seconds": 5
}
```

`persist_history` defaults false, retaining the previous in-memory behavior. True requires global `console_logs`; configuration rejects a missing store. The budget uses that existing protected per-UUID directory, not the console text itself. Managed installation still requires `InstallationRoot\logs\console`. No additional game-account grants are needed. The complete log tree remains forbidden to game identities and outside their file API roots.

This access protection requires the separate `run_as` identity path. Legacy profiles running as the daemon can access its budgets and credentials; use only trusted code on a private node. Checksums and snapshots do not provide container isolation.

Restart the daemon during maintenance to load profile changes. Keeping `persist_history` true while disabling `enabled` preserves the budget without enabling automatic recovery. Disabling persistence deliberately returns to in-memory operation; reservations made during that period are not subsequently reconstructed. Changing UUID/directory or deleting snapshots is an administrative reset and is not a way to preserve continuity.

## Reservation and restoration

Before an automatic launch, reserve a slot in memory, write a snapshot and complete file Sync. Only a successful write permits normal launch/preflight to proceed. An attempt cancelled after reservation, or one whose preflight fails, still consumes its slot. Manual starts do not reset the budget or consume automatic slots.

Persistent reservation timestamps use the automatic-start deadline, at most 30 seconds ahead of reservation. This conservatively extends retention so a slow write cannot expire a slot before the permitted launch. Future timestamps remain counted after a wall-clock rollback; they are not discarded as invalid. Wall-clock advances can expire reservations. This is a bounded retry policy, not a trusted monotonic clock across restarts.

Startup loads valid reservations and applies the current rolling window. Old reservations expire normally, and lowering max_restarts can leave the budget exhausted. Increasing a window cannot recover timestamps already pruned by an earlier save. A restart leaves games offline and timers cancelled, even if a reservation was pending or a game was running before termination. An operator must start the game; a later crash is eligible only if the remaining budget permits it.

## Files and failures

Each opted-in UUID owns three additional files:

```text
.recovery.lock       exclusive budget writer lease
recovery.0.json      snapshot, at most 4 KiB
recovery.1.json      snapshot, at most 4 KiB
```

Snapshots contain schema version, UUID, sequence, up to ten reservation timestamps and a SHA256 checksum. The checksum detects accidental corruption; it does not authenticate administrative edits. Alternating slots retain the previous generation while updating the next. Each owned file must be a regular single-link file. Retained directory/file handles reject observed links, duplicate writers and replacement of pinned paths. The budget has an independent lease from the console writer, and console rotation does not remove budget files.

Nonempty malformed, oversize, unknown-version, mismatched-UUID/checksum, missing-generation or inconsistent-sequence state fails initial node synchronization. No older-slot fallback silently forgets attempts, and corrupt files are not repaired/deleted automatically. Existing empty initial slots represent an unused budget. Administrative deletion of both snapshots cannot be distinguished from a new installation. Stop the daemon and inspect/archive the exact UUID directory before any deliberate reset; no online reset endpoint is provided.

A write failure disables automatic recovery for that server until daemon restart and reports `storage_error=reservation_write_failed`. The current reservation stays consumed. Live console and manual controls remain available; a corrupt initial store instead prevents the node from becoming ready. Slow OS writes can delay the automatic power action and ordinary queued starts/stops. Disk I/O runs outside the server lock, so emergency kill can cancel the launch without waiting for the write. After an overdue/cancelled write completes, normal context cancellation still prevents launch.

Store close waits at most two seconds for the caller. A blocked write/close can retain handles until I/O finishes or the process exits. Sync completion is required before launch, but no power-loss/storage-hardware guarantee is claimed. Torn records fail closed on the next startup. The store is not a transactional install journal, full crash history, game-state restore, service watchdog or hostile-administrator defense.

## Status and validation

`native_recovery` adds `persistent_history`, `restored_attempts` (timestamps loaded before window pruning), and `storage_error` when a write fails. Existing `attempts_in_window`, `pending`, limits and standard game states remain unchanged. Status reveals no host path, raw storage error or credential. Console history and [service lifecycle events](SERVICE-EVENTS.md) remain separate.

Local fixtures cover repeated save/reopen, independent console/budget ownership, corrupt/torn snapshots, checksums/UUID/version/bounds/sequences, hardlinks, sticky errors, bounded close, restoration/expiry/clock rollback, failed-sync cleanup, reservation-before-launch, cancellation during a blocked save and manual start after write failure. Exact packaged Server 2022 results are recorded in [verification](VERIFICATION.md). Reboot/profile recovery remains explicitly deferred; actual game/panel, WFP, persistent game state and interrupted provisioning remain open.
