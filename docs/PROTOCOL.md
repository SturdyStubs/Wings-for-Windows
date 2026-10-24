# Architecture and protocol coverage

This is an independent native Windows runtime, not a build of upstream Wings with Docker removed. The upstream tree depends extensively on Linux Docker behavior, filesystem primitives and Linux installation scripts. The implementation follows the public API conventions found in Wings `747bc43` and makes unsupported behavior explicit.

See the [original source parity audit](WINGS-PARITY-AUDIT.md) and [current progress](../ROADMAP.md). Alpha 2 fixes the listing URL/schema, file denylist, derived environment and emergency-kill defects. The route table describes implemented behavior; live panel compatibility remains unverified.

Alpha 3 adds decoded egg process configuration and live Job Object CPU/memory updates during sync. Opt-in `use_egg_process_configuration` supports readiness literals/regexes, ANSI stripping, command stops and explicit SIGKILL. Local mode remains the default. Unsupported signal/native-stop strategies and unsupported `configs` rewrites fail explicitly in egg mode. Alpha 23 adds approved plain-text file rewriting; alpha 24 adds a bounded JSON subset. See [text](EGG-TEXT-CONFIG.md) and [JSON scope/limits](EGG-JSON-CONFIG.md). Alpha 25 adds approved [properties](EGG-PROPERTIES-CONFIG.md) with expansion disabled. Alpha 26 adds a bounded [INI subset](EGG-INI-CONFIG.md) with exact section updates and literal values. Alpha 27 adds a bounded [YAML/yml subset](EGG-YAML-CONFIG.md) reusing JSON paths without alias expansion. Alpha 28 adds a bounded [XML subset](EGG-XML-CONFIG.md) with text/attribute updates and strict namespace-free paths. Alpha 29 adds opt-in [durable running intent restoration](RUNNING-RESTORATION.md) after readiness with fresh guards and persistent restart reservations. User-interaction adapters remain unimplemented.

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

Alpha 36 adds an operator-only [incremental game permission command](GAME-PERMISSIONS.md). It does not expose an HTTP provisioning route or change upstream route coverage. Account creation and credential storage remain separate.

Alpha 37 adds an operator-only [game-account creation command](GAME-ACCOUNTS.md). It returns identity/profile metadata without rewriting node JSON or adding an HTTP route. Automatic panel-driven provisioning remains open.

Alpha 38 adds an operator-only [verified local game-file importer](GAME-FILES.md) and blocks unfinished code staging during incremental permission preparation. It adds no HTTP route; automatic game downloads and code updates remain open.

Alpha 39 adds operator-only [staging inspection and quarantine](GAME-FILE-RECOVERY.md), with fingerprint approval and retained bytes. It adds no HTTP route, automatic promotion/resume or durable crash journal.

Alpha 40 adds the existing panel/upstream [directory creation contract](FILE-DIRECTORIES.md), including missing parents, existing-directory replay, Windows path bounds and denylist/link checks. It is the first M4 addition; other file mutations, scoped upload/download and SFTP remain open.

Alpha 41 adds [file/directory rename and move](FILE-RENAME.md) with the existing panel request shape, native no-replace behavior, case-only names, batch preflight and descendant policy checks. Existing destination parents are required; dependent operations and broader filesystem behavior remain open.

Alpha 42 adds [regular-file copy](FILE-COPY.md), bounded binary streaming and staged no-replace publication. Full disk accounting/enforcement, recursive copy and interrupted-copy recovery remain open.

## Packages

Alpha 32 adds optional [account-scoped RCON credential references](RCON-CREDENTIALS.md) and local metadata/remove CLI. Existing network command routes and transport authentication remain unchanged; no new panel route or actual-game certification is implied.

Alpha 31 adds optional [game log-file console/readiness](LOG-FILE-CONSOLE.md), selected locally. `native_log_file` status reports activation/failure without paths. Authorization and native process state remain unchanged.

