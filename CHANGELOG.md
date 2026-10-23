# Changelog

## 0.1.0-alpha.40 - 2026-10-22

- Add the panel/upstream `POST /files/create-directory` contract, nested parents and idempotent existing-directory success. Preserve native ENOTDIR handling and directory contents/permissions.
- Validate full Windows paths and egg denylist prefixes before mutation, reject observed reparse/link entries, traverse through confined directory handles and retain inherited DACLs. Bound request bytes, depth and component lengths; cancellation or later I/O failure can retain new parents without destructive rollback.
- Add upstream/panel fixtures, invalid/malformed/auth/denylist/link/Unicode/long-path/cancellation regressions and an optional non-administrator SCM/distinct-game-account directory phase. Route coverage is now 17/38, with partial semantics; M4 and live panel/game certification remain open. See [scope](docs/FILE-DIRECTORIES.md) and [verification](docs/VERIFICATION.md).

## 0.1.0-alpha.39 - 2026-10-22

- Add stopped-node staging inspection with bounded content/owner/group/DACL fingerprints and an explicit fingerprint-approved quarantine command. Refuse unsupported names, unsafe ACLs/owners, links, configured path references and existing quarantine destinations.
- Recheck under the shared maintenance lock and held configuration, seal the selected stage to SYSTEM/Administrators and move it into private/code-quarantine without deleting or publishing files. Failed moves retain bytes and any stricter ACLs for inspection.
- Add inspection, quarantine, stale fingerprint, path-reference, link/ACL, empty-stage, move failure and destination-race tests plus a simulated interrupted-copy Server 2022 fixture. No actual crash/reboot, durable journal, automatic resume/deletion or live game/panel certification is claimed. See [workflow](docs/GAME-FILE-RECOVERY.md).

- Validate all 833 local cases/subtests, zero skipped, vet, clean package checks, 14 native and 114 combined Server 2022 checks, with independent cleanup. See [evidence](docs/evidence/server2022-alpha39-stage-recovery.md).

## 0.1.0-alpha.38 - 2026-10-22

- Add a stopped-node administrator command to import approved local game files into a new bin/games/UUID directory using a pinned SHA-256 manifest. Bound files/bytes/paths; reject links, duplicates, traversal, stale staging and existing destinations.
- Pin ordinary source writes during verification/copy, stage privately, flush and rehash copied bytes, apply daemon read/execute with no game write grant, then publish by same-parent directory move. Clean only owned staged paths on ordinary failure; preserve existing code/data/ACLs. No downloads, overwrites, installer execution or durable crash recovery.
- Prevent incremental game permission preparation from exposing unfinished .stage-* code. Add hash/path/sharing/copy/promotion/stale-stage tests and exercise imported executable setup/game launch plus actual dependency write denial on Server 2022. See [workflow](docs/GAME-FILES.md).

- Validate all 815 local cases/subtests, zero skipped, vet, clean package checks, 14 native and 107 combined Server 2022 checks, with independent cleanup. See [evidence](docs/evidence/server2022-alpha38-game-files.md).

## 0.1.0-alpha.37 - 2026-10-22

- Add an elevated Windows PowerShell 5.1 game-account command for a stopped existing managed node. Refuse existing account names, local UUID profiles/data and stale service binding; support WhatIf without a password prompt or state changes.
- Accept SecureString password input or prompt securely, create a dedicated enabled non-expiring game identity with self-service password changes disabled, and return only SID/name/UUID/run_as metadata. Share the maintenance lock, hold configuration against ordinary writes and roll back failed validation by the newly returned SID only.
- Keep profile approval, permissions, credentials, password rotation and panel-driven provisioning explicit separate steps. Add collision/password/validation/rollback tests and use the packaged account command in the third-account SCM fixture. See [workflow](docs/GAME-ACCOUNTS.md).

- Validate all 791 local cases/subtests, zero skipped, vet, clean package checks, 14 native and 101 combined Server 2022 checks, with independent cleanup. See [evidence](docs/evidence/server2022-alpha37-game-accounts.md).

## 0.1.0-alpha.36 - 2026-10-22

- Add an elevated Windows PowerShell 5.1 command to prepare one precreated game account and empty UUID directory on a stopped managed node without rerunning initial installation over populated data/logs.
- Validate the existing service binding, approved profiles/code, dedicated identities, bounded managed ACLs and paths. Add non-inheriting shared read/execute grants, private target data ACLs and only a missing batch-logon right; serialize changes with credential provisioning and compensate failures.
- Keep account creation, credential storage, binary distribution and populated-target repair separate. Add WhatIf, rollback, stale-plan and unsafe-path tests plus a distinct third-account SCM fixture. See [operator workflow](docs/GAME-PERMISSIONS.md).

