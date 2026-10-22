# Approved native setup recipes

Alpha 33 adds asynchronous `POST /api/servers/{uuid}/install` and `/reinstall` for a server already assigned by the panel and registered in local configuration. These execute an operator-approved Windows executable to initialize **data for pre-provisioned game binaries**. This is not a Windows package manager or a translator for Linux egg scripts.

Add `installer` to that server's local profile, then restart the daemon:

```json
"installer": {
  "executable": "C:\\WingsWindows\\bin\\wings-windows-demo.exe",
  "arguments": ["install", "success"],
  "allowed_environment": [],
  "timeout_seconds": 600,
  "memory_mb": 128,
  "requirements": {"architecture": "amd64", "required_files": []}
}
```

The packaged demo writes `install.marker` in its working directory, prints harmless output and exits. It is not a real game installer. See [panel setup and the demo egg](PANEL-WINDOWS-SETUP.md).

## Execution and permissions

The local executable must be a clean absolute `.exe` path. Arguments are an array with the existing `{{VARIABLE}}` substitution; no shell is invoked. Only explicitly allowed environment variables plus normal derived allocation/startup/timezone values and the minimal Windows runtime environment are passed. Never substitute an untrusted startup command into a shell argument. The HTTP request body, panel invocation and remote egg install script cannot select an executable or recipe.

The installer uses the game's data directory, `.tmp`, `run_as` credential/SID, restricted launch path, configured network policy, panel CPU cap and the lower applicable memory cap. The game and installer code and declared dependencies are protected from game-account writes. Managed setup requires installer code/dependencies under `bin`, just like game code. No installer privilege elevation or separate privileged installation identity is available. Supply and maintain game binaries separately as the operator; downloading/updating/promoting protected binaries remains future work.

`timeout_seconds` defaults to 600, range 1–7200. `memory_mb` defaults to 256, range 16–8192. Game preflight runs before setup and after successful exit; installer preflight runs before its launch. Missing or invalid game code therefore cannot be repaired by this first runner. An explicit panel `skip_egg_scripts` skips the executable after game preflight and reports success. It does not fetch or execute the remote Linux script.

## Lifecycle and panel contract

- Accepted requests return 202 and survive HTTP disconnect. No recipe returns 501, a suspended server 400, and a concurrent install/power operation 409. Initial install requires an offline game; reinstall stops a running game before setup. Unknown/unassigned UUIDs return 404.
- Install/reinstall reserve the same per-server power lock. Other power actions and commands are rejected while active; emergency `kill`, suspension sync and daemon shutdown cancel the installer and terminate its Job Object tree. A durable stopped intent is written before setup when restoration is enabled.
- Installer stdout/stderr emits `install started`, `install output`, `install completed` events and prefixed console lines. It never signals game readiness. Console retention is capped at 1 MiB for this operation, lines at 16 KiB; excess output is drained. Do not print secrets from approved recipes.
- Completion posts `{"successful":true,"reinstall":false}` (appropriate booleans) to `/api/remote/servers/{uuid}/install` with normal node credentials. Nonzero exit, cancellation, timeout or failed checks report failure. Successful setup leaves the game offline.
- `GET /api/servers/{uuid}` adds `native_install` with `enabled`, `active`, `stage`, `successful` and `callback_error`. Stages are `idle`, `preflight`, `running`, `notifying`, `complete`, `failed`, `cancelled`, `timed_out`. Local success and panel notification success are separate: `notification_failed` means inspect panel state before deciding what to retry.

There is one bounded callback attempt; no durable outbox, automatic callback retry or interrupted-install recovery exists. Daemon death/reboot can leave partial data or an unresolved panel installation state. Setup has no transactional rollback, download verification, disk quota or binary promotion. Make recipes idempotent, back up data before reinstall and inspect partial results after interruption. Do not rerun a destructive installer just to repeat its notification.

Server creation (`POST /api/servers`) and deletion remain unsupported. For a panel that supports retaining an assigned record, bind its full UUID to a local profile and invoke reinstall after discovery. The reviewed panel 2.0.0-beta.1 deletes a newly created record when daemon creation fails; it must wait for native creation support. This setup must still be tested with a real panel; local/mock protocol tests do not certify the live workflow.
