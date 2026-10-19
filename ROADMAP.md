# Progress toward Wings compatibility

Last updated: October 18, 2026. Current version: **0.1.0-alpha.8**. Active development: **M2 — isolation and supported threat model**. M1 remains open for its live panel gate.

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
  - [ ] Verify successful distinct-account batch logon/launch under the intended Windows Server service identity.
  - [ ] Complete account/credential lifecycle, recursive/ancestor/link/race policy, shared executable/mount rules, daemon process-handle and network enforcement.
- [ ] Test cross-server/account access, credentials, hardlinks, reparse points, case aliases and filesystem races. Local fixtures now cover a denied root with an exposed child, real NTFS hardlinks/junctions, ambiguous paths, handle-pinned rename rejection and scan bounds/cancellation; live identity and adversarial race gates remain open.
- [ ] Publish the supported Windows host/runtime/game matrix and remaining restrictions.

Current scope remains a trusted private node. File denylist/path checks are not process isolation.

## M3: Windows provisioning and lifecycle recovery — in progress

[Tracking issue #3](https://github.com/SturdyStubs/Wings-for-Windows/issues/3)

- [x] Versioned Windows profiles and dependency/OS checks; read-only CLI preflight and enforcement before start/manual installation acknowledgement. See [profile v1](docs/PROFILES.md).
- [ ] Create/delete, native install/reinstall, output/progress, cancellation, timeouts and panel callbacks.
- [ ] Egg configuration parsers for text, JSON, YAML, INI, properties and XML.
- [ ] Crash detection/restart policy, persistent state/logs, reboot and interrupted-operation recovery.
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
- [ ] Sustained multi-server/load testing on supported Windows Server and panel versions.

## Update discipline

Each implementation batch updates this tracker, the corresponding GitHub issue, CHANGELOG.md and verification evidence. Release notes identify local checks and unverified live behavior separately. An unsupported route returning 501 is not counted as implemented. Do not mark a milestone complete based only on a test count or route count.