- Validate all 774 local cases/subtests, zero skipped, vet, clean package checks, 14 native and 96 combined Server 2022 checks, with independent cleanup. See [evidence](docs/evidence/server2022-alpha36-game-permissions.md).

## 0.1.0-alpha.35 - 2026-10-22

- Add installer.persist_state with private dual-snapshot receipts for running, completed and acknowledged attempts, exclusive writer/ancestor guards, checksums and validated transitions.
- Save completion before callback delivery, preserve pending results across daemon restarts and reject overwriting unresolved attempts. Block game starts after an interrupted setup.
- Add a node-authenticated native install/reconcile route to retry a recorded notification or explicitly report an interrupted attempt as failed, with exact attempt matching and no installer execution.
- Fail closed on storage/corruption errors and expose uncertainty separately from local setup success. No automatic retry, partial-data rollback or transactional delivery guarantee is claimed.
- Validate 761 local cases/subtests, zero skips, vet, a clean release, 14 packaged native Server 2022 cases and all 89 combined service checks, including daemon interruption during setup and independent cleanup.

## 0.1.0-alpha.34 — 2026-10-22

- Add opt-in manual-approval creation through POST /api/servers. Verify assigned UUID v4 using node credentials, retain a pending record and expose its approval status without creating data, accounts, installers or game processes.
- Reconstruct pending entries from assigned-server discovery after restart; allow zero-profile bootstrap, idempotent pending retries and at most 1,024 entries. Reject unsupported automatic start explicitly. Approved profile duplicates remain conflicts.
- Block pending power/install/file operations; local profile approval and a daemon restart hand off to existing native reinstall. Update node/demo setup guidance without claiming automatic provisioning or live panel certification.
- Verify 731 local cases/subtests with zero skips, vet, a clean release, 14 packaged native Server 2022 cases and all 79 combined service checks. Independently inspect cleanup and retain explicit profile/history and mock-only limits.

## 0.1.0-alpha.33 — 2026-10-22

- Add locally approved native data/setup installers through Wings install/reinstall routes, serialized with power controls. Reinstall stops a running game; kill, suspension and shutdown cancel the installer job tree. Protected game binaries must already be provisioned.
- Reuse game-account launch guards, network policy and resource caps; protect installer code/dependencies in ACL, egg rewrite, log reader and initial managed-layout checks. Bound runtime, captured lines and total output.
- Emit Wings installer events and authenticated completion callbacks with exact successful/reinstall flags. Expose local native_install status and notification failure separately; no automatic rerun, durable outbox or partial-data rollback.
- Package a harmless PTDL_v2 demo egg, matching local profile and first Windows node guide. Document unsupported server creation and the remaining real-panel/game validation gate.
- Validate 707 local cases/subtests with zero skips, vet, a clean package, 14 packaged Server 2022 native cases and all 73 combined service checks. Correct a VM-discovered graceful-stop gate defect; preserve the preliminary failure and independent cleanup evidence.

## 0.1.0-alpha.32 - October 21, 2026

- Add exact per-server Windows Credential Manager references for Source RCON, separate from game-account credentials. Require exactly one vault/environment reference; read the selected vault entry per command and fail without fallback if unavailable or malformed.
- Add SecureString store/replace and metadata/remove CLI, plus a managed service-account helper using the existing protected pipe/task/operation-lock workflow. No RCON password is passed through task arguments, node JSON or environment. Preserve game-account credential management and existing transport redaction/cancellation.
- Add namespace/config/format/rotation tests and seven optional service checks, with conservative credential cleanup after interrupted worker responses. Existing environment references remain supported. Correct the inherited credential blob limit to 2560 bytes/1280 UTF-16 units before writing. All 672 local cases/subtests, 14 native cases and 66 combined Server 2022 checks pass, with independent cleanup. See [evidence](docs/evidence/server2022-alpha32-rcon-credentials.md). No real-game or new reboot certification is claimed.

## 0.1.0-alpha.31 - October 21, 2026

