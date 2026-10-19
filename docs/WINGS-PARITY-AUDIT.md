# Wings parity audit

Source review: October 18, 2026. Compared Windows Native `0.1.0-alpha.1` with upstream Wings commit [`747bc4393daf5d95fdf61d76382f5e23749cffe9`](https://github.com/pterodactyl/wings/tree/747bc4393daf5d95fdf61d76382f5e23749cffe9). A remote `git ls-remote` check confirmed that this was upstream HEAD at review time. Also inspected the local Pterodactyl 2.0.0-beta.1 panel's daemon file client to check the actual request/response contract.

This is a source-level feature and compatibility review, not a live certification or an exhaustive security assessment. It identifies required work; the runtime fixes and features below have not been implemented by this review.

## Finding

The current implementation is a native Windows process-control prototype with part of the Wings wire protocol. It is substantially short of a Wings replacement. Local mock-panel tests prove individual runtime behavior, not compatibility with the complete real panel.

Upstream explicitly registers **38 HTTP routes** in `router/router.go`. Our daemon has handlers at **12 matching method/path combinations**, with partial semantics, and one additional nonstandard directory-listing path. The remaining **26 upstream method/path combinations are absent**. SFTP and internal lifecycle/security behavior add substantial work outside that count. These counts are route inventory, not a completion percentage.

There are two separate targets:

1. **Windows feature parity:** equivalent panel workflows for games that have Windows-compatible binaries, installation recipes and dependencies. Most management workflows can be implemented, with Windows-specific security, filesystem, networking and process-control behavior.
2. **Unchanged Linux egg compatibility:** Linux Docker images, binaries, shell entrypoints, filesystem paths and signal semantics. Native Windows EXE execution cannot supply these. This requires a Linux execution backend, such as upstream Wings in a Linux VM, alongside the Windows backend. Adding HTTP endpoints or a panel extension cannot translate arbitrary Linux game binaries or Docker images into Windows software.

Even upstream Wings does not guarantee that every third-party egg works on every host. Versions, architecture, dependencies, privileges and game-specific requirements still matter.

## Immediate compatibility defects

These are defects in the existing subset, not just missing optional features.

| Priority | Finding | Evidence / effect |
| --- | --- | --- |
| P0 | Directory-listing route does not match Wings. | Upstream and the panel call `GET /api/servers/{uuid}/files/list-directory`; our helper accepts `/files/list`. The real request receives 501. |
| P0 | File-list response field names do not match Wings. | Upstream `server/filesystem/stat.go` emits `file`, `directory`, `symlink`, `mime`, `created`, `modified`. Ours emits `is_file`, `is_symlink`, `mimetype`, `created_at`, `modified_at` and omits `directory`. Fixing the URL alone is insufficient. |
| P0 | Egg file denylist is not enforced. | Upstream builds its filesystem with `egg.file_denylist`; ours retains egg JSON but never applies its denylist to reads, writes or listing. Server-root confinement is useful but does not enforce egg-level restrictions inside that root. |
| P0 | Emergency kill cannot interrupt a busy power action. | Upstream `server/power.go` deliberately lets termination bypass a busy power lock. Ours puts all four actions behind `TryLock`, so kill receives a conflict while a stop/restart is in progress. |
| P1 | Egg process configuration is dropped. | Upstream fetches `process_configuration` with startup matchers, stop behavior and config-file replacements. Our remote models do not decode/use it. Local readiness/stop profiles are not equivalent. |
| P1 | Standard runtime environment values are missing. | Upstream derives `SERVER_IP`, `SERVER_PORT`, `SERVER_MEMORY`, `STARTUP` and `TZ`. Ours only substitutes allowlisted values found in the panel's ordinary environment map, so a template using a derived value may fail even with a valid allocation. |
| P1 | Sync does not update active job limits. | Upstream calls `InSituUpdate`; ours changes cached settings but applies Job Object limits only on the next launch. Displayed limits can therefore differ from active enforcement after sync. |
| P1 | Power API behavior differs. | Ours ignores `wait_seconds`, returns 409 for all busy actions and queues suspended starts before reporting failure in console. Upstream has specific lock-wait, suspension and lifecycle-state behavior. |
| P1 | File contents and metadata behavior is incomplete. | Upstream supplies MIME/content-length/download behavior and real filesystem metadata. Ours forces text content, caps at 8 MiB, uses generic MIME and reports modification time as creation time. |
| P1 | System and resource information is partial. | The v2 system response lacks fields such as `system.os_type` and `system.memory_bytes`; Docker counts are placeholders. Disk/network usage are zero placeholders with a native-only availability extension that the normal panel does not automatically understand. |
| P1 | Browser HTTP integration is incomplete. | Upstream has CORS headers and unauthenticated OPTIONS/preflight handling. Our WebSocket origin validation exists, but equivalent HTTP CORS/preflight support does not. This matters for future direct-browser upload/download workflows. |
| P1 | Tests do not currently enforce the upstream contract. | File tests use our nonstandard listing URL and schema. They can pass while the real panel's file browser fails. Add independent fixtures from upstream and the panel, rather than expectations copied from our implementation. |

## HTTP route inventory

`{s}` means server UUID; `{b}` means backup UUID. “Partial” means a handler exists at the correct method/path but its full upstream behavior has not been matched or live-verified. The normal server-root/auth setup is a prerequisite for every supported server route.

| Method | Upstream route | Windows Native status |
| --- | --- | --- |
| GET | `/download/backup` | Missing: scoped signed download, archive streaming. |
| GET | `/download/file` | Missing: scoped signed file download. |
| POST | `/upload/file` | Missing: scoped upload, multipart streaming and permissions. |
| GET | `/api/servers/{s}/ws` | Partial: basic console/status/stats/auth present. |
| POST | `/api/transfers` | Missing: destination-side transfer reception. |
| POST | `/api/update` | Missing: runtime configuration update/token rotation. |
| GET | `/api/system` | Partial: basic Windows information, incomplete v2 fields/placeholders. |
| GET | `/api/servers` | Partial: only locally preconfigured profiles registered. |
| POST | `/api/servers` | Missing: automatic creation and provisioning. |
| DELETE | `/api/transfers/{s}` | Missing: destination transfer cancellation/cleanup. |
| POST | `/api/deauthorize-user` | Partial: WebSocket revocation exists; SFTP absent. |
| GET | `/api/servers/{s}` | Partial: status/configuration/utilization subset. |
| DELETE | `/api/servers/{s}` | Missing: stop, cancel work, unregister, cleanup and delete. |
| GET | `/api/servers/{s}/logs` | Partial: in-memory history only; ignores requested `size`. |
| POST | `/api/servers/{s}/power` | Partial: four basic actions, differing lock/wait/error semantics. |
| POST | `/api/servers/{s}/commands` | Partial: stdin only; additional local bounds/error differences. |
| POST | `/api/servers/{s}/install` | Missing: installer lifecycle, output and callbacks. |
| POST | `/api/servers/{s}/reinstall` | Missing: coordinated reinstall workflow. |
| POST | `/api/servers/{s}/sync` | Partial: basic settings/suspension only. |
| POST | `/api/servers/{s}/ws/deny` | Partial: legacy JTI revocation present. |
| POST | `/api/servers/{s}/transfer` | Missing: source transfer orchestration/archive push. |
| DELETE | `/api/servers/{s}/transfer` | Missing: source transfer cancellation. |
| GET | `/api/servers/{s}/files/contents` | Partial: confined reading, incomplete upstream headers/denylist. |
| GET | `/api/servers/{s}/files/list-directory` | Missing at correct path; nonstandard `/files/list` helper exists. |
| PUT | `/api/servers/{s}/files/rename` | Missing. |
| POST | `/api/servers/{s}/files/copy` | Missing. |
| POST | `/api/servers/{s}/files/write` | Partial: confined writing, no egg denylist/disk accounting. |
| POST | `/api/servers/{s}/files/create-directory` | Missing. |
| POST | `/api/servers/{s}/files/delete` | Missing. |
| POST | `/api/servers/{s}/files/compress` | Missing. |
| POST | `/api/servers/{s}/files/decompress` | Missing. |
| POST | `/api/servers/{s}/files/chmod` | Missing; requires a deliberate POSIX-to-Windows permission policy. |
| GET | `/api/servers/{s}/files/pull` | Missing: active download status. |
| POST | `/api/servers/{s}/files/pull` | Missing: guarded remote fetch, progress and cancellation. |
| DELETE | `/api/servers/{s}/files/pull/{download}` | Missing: cancel remote fetch. |
| POST | `/api/servers/{s}/backup` | Missing: local/S3 backup pipeline. |
| POST | `/api/servers/{s}/backup/{b}/restore` | Missing: coordinated restore and status notification. |
| DELETE | `/api/servers/{s}/backup/{b}` | Missing: backup deletion. |

Returning 501 is an honest unsupported response, not a compatible implementation. For absent signed/JWT endpoints, our generic bearer gate can instead return 401 to a legitimate upstream-style request; those endpoints need their own appropriate authentication flow.

## Missing subsystems beyond routes

### Egg execution and provisioning

Wings consumes egg settings/process configuration supplied by the panel; it does not contain the whole egg catalog. The upstream installer retrieves `container_image`, `entrypoint` and `script`, pulls the image and executes `/mnt/install/install.sh` in a constrained installation container. Our manual `--mark-installed` callback is not that installer.

Required Windows work:

- A defined Windows runtime/installer profile format with OS/architecture checks, executable/dependency requirements, variables, arguments, working directory, readiness and stop strategy.
- Approved native installers: PowerShell/script runners or managed download/extraction/SteamCMD recipes, with isolated execution, progress, cancellation, timeouts and success/failure callbacks. SteamCMD and updates are recipe/game behavior, not a universal built-in Wings updater.
- Full install/reinstall state and power-operation locking; respect skip-install settings and prevent accidental concurrent destructive operations.
- Startup config-file rewriting equivalent to upstream `parser`: text, JSON, YAML, INI, properties and XML, including template replacement semantics.
- Egg-driven literal/regex readiness matching and ANSI stripping; a defined mapping for command stops, signal stops and native runtime stops.
- Derived environment values and safe variable handling. Existing Linux invocation strings must be rejected or mapped explicitly, not blindly executed by a Windows shell.
- A reviewed Windows profile for each supported game. A game without a supported Windows executable cannot be added through profile metadata alone.

### Lifecycle, recovery and console

- Crash detection with exit-reason reporting, restart policy and repeated-crash suppression. The panel's crash-detection flag is currently dropped.
- Persisted desired/runtime states, reboot recovery and interrupted-install/restore reconciliation with the panel.
- A process-supervision architecture for daemon restarts. Current kill-on-close jobs terminate games when the daemon exits; upstream can reattach to surviving Docker containers. Simply persisting a PID cannot recover redirected pipes or prevent PID-reuse mistakes.
- Durable console logs and rotation instead of only in-memory history.
- Upstream-style power-lock waiting and emergency kill; coordinated installing/restoring/transferring/deleting states.
- WebSocket install, backup, restore, transfer, daemon-message and deletion events, with the corresponding permissions, close behavior and payload shapes.
- Console-output and command/event throttling, per-server/user connection limits and suspension handling. Current bounded queues and global 256-socket cap do not match these policies.
- Windows console adapters where needed: ConPTY/control-event support, game log tailing or RCON. These are Windows/game integrations, not proof that upstream contains a generic RCON subsystem.

### Files, backups and transfers

- Full file manager/SFTP semantics, efficient streaming, MIME metadata, large-file support and consistent errors.
- Egg denylist enforcement across every file-access path, not only the editor.
- SFTP listener/host keys, panel-backed password/public-key authentication, per-operation user permissions, read-only mode, session limits, revocation and activity reporting.
- Archive formats and safe extraction with traversal/reparse-point, case-collision, reserved-name and expansion/quota defenses.
- Remote fetch with the upstream protections against internal/loopback destinations, DNS rebinding and unsafe redirects; progress, cancellation and quotas.
- Local and S3/multipart backups, ignore patterns and `.pteroignore`, checksums/sizes, panel callbacks, signed downloads, deletion, failure cleanup and bandwidth limits.
- Restore state/locking, stop-before-restore, optional root truncation, safe extraction and completion callbacks/events. Windows file locks and long paths need explicit tests.
- Source/destination node transfers with scoped authentication, streamed archives, integrity checks, progress, cancellation, rollback and panel callbacks. Transferring files between Linux and Windows does not convert their binaries or make a Linux egg runnable.

### Limits, networking, mounts and isolation

- Accurate aggregate job CPU/memory statistics, rather than root-process-only sampling. Match the intended memory-accounting/overhead policy and distinguish memory-limit exits from other failures where Windows makes that possible.
- Live limit updates, CPU affinity/thread assignments, disk accounting and write checks, plus explicit supported mappings for swap/OOM and I/O-weight settings. POSIX cgroups and Windows jobs are different systems.
- Disk enforcement that covers HTTP, SFTP, archives, restores, installers and game writes. HTTP write accounting alone does not stop a native executable from filling the disk.
- Allocation handling, bind/port validation, per-server network restrictions, DNS and outgoing-IP behavior. Processes currently share the host network and can bind conflicting/unassigned ports if the account permits it.
- Custom shared/read-only mounts with an allowlist and tested ACL semantics. A directory junction is not inherently a read-only security boundary.
- A real isolation decision. Upstream configures user identity, a read-only container root, dropped capabilities, no-new-privileges, mounts and Docker networking. Our games share the daemon account, including potential access to node credentials and other server files.

Separate Windows identities, restricted tokens and ACLs are useful native controls, but cannot simply be declared equivalent to container isolation. A stronger Windows container/VM design should be evaluated and tested if mutually untrusted hosting is a goal; game compatibility and host prerequisites must be checked. Merely adding Docker for Windows does not make Linux images run as native Windows containers.

### Node operations, reporting and configuration

- `/api/update`, durable configuration updates and safe credential rotation for incoming API, outgoing panel calls and token verification.
- Panel-generated configuration onboarding or a supported converter; upstream YAML is not accepted by our JSON loader.
- Complete system information/capability representation without presenting placeholder values as measured data.
- Persistent activity/audit storage and retrying delivery to the panel, including console/power/SFTP/file events and user/IP attribution.
- Remote API retry/backoff and callback recovery; startup resetting/reconciling interrupted panel states.
- HTTP CORS/preflight, request IDs, compatible errors, proxy/client-IP policy and authentication-appropriate timeouts for long authenticated streams.
- Diagnostics, persistent/rotated daemon logs, service/Event Log integration, certificate renewal and tested service/reboot/update behavior. Upstream also has configure/diagnostics/automatic-TLS options; Windows equivalents need an explicit support policy.

## Things owned by the panel

The egg catalog, user-facing schedule definitions, database provisioning integrations and server-allocation records are panel responsibilities. Windows Wings should implement the operations the panel requests; it does not need a duplicate panel scheduler or database server just for API parity. Scheduled commands/power can use existing daemon routes once behavior is correct; scheduled backups need the missing backup implementation.

A Windows panel extension would help provide profile selection, OS/capability labels, installer configuration and appropriate UI for unsupported/Linux-only settings. It is not a substitute for daemon behavior or operating-system compatibility. Our custom `capabilities` field alone does not cause the existing panel to hide unsupported actions.

## Recommended implementation order

| Stage | Deliverable | Required evidence |
| --- | --- | --- |
| 1. Correct the existing contract | Listing URL/schema, file metadata/denylist, derived variables, emergency kill, power responses and system payloads. | Independent panel/upstream fixtures; real-panel demo start/console/status/file editing; negative-permission tests. |
| 2. Set the isolation architecture | Decide trusted private runtime versus stronger isolation; separate daemon credentials from game identities where feasible. | Cross-server/account/network escape tests; explicit supported threat model. Do not market public-hosting parity before this. |
| 3. Windows provisioning and core operations | Versioned Windows profiles, installer/reinstaller, config parsers, create/delete, crash/reboot recovery and console adapters. | At least one real Windows game completes create, install, start, stop, restart, reinstall and delete from the panel. |
| 4. Files and storage | Full file API, SFTP, uploads/downloads, guarded remote fetch, archives, disk accounting/enforcement. | Subuser permissions, large files, NTFS junctions/symlinks/hardlinks, long paths, locked files, disk-full and malicious archive tests. |
| 5. Backups and transfers | Local/S3 backup/restore and native-to-native node transfer with callbacks/events. | Failure/cancel/reboot recovery and integrity tests; restore a real server and boot it successfully. |
| 6. Operational parity | Live limits, aggregate statistics, networking/mount policy, token rotation, audit delivery, diagnostics and service lifecycle. | Sustained load/multi-server tests on supported Windows Server versions and each supported panel version. |
| 7. Unchanged Linux workloads, if required | Retain/add a Linux Wings execution backend and route workloads by OS/runtime. | Existing Linux eggs tested on the Linux backend; Windows recipes tested on Windows. No universal native compatibility claim. |

Where practical, reuse upstream's portable route/state/parser/backup logic behind platform interfaces instead of independently reproducing every behavior. The upstream `environment.ProcessEnvironment` is a useful seam, but `server/manager.go` hardcodes Docker, the installer creates Linux containers, and `internal/ufs` relies on Unix primitives. A genuine shared-core port must separate those dependencies first; inserting a Windows process implementation alone is insufficient.

## Source map

The local upstream reference checkout is `.reference-wings/` (ignored by Git). Reproducible upstream links:

- [Registered HTTP routes](https://github.com/pterodactyl/wings/blob/747bc4393daf5d95fdf61d76382f5e23749cffe9/router/router.go)
- [Egg/process and installation response types](https://github.com/pterodactyl/wings/blob/747bc4393daf5d95fdf61d76382f5e23749cffe9/remote/types.go)
- [Installer and Linux container execution](https://github.com/pterodactyl/wings/blob/747bc4393daf5d95fdf61d76382f5e23749cffe9/server/install.go)
- [Startup configuration parsing](https://github.com/pterodactyl/wings/blob/747bc4393daf5d95fdf61d76382f5e23749cffe9/parser/parser.go)
- [Power locking and pre-start behavior](https://github.com/pterodactyl/wings/blob/747bc4393daf5d95fdf61d76382f5e23749cffe9/server/power.go)
- [Crash handling](https://github.com/pterodactyl/wings/blob/747bc4393daf5d95fdf61d76382f5e23749cffe9/server/crash.go)
- [State persistence and Docker construction](https://github.com/pterodactyl/wings/blob/747bc4393daf5d95fdf61d76382f5e23749cffe9/server/manager.go)
- [File metadata contract](https://github.com/pterodactyl/wings/blob/747bc4393daf5d95fdf61d76382f5e23749cffe9/server/filesystem/stat.go)
- [SFTP server](https://github.com/pterodactyl/wings/blob/747bc4393daf5d95fdf61d76382f5e23749cffe9/sftp/server.go)
- [Backup coordination](https://github.com/pterodactyl/wings/blob/747bc4393daf5d95fdf61d76382f5e23749cffe9/server/backup.go)
- [Transfer implementation](https://github.com/pterodactyl/wings/tree/747bc4393daf5d95fdf61d76382f5e23749cffe9/server/transfer)
- [Docker runtime/security configuration](https://github.com/pterodactyl/wings/blob/747bc4393daf5d95fdf61d76382f5e23749cffe9/environment/docker/container.go)
- [Remote callbacks](https://github.com/pterodactyl/wings/blob/747bc4393daf5d95fdf61d76382f5e23749cffe9/remote/servers.go)
- [Panel activity delivery](https://github.com/pterodactyl/wings/blob/747bc4393daf5d95fdf61d76382f5e23749cffe9/internal/cron/activity_cron.go)

Our implementation was compared in `internal/api/{api,files,websocket}.go`, `internal/node/{panel,server}.go`, `internal/native/process_windows.go`, `internal/config/config.go`, the CLI/service entry points and their tests.
