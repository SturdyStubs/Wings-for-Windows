# Progress toward Wings compatibility

Last updated: October 21, 2026. Current version: **0.1.0-alpha.20**. Active development: **M3 — Windows lifecycle features**. M1/M2 remain open for live panel and isolation gates.

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
- HTTP inventory: handlers at **13 of upstream's 38 method/path combinations**, with partial behavior; **25 remain absent**. SFTP, security and recovery are additional work outside this count. Legacy `/files/list` is an extra alias, not an upstream route.
- Automatic installation, full file manager/SFTP, backups/restores, transfers, strong isolation and live certification remain open.
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
  - [ ] Complete cold boot/profile recovery and actual account-password lifecycle checks.
  - [ ] Complete account/credential lifecycle, recursive/ancestor/link/race policy, shared executable/mount rules, daemon process-handle and network enforcement.
- [ ] Test cross-server/account access, credentials, hardlinks, reparse points, case aliases and filesystem races. Local fixtures now cover a denied root with an exposed child, real NTFS hardlinks/junctions, ambiguous paths, handle-pinned rename/write rejection, retained startup handles with rollback and scan/handle bounds/cancellation; live identity and adversarial race gates remain open.
- [ ] Publish the supported Windows host/runtime/game matrix and remaining restrictions. The Server 2022 baseline and separate-account batch probes pass; real game, reboot/profile and full-policy certification remain open.

Current scope remains a trusted private node. File denylist/path checks are not process isolation.

## M3: Windows provisioning and lifecycle recovery — in progress

[Tracking issue #3](https://github.com/SturdyStubs/Wings-for-Windows/issues/3)

- [x] Versioned Windows profiles and dependency/OS checks; read-only CLI preflight and enforcement before start/manual installation acknowledgement. See [profile v1](docs/PROFILES.md).
- [ ] Create/delete, native install/reinstall, output/progress, cancellation, timeouts and panel callbacks.
- [ ] Egg configuration parsers for text, JSON, YAML, INI, properties and XML.
- [ ] Crash detection/restart policy, persistent state/logs, reboot and interrupted-operation recovery.
  - [x] Add opt-in bounded recovery for unexpected nonzero game exits with fresh launch guards, operator cancellation and status/reason reporting.
  - [x] Add opt-in persistent, bounded server console logs with restored recent history and observable storage failures.
  - [ ] Add persistent game/recovery state, daemon diagnostics and interrupted-operation recovery. Reboot tests are deferred at the operator's request; the gate remains open.
- [ ] Console/control-event, log-file or RCON adapters where required by a game.
- [ ] Complete a real game's create → install → start → stop/restart → reinstall → delete workflow from the panel.

## M4: Full files and SFTP — planned

[Tracking issue #4](https://github.com/SturdyStubs/Wings-for-Windows/issues/4)

- [ ] Rename/copy/mkdir/delete, permissions policy, scoped uploads/downloads and large-file streaming.
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
