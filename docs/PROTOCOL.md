# Architecture and protocol coverage

This is an independent native Windows runtime, not a build of upstream Wings with Docker removed. The upstream tree depends extensively on Linux Docker behavior, filesystem primitives and Linux installation scripts. The implementation follows the public API conventions found in Wings `747bc43` and makes unsupported behavior explicit.

See the [original source parity audit](WINGS-PARITY-AUDIT.md) and [current progress](../ROADMAP.md). Alpha 2 fixes the listing URL/schema, file denylist, derived environment and emergency-kill defects. The route table describes implemented behavior; live panel compatibility remains unverified.

Alpha 3 adds decoded egg process configuration and live Job Object CPU/memory updates during sync. Opt-in `use_egg_process_configuration` supports readiness literals/regexes, ANSI stripping, command stops and explicit SIGKILL. Local mode remains the default. Unsupported signal/native-stop strategies and `configs` rewrites fail explicitly in egg mode. User-interaction adapters remain unimplemented.

```text
Pterodactyl panel backend -- node bearer / HTTPS --> Windows Native HTTP API
Browser console --------- scoped JWT / WSS ------> WebSocket console bridge
Windows Native ---------- node credentials -----> panel /api/remote
                                                |
                         locally approved server UUID + executable profile
                                                |
                         suspended CreateProcess -> assign Job Object -> resume
                                                |
                               stdin / stdout / stderr / process counters
```

## Packages

| Package | Responsibility |
| --- | --- |
| `internal/config` | Strict JSON configuration, UUID/profile rules, TLS/loopback requirements and exact origins. |
| `internal/native` | Windows process creation, restricted handle inheritance, Job Objects, CPU/memory limits, console pipes and process counters. Other platforms fail explicitly. |
| `internal/node` | Panel API client, local server registry, templates, lifecycle, readiness, bounded console queues and log history. |
| `internal/api` | Node bearer API, confined text-file access, JWT/permission enforcement, revocation and WebSocket events. |
| `cmd/wings-windows` | CLI, HTTP lifecycle and Windows SCM integration. |
| `cmd/testserver` | Harmless console fixture included for reproducible local verification. |

## HTTP authentication

Incoming panel-to-daemon requests use `Authorization: Bearer <node token>`. Outgoing daemon-to-panel requests use `Authorization: Bearer <token ID>.<node token>`. Redirects are rejected so credentials cannot follow an unexpected panel redirect.

Except for the WebSocket upgrade, API endpoints require the incoming node bearer. There is no anonymous file endpoint. JSON request bodies, file content, log history and console queues have explicit size limits. The browser must present a valid scoped token through the WebSocket auth frame before receiving console data or issuing commands.

## Supported routes

| Route | Behavior |
| --- | --- |
| `GET /api/system` | Basic Windows runtime information and capability list. |
| `GET /api/system?v=2` | Nested system information with an explicitly unavailable Docker runtime descriptor. The Docker counters are compatibility placeholders, not native process counts. |
| `GET /api/servers` | Locally registered, panel-assigned server state/configuration/utilization. |
| `GET /api/servers/{uuid}` | One server's response. |
| `POST /api/servers/{uuid}/power` | JSON `action`: `start`, `stop`, `restart`, `kill`. Returns 202 when queued; an active power action returns 409. Startup failures appear in console logs. `wait_seconds` is not implemented. |
| `POST /api/servers/{uuid}/commands` | JSON `commands`: up to 32 single-line commands, each at most 4096 bytes. |
| `GET /api/servers/{uuid}/logs` | Most recent 100 lines. Custom `size` is not implemented. |
| `POST /api/servers/{uuid}/sync` | Reload settings, file and process policies; apply active job limits without restart. Suspension stops the game even if another new egg setting is invalid. Failed limit updates do not change the reported applied cap. |
| `GET /api/servers/{uuid}/ws` | Origin-checked WebSocket upgrade, followed by JWT auth. |
| `POST /api/servers/{uuid}/ws/deny` | Revoke old tokens for supplied `jtis`. |
| `POST /api/deauthorize-user` | Revoke existing user tokens with JSON `user` and optional `servers`. An empty server list revokes access on all locally registered servers. |
| `GET /api/servers/{uuid}/files/list-directory?directory=/` | List a confined directory, up to 10,000 entries; uses the upstream metadata schema and hides denied paths. Legacy `/files/list` remains an alias. |
| `GET /api/servers/{uuid}/files/contents?file=/name` | Read a regular text file up to 8 MiB. |
| `POST /api/servers/{uuid}/files/write?file=/name` | Write a confined text file up to 8 MiB. Parent directories must already exist. |