- Add opt-in locally approved UTF-8 game log-file console output and optional log-only readiness. Snapshot existing history before launch, skip stale/incomplete old lines, follow file rotation/visible truncation and retain bounded append backlog.
- Pin local parents and reject observed links, protected paths, denylisted names and malformed text. Bound each poll, cancel on process exit and expose safe `native_log_file` status. Runtime reader failure leaves native power control available. Prevent old process stdout from publishing into a replacement generation.
- Add reader/config/native lifecycle and combined RCON/log tests, plus an optional SCM phase. Read sharing, existing parents and supported UTF-8/newline output are required; fast truncate/regrow and hostile filesystem races remain outside the guarantee. All 654 local cases/subtests, 14 packaged native cases and 59 combined Server 2022 checks pass, with independent cleanup. See [evidence](docs/evidence/server2022-alpha31-log-file.md). No actual game or new reboot certification is claimed.

## 0.1.0-alpha.30 - October 21, 2026

- Add opt-in `source_rcon` for loopback Source TCP commands and graceful stops, with a daemon environment secret reference and exact password redaction across assembled reply chunks.
- Authenticate before sending and validate bounded packets, matching IDs/types and explicit completion. Bound time/packets/bytes/lines, discard incomplete output, cancel on process exit and reject concurrent commands. Never retry or fall back to stdin; command execution can be unconfirmed after delivery errors. Preserve native exit tracking, emergency kill and forced-stop fallback.
- Add protocol/config/native lifecycle tests and optional combined SCM coverage. Existing profiles retain stdin. Other RCON dialects, log-tail/control events, secret provisioning and real-game certification remain open. All 619 local cases/subtests, 14 packaged Server 2022 cases and 54 combined service checks pass, with independent cleanup. See [evidence](docs/evidence/server2022-alpha30-source-rcon.md). No new reboot or real-game certification is claimed.

## 0.1.0-alpha.29 - October 21, 2026

- Add opt-in `restore_running`, requiring private logs and enabled persistent crash recovery. Store running/stopped power intent in guarded, synced, UUID/checksum/sequence-bound dual snapshots initialized stopped; refuse uncertain state instead of rolling back a stop.
- Restore only after daemon readiness, with at most two startup actions, fresh launch guards and write-before-launch recovery reservations. Preserve intent on node shutdown, respect explicit stops/kills and clear clean exits. A slow intent write cannot delay killing the game tree. Expose `native_restore` status and safe storage failures.
- Add storage/config/lifecycle/concurrency checks and extend SCM/reboot fixtures for intent-file denial, running/stopped restoration, corrupt-state refusal and automatic cold-profile game launch. Require complete cleanup flags before post-boot success. All 584 local cases/subtests, 14 packaged Server 2022 cases, 49 combined service checks and seven actual post-boot checks pass. Automatic cold-profile game restoration, pinned SSH reconnect and unattended/independent cleanup verified. See [evidence and remaining gates](docs/evidence/server2022-alpha29-running-restoration.md).

## 0.1.0-alpha.28 - October 21, 2026

- Add approved XML text/attribute rewrites with root/exact/wildcard paths, bounded mutation/encoding and full data-tree reparse. Reject unsupported names, DTDs/entities, namespaces, duplicate attributes, mixed content and nonempty conditions; reuse guarded batch writes.
- Add opt-in disposable two-phase reboot preparation/resume, delayed service/mock startup and unattended cold credential/history/budget/game-tree checks and cleanup. Store temporary cleanup credentials using machine DPAPI in an Administrators/SYSTEM-only directory. The operator has authorized reboot testing; broader reboot parity remains open.
- Extend local and combined service tests for malformed sixth-file refusal and XML text/attribute/escaping/Unicode/literal edits with retained ACLs. All 558 local cases/subtests, 14 native cases and 45 combined Server 2022 checks pass. A separate actual VM reboot passes seven unattended post-boot checks, with pinned SSH reconnect and verified cleanup. See [evidence and remaining gates](docs/evidence/server2022-alpha28-xml-reboot.md).

## 0.1.0-alpha.27 - October 21, 2026

- Add approved YAML/yml egg configuration with shared JSON paths, arrays/wildcards, literal conditions and scalar coercion. Preserve untouched numeric spellings without floating point; support multiline strings and quote output strings explicitly.
- Reject aliases/anchors/merges, unsupported tags/numeric forms, duplicate/non-string keys and multiple documents. Bound input/result structures and JSON mutation size, cap streaming output and reparse raw data before guarded writes. Pin upstream's gopkg.in/yaml.v3 v3.0.1 and package complete MIT/Apache notices.
- Add contract, encoding/shape/bounds/output/cancellation, mixed-batch and native-launch tests; extend the Server 2022 service fixture for fifth-file refusal and actual typed/precise/conditional/literal/Unicode edits with unchanged SDDL. All 525 local cases/subtests pass with zero skips; vet, clean release build and packaged checks pass. The exact package passes 14 native and all 43 combined Server 2022 checks. Independent cleanup passes; one loaded profile is retained. See [evidence](docs/evidence/server2022-alpha27-egg-yaml.md). Reboot tests remain deferred.

