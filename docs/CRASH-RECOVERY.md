# Bounded game crash recovery

Alpha 19 adds opt-in automatic restart after an unexpected **nonzero** game-process exit. This runs inside the active daemon; it does not restart the daemon, auto-start games after boot, or restore state after a service restart. Reboot testing is deferred at the operator's request and remains unverified.

Add this object to a local executable profile, then restart the daemon during maintenance to load the change:

```json
"crash_recovery": {
  "enabled": true,
  "max_restarts": 3,
  "window_seconds": 300,
  "delay_seconds": 5
}
```

Omitting the object or setting `enabled` to false disables recovery. Defaults are three automatic attempts in a rolling 300-second window, delayed five seconds. Limits are 1..10 attempts, a 30..3600-second window and a 1..300-second delay shorter than the window. Configuration rejects invalid limits. A crash during startup is eligible, even before the ready pattern appears.

## Behavior and controls

The daemon records the failed exit, waits the configured delay, acquires the existing power-action lock and attempts a normal start. Each attempt fetches fresh panel settings and reruns executable/dependency preflight, account credential/SID checks, permissions guards and configured network policy. It never falls back to the daemon identity.

A clean exit (code zero), explicit stop/kill, suspension, node shutdown or runtime cleanup failure does not trigger recovery. Accepted manual start/restart replaces a pending automatic action. Accepted stop/kill cancels both a pending delay and an automatic launch already waiting on the panel or native preflight. The power queue, game Job Object and shutdown cancellation remain in use.

Each automatic launch attempt consumes one slot, including an attempt that fails during preflight. The rolling window expires old attempts; reaching the budget leaves the server offline and logs the reason. There is no timer that starts an offline game when the window later expires: an operator must start it again. Manual starts do not consume automatic slots or clear the budget.

A failed automatic start (for example, unavailable panel, missing executable, bad game password or fresh suspension) ends recovery and reports that manual start is required. It does not recursively retry the failed start. Waiting for another power action is bounded to 30 seconds; normal startup preflight retains its existing timeout. A hung running/starting process is not treated as an exit and is not a health-check failure detector.

## Status and evidence

Recovery scheduling, attempt, failure and budget messages appear in the existing console stream and bounded in-memory log history. `GET /api/servers/<UUID>` adds native-only `native_recovery` metadata: `enabled`, `pending`, `attempts_in_window`, configured `max_restarts`/`window_seconds`, and `next_attempt_at` (UTC) while an attempt is pending. The existing standard state remains offline during the delay and starting/running during launch; no unsupported panel state is invented.

Attempt history and timers are in memory. Service restart clears them and leaves games offline; there is no persistent crash ledger, watchdog, boot recovery or restart continuity claim. Strong isolation, actual WFP filtering and real-game/panel certification remain separate gates.

Local Windows fixture tests exercise nonzero and startup crashes, bounded loops, clean/disabled exits, operator/queue/suspension/shutdown cancellation, cancellation while fresh panel lookup blocks, fresh suspension rejection, failed preflight and rolling-window expiry. Packaged Server 2022 results are recorded separately in [verification](VERIFICATION.md).