Unknown/unsupported authenticated routes return 501, including server creation/deletion, automatic install/reinstall, file uploads/downloads, directory creation, file deletion/renaming, compression, pull, SFTP, backups and transfers. A profile that does not belong to this node is not registered and returns 404.

Filesystem access uses Go `os.Root` for confinement across symlinks. The path parser also rejects Windows drive/UNC syntax, traversal, alternate data streams, device names and trailing-dot/space aliases. This protects HTTP file operations; it does not sandbox executable code running under the same Windows account.

Egg `file_denylist` patterns are bounded, matched case-insensitively and refreshed on sync/start. Reads, writes and listing check requested and resolved handle paths before returning content or truncating a file. Absolute junctions may be rejected by `os.Root` itself, including links to locations inside the root. Hardlink identity policy and adversarial filesystem races remain part of the isolation work; pathname matching is not a security boundary against code running as the same account.

## WebSocket protocol

Frames are JSON objects with `event` and an array of string `args`:

```json
{"event":"auth","args":["SIGNED-PANEL-JWT"]}
```

Tokens must use HS256 with the node token, match the panel issuer and server UUID, include user UUID, issued-at and expiration, have `scope: websocket` and include `websocket.connect`. Tokens issued before daemon boot or a matching revocation are rejected. `nbf` and future issued-at are validated. Audience is not pinned in this alpha; server binding, issuer, scope and signature are enforced. Panel tokens can be refreshed with another `auth` event.

Commands and state changes check permissions on every frame. `control.console`, `control.start`, `control.stop` and `control.restart` gate their corresponding actions; `kill` requires `control.stop`. The wildcard grants known actions only. An unknown power action is rejected even with wildcard permissions.

Input events: `auth`, `send logs`, `send stats`, `send command`, `set state`.

Output events: `auth success`, `status`, `stats`, `console output`, `jwt error`, `daemon error`, `token expiring`, `token expired`. Expiry/revocation is checked every second and on each command. Authentication must arrive within ten seconds. There is a node-wide cap of 256 WebSocket connections; slow output consumers are disconnected instead of blocking a game process.

## Lifecycle and data

Only a fixed `.exe` path in the local configuration can run. Panel `invocation`, Docker image and Linux install scripts are not executed. Approved `{{VARIABLE}}` placeholders are substituted separately in an argument array. Missing approved variables fail startup. Each game gets a minimal environment and its own working/temp directories.

Power operations serialize per server; emergency kill cancels any active action and bypasses its lock. A cancelled restart cannot relaunch the game. Every start fetches the current panel settings before launching and checks suspension. Process creation uses `CREATE_SUSPENDED`, a restricted inherited pipe-handle list and Job Object assignment before resume. Assignment failure terminates the suspended process. The job uses kill-on-close and optional aggregate memory/CPU controls. Exiting the root also closes the job, so detached child processes do not remain as unmanaged servers.

State progresses `offline -> starting -> running -> stopping -> offline`. `running` requires the profile's output regex. Exit before readiness returns offline. There is no restart-on-crash loop and no adoption of external processes. Closing the daemon kills managed games and a later start fetches fresh panel configuration.

Console history is in memory and resets after daemon restart. Persistent logs, state recovery and service Event Log integration remain future work. Panel configuration remains authoritative for variables, limits and suspension; local profiles are authoritative for the executable, argument structure, readiness and stop behavior.

## Sources

- [Wings router](https://github.com/pterodactyl/wings/blob/747bc43/router/router.go)
- [Wings HTTP middleware](https://github.com/pterodactyl/wings/blob/747bc43/router/middleware/middleware.go)
- [Wings WebSocket protocol](https://github.com/pterodactyl/wings/blob/747bc43/router/websocket/websocket.go)
- [Wings token scope and revocation](https://github.com/pterodactyl/wings/blob/747bc43/router/tokens/websocket.go)
- [Wings remote panel client](https://github.com/pterodactyl/wings/blob/747bc43/remote/http.go)
- [Microsoft Job Objects](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects)
- [Microsoft process creation](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-createprocessw)

The upstream snapshot was inspected directly in the local reference checkout. The Microsoft links are supporting documentation for the Windows APIs used; this release's verification is the local test suite, not a claim of certification.