Alpha 30 adds optional [Source TCP RCON command delivery](SOURCE-RCON.md), selected only by local profile. REST/WebSocket authorization and native process status remain unchanged. No new HTTP route or real-game certification is implied.

| Package | Responsibility |
| --- | --- |
| `internal/config` | Strict JSON configuration, UUID/profile rules, TLS/loopback requirements and exact origins. |
| `internal/identity` | Per-server credential references, local batch logon, SID/group checks, restricted tokens and read-only protected-path DACL audit. |
| `internal/networkpolicy` | Optional account-scoped WFP block installation, dynamic sessions and rollback; live filtering remains unverified. |
| `internal/native` | Windows process creation, restricted handle inheritance, Job Objects, CPU/memory limits, console pipes and process counters. Other platforms fail explicitly. |
| `internal/provision` | Read-only Windows executable, architecture, OS build and declared dependency checks. |
| `internal/node` | Panel API client, local server registry, templates, lifecycle, readiness, bounded console queues and log history. |
| `internal/sourcercon` | Optional loopback Source TCP authentication, bounded complete replies and cancellation. |
| `internal/logtail` | Optional pinned-parent game log polling, bounded text assembly and file-generation handling. |
| `internal/api` | Node bearer API, confined text-file access, JWT/permission enforcement, revocation and WebSocket events. |
| `cmd/wings-windows` | CLI, HTTP lifecycle and Windows SCM integration. |
| `cmd/testserver` | Harmless console fixture included for reproducible local verification. |

## HTTP authentication

Incoming panel-to-daemon requests use `Authorization: Bearer <node token>`. Outgoing daemon-to-panel requests use `Authorization: Bearer <token ID>.<node token>`. Redirects are rejected so credentials cannot follow an unexpected panel redirect.

Except for the WebSocket upgrade and non-mutating OPTIONS preflight, API endpoints require the incoming node bearer. There is no anonymous file endpoint. JSON request bodies, file content, log history and console queues have explicit size limits. The browser must present a valid scoped token through the WebSocket auth frame before receiving console data or issuing commands.

Every REST response receives a generated UUID `X-Request-Id`; REST error JSON includes the same `request_id`. Malformed/trailing JSON returns 400, oversized JSON 413, and invalid action values 422. Missing/invalid bearer credentials return 401 with a Bearer challenge.

OPTIONS returns 204 without credentials. CORS allows the exact panel origin and configured `allowed_origins`, includes credentials and exposes request IDs/file headers. Unknown origins are not reflected. Actual operations still require authentication. `allow_cors_private_network` defaults false; enable explicitly only when private-network browser preflight is needed. WebSocket upgrades carry response headers and still enforce their origin/token checks.

Alpha 33 adds [locally approved native setup](NATIVE-INSTALLERS.md), including install/reinstall completion callbacks. This initializes data around pre-provisioned code, not arbitrary remote egg scripts.

Alpha 34 adds opt-in [manual-approval creation](PENDING-CREATION.md), with no pending execution resources and assignment-based reconstruction.

## Supported routes

