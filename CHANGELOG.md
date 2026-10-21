# Changelog

## 0.1.0-alpha.22 - October 21, 2026

- Add per-profile crash_recovery.persist_history, requiring the existing protected console log store. Persist retry reservations before launch, retain the consumed budget across service/daemon restarts and keep games offline with no resumed timer.
- Use independent writer ownership and two bounded checksummed snapshots; reject corrupt, mismatched or inconsistent state without silently resetting budgets. Expose restoration and generic write-failure status. Keep I/O outside the server lock, disable automatic retries on write failure and bound close waiting.
- Add storage, config, restart/window/clock, reservation/cancellation/failure and cleanup regressions; extend the disposable VM fixture for actual private budget access, restart preservation, forced daemon termination and corrupt-state startup rejection. Final package/VM evidence follows separately. Reboot tests remain deferred.

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