## 0.1.0-alpha.26 - October 21, 2026

- Add approved INI egg configuration with default/section/bracketed paths, scalar/legacy values, exact section updates and literal interpolation. Disable continuations to preserve Windows backslashes; reject unsupported conditions and multiline/dialect shapes explicitly.
- Bound input, structures and encoding; use a deterministic encoder and reparse to verify all raw data before guarded writes. Reuse mixed-batch preflight and existing ACL/identity protection. Pin upstream's gopkg.in/ini.v1 v1.67.0 and package its complete Apache-2.0 license.
- Add contract, encoding/shape/bounds/growth/cancellation, mixed-batch and native-launch tests; extend the Server 2022 service fixture for fourth-file refusal and actual section/Unicode/literal/path edits with unchanged SDDL. All 483 local cases/subtests pass with zero skips; vet, clean release build and packaged checks pass. The exact package passes 14 native and all 41 combined Server 2022 checks. Independent cleanup passes; one loaded profile is retained. See [evidence](docs/evidence/server2022-alpha26-egg-ini.md). Reboot tests remain deferred.

## 0.1.0-alpha.25 - October 21, 2026

- Add approved properties egg parsing with exact keys/literal conditions, scalar/legacy replacements, continuation/Unicode normalization and bounded Java-style escaping. Preserve initial comments, normalize duplicate declarations and keep environment/property expansion disabled.
- Reuse three-parser preflight and guarded writes, reject malformed/oversize properties and retain native launch checks. Pin/checksum magiconair/properties v1.8.10 and package its BSD-2-Clause notice.
- Add parsing/escaping/continuation/Unicode, literal environment/cycle, limits/cancellation, mixed-batch and actual native-launch tests; extend the disposable VM fixture for malformed-third-file refusal and actual properties edits with retained ACLs. All 446 local cases/subtests pass with zero skips; vet, clean release build and packaged checks pass. The exact package passes 14 native and all 39 combined Server 2022 checks. Independent cleanup passes; one loaded profile is retained. See [evidence](docs/evidence/server2022-alpha25-egg-properties.md). Reboot tests remain deferred.

## 0.1.0-alpha.24 - October 21, 2026

- Add approved JSON egg file parsing before launch: nested object keys, array indices, one wildcard level and literal conditions, with upstream-compatible scalar coercion and untouched number precision. Reuse exact target approval and guarded writes.
- Reject duplicate/malformed/unsupported JSON, bound depth/node count/wildcard growth and encode through a capped writer. Validate mixed JSON/text batches before writes. Regex conditions, node templates and remaining parser types stay unsupported.
- Add contract/bounds/shape/cancellation, mixed-batch and native-launch tests; extend the disposable VM fixture for malformed-JSON launch refusal without changing earlier text and actual typed JSON edits with preserved precision/ACLs. All 425 local cases/subtests pass with zero skips; vet, clean release build and packaged negative checks pass. The exact package passes 14 native and 37 combined Server 2022 checks, including malformed mixed-batch refusal and typed JSON edits with preserved large integers and SDDL. Independent cleanup passes; one loaded profile is retained. Reboot tests remain deferred.

## 0.1.0-alpha.23 - October 21, 2026

- Add per-profile egg_config_files, requiring egg process configuration, and the supported plain-text file parser before launch. Validate exact local approval, scalar/legacy replacements, prefix order and bounded UTF-8/LF behavior. Unsupported parsers, conditions and unresolved templates fail explicitly.
- Preflight/render every existing target before writing; pin ancestors and regular single-link handles, reject protected configuration/TLS/code identities and preserve file ACLs with in-place writes. I/O failure/cancellation may leave partial edits; no transactional rollback or interrupted-operation guarantee is claimed.
- Add contract, path, encoding/bounds, cancellation, guarded-write and native-launch tests; extend the disposable VM fixture for missing-file refusal and actual non-administrator SCM rewriting with retained ACLs. All 394 local cases/subtests pass with zero skips; vet, clean release build and packaged negative checks pass. The exact package passes 14 native and 35 Server 2022 service/persistence/text checks, including missing-file refusal and actual non-administrator UTF-8 editing with unchanged SDDL. Independent cleanup passes; one loaded profile is retained. Reboot tests remain deferred.