| Route | Behavior |
| --- | --- |
| `GET /api/system` | Measured Windows architecture, active CPU count and kernel version, plus capability list. |
| `GET /api/system?v=2` | Upstream-shaped system information with measured physical memory and Windows version. Docker is explicitly unavailable with zero container counters; separate `native` fields report server state counts, actual data filesystem and lack of container isolation. |
| `POST /api/servers` | Opt-in manual-approval creation, assigned UUID v4 only, 202 pending local profile. Automatic start returns 422; [contract and limits](PENDING-CREATION.md). |
| `POST /api/update` | Explicit `ignore_panel_config_updates` mode: authenticated bounded JSON object returns `200 {"applied":false}` without changing anything. Default mode is 501; remote application/rotation remain unsupported. |
| `POST /api/servers/{uuid}/files/delete` | Bounded 1–100 targets / 1,024 total entries, whole-batch path/policy/link/overlap preflight, missing replay and pinned native child-first deletion. Later I/O/cancellation can retain partial effects. |
| `POST /upload/file?token=...&directory=/...` | Scoped one-use multipart `files`; approved server/existing parent, 100 MiB aggregate/16 files, owned staging and no-replace publication. Batch publication can retain partial effects. [Scope](FILE-UPLOADS.md). |
| `GET /download/file?token=...` | Public JWT-authenticated, server/user/file-bound one-use download; HS256/issuer/time/scope and revocation checks, bounded pinned regular-file streaming. No master bearer fallback, Range/resume or directory downloads. |
| `GET /api/servers` | Locally registered, panel-assigned server state/configuration/utilization. |
| `GET /api/servers/{uuid}` | One server's response. |
| `POST /api/servers/{uuid}/install`, `POST /api/servers/{uuid}/reinstall` | Opt-in approved native data/setup executable; 202 asynchronous, 501 without recipe, 400 suspended, 409 busy. Reinstall stops the game. Emits installer events and callback booleans; [limits](NATIVE-INSTALLERS.md). |
| `POST /api/servers/{uuid}/power` | JSON `action`: `start`, `stop`, `restart`, `kill`. Returns 202 when accepted asynchronously. `wait_seconds` bounds lock acquisition: omitted/zero tries once, 1–300 waits that many seconds, out-of-range values use 30. Failures/timeouts appear in console logs. At most 16 requests may await execution per server (429 when full). Kill bypasses the queue and invalidates queued actions. Suspended start/restart returns 400; shutdown rejects requests with 409. |
| `POST /api/servers/{uuid}/commands` | JSON `commands`: up to 32 single-line commands, each at most 4096 bytes. Returns 502 without a live process, including an empty command list. |
| `GET /api/servers/{uuid}/logs` | Most recent `size` lines, capped at 100; omitted, invalid or nonpositive values use 100. |
| `POST /api/servers/{uuid}/sync` | Reload settings, file and process policies; apply active job limits without restart. Suspension stops the game even if another new egg setting is invalid. Failed limit updates do not change the reported applied cap. |
| `GET /api/servers/{uuid}/ws` | Origin-checked WebSocket upgrade, followed by JWT auth. |
| `POST /api/servers/{uuid}/ws/deny` | Revoke old tokens for supplied `jtis`. |
| `POST /api/deauthorize-user` | Revoke existing user tokens with JSON `user` and optional `servers`. An empty server list revokes access on all locally registered servers. |
| `GET /api/servers/{uuid}/files/list-directory?directory=/` | List a confined directory, up to 10,000 entries; uses the upstream metadata schema and hides denied paths. Legacy `/files/list` remains an alias. |
| `GET /api/servers/{uuid}/files/contents?file=/name` | Read a regular text file up to 8 MiB. |
| `POST /api/servers/{uuid}/files/pull`, `GET /api/servers/{uuid}/files/pull`, `DELETE /api/servers/{uuid}/files/pull/{id}` | Opt-in direct public HTTPS only; bounded foreground/background staging, active progress and server-scoped cancellation. Default 501. [Scope](REMOTE-PULLS.md). |
| `POST /api/servers/{uuid}/files/decompress` | JSON `root` and `file`; single gzip/tar member or bounded Store/Deflate ZIP, 101 MiB source/100 MiB file data/256 paths; tar decoded stream 102 MiB, ZIP central directory 1 MiB. Complete validation, owned staging and new destinations only; late publication can retain partial effects. [Scope](FILE-EXTRACTION.md). |
| `POST /api/servers/{uuid}/files/compress` | JSON `root` and `files`; bounded tar.gz creation and file-metadata response. Pinned regular files/directories, 256 entries/100 MiB input/101 MiB output, flushed staging and no replacement. [Scope](FILE-ARCHIVES.md). |
| `POST /api/servers/{uuid}/files/copy` | JSON `location`; copy one regular file to a numbered sibling. 1 GiB source cap, owned staging/no-replace publication, no panel quota enforcement. [Scope](FILE-COPY.md). |
| `PUT /api/servers/{uuid}/files/rename` | JSON `root` and `files: [{from,to}]`; file/directory move/rename, case-only names and ignored missing sources. No overwrite; bounded preflight and existing destination parents required. [Scope](FILE-RENAME.md). |
| `POST /api/servers/{uuid}/files/write?file=/name` | Write a confined text file up to 8 MiB. Parent directories must already exist. |
| `POST /api/servers/{uuid}/files/create-directory` | JSON `name` and `path`; create nested directories and missing parents, 204 for success/existing directory. Bounded Windows names, denylist preflight and observed reparse/link refusal; [scope](FILE-DIRECTORIES.md). |

