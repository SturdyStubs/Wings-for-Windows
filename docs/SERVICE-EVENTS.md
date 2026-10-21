# Windows service diagnostics

Alpha 21 adds fixed, secret-safe lifecycle events to **Event Viewer → Windows Logs → Application**, source **WingsWindowsNative**. This is separate from [per-server console history](CONSOLE-LOGS.md). It does not save raw game output or arbitrary daemon errors.

## Enable

From an elevated Windows PowerShell prompt, run the packaged script once before starting the service:

```powershell
.\scripts\Register-EventSource.ps1 -WhatIf
.\scripts\Register-EventSource.ps1
Start-Service WingsWindowsNative
```

The script creates only the named Application source, using the Windows EventCreate message resource. It requires Administrator access; the runtime writes as its existing non-administrator service identity. Matching managed registrations are preserved on rerun. Unrecognized registrations or a source registered in another log are rejected; the script does not overwrite them. Registration is independent of service installation and is not enabled by Install-Service.ps1. No game account receives registry or Event Log administration rights.

If registration is absent or Event Log open/write fails, the runtime emits a generic stderr warning and continues. It does not grant itself permissions or modify the registry. Windows controls Application log retention and access: ordinary users may be able to read these events. Messages therefore include only fixed text, version, PID and an allowlisted failure stage. They exclude configuration contents, tokens, URLs, passwords, paths and raw errors.

## Events

| ID | Level | Meaning |
| --- | --- | --- |
| 100 | Information | Service handler started; configuration/sync may still fail. |
| 101 | Information | Initial configuration and panel synchronization succeeded, TLS material loaded if configured, and the API TCP listener bound. SCM now reports Running. |
| 102 | Information | SCM stop/shutdown requested; cancellation and cleanup began. |
| 103 | Information | Service runtime returned successfully, including requested startup cancellation. |
| 200 | Error | Runtime returned a failure; SCM receives nonzero exit status. |

Startup/stop pending states publish checkpoints while initialization or shutdown is in progress. An interrogate returns the current state. A startup error does not first publish Running. Listener readiness does not certify trusted TLS from another host, browser connectivity, game startup or a real panel workflow.

Event 200 stages:

| Stage | Inspect |
| --- | --- |
| configuration | Run `wings-windows.exe --config C:\path\config.json --check` locally as an appropriate operator; check file access for the service identity. |
| panel_sync | Panel availability, node assignment, credentials and trusted TLS. |
| tls | Certificate/key existence, matching pair and service-account read access. |
| listener | Port/address conflicts and bind permissions. |
| http | Listener/runtime failure after initialization. |
| shutdown | HTTP graceful shutdown failure; active connections are force-closed. |
| runtime | An unclassified runtime failure. |

Read recent entries without exposing node configuration:

```powershell
Get-WinEvent -FilterHashtable @{
    LogName='Application'; ProviderName='WingsWindowsNative'; StartTime=(Get-Date).AddHours(-1)
} | Select-Object TimeCreated, Id, LevelDisplayName, Message
```

If an event description cannot be rendered, the XML `EventData` insertion string still contains the fixed message. Do not send private configuration or account passwords as diagnostic output.

## Removal and limits

After separately stopping and removing the service registration, an Administrator may remove this managed event source:

```powershell
.\scripts\Register-EventSource.ps1 -Remove -WhatIf
.\scripts\Register-EventSource.ps1 -Remove
```

Removal refuses any existing WingsWindowsNative service, including a stopped one. A missing source is a no-op. Historical Application events are not cleared. This command does not remove accounts, credentials, profiles or game data. Register again before reinstalling the service if events are wanted.

These are best-effort lifecycle diagnostics, not a transactional journal, crash dump, per-game state store, Windows service auto-restart policy or interrupted-install recovery. Failures before SCM invokes the handler, abrupt process termination, a panic, Event Log outages and host power loss may leave no final event. Registration is a trusted administrative operation, not protection against concurrent administrative registry changes. No reboot is required by this workflow; reboot/profile recovery remains unverified and deferred.

Local injected-handler tests cover readiness/state transitions, stop/shutdown and startup cancellation, closed control channels, unavailable/failing sinks, failure stages and omission of untrusted error details. Listener fixtures cover configuration, panel sync, missing TLS, bind failures, HTTP readiness and an actual self-signed HTTPS request. Exact packaged Server 2022 evidence is recorded separately in [verification](VERIFICATION.md).