## 0.1.0-alpha.22 - October 21, 2026

- Add per-profile crash_recovery.persist_history, requiring the existing protected console log store. Persist retry reservations before launch, retain the consumed budget across service/daemon restarts and keep games offline with no resumed timer.
- Use independent writer ownership and two bounded checksummed snapshots; reject corrupt, mismatched or inconsistent state without silently resetting budgets. Expose restoration and generic write-failure status. Keep I/O outside the server lock, disable automatic retries on write failure and bound close waiting.
- Add storage, config, restart/window/clock, reservation/cancellation/failure and cleanup regressions; extend the disposable VM fixture for actual private budget access, restart preservation, forced daemon termination and corrupt-state startup rejection. All 345 local cases/subtests pass with zero skips; vet, clean release build and packaged negative checks pass. The exact package passes 14 native and 33 Server 2022 service/persistence checks, including actual game-token denial, restored exhaustion, forced daemon termination and corrupt-state refusal. Independent cleanup passes; one loaded profile is retained. Reboot tests remain deferred.

## 0.1.0-alpha.21 - October 21, 2026

- Add fixed Windows Application service lifecycle/failure events with version/PID and allowlisted stages; exclude raw errors and configuration secrets. Offer explicit, idempotent managed source registration/removal without changing runtime privileges or clearing historical events.
- Report SCM Running only after initial config/panel sync, TLS material loading and API listener binding. Publish pending checkpoints, honor stop during initialization and preserve current status on interrogation. Missing/failed Event Log sinks remain nonfatal.
- Add injected service and real HTTP/HTTPS readiness regressions; extend the disposable Server 2022 fixture with registration, lifecycle and secret-safe startup-failure checks. All 317 local cases/subtests pass with zero skips; vet, clean release build and packaged negative checks pass. The exact package passes 14 native and 29 SCM/credential/recovery/logging/events checks on Server 2022, including actual non-administrator event writes and a secret-safe startup failure. Independent cleanup preserves historical events and one loaded profile. Reboot tests remain deferred; persistent state/interrupted operations and broader deployment gates stay open.

## 0.1.0-alpha.20 - October 21, 2026

- Add opt-in per-server JSONL console storage, file-size/count limits, rotation, recent-history restoration and native_console_logs counters/error status. Preserve the existing console and /logs contracts.
- Keep disk I/O behind a bounded queue; count overflow/truncation/corruption, report sticky storage errors and bound graceful console drain/flush. Reject observed links and duplicate writers; recursively protect logs from game access and keep them outside server data/config/code/TLS paths.
- Add retention, history, error, concurrency, path/identity and service-fixture coverage. Correct test-only Windows PowerShell child module-path handling for developer hosts using PowerShell 7; no global environment setting is changed.
- Full suite: 294 cases/subtests, zero skipped; vet and clean release build passed. The exact package passes 14 portable native cases and 25 Server 2022 SCM/credential/recovery/logging checks. Correct an oversized developer-fixture command batch; record both runs and retained-profile limits in evidence.
- Persistent game/recovery state, daemon startup/service diagnostics, interrupted-operation and reboot recovery remain open. Reboot tests stay deferred by operator instruction.

## 0.1.0-alpha.19 - October 20, 2026

- Add opt-in delayed game crash recovery for unexpected nonzero exits, with a bounded rolling attempt budget. Reuse normal power locking, fresh panel settings, executable/account/access/network checks and cancellation.
- Cancel recovery on explicit operator power, suspension or shutdown; skip clean/manual exits and cleanup errors. Stop on launch/preflight failure or exhausted budget and log why manual start is required.
- Expose native_recovery metadata through the existing server response and keep recovery/exit reasons in bounded in-memory console history.
- Add 27 policy/process/cancellation cases/subtests: 258 full cases pass, zero skipped; vet and clean release build pass. Final Server 2022 package passes 14 baseline cases and all 22 SCM/credential/recovery checks. Correct developer fixture cleanup ordering after a stale mock caused startup failure; retain loaded profiles and record the earlier disposable-credential cleanup limit. Recovery history is not persistent; service restart leaves games offline and clears the budget. Reboot validation is deferred at the operator's request.

## 0.1.0-alpha.18 - October 20, 2026