Unknown/unsupported authenticated routes return 501, including server deletion, automatic binary provisioning, resumable transfers, other archive formats, broader remote-fetch policies, SFTP, backups and transfers. A profile that does not belong to this node is not registered and returns 404.

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

Profiles use [schema v1](PROFILES.md), with legacy profiles normalized to v1. Before each start and manual installation acknowledgement, local preflight validates the Windows PE executable and declared host/dependency requirements. `--check-profiles` runs the same checks without contacting the panel or launching a process. These checks are not installation or isolation; see the [isolation design](ISOLATION.md).

Local credential management uses `--credential-status <UUID>` and `--remove-credential <UUID>` without loading node config or contacting the panel. Metadata excludes passwords; removal is exact-target and idempotent. These are local operator commands, not new HTTP endpoints. See [credential lifecycle](CREDENTIAL-LIFECYCLE.md).

`--check-identity <UUID>` loads local configuration and reports profile/credential/logon/token/daemon-access/filesystem/service-privilege checks as JSON. It makes no panel request, creates no server roots and launches no executable; unlike credential-status it explicitly attempts one account logon. See [diagnostic semantics](IDENTITY-DIAGNOSTICS.md).

`--check-network-policy <UUID>` is a separate explicitly mutating operator diagnostic: after identity preflight it temporarily installs the configured block, checks owned rules and independently inspects cleanup. It performs no traffic probe or game launch. See [WFP diagnostic](NETWORK-DIAGNOSTICS.md).

Opt-in identity startup retains read-sharing-only handles for nonrecursive protected paths, executable, working directory and ancestors before the ACL audit until process creation/resume returns. Conflicting writers fail closed; recursive content, ACL/mapping and runtime-loader races remain open. See [startup guard](LAUNCH-GUARD.md).

Identity startup also checks effective executable read/execute, declared dependency read and working/temporary-directory list/traverse/create grants before WFP installation or process creation. `--check-identity` exposes the same read-only checks as `launch_access`; it creates no directories. See [launch permissions](LAUNCH-ACCESS.md).

Only a fixed `.exe` path in the local configuration can run. Panel `invocation`, Docker image and Linux install scripts are not executed. Approved `{{VARIABLE}}` placeholders are substituted separately in an argument array. Missing approved variables fail startup. Each game gets a minimal environment and its own working/temp directories.

Power operations serialize per server; emergency kill cancels active and queued actions and bypasses the lock. Accepted HTTP actions survive request disconnection, but shutdown cancels them. Waiting actions are bounded and do not promise FIFO ordering; WebSocket power requests still attempt the lock immediately. A cancelled restart cannot relaunch the game. Every start fetches the current panel settings before launching and checks suspension. Optional `run_as` profiles resolve a local Credential Manager reference, enforce the pinned account SID, reject daemon/privileged accounts and use a restricted primary token with daemon process/primary-token DACL checks, declared executable/dependency write checks, handle-based protected-path ACL checks, ancestor rights checks and bounded recursive scans of other configured server trees; failure never falls back. See [identity limits](IDENTITIES.md). An optional `network_policy.mode: block_connections` installs account-SID block rules before process creation and retains the dynamic session through job cleanup; failure never falls back to unmanaged. See [network policy](NETWORK-POLICY.md). Process creation uses `CREATE_SUSPENDED`, a restricted inherited pipe-handle list and Job Object assignment before resume. Assignment failure terminates the suspended process. The job uses kill-on-close and optional aggregate memory/CPU controls. Exiting the root also closes the job, so detached child processes do not remain as unmanaged servers.

