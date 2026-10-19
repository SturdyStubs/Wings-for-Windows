# Changelog

## 0.1.0-alpha.4 — October 18, 2026

- Accept HTTP power actions asynchronously with bounded `wait_seconds` lock acquisition. Limit pending requests to 16; kill and shutdown invalidate queued starts/restarts as well as active work. WebSocket power remains immediate.
- Report measured Windows CPU count, physical memory, kernel version and data filesystem. Match the system-v2 schema while keeping unavailable Docker counters separate from native server counts.
- Add exact-origin CORS/preflight, optional private-network response headers, request IDs on REST responses/errors and WebSocket handshakes, Bearer challenges and distinct malformed/oversized/invalid-action responses.
- Honor recent-log size requests and return 502 for commands when no process is alive.
- Add Windows queue/cancellation and independent HTTP/system contract regressions. Live panel/TLS/subuser/file validation remains open; M1 is not complete.

## 0.1.0-alpha.3 — October 18, 2026

- Add opt-in `use_egg_process_configuration` for supported Windows eggs. Decode process settings from node discovery and individual fetches; apply multiple literal/regex readiness matchers, ANSI stripping and command/SIGKILL stop strategies. Preserve local profile behavior by default.
- Reject unsupported signals, Docker-native stop strategies, missing readiness and unimplemented config-file rewrites explicitly in egg-driven mode.
- Apply CPU/memory limits to running Windows jobs on sync without restarting games; preserve local memory caps, support removing limits, retain kill-on-close and roll back memory if a CPU update fails.
- Keep reported enforcement unchanged on rejected updates. Apply suspension even when another new egg setting is invalid.
- Add regression tests that query actual Windows job limits, run an egg-controlled native fixture, check live readiness changes and verify cancellation/suspension behavior.
- Add six GitHub milestones, granular deliverable issues and automatically refreshed README progress badges. Include an idempotent progress setup/refresh script; existing reopened issues are never automatically closed again.

Live deployment remains a separate open gate. Config-file rewriting, Windows Ctrl+C/CTRL_BREAK adapters and other unsupported egg behavior are not supplied by the opt-in setting.

## 0.1.0-alpha.2 — October 18, 2026

First compatibility-fix batch after auditing upstream Wings `747bc43`.

- Correct the panel directory-listing route and upstream file metadata keys; retain `/files/list` as an alias.
- Report actual Windows creation time, MIME type, Content-Length and authenticated attachment headers.
- Apply bounded, case-insensitive egg file restrictions to listing/read/write and refresh them on sync/start. Check resolved handles before returning content or truncating files; retain `os.Root` confinement.
- Derive allocation/startup/memory/timezone environment values with authoritative precedence over egg variables. Add an optional IANA `timezone` setting, default UTC.
- Make emergency kill interrupt a graceful stop/restart and cancel pending panel synchronization/startup. Cancelled restarts cannot relaunch the game.
- Match the upstream HTTP 400 response for suspended start/restart requests.
- Add independent metadata contract fixtures and tests for file restrictions, sync, derived variables, junction rejection and cancellation races.
- Pin MIME detection to unretracted v1.4.16 rather than upstream's retracted v1.4.15 dependency.
- Establish the standalone GitHub repository, progress tracker and six tracking issues.

Live panel/game/service validation remains pending. Existing Linux eggs are unsupported by the native Windows backend. There is no automatic installer, full file manager/SFTP, backup/restore/transfer support or container-equivalent isolation yet.

## 0.1.0-alpha.1 — October 18, 2026

- Initial native EXE runtime with suspended process creation, Windows Job Objects and optional job resource limits.
- Basic node bearer API, panel synchronization, power, stdout/stderr/stdin console, status, readiness and root-process statistics.
- JWT-authenticated console WebSockets with permissions, expiry and revocation.
- Initial confined file helpers, configuration, manual installation acknowledgement and Windows SCM entry point.
- Local Windows process/console/child-cleanup tests, packaging and source parity audit.

The original file-list route/schema and power locking defects were identified by the audit and corrected in alpha 2.