- Add Set-ServiceGameCredential.ps1 for managed-node credential create, metadata inspection, explicit same-account replacement and exact-target removal as the registered daemon account, while the service is stopped.
- Bind provisioning to service registration, profile UUID/credential target and pinned game SID. Use a temporary password-backed limited task, SID-restricted duplex pipe, OS peer-process checks, bounded frames/timeouts and an exclusive private-tree operation lock. Return metadata only.
- Stop immediately on Task Scheduler/CIM errors, inspect the exact owned task name even after registration errors, and compensate partially created tasks. Regression tests cover absent/partial registration and failed right cleanup.
- Preserve preexisting batch rights; remove only the helper's temporary right and task. Surface incomplete cleanup and uncertain credential outcomes instead of silently retrying. Never create a plaintext password file or put game passwords in worker arguments.
- Add 25 planner/pipe/failure cases/subtests: 231 full development cases/subtests pass with zero skipped; vet passes. Server 2022 development scripts pass 15 public-helper/SCM fixture checks. Final package passes all 18 checks, including bad-password cleanup, lock contention and preservation of preexisting batch rights. Independent cleanup audit passes; one loaded profile is retained. Reboot/profile recovery and actual Windows-account password changes remain open.

## 0.1.0-alpha.17 - October 20, 2026

- Add opt-in dedicated local service-account creation and managed initial bin/private/data/logs permissions. Grant only required daemon service/process-launch rights and per-game batch-logon rights; preserve existing rights.
- Reject populated initial data/logs, reused identities, code/private paths outside the owned layout, observed reparse points and multi-hardlink code. Game-account creation and credential bootstrap remain separate operator steps.
- Compensate failed setup by restoring snapshotted DACLs, removing added rights and deleting only the newly created service account. If service removal fails, retain account/permission state for inspection. Empty created directories remain; no recursive data deletion.
- Final alpha 17 package passes 11 actual Server 2022 SCM demo checks, including registration/start rollback and full preexisting DACL restoration. Independent cleanup audit confirms no temporary users, rights, tasks, service or processes remain; one loaded profile is retained.
- Add 14 planner cases/subtests without changing local accounts/rights/ACLs. Full suite: 206 cases/subtests, zero skipped; vet passed. Document actual Server 2022 SCM demo validation and remaining reboot/profile, real panel/game and WFP gates.

## Unreleased — direct Server 2022 validation

- Verify the unchanged alpha 16 package over SSH on Server 2022 build 20348: default runner, all 14 native cases/subtests and retrieved full reports/logs.
- Validate eight real separate-account native stages from SYSTEM and non-administrator daemon password-backed tasks. Confirm actual child SID, own/temp writes, peer/private read denial, daemon memory denial, stdio and graceful exit.
- Retain the developer-only probe source and sanitized evidence. No daemon protocol/runtime changes or new release artifact are introduced by this evidence update.
- Record key-authenticated SSH credential-store failure (1312), successful password-backed provisioning, cleanup of temporary users/rights/tasks/credentials, and intentionally retained loaded profile. SCM, recovery, panel/game and network gates remain open.

## 0.1.0-alpha.16 — October 20, 2026

- Fix Test-Host.ps1 default package resolution by evaluating script-relative paths after parameter binding, with a MyInvocation fallback. Resolve the default output directory in the body beside the package, avoiding Windows PowerShell startup working-directory quirks.
- Reproduce the parameter-free Windows PowerShell -File invocation in clean release builds, from outside the package. Previous package verification passed explicit paths and missed this failure.
- Document the alpha 15 workaround: run from the extraction root with -PackageDirectory .; retain all existing package integrity and coverage requirements.
- Full suite remains 192 cases/subtests, zero skipped; portable default-invocation baseline passes 14 cases/subtests. The user reports 14 passing cases on Server 2022 after using the explicit-path workaround; record the reviewed metadata and matching executable hashes without the full SID. Distinct-account/service/panel/network gates remain open.

## 0.1.0-alpha.15 — October 20, 2026

- Package a compiled native runtime-test binary and exact test/hash manifest; allow an explicit existing harmless fixture so the VM needs neither Go nor repository source.
- Add Test-Host.ps1: verify clean/matching source revisions and binary hashes, record OS/build/invoking-token metadata, run every manifest test with zero skips, and save JSON plus logs in a new evidence directory.
- Keep panel, distinct-account, service and real network-filter verification false even when the runtime baseline passes. No node config, credentials, service setup or WFP mutation is used.
- Add a staged Windows Server VM workflow and baseline/remaining-gate matrix. Verify local no-Go execution, damaged/dirty packages, reused output directories and incomplete test inventory rejection.
- Full development suite: 192 cases/subtests, zero skipped; portable native baseline: 14 cases/subtests, zero skipped. Actual Server 2022/service/panel/game validation remains pending; M2 remains open.