State progresses `offline -> starting -> running -> stopping -> offline`. `running` requires the profile's output regex. Exit before readiness returns offline. There is no restart-on-crash loop and no adoption of external processes. Closing the daemon kills managed games and a later start fetches fresh panel configuration.

Console history is in memory and resets after daemon restart. Persistent logs, state recovery and service Event Log integration remain future work. Panel configuration remains authoritative for variables, limits and suspension; local profiles are authoritative for the executable, argument structure, readiness and stop behavior.

## Portable host baseline

The package includes compiled `internal/native` tests and `scripts/Test-Host.ps1` for offline VM evidence without Go or source files. This runs harmless processes under the invoking account, including a self-derived restricted token, and records exact suite coverage. It invokes no panel, SCM, real game-account logon or WFP filter installation. See [VM workflow](WINDOWS-SERVER-TESTING.md). No protocol route is added.

## Sources

- [Wings router](https://github.com/pterodactyl/wings/blob/747bc43/router/router.go)
- [Wings HTTP middleware](https://github.com/pterodactyl/wings/blob/747bc43/router/middleware/middleware.go)
- [Wings WebSocket protocol](https://github.com/pterodactyl/wings/blob/747bc43/router/websocket/websocket.go)
- [Wings token scope and revocation](https://github.com/pterodactyl/wings/blob/747bc43/router/tokens/websocket.go)
- [Wings remote panel client](https://github.com/pterodactyl/wings/blob/747bc43/remote/http.go)
- [Microsoft Job Objects](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects)
- [Microsoft process creation](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-createprocessw)

The upstream snapshot was inspected directly in the local reference checkout. The Microsoft links are supporting documentation for the Windows APIs used; this release's verification is the local test suite, not a claim of certification.

## Initial Windows service provisioning

Alpha 17 adds opt-in local service-account creation, scoped LSA rights and protected initial node ACLs through [Install-Service.ps1](SERVICE-PROVISIONING.md). This changes operator setup only, with no new API routes. A Server 2022 SCM demo test uses a loopback mock panel; it does not satisfy the live panel/TLS gate.

Alpha 18 adds managed-service credential create/inspect/replace/remove through a temporary password-backed task. It is an operator command, adds no daemon API routes and does not replace real-panel certification. See [credential setup](SERVICE-CREDENTIALS.md).

## Game crash recovery

Alpha 19 adds opt-in [bounded automatic restart](CRASH-RECOVERY.md) through the existing normal launch path. GET /api/servers/<UUID> includes native_recovery status; no new routes or standard power/status values are introduced. Operator controls cancel automatic recovery. Timers/history are not persisted across service restarts.

## Persistent console history

Alpha 20 optionally persists per-server console output in an administratively protected log tree. Existing /logs responses can include restored lines after service restart. The server response adds native_console_logs counters/error status; no new upstream routes are implemented. See [limits and retention](CONSOLE-LOGS.md).

The creation fixture `internal/api/testdata/upstream-create-server.json` follows Wings `747bc43` installer.ServerDetails and the reviewed panel Services/Daemon/ServerClient::create fields. API tests reproduce the panel action's persist-before-request/delete-on-failure behavior. Manual approval is a documented divergence from automatic upstream installation.

Alpha 35 adds an extra native `POST /api/servers/{uuid}/install/reconcile` route, requiring the normal incoming node bearer and opt-in persistent installer state. It is not counted as another upstream route. It retries only recorded callback booleans or explicitly records interrupted failure, never executing an installer; see [contract and uncertainty](INSTALL-RECOVERY.md).
