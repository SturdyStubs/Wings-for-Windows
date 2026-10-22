# Recover installer results without rerunning setup

Alpha 35 adds opt-in durable installation receipts and a native node-authenticated reconciliation route. This preserves a completed setup result when its panel callback fails. Retrying notification **does not execute the installer**, repair files or start the game.

Enable it in an approved local profile, with private console storage already configured:

```json
"installer": {
  "executable": "C:\\WingsWindows\\bin\\wings-windows-demo.exe",
  "arguments": ["install", "success"],
  "timeout_seconds": 600,
  "memory_mb": 128,
  "persist_state": true
}
```

The node must have `console_logs.directory` outside server data, normally `C:\WingsWindows\logs\console` with the managed private logs permissions. Restart after editing profiles. The default `persist_state: false` retains the previous one-shot, in-memory behavior. Pending unapproved server records do not get receipt stores or execution approval.

## What is saved

Each configured server owns a separate `.install.lock` and two bounded `install.0.json` / `install.1.json` snapshots under its private console directory. Each snapshot contains the server UUID, monotonic sequence, SHA256 checksum and latest attempt's ID, phase and successful/reinstall booleans. It contains no token, password, executable arguments or console output. Checksums detect accidental damage; protected filesystem permissions remain the security boundary.

The daemon synchronizes each write before moving on:

1. Save `running` with a fresh attempt UUID before any setup execution.
2. Save `result` after setup has a known success/failure outcome and before its callback.
3. Send the normal Wings completion callback with the saved booleans.
4. Save `acknowledged` after the panel accepts it.

Only the latest attempt is retained, not a historical audit journal. A new installation cannot overwrite a running/interrupted attempt or an unacknowledged result. Once acknowledged, another explicit install/reinstall can create a new attempt.

There is **no automatic retry** during operation or startup. A received callback and its local acknowledgement are not a distributed transaction: if the daemon dies or acknowledgement storage fails after delivery, the panel might have received it while the local result still appears pending. Repeating a notification can repeat panel-side effects. Inspect panel state before retrying an uncertain delivery.

## Inspect and retry a completed result

`GET /api/servers/{uuid}` adds these fields inside `native_install`:

- `persistent`, `attempt_id` and `receipt_phase` (`idle`, `running`, `result`, `acknowledged`).
- `pending_notification`, `storage_error`, existing local `successful`, `stage` and `callback_error`.
- On restart, a saved result appears as `stage: awaiting_notification`. A saved acknowledgement restores local complete/failed state without notifying again.

Use the same incoming node bearer as other administrative daemon requests:

```http
POST /api/servers/FULL_SERVER_UUID/install/reconcile
Authorization: Bearer NODE_TOKEN
Content-Type: application/json

{"attempt_id":"ATTEMPT_UUID_FROM_GET","action":"notify"}
```

The route returns 204 after successful delivery and durable acknowledgement. It requires `persist_state`; disabled returns 501. Malformed JSON returns 400, invalid attempt/action 422, stale attempts/no pending result/busy/shutdown 409, callback failure 502 and storage failure 503. Assignment to a local approved server is required. There is no anonymous or browser-JWT retry command, no panel UI button and no upstream Wings equivalent for this extra route.

The request uses a five-second callback context and shares the per-server power lock. Other install/power/command requests cannot run during reconciliation; kill/shutdown can cancel its network work. It performs one callback attempt and never runs an executable. Replaying an already acknowledged attempt returns 409 instead of notifying again.

## Interrupted installation

If the daemon dies after `running` was saved but before `result`, the next startup exposes `stage: interrupted`, `callback_error: outcome_unknown`. It does not infer success from a marker, exit text or preflight. A normal `notify` is rejected. Game start/restart and another install are blocked so partial setup cannot silently resume as a ready server. Existing durable running intent cannot bypass this guard.

Inspect partial data and back up anything needed. To conservatively tell the panel this attempt failed, use the exact current attempt ID:

```json
{"attempt_id":"INTERRUPTED_ATTEMPT_UUID_FROM_GET","action":"report_failed"}
```

This saves an unsuccessful result, sends its failed callback and acknowledges it on acceptance. It does not roll back files or prove the installer made no changes. If notification fails, the record remains a known failed result; retry with `action: notify`. There is intentionally no `report_success` action for an unknown outcome. Once failure is acknowledged, separately choose repair/reinstall or another operator action.

## Storage and recovery limits

Unconfirmed writes block further installer operations. A failed initial save starts no installer and sends no callback. A failed result save sends no callback. A failed acknowledgement save may follow a delivered callback; `stage: result_unconfirmed`, `storage_error: write_failed` requires inspection. Do not rerun setup just to clear that error.

Both snapshots, their UUID/version/checksum, adjacent sequences, phase transitions and attempt IDs are validated at startup. Corrupt, missing, hardlinked, misplaced or conflicting state fails closed rather than falling back to an older success. Failure in one opted-in store prevents daemon initial sync/service readiness. There is no automatic repair or operator “clear” route: stop the daemon, retain evidence and restore a verified coherent snapshot pair through the administrative recovery process. Removing both files discards the safety record and is not a supported fix for an unknown outcome.

Local disk synchronization can block inside Windows; context deadlines bound callback/caller waits, not arbitrary stalled kernel I/O. Guards reject observed links and concurrent writers but do not constitute complete hostile isolation. Account/ACL provisioning, game-binary promotion, partial-data rollback and general transactional recovery remain separate work. See [native installer scope](NATIVE-INSTALLERS.md) and [roadmap](../ROADMAP.md).