## 0.1.0-alpha.14 — October 20, 2026

- Check effective game-token DACL grants for executable read/execute, declared dependency reads and working/temporary-directory list, traverse, metadata and file/subdirectory creation before network-policy installation or process creation.
- Add the same read-only checker to `--check-identity` as the ordered `launch_access` stage. Missing directories fail; no permission repair, root creation, test write or extra account logon occurs.
- Retain startup pins for the actual temporary directory and dependency inputs; carry a dependency snapshot from the profile into the native launch spec.
- Verify real restricted-token file opens and allowed/denied temporary-file creation against ACL preflight results; add missing/type/cancellation/denial fixtures and node/diagnostic contracts. Full suite: 191 cases/subtests, zero skipped.
- Document conservative traversal requirements and the remaining existing-child, inheritance, loader, race and live deployment gates. M2 remains open.

## 0.1.0-alpha.13 — October 18, 2026

- Retain read-sharing-only launch handles before the identity ACL audit until network setup, suspended process creation, job assignment and resume finish. Cover nonrecursive protected paths, executable, working directory and ancestors; release on success or failure.
- Reject conflicting writers, observed reparse points, non-single-link files and more than 1,024 distinct path components. Preserve case variants, cancellation and rollback after partial acquisition. No silent fallback to unguarded identity startup.
- Correct audit opens to request data/list access, making their no-delete sharing effective independently of an existing writer. Daemon read/list access is now required on audited paths.
- Add actual Windows sharing/rename, partial rollback, bound, link and pinned-image creation tests, including a restricted-token child. The full suite passes 175 cases/subtests with zero skipped.
- Document updater conflicts, diagnostic exclusions and startup-only scope. ACL changes, writable mappings, recursive/runtime/loader races and separate-account/live-host validation remain open; M2 is incomplete.

## 0.1.0-alpha.12 — October 18, 2026

- Add explicit --check-network-policy <UUID> operator diagnostic, requiring configured block_connections and successful identity preflight before any temporary WFP installation.
- Read back owned rule IDs/count/content, sublayer and exact SID matcher, close the session even on readback/cancellation failure, then independently inspect cleanup through a second session.
- Report owned IDs, stage results and cleanup_verified; traffic_tested/game_launched remain false. Reject unknown cleanup state and preserve unrelated objects.
- Add injected-backend altered/stale/inspection/close/observer/cancellation tests and node/CLI preflight gating. Automated tests install no real filters.
- Document the command's temporary account-blocking effect, permissions, cleanup/time-limit semantics and separate live traffic/deployment gates. M2 remains incomplete.

## 0.1.0-alpha.11 — October 18, 2026

- Start network enforcement plumbing with optional network_policy.mode block_connections for dedicated run_as identities. Prepare four hard SID-scoped IPv4/IPv6 connect/receive-accept block rules before creating the process; default remains unmanaged.
- Use dynamic WFP sessions with partial-install rollback, cancellation checks and no unmanaged fallback. Pin tailscale/wf bindings and include its/netipx runtime notices.
- Retain the session through job shutdown, bounded active-process and captured-handle waits; retain blocks until daemon exit and warn if cleanup cannot establish completion.
- Test rule contracts and install/rollback errors with an injected engine; test descendant-before-lease-release ordering using an actual job and fake policy lease. No real firewall/filter change occurs in local tests.
- Actual WFP installation/traffic behavior, allocation allowlists, crash/restart cleanup and full network isolation remain unverified/open. M2 is not complete.

## 0.1.0-alpha.10 — October 18, 2026

- Add `--check-identity <UUID>` with ordered JSON profile, file, credential-presence, batch-logon/token, daemon-access, filesystem and service-privilege checks. Stop on the first failure and mark later checks not_run.
- Reuse startup's protected-path rule builder without manager initialization, panel sync, root creation or process launch. The command explicitly attempts account logon when earlier stages pass.
- Record unperformed game/panel/network checks as false even on diagnostic success. Check expected service privileges conservatively without enabling or granting rights.
- Add missing-credential, invalid/shared/unknown-profile, missing-file and cancellation regressions; document deployment workflow, synchronous-logon limits and the remaining Windows Server/service validation gate.
- No successfully provisioned second account or live deployment is certified by local diagnostic tests. Full M2 enforcement/recovery remains open.

## 0.1.0-alpha.9 — October 18, 2026

