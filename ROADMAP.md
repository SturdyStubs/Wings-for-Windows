# Progress toward Wings compatibility

Last updated: October 22, 2026. Current version: **0.1.0-alpha.42**. Active development: **M3 — Windows lifecycle features**, with **M4 directory creation, rename/move and bounded file copying** implemented. M1/M2 remain open for live panel and isolation gates.

[Repository](https://github.com/SturdyStubs/Wings-for-Windows) · [Tracking issues](https://github.com/SturdyStubs/Wings-for-Windows/issues) · [Changelog](CHANGELOG.md) · [Original audit](docs/WINGS-PARITY-AUDIT.md) · [Verification](docs/VERIFICATION.md)

This file tracks the current implementation. The audit preserves the alpha 1 baseline. A checked implementation item means code and local regression tests exist; a live verification item stays open until exercised against a real panel/host/game. Route coverage is not a completion percentage.

## Current state

- Native Windows EXE runtime, Job Objects, basic power/console/status and root-process statistics: locally tested.
- Alpha 2 fixes: canonical file listing and metadata, file MIME/length headers, egg file-denylist policy, standard derived variables, suspended power response and emergency kill cancellation: locally tested.
- Alpha 3: opt-in supported Windows egg readiness/stopping, live job limit changes/removal, preservation of local caps and suspension despite invalid egg settings: locally tested.
- Alpha 4: bounded asynchronous power waits, queued-action cancellation, measured Windows host/system schema, CORS preflight, request IDs, JSON errors and log sizing: locally tested.
- Alpha 5: profile schema v1 with legacy migration, executable/OS/dependency checks before start and installation acknowledgement, and an offline profile-check CLI: locally tested. The native isolation model is documented; alpha 6 adds an initial opt-in launch path, while full enforcement remains open.
- Alpha 6: opt-in local identity references, pinned/distinct SIDs, per-server Credential Manager lookup, restricted-token launch and protected-path DACL rejection checks. Restricted-token/process and ACL fixtures pass locally; successful separate-account Windows Server deployment remains unverified.
- Alpha 7: handle-based ACL/metadata checks, full ancestor audit, bounded recursive other-server scans, observed reparse-point and multi-hardlink rejection. A real restricting-token fixture proves exposed children are detected even when root access is denied. These are snapshots, not complete race/network confinement.
- Alpha 8: daemon process/primary-token DACL guards, configured executable/dependency write audits, deterministic merged rules and explicit rejection of unsupported permission ACEs. A restricted-token OS test verifies actual denial of daemon memory reads and token duplication; separate-account deployment remains unverified.
- Alpha 9: exact-UUID credential metadata/removal CLI, idempotent missing-entry handling and explicit same-account credential rotation with overwrite protection. Fake-credential lifecycle fixtures pass; Windows account creation/password changes/recovery and service validation remain open.
- Alpha 10: `--check-identity <UUID>` emits ordered JSON readiness checks without panel access, game launch or root creation. It can explicitly attempt batch logon and run the launch-time guards; missing-credential/cancellation/failure reports are locally tested. No successful second-account Windows Server deployment is claimed.
- Alpha 11: optional account-scoped WFP connection-blocking path before process creation, dynamic session rollback and job-aware cleanup. Contract/injected-backend and real-job/fake-lease tests pass; actual filter installation/traffic behavior is not live verified.
- Alpha 12: explicit `--check-network-policy <UUID>` diagnostic with identity gating, owned-rule readback and independent cleanup inspection. Injected-backend success/failure/cancellation checks pass; no real filter installation or traffic result is claimed.
- Alpha 13: read-sharing-only startup handles retain declared protected paths, executable, working directory and ancestors across ACL audit, WFP setup and suspended process creation/resume. Audit handles now request data/list access for effective no-delete sharing. Real write/rename/rollback/bound/link and pinned process-launch tests pass; runtime/ACL/mapping/loader races remain open.
- Alpha 14: required game-identity DACL grants for executable read/execute, declared dependency reads and working/temporary-directory list/traverse/create permissions, shared between startup and the new diagnostic `launch_access` stage. Actual restricted-token file opens and allowed/denied temporary-file creation match the checks. No second-account/live-host validation is claimed.
- Alpha 15: portable compiled native tests plus a hash/revision-checked PowerShell host-report runner and staged Windows Server VM instructions. Local no-Go execution passes 14 runtime cases/subtests with zero skipped; host evidence explicitly excludes panel, distinct-account, service and network-filter verification. Server VM results remain pending.
- Alpha 16: resolve host-runner path defaults after parameter binding, fixing the reported Windows PowerShell `-File` failure when `$PSScriptRoot` is empty in a parameter initializer. Clean release builds exercise both omitted defaults from outside the package. The user reports 14 passing cases on the Server 2022 VM using the alpha 15 explicit-path workaround; the JSON metadata and executable hashes match the retained alpha 15 package (build 20348, Administrator token). Distinct-account/service/panel/network gates remain open.
- Server 2022 follow-up: direct alpha 16 default runner passes all 14 cases/subtests. Real separate-game-account launch passes eight fixture stages from both SYSTEM and non-administrator daemon batch tasks, including actual file/memory denial and console/stop. Key-SSH credential storage failed; password-backed batch storage worked. SCM, profile cleanup and remaining isolation gates stay open. See [evidence](docs/evidence/server2022-alpha16-ssh-identity.md).
- Alpha 17: opt-in local service-account creation, scoped service/launch/batch rights and protected initial bin/private/data/logs ACLs. Managed setup rejects populated data, shared identities and observed links, and compensates failed service registration/start. Actual Server 2022 SCM demo start/console/stop/restart passed under a non-administrator daemon with a separate game identity and loopback mock panel. Game credentials still require credential-backed provisioning; cold boot/profile recovery, real panel/game and WFP gates remain open. See [service setup](docs/SERVICE-PROVISIONING.md).
- Alpha 18: add an elevated managed-node command for creating, inspecting, deliberately replacing and removing a configured game's credential in the stopped daemon account's credential store. Bind to the registered service identity and pinned game SID; use a bounded SID-restricted pipe, temporary password-backed task and narrowly scoped batch right with cleanup. The final package passes 18 Server 2022 public-helper/SCM checks, including bad-password cleanup, lock contention and preserved preexisting rights. Full suite: 231 cases/subtests, zero skipped. See [evidence](docs/evidence/server2022-alpha18-credentials.md); cold reboot/profile recovery and actual account-password changes remain open.
- Alpha 19: opt-in bounded game crash recovery after unexpected nonzero exits, fresh launch/security checks, rolling attempt budget and cancellation on manual power/suspension/shutdown. Native-only API metadata and console/log reasons are available. State is in memory; cold boot/profile recovery is deferred at the operator's request, not verified. Local suite: 258 cases/subtests, zero skipped; final Server 2022 package: 22 SCM/credential/recovery checks passed. See [recovery guide](docs/CRASH-RECOVERY.md) and [evidence/cleanup limits](docs/evidence/server2022-alpha19-recovery.md).
- Alpha 20: opt-in persistent per-server console history with bounded JSONL rotation, asynchronous writes, explicit drop/error status and restored recent lines after service restart. Protect shared logs from game access, reject observed links/duplicate writers and keep storage outside server data. Game/recovery state and daemon diagnostics are not persisted. Local suite: 294 cases/subtests, zero skipped; final package passes 14 native and 25 SCM/credential/recovery/logging checks on Server 2022. See [console logging](docs/CONSOLE-LOGS.md) and [evidence/cleanup limits](docs/evidence/server2022-alpha20-console.md).
- Alpha 21: fixed, secret-safe Windows Application lifecycle/failure events, explicit managed source registration and SCM Running only after initial sync/TLS/listener setup. Source registration and log failures do not grant runtime privileges. Full suite: 317 cases/subtests, zero skipped; exact package passes 14 native and 29 SCM/credential/recovery/logging/events checks on Server 2022. See [service diagnostics](docs/SERVICE-EVENTS.md) and [evidence/cleanup limits](docs/evidence/server2022-alpha21-service-events.md).
- Alpha 22: opt-in persistent automatic-restart reservations in the existing private log store, write-before-launch, conservative retention, fail-closed startup validation and write-error status. Games/timers are not automatically restored. Full suite: 345 cases/subtests, zero skipped; exact package passes 14 native and 33 service/persistence checks on Server 2022, including forced daemon termination and corrupt-state startup refusal. See [recovery state](docs/RECOVERY-STATE.md) and [evidence/cleanup limits](docs/evidence/server2022-alpha22-recovery-state.md).
- Alpha 23: opt-in plain-text egg file parsing before launch with exact local target approval, bounded prefix/scalar rules, full-batch input preflight, handle-based link/code protection and in-place writes preserving ACLs. Other parsers and transaction recovery remain open. Full suite: 394 cases/subtests, zero skipped; exact package passes 14 native and 35 service/persistence/text checks on Server 2022, including missing-file startup refusal and UTF-8 edits with unchanged SDDL. See [text configuration](docs/EGG-TEXT-CONFIG.md) and [evidence/cleanup limits](docs/evidence/server2022-alpha23-egg-text.md).
- Alpha 24: approved JSON egg configuration before launch, nested keys/arrays/one wildcard level, literal conditions, scalar coercion and untouched number precision. Enforce structural, mutation and encoding bounds with mixed text/JSON preflight before writes. Full suite: 425 cases/subtests, zero skipped; exact package passes 14 native and 37 combined checks on Server 2022, including malformed mixed-batch refusal and actual typed/precise JSON edits with preserved SDDL. See [JSON guide](docs/EGG-JSON-CONFIG.md) and [evidence/cleanup limits](docs/evidence/server2022-alpha24-egg-json.md).
- Alpha 25: approved properties parsing before launch with literal conditions, escaped keys, continuations and Unicode; disable environment/property expansion and use a bounded Java-style encoder. Reuse mixed-batch preflight and guarded writes; package the pinned dependency license. All 446 local cases/subtests pass with zero skips; exact package passes 14 native and 39 combined Server 2022 checks, including malformed third-file preservation and actual Unicode/literal/conditional edits with retained SDDL. See [properties guide](docs/EGG-PROPERTIES-CONFIG.md) and [evidence/cleanup limits](docs/evidence/server2022-alpha25-egg-properties.md).
- Alpha 26: bounded INI parsing with default/section/bracketed paths, exact section updates and literal interpolation values. Preserve Windows backslashes, reject unsupported conditions/multiline/dialects, encode with bounds and verify round-trip data before writes. Pin the upstream INI dependency and package its Apache license. All 483 local cases/subtests pass with zero skips; exact package passes 14 native and all 41 combined Server 2022 checks, including malformed fourth-file preservation and actual section/Unicode/literal/path/quote edits with retained SDDL. See [INI guide](docs/EGG-INI-CONFIG.md) and [evidence/cleanup limits](docs/evidence/server2022-alpha26-egg-ini.md).
- Alpha 27: approved YAML/yml parsing with JSON paths/scalar rules, exact numeric spelling, explicit alias/tag/document restrictions, input/result bounds and capped stream encoding with semantic reparse. Reuse mixed-batch preflight and guarded writes; package the pinned dependency notices. All 525 local cases/subtests pass with zero skips; exact package passes 14 native and all 43 combined Server 2022 checks, including malformed fifth-file preservation and actual typed/precise/conditional/literal/Unicode edits with retained SDDL. See [YAML guide](docs/EGG-YAML-CONFIG.md) and [evidence/cleanup limits](docs/evidence/server2022-alpha27-egg-yaml.md).
- Alpha 28: bounded namespace-free XML text/attribute rewriting, exact/wildcard element paths, DTD/entity/mixed-content refusal and bounded streaming round-trip verification. Add opt-in two-phase reboot fixture with delayed startup and unattended checks/cleanup. All 558 local cases/subtests, 14 native cases, 45 combined Server 2022 checks and seven actual post-boot checks pass; pinned SSH reconnect and unattended cleanup verified. See [XML guide](docs/EGG-XML-CONFIG.md), [reboot scope](docs/REBOOT-VALIDATION.md) and [evidence](docs/evidence/server2022-alpha28-xml-reboot.md).
- Alpha 29: opt-in persisted running/stopped intent with private dual snapshots, manual-stop preservation, readiness-gated/budgeted replay, two startup workers and fresh launch guards. Add SCM and automatic post-boot validation modes. See [restoration guide](docs/RUNNING-RESTORATION.md); 584 local cases/subtests, 14 native cases, 49 combined service checks and seven actual post-boot checks pass, with pinned SSH reconnect and unattended/independent cleanup. See [evidence](docs/evidence/server2022-alpha29-running-restoration.md).
- Alpha 30: opt-in loopback Source TCP RCON commands and graceful stops, bounded complete replies, daemon environment secret reference, redaction, cancellation and no retry/stdin fallback. See [adapter scope](docs/SOURCE-RCON.md); 619 local cases/subtests, 14 packaged Server 2022 cases and all 54 combined service checks pass, with independent cleanup. See [evidence](docs/evidence/server2022-alpha30-source-rcon.md).
- Alpha 31: opt-in game log-file console/readiness, stale-history skipping, bounded append/rotation/truncation polling, safe filesystem checks, native cancellation and status. See [guide](docs/LOG-FILE-CONSOLE.md); 654 local cases/subtests, 14 native cases and all 59 combined Server 2022 checks pass, with independent cleanup. See [evidence](docs/evidence/server2022-alpha31-log-file.md).
- Alpha 32: opt-in per-server RCON vault references, scoped metadata/removal CLI and SecureString/service-account provisioning with deliberate rotation. Preserve environment compatibility and reject cross-server references or missing-secret fallback. See [guide](docs/RCON-CREDENTIALS.md); 672 local cases/subtests, 14 native cases and all 66 combined Server 2022 checks pass, with independent cleanup. See [evidence](docs/evidence/server2022-alpha32-rcon-credentials.md).
- Alpha 33: approved native data/setup install and reinstall routes, shared lifecycle locking, restricted game identity, bounded output/time/memory, cancellation and Wings completion callbacks. Add a packaged demo egg/profile and first-node walkthrough. Protected game binaries remain operator-provisioned; server creation, binary promotion, durable installer recovery and live panel/game verification remain open. See [installer scope](docs/NATIVE-INSTALLERS.md) and [node setup](docs/PANEL-WINDOWS-SETUP.md). All 707 local cases/subtests, 14 packaged native cases and 73 combined Server 2022 checks pass; independent cleanup verified. See [evidence](docs/evidence/server2022-alpha33-native-installers.md).
- Alpha 34: opt-in creation acceptance pending manual UUID/profile approval, fresh node-assignment lookup, no unapproved execution/files/callbacks, idempotent pending replay and reconstruction from the panel after restart. Zero-profile bootstrap avoids requiring the UUID before it exists. Automatic install/start, account/permission provisioning and real panel certification remain open; see [scope](docs/PENDING-CREATION.md). All 731 local cases/subtests, 14 packaged native cases and 79 combined Server 2022 checks pass; independent cleanup verified. See [evidence](docs/evidence/server2022-alpha34-pending-creation.md).
- Alpha 35: opt-in durable installer receipts, completed-result notification retry without execution, conservative interrupted-attempt resolution and fail-closed storage/phase validation. No automatic callback retry, setup rollback or full transactional recovery is claimed. See [installer recovery](docs/INSTALL-RECOVERY.md). All 761 local cases/subtests, 14 packaged native cases and 89 combined Server 2022 checks pass, including daemon interruption during setup; independent cleanup verified. See [evidence](docs/evidence/server2022-alpha35-install-recovery.md).
- Alpha 36: incremental game permissions on an existing stopped managed node for a precreated account and empty UUID target, with WhatIf, bounded validation and compensating rollback. Older populated data/logs remain unchanged. All 774 local cases/subtests, 14 packaged native cases and 96 combined Server 2022 checks pass; independent three-identity cleanup verified. Account creation and broader provisioning remain open. See [workflow](docs/GAME-PERMISSIONS.md) and [evidence](docs/evidence/server2022-alpha36-game-permissions.md).
- Alpha 37: operator-only game-account creation for an unused UUID/name, with SecureString input, WhatIf, collision/lock guards, SID/profile output and owned-SID validation rollback. The created account completes new-UUID permission, credential, installer and game launch handoff. All 791 local cases/subtests, 14 packaged native cases and 101 combined Server 2022 checks pass; independent three-identity cleanup verified. Automatic panel-driven provisioning, password rotation/deletion and broader lifecycle remain open. See [workflow](docs/GAME-ACCOUNTS.md) and [evidence](docs/evidence/server2022-alpha37-game-accounts.md).
- Alpha 38: manifest/hash-approved local game-file import with source sharing guards, private staging, flushed copy verification, daemon read-only code ACLs and same-parent new-directory publication. Refuse overwrites and stale staging; the imported executable completes dedicated-account setup/game launch and dependency write denial. All 815 local cases/subtests, 14 packaged native cases and 107 combined Server 2022 checks pass; independent three-identity cleanup verified. Automatic downloads/updates, durable promotion recovery and panel-driven provisioning remain open. See [workflow](docs/GAME-FILES.md) and [evidence](docs/evidence/server2022-alpha38-game-files.md).
- Alpha 39: bounded staging inspection and fingerprint-approved quarantine into administrator-only private storage, with retained bytes, no deletion/publication and explicit stale-content/ACL, path-reference and maintenance-lock guards. All 833 local cases/subtests, 14 packaged native cases and 114 combined Server 2022 checks pass using simulated interrupted-copy staging followed by the normal import/setup/game handoff; independent cleanup verified. Actual crash/reboot and durable promotion recovery remain open. See [workflow](docs/GAME-FILE-RECOVERY.md) and [evidence](docs/evidence/server2022-alpha39-stage-recovery.md).
- Alpha 40: panel-compatible nested directory creation with whole-path preflight, case-insensitive denylist checks, reparse refusal, confined parent traversal, bounded Windows names and inherited directory permissions. All 871 local cases/subtests, 14 packaged native cases, 38 portable API cases/subtests and 23 focused Server 2022 service checks pass; independent cleanup verifies removal with one retained/loaded profile. See [exact evidence](docs/evidence/server2022-alpha40-file-directories.md). Rename/copy/delete, scoped upload/download and SFTP remain open. See [directory scope](docs/FILE-DIRECTORIES.md).
- Alpha 41: panel-compatible file/directory rename/move with case-only names, confined relative parent moves, batch preflight, descendant denylist scans, pinned sources and native no-replace destination protection. Existing destination parents are required; chains/swaps/overlaps are refused and execution is not transactional. All 905 local cases/subtests, 14 packaged native VM cases, 72 portable file API cases/subtests and 30 focused SCM checks pass; independent cleanup passes with one retained/loaded profile. See [scope](docs/FILE-RENAME.md) and [exact evidence](docs/evidence/server2022-alpha41-file-rename.md).
- Alpha 42: bounded regular-file copies with upstream names, binary streaming, locked-source/no-link checks, owned temporary staging, exact-handle cleanup and no-replace publication. One-GiB source bound and 51 candidate names; quotas and durable orphan recovery remain open. All 931 local cases/subtests, 14 packaged native VM cases, 98 portable file API cases/subtests and 35 focused SCM checks pass; independent cleanup passes with one retained/loaded profile. See [scope](docs/FILE-COPY.md) and [exact evidence](docs/evidence/server2022-alpha42-file-copy.md).
- HTTP inventory: handlers at **19 of upstream's 38 method/path combinations**, with partial behavior; **19 remain absent**. SFTP, security and recovery are additional work outside this count. Legacy `/files/list` is an extra alias, not an upstream route.
- Automatic binary provisioning, full file manager/SFTP, backups/restores, transfers, strong isolation and live certification remain open.
- Unchanged Linux eggs require a Linux execution backend. Native Windows profiles do not translate Linux images/binaries.

## M1: Core protocol compatibility — in progress

[Tracking issue #1](https://github.com/SturdyStubs/Wings-for-Windows/issues/1)

- [x] Compare implementation with upstream source and preserve a reproducible audit.
- [x] Correct `/files/list-directory` and the `file`, `directory`, `symlink`, `mime`, `created`, `modified` metadata contract.
- [x] Supply MIME, Content-Length and attachment headers on authenticated file reads.
- [x] Apply bounded, case-insensitive egg denylist patterns to listing, reads and writes; refresh on sync/start.
- [x] Check resolved handles before returning contents/truncating files; preserve server-root confinement.
- [x] Derive `SERVER_IP`, `SERVER_PORT`, `SERVER_MEMORY`, `STARTUP` and `TZ`; prevent egg variables overriding derived allocation values.
- [x] Let emergency kill interrupt stop/restart and cancel a pending start; prevent cancelled restart from relaunching a game.
- [x] Return HTTP 400 for suspended start/restart requests.
- [x] Add upstream/panel-derived file contract fixtures and Windows regression tests.
- [x] Decode and use opted-in egg `process_configuration` for supported Windows command/SIGKILL stop and literal/regex/ANSI readiness behavior; reject unsupported strategies explicitly.
- [x] Match `wait_seconds`, remaining power errors and lifecycle locking; bound waiting requests and cancel them on kill/shutdown.
- [x] Update active Job Object CPU/memory limits during sync, including removal and retained local caps; verify actual Windows job state.
- [x] Complete system information, HTTP CORS/preflight and consistent error/request-ID handling.
- [ ] Verify node connectivity, console, subuser revocation and file browsing/editing through a real panel over trusted TLS.

M1 implementation tasks are locally verified; the real-panel TLS/console/subuser/file gate remains open. Next development target: implement the documented native identity policies in M2 and cancellation-aware Windows provisioning in M3. M1 cannot close until live validation passes.

## How to read progress

[GitHub Milestones](https://github.com/SturdyStubs/Wings-for-Windows/milestones) provide the live completion bars. Six milestones contain 35 deliverable issues; the six original tracking issues remain summaries outside milestone counts. At alpha 4, M1 has 10 of 11 deliverables closed; M2 has 1 of 4 decision/implementation deliverables closed and M3 has 1 of 6; all milestones remain open. This is issue completion, not an effort estimate or a claim that Wings is 91% ported.

README badges query milestone counts automatically. Close an implementation issue only with test evidence, and close a live gate only after actual host/panel/game validation. Close a milestone after every required deliverable is accepted. **Project completion requires all six milestones closed**, including the live gates. There is no declared completion date yet.

The seed definitions are in `docs/progress-plan.json`. `scripts/Update-Progress.ps1 -Initialize` adds missing planned milestones/issues and records seeded historical work; it does not re-close existing issues. Run without the switch to refresh README links. Reopened issues lower the live counts automatically, subject to badge caching. Issue state in GitHub is authoritative over historical seed metadata.

## M2: Isolation and supported threat model — in progress, required before untrusted hosting

[Tracking issue #2](https://github.com/SturdyStubs/Wings-for-Windows/issues/2)

- [x] Record the next native per-server identity/restricted-token/ACL model and separate virtualization direction for untrusted hosting in [the isolation decision](docs/ISOLATION.md); the initial native launch path is implemented but full policy enforcement and virtualization remain open.
- [ ] Separate daemon credentials and game identities; define filesystem, network and mount policies.
  - [x] Add optional per-server SID/credential references, config binding/uniqueness, daemon/admin-group rejection and no shared-account fallback.
  - [x] Add restricted-token suspended launch and a read-only protected-path DACL guard; verify real restricted child privileges, console and job cleanup.
  - [x] Add a secure-input credential helper and temporary fake-credential round-trip tests with cleanup.
  - [x] Audit ancestor replacement rights and existing children recursively using handle-derived descriptors; reject observed reparse points/multi-hardlinks, cap scan size/depth and honor startup cancellation.
  - [x] Audit the daemon process/primary token and declared executable/dependency files; test actual process memory/token access denial and reject unsupported applicable ACL entries.
  - [x] Add scoped metadata/removal commands and deliberate credential replacement; test fake-credential rotation, rejection, cleanup and missing-entry behavior without account logon.
  - [x] Add an explicit per-server deployment diagnostic with stage failures/not-run states and clear game/panel/network verification exclusions.
  - [x] Add opt-in SID-scoped WFP block installation/lifecycle paths with partial-install rollback and job-aware cleanup; verify injected-backend contracts and real-job/fake-lease ordering.
  - [x] Add explicit mutating WFP installation/readback/independent-cleanup diagnostic, gated by identity preflight and tested with an injected backend.
  - [x] Retain bounded startup handles for nonrecursive protected paths, executable, working directory and ancestors; reject conflicting writers and test rollback, link rejection and guarded image creation.
  - [x] Check required executable/dependency/data-directory permissions under the game token before launch and in offline identity diagnostics; verify fixture allow/deny results against actual restricted-token file operations.
  - [ ] Verify actual WFP installation, traffic blocking, crash/restart behavior and allocation/outbound policy on a disposable host.
  - [x] Verify a real distinct-game-account native launch from a non-administrator daemon password-backed batch task on Server 2022, with actual child SID and file/memory denial checks.
  - [x] Verify distinct-account demo launch under a non-administrator Windows Server SCM identity, including console, job-tree stop and restart (mock panel).
  - [x] Add service-account credential provisioning without operator-managed bootstrap tasks: create/inspect/explicit replace/remove with exact profile/SID binding and task/right cleanup.
  - [x] Verify one delayed-auto SCM deployment through an actual VM reboot, cold stored-credential game launch and history/budget restoration; see alpha 28 evidence.
  - [ ] Complete broader cold boot/profile recovery, immediate-start races and actual account-password lifecycle checks.
  - [ ] Complete account/credential lifecycle, recursive/ancestor/link/race policy, shared executable/mount rules, daemon process-handle and network enforcement.
- [ ] Test cross-server/account access, credentials, hardlinks, reparse points, case aliases and filesystem races. Local fixtures now cover a denied root with an exposed child, real NTFS hardlinks/junctions, ambiguous paths, handle-pinned rename/write rejection, retained startup handles with rollback and scan/handle bounds/cancellation; live identity and adversarial race gates remain open.
- [ ] Publish the supported Windows host/runtime/game matrix and remaining restrictions. The Server 2022 baseline and separate-account batch probes pass; real game, reboot/profile and full-policy certification remain open.

Current scope remains a trusted private node. File denylist/path checks are not process isolation.

## M3: Windows provisioning and lifecycle recovery — in progress

[Tracking issue #3](https://github.com/SturdyStubs/Wings-for-Windows/issues/3)

- [x] Versioned Windows profiles and dependency/OS checks; read-only CLI preflight and enforcement before start/manual installation acknowledgement. See [profile v1](docs/PROFILES.md).
- [ ] Create/delete, native install/reinstall, output/progress, cancellation, timeouts and panel callbacks.
  - [x] Execute approved native data/setup recipes with cancellation, bounded resources/output and install/reinstall callbacks using the game identity.
  - [x] Provide an importable harmless Windows demo egg, matching local profile and node setup walkthrough.
  - [x] Add opt-in panel-record creation acceptance pending operator-owned UUID/profile approval and assignment-based restart discovery.
  - [x] Persist the latest opted-in installer attempt/result/acknowledgement and explicitly reconcile notifications or interrupted failure without executing setup again.
  - [x] Prepare one precreated game identity and empty UUID directory on a stopped existing managed node, with bounded ACL validation, rollback and preserved other-server data/logs. [Operator workflow](docs/GAME-PERMISSIONS.md).
  - [x] Create one dedicated game account for an unused UUID/name through a stopped-node operator command, with SecureString input, profile-binding output and owned-SID validation rollback. [Workflow](docs/GAME-ACCOUNTS.md).
  - [x] Import hash-approved local files through protected staging and verified same-parent publication into a new UUID code directory, with no overwrites or game write grants. [Workflow](docs/GAME-FILES.md).
  - [x] Inspect bounded unfinished code staging and explicitly quarantine one fingerprint-approved tree with bytes retained and administrator-only ACLs. [Workflow](docs/GAME-FILE-RECOVERY.md).
  - [ ] Add automatic protected game-binary download/update/promotion recovery and a durable journal, panel-driven account/create/install/start, server deletion and broader interrupted-operation recovery.
- [ ] Egg configuration parsers for text, JSON, YAML, INI, properties and XML.
  - [x] Add opt-in plain-text file parser with exact local approval, startup integration and bounded/guarded writes.
  - [x] Add a bounded JSON subset with nested/array/wildcard paths, literal conditions, preserved numeric precision and shared startup integration.
  - [x] Add bounded properties parsing with literal conditions, safe escaping and disabled daemon environment/property expansion.
  - [x] Add bounded single-line INI parsing with section paths, literal values and round-trip verification.
  - [x] Add a bounded YAML/yml subset with JSON paths, literal conditions, preserved numbers and no alias expansion.
  - [x] Add bounded XML text/attribute rewriting with strict paths and streaming preflight.
  - [ ] Extend supported dialects, templates and conditions; initial family support is not full parser parity.
- [ ] Crash detection/restart policy, persistent state/logs, reboot and interrupted-operation recovery.
  - [x] Add opt-in bounded recovery for unexpected nonzero game exits with fresh launch guards, operator cancellation and status/reason reporting.
  - [x] Add opt-in persistent, bounded server console logs with restored recent history and observable storage failures.
  - [x] Add secret-safe service lifecycle/failure events and explicit Event Log source setup.
  - [x] Persist opted-in automatic-restart budgets across daemon restarts with reservation-before-launch and corruption/write failure handling.
  - [x] Add opt-in durable running/stopped intent and bounded startup restoration using persistent restart reservations.
  - [ ] Add broader persistent lifecycle state, daemon diagnostics and interrupted-operation recovery. The operator authorized reboot testing; a real delayed-start VM reboot passes seven scoped checks with SSH reconnect and unattended cleanup. Broader reboot recovery remains open.
- [ ] Console/control-event, log-file or RCON adapters where required by a game.
  - [x] Add bounded opt-in Source TCP RCON command/graceful-stop delivery with local endpoint approval and lifecycle cancellation.
  - [x] Add bounded game log-file console output and optional log-only readiness with stale-history skipping, rotation handling and cancellation.
  - [x] Add account-scoped per-server RCON credential references and managed service provisioning, rotation, metadata/removal and namespace separation.
  - [ ] Verify real games, extend RCON/log adapters, add Windows control events and validate broader account/profile/password lifecycle.
- [ ] Complete a real game's create → install → start → stop/restart → reinstall → delete workflow from the panel.

## M4: Full files and SFTP — in progress

[Tracking issue #4](https://github.com/SturdyStubs/Wings-for-Windows/issues/4)

- [ ] Rename/copy/mkdir/delete, permissions policy, scoped uploads/downloads and large-file streaming.
  - [x] Add the upstream directory-creation route, including missing parents, existing-directory replay, bounded Windows path validation, denylist preflight, link refusal and inherited DACLs.
  - [x] Add the upstream rename/move route for files and directories, including case-only names, batch preflight, descendant policy checks and native no-replace protection.
  - [x] Add the upstream file-copy route with bounded binary streaming, collision-safe names and owned staging/publication.
  - [ ] Complete delete, broader copy/rename behavior, permissions editing and scoped uploads/downloads; verify full workflows through a real panel.
- [ ] Safe archives/extraction and guarded remote fetch with progress/cancel.
- [ ] SFTP with panel authentication, per-operation permissions, revocation and session limits.
- [ ] Disk accounting/enforcement across file, installer, archive and game writes.
- [ ] Locked/long paths, disk-full, malicious archive and link/race tests.

## M5: Backups and transfers — planned

[Tracking issue #5](https://github.com/SturdyStubs/Wings-for-Windows/issues/5)

- [ ] Local/S3 backups, ignore rules, checksums, deletion and downloads.
- [ ] Safe restore, stop/operation locking, optional truncation and callbacks/events.
- [ ] Source/destination transfers, streaming/integrity, progress, cancel and failure cleanup.
- [ ] Recover from interrupted operations; restore and boot a real game successfully.

## M6: Operational parity and deployment validation — planned

[Tracking issue #6](https://github.com/SturdyStubs/Wings-for-Windows/issues/6)

- [ ] Aggregate job metrics, disk/network measurement, affinity and supported resource-limit mappings.
- [ ] Allocation/network restrictions, DNS/outgoing-IP behavior and shared/read-only mount policy.
- [ ] Runtime config updates/token rotation, durable activity delivery and callback retry/reconciliation.
- [ ] Diagnostics, service logs, certificate renewal, upgrade/reboot behavior and Windows CI.
  - [x] Package a no-Go native runtime-test suite, require exact test coverage/zero skips and record host/version/hash/identity evidence without declaring live gates complete.
- [ ] Sustained multi-server/load testing on supported Windows Server and panel versions.

## Update discipline

Each implementation batch updates this tracker, the corresponding GitHub issue, CHANGELOG.md and verification evidence. Release notes identify local checks and unverified live behavior separately. An unsupported route returning 501 is not counted as implemented. Do not mark a milestone complete based only on a test count or route count.

Alpha 17 final-package SCM validation: **11 checks passed** on Server 2022, including registration/start failure rollback. [Evidence and cleanup limits](docs/evidence/server2022-alpha17-service.md). Real panel/game, reboot/profile recovery and WFP gates remain open.
