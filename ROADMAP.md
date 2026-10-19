# Progress toward Wings compatibility

Last updated: October 18, 2026. Current version: **0.1.0-alpha.2**. Current milestone: **M1 — core protocol compatibility, in progress**.

[Repository](https://github.com/SturdyStubs/Wings-for-Windows) · [Tracking issues](https://github.com/SturdyStubs/Wings-for-Windows/issues) · [Changelog](CHANGELOG.md) · [Original audit](docs/WINGS-PARITY-AUDIT.md) · [Verification](docs/VERIFICATION.md)

This file tracks the current implementation. The audit preserves the alpha 1 baseline. A checked implementation item means code and local regression tests exist; a live verification item stays open until exercised against a real panel/host/game. Route coverage is not a completion percentage.

## Current state

- Native Windows EXE runtime, Job Objects, basic power/console/status and root-process statistics: locally tested.
- Alpha 2 fixes: canonical file listing and metadata, file MIME/length headers, egg file-denylist policy, standard derived variables, suspended power response and emergency kill cancellation: locally tested.
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
- [ ] Decode and use egg `process_configuration` for supported Windows stop/readiness behavior.
- [ ] Match `wait_seconds`, remaining power errors and lifecycle locking.
- [ ] Update active Job Object limits during sync.
- [ ] Complete system information, HTTP CORS/preflight and consistent error/request-ID handling.
- [ ] Verify node connectivity, console, subuser revocation and file browsing/editing through a real panel over trusted TLS.

Next implementation target: egg process configuration plus live resource-limit sync. M1 remains open until the remaining implementation and real-panel checks pass.

## M2: Isolation and supported threat model — planned, required before untrusted hosting

[Tracking issue #2](https://github.com/SturdyStubs/Wings-for-Windows/issues/2)

- [ ] Decide the supported native account/restricted-token model versus a Windows container/VM backend.
- [ ] Separate daemon credentials and game identities; define filesystem, network and mount policies.
- [ ] Test cross-server/account access, credentials, hardlinks, reparse points, case aliases and filesystem races.
- [ ] Publish the supported Windows host/runtime/game matrix and remaining restrictions.

Current scope remains a trusted private node. File denylist/path checks are not process isolation.

## M3: Windows provisioning and lifecycle recovery — planned

[Tracking issue #3](https://github.com/SturdyStubs/Wings-for-Windows/issues/3)

- [ ] Versioned Windows profiles and dependency/OS checks.
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