- Continue M2 with local `--credential-status <UUID>` JSON metadata and `--remove-credential <UUID>` exact-target removal. Missing entries are handled idempotently; neither operation loads node config, logs on an account or changes running processes.
- Require `-Replace` for deliberate same-account credential rotation; refuse accidental overwrite, account-name changes and rotation of a missing target. Retain secure-input handling and clear returned credential memory copies.
- Reject conflicting CLI operations, empty credential UUIDs and positional arguments before management actions.
- Add temporary fake-credential lifecycle regressions and document inspection, rotation, retirement, concurrency limits and interrupted-password-change recovery.
- Windows account provisioning/deletion, actual password changes, token revocation, durable recovery and live service validation remain open. These tools do not complete M2.

## 0.1.0-alpha.8 — October 18, 2026

- Continue M2 with read-only daemon process and primary-token DACL guards before identity launches. Reject dangerous memory, injection, duplication, termination, token adjustment and ownership/security grants.
- Protect all configured game executables and declared dependency files against game-account writes/replacement. Merge duplicate path rules deterministically without weakening stricter secret policy; retain case-distinct declarations.
- Reject unsupported applicable generic/complex permission ACEs instead of allowing AccessCheck to silently ignore raw generic masks. Ignore inheritance-only entries when evaluating the current object.
- Add actual Windows restricted-token impersonation checks proving daemon memory reads and primary-token duplication are denied, plus process/token rights and generated code-rule regressions.
- Keep previously acquired handles, transitive loader/mount rules, permission races, network enforcement and successful separate-account Windows Server deployment open. M2 is not complete.

## 0.1.0-alpha.7 — October 18, 2026

- Continue M2 with handle-derived ACL/metadata checks and ancestor deletion/ownership/DACL checks up to the local drive root.
- Recursively audit existing descendants of other configured server roots, including known child files behind a root that denies listing. Enforce 100,000 entry visits, 64 path components, batched enumeration and startup cancellation; incomplete scans abort launch.
- Reject observed reparse-point components, regular files with multiple hardlinks, UNC/device paths and ambiguous ADS/trailing-dot/device aliases. Pin components against delete sharing during each check while retaining compatibility with existing writers.
- Add actual NTFS hardlink/junction fixtures and a real restricting-token ACL fixture that demonstrates the root-only bypass and its recursive rejection. Test ancestor coverage, bounds/cancellation and rename rejection while pinned.
- Keep scan atomicity, future file/ACL/reparse changes, complete mount/process/network policy and successful separate-account Windows Server validation open. M2 enforcement and adversarial/live gates are not complete.

## 0.1.0-alpha.6 — October 18, 2026

- Start M2 enforcement with optional local `run_as` references: pinned non-built-in SID, UUID-bound Credential Manager target and distinct account per configured server.
- Read credentials from the daemon account's credential set, perform local batch logon, reject SID mismatch, daemon identity and selected privileged groups, then remove privileges with CreateRestrictedToken. Clear temporary password buffers on return.
- Launch through CreateProcessAsUser while retaining suspended creation, explicit inherited handles, Job Object assignment before resume and termination on failure. Identity failures never fall back to the daemon account.
- Add read-only AccessCheck rejection for exposed configuration/TLS keys, writable daemon binary/parent directories and accessible other server roots. This is a DACL snapshot, not recursive/link/race/network confinement.
- Include a secure-input credential storage helper; do not create accounts, grant rights, alter ACLs or modify firewall policy automatically.
- Test real restricted child privileges/console/job termination, credential round-trip cleanup, reference bounds and DACL grants. Successful separate-account Windows Server/service deployment remains unverified; M2 issues #19–21 stay open.

## 0.1.0-alpha.5 — October 18, 2026

- Add local profile schema v1. Normalize existing omitted/zero versions to v1 and reject unsupported future versions.
- Add read-only preflight for Windows PE applications, declared amd64/386 architecture, optional minimum Windows build and up to 64 explicitly declared absolute dependency files. Text files, DLL/object applications and unsupported architectures fail before launch.
- Recheck requirements before every start and manual installation acknowledgement. Add `--check-profiles` to inspect all local profiles without panel requests, process execution or filesystem writes.
- Document migration, supported checks and their limits. Preflight does not resolve/load dependencies, run installers, certify games or isolate executable code.
- Record the planned per-server native identity/token/ACL stage and separate virtualization direction for untrusted hosting. No identity/isolation configuration is enabled by this release.
- Add profile/PE/OS/dependency regressions, start recheck coverage and guarded installation-callback tests. Real panel/game/Windows Server validation remains open.

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
