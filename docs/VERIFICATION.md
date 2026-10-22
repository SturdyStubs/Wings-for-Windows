# Verification and release status

Alpha 30 adds a bounded Source TCP RCON adapter. Protocol/configuration/native lifecycle tests and an optional SCM phase cover authentication, complete responses, malformed frames, redaction, cancellation, graceful stop and fallback. Final clean-package/VM evidence follows separately; see [scope](SOURCE-RCON.md).

Version `0.1.0-alpha.30`, verified locally October 21, 2026 with Go 1.26.9 on Windows x64 (Windows NT build 26200). The local automated suite changes no production panel, game server, service registration, local user account, existing host ACL, logon right or firewall configuration. Separate disposable VM service tests deliberately create and clean up temporary users, rights, tasks, service registration and fixture ACLs. Identity tests write/read/delete one unique temporary fake generic credential without account logon, and adjust ACLs only on disposable test fixtures.

The [alpha 1 audit](WINGS-PARITY-AUDIT.md) identified contract mismatches missed by the original tests. Alpha 2 adds upstream/panel-derived metadata fixtures and canonical route assertions, plus regression checks for the fixes below. Passing these tests is still not proof that a complete live panel/game deployment works. Current outstanding work is tracked in [ROADMAP.md](../ROADMAP.md).

## Automated checks

The release build runs `go test -count=1 -timeout 90s ./...` and `go vet ./...`. Tests launch harmless compiled fixture executables in temporary directories and use a mock panel API. They do not start Docker or contact a live panel.

Alpha 30 local run: **619 test cases/subtests passed, zero skipped**; `go vet ./...` passed. The release build repeats these checks and records counts/source revision in BUILD-INFO.json.

Coverage includes:

- Pure managed-permission planning with mocked identity lookup: canonical layout, distinct accounts, TLS/code/dependency containment, empty data/logs, hardlink/junction rejection and no planning mutations.

- Public-listener TLS requirements, remote HTTP rejection, exact origins, invalid profiles, strict JSON and returned configuration defaults.
- Node bearer direction, invalid/absent credentials, unsupported routes and invalid power actions.
- JWT expiration, future/missing issued-at, preboot tokens, issuer, server binding, scope, permissions and revocation.
- Browser WebSocket origin rejection, permission enforcement and live revocation.
- Real Windows stdin/stdout, Unicode and argument quoting, readiness, root-process metrics, graceful stop, restart and forced kill.
- Job Object cleanup of descendant processes after both explicit kill and normal parent exit.
- Bounded console input when the executable does not read stdin.
- Allowed environment/template substitution, missing-variable failures, and excluding the daemon token/unapproved variables from child environments.
- File listing/read/write, Windows traversal/device/ADS path rejection, and attempted read/write through an escaping Windows junction. This test uses a real reparse-point junction without enabling Developer Mode or requiring symbolic-link privileges.
- Combined API-to-native-process-to-WebSocket flow: start, running status, console command/response and stop.
- Upstream file-list URL and metadata field/type contract; MIME/content length/attachment headers.
- Egg denylist case folding, glob/negation/bounds, filtering, blocked reads/writes, policy refresh and unchanged contents after denied writes.
- Rejected reads/writes/new-file creation through a junction to a denied directory. `os.Root` rejects absolute junctions before access; resolved-handle policy checks also run on successfully opened targets.
- Emergency kill interrupting stop/restart, cancelling an in-flight panel configuration fetch, releasing the operation lock and preventing restart after cancellation.
- Standard allocation/memory/startup/timezone values overriding untrusted egg variables, IANA timezone validation/defaults and HTTP 400 for suspended starts/restarts.
- Egg process fixtures: literal/regex matching, invalid-regex literal fallback, ANSI stripping, native command stopping, local-policy preservation and explicit rejection of unsupported signals/rewrites.
- Live Windows Job Object queries verify CPU/memory changes and removal while retaining kill-on-close. Integration checks retain the same game process/uptime and the local memory ceiling.
- Failed limit updates retain reported enforcement; suspension still terminates a game when other new egg settings are invalid. Readiness policy refresh is tested while a game is starting.
- Power wait acquisition, zero-wait behavior, one-second expiry, bounded pending requests and cancellation on kill/shutdown; queued start/restart cannot relaunch after kill.
- Upstream system-v2 field/type fixture, real Windows physical memory/CPU/version, native server counts and separate zero Docker counters.
- Anonymous OPTIONS preflight, exact origins, authenticated operations, explicit private-network opt-in, generated request IDs, 400/413/422 JSON errors and Bearer challenges.
- HTTP 502 for commands without a process and requested recent-log sizing in the API/native/WebSocket integration flow.
- Profile schema migration and future-version rejection, requirement bounds, unsupported architectures and absolute dependency path validation.
- Real PE inspection rejects text, DLL and ARM64 fixtures; checks declared architecture, missing/non-file dependencies, minimum Windows build and cancellation.
- Node start remains offline with an unmet dependency; requirements are checked again after a successful start/stop. Manual installation acknowledgement sends no callback on failed preflight and exactly one after requirements pass.
- Identity reference/SID bounds, UUID binding and per-server account uniqueness; daemon-account and pinned-SID mismatch rejection.
- Actual restricted-token launch via CreateProcessAsUser, child token privilege queries, stdin/stdout and Job Object kill. This uses a restricted token derived from the existing test identity, not a provisioned second account.
- Effective DACL grants for read-only, explicit denial, world access and no grants; protected-path exposure/missing-path rejection and generated secret/daemon/other-root rules.
- Credential helper write/read/delete round-trip with a unique fake credential and absent-reference failure. No real game account is authenticated.
- Recursive audit with a real restricting token and temporary file ACLs: denied root passes root-only checks, while its exposed child is rejected by recursive checks. No separate account is created or authenticated.
- Actual NTFS hardlink and junction rejection (including a junction ancestor), every ancestor checked through the drive root, path alias/device/ADS rejection, entry bounds and cancellation. Existing writers remain compatible; rename fails while the checked file handle is pinned.
- Daemon process/primary-token DACL checks and actual Windows denial of process-memory reads/token duplication under a restricting token. Thread impersonation is reverted before test assertions; no account is created or authenticated.
- Process/token rights fixtures distinguish dangerous access from query-only rights. Raw generic applicable permission entries fail explicitly; inheritance-only generic entries remain compatible with ordinary Windows root ACLs.
- Configured game executable/dependency write guards, shared-file rule deduplication, retained case variants and preservation of stricter secret bans when paths overlap.
- Exact-UUID credential lifecycle: missing status/removal, metadata without password fields, explicit same-account rotation, refused accidental overwrite/identity change, removal/repeated removal and invalid namespace/wildcard rejection. All credential values are fake fixtures and cleaned up; no Windows account password is changed.
- Identity diagnostic failure contracts for invalid/shared/unknown profiles, missing executables/credentials, cancellation and later-stage `not_run` states. Reports explicitly retain false game/panel/network verification flags. Random missing credential targets avoid account logon; diagnostic tests create no server data roots.
- Network mode/reference validation and independent four-layer WFP contract; injected-engine install failures at each stage, cancellation, rollback errors and idempotent session close. No real filters are installed by these tests.
- Actual Windows Job Object child exit with a fake network lease verifies captured descendants finish before release. Actual WFP access, packet blocking and crash/restart behavior remain unverified.
- Network installation diagnostic contract with an injected backend: successful owned-rule readback and independent cleanup, altered contents, stale objects, denied reads, close/observer failures and cancellation. Unrelated sentinel filters are preserved; traffic/game verification flags stay false.
- Missing-credential network diagnostic stops at identity preflight without opening WFP or creating server roots. No actual filters are installed by automated tests.
- Startup guard across the completed snapshot audit: actual file writes, file/ancestor renames and directory write-handle opens fail while held; ordinary reads and child-file creation still work. Conflicting pre-existing writers fail closed. Close/partial-acquisition rollback, 1,024-handle bound, cancellation, hardlinks and junction ancestors are tested.
- Actual Windows image creation with launch pins held, including a restricted-token fixture; pins can be released after startup. A separate no-existing-writer regression verifies audit handles themselves enforce no-delete sharing. These tests do not validate concurrent ACL changes, existing writable mappings, runtime loader paths or a hostile race campaign.
- Required launch permissions using a real restricting SID and disposable ACL fixtures: executable execute denial, explicit denies, unreadable dependencies, root file-creation denial, read-only temporary directories, missing paths/types, cancellation and dependency bounds. Only fixture ACLs are modified.
- Actual file read/execute opens and allowed/denied temporary-file creation under an impersonated restricted token agree with preflight results; impersonation is reverted before assertions. No account is created/logged on. Node specs preserve dependency snapshots and check the same temporary directory passed to the child. Missing-credential JSON reports leave `launch_access` as `not_run`.
- Portable native test argument rejection and no-Go packaged execution: 14 cases/subtests, zero skipped. The PowerShell runner verifies clean/matching source revisions and binary hashes, refuses existing evidence directories, rejects incomplete manifest coverage, and keeps all live panel/account/service/WFP verification flags false.
- GitHub milestones and README badge endpoints are checked against actual repository counts. Progress initialization is idempotent; closing/reopening issues remains authoritative.

The build script produces the Windows amd64 daemon, demo executable, documentation, dependency licenses and a ZIP checksum. Go race-detector testing is not included; no compatible C toolchain is configured on this host.

## Portable VM evidence

Use [the Windows Server VM workflow](WINDOWS-SERVER-TESTING.md) to run the packaged native tests without Go. The local runner was checked with Windows PowerShell 5.1 and Go removed from PATH, including checksum/dirty-source/reused-output/incomplete-suite rejection. Reports record the actual OS build and invoking SID; a client-Windows pass is not relabeled as a Windows Server result. Successful local baseline: 14 cases/subtests, zero skipped. The operator subsequently supplied a passing alpha 15 Server 2022 report: build 20348, PowerShell 5.1.20348.558, Administrator token/session 2; 14 cases passed, zero failed/skipped. The source revision and both executable hashes match the retained alpha 15 package. See [evidence and scope](evidence/server2022-alpha15-runtime.md). This is operator-provided report evidence; the VM/full test log was not independently inspected. Remaining live gates below stay open.

Alpha 16 additionally verifies the Windows PowerShell `-File` invocation with both package/output arguments omitted, including execution from outside the package. This regression now runs during every clean-source release build; dirty development builds are not certified by the host runner.

## Direct Server 2022 follow-up

The agent subsequently transferred the checksum-verified alpha 16 package over pinned-host-key SSH, ran the default command directly and retrieved the full JSON/log: all 14 runtime cases passed. Real account probes then passed eight native stages from SYSTEM and non-administrator daemon password-backed batch tasks, including actual game SID, own/temp writes, denied peer/private reads and denied daemon memory. The helper calls the existing native/identity APIs directly; it is not node/panel/SCM verification.

The non-administrator native report passed, but the cleanup wrapper refused to delete a still-loaded temporary profile and exited nonzero. Accounts/tasks/credentials/rights were removed and independently checked; profile teardown remains incomplete. See [full evidence, contexts and limits](evidence/server2022-alpha16-ssh-identity.md).

## Required live checks

These remain open and limit the release to an experimental private node:

- [ ] Verify optional WFP filter installation/readback, IPv4/IPv6 TCP/UDP/loopback behavior, unrelated-account preservation, children, rollback and daemon/filter-engine crash/restart cleanup.
- [x] Verify real distinct-account native fixture launch, pinned child SID, file/memory denial, stdio and stop on Server 2022 from a non-administrator daemon batch task.
- [x] Run a non-administrator SCM demo workflow, separate game identity, console, stop/restart and installer failure rollback on Server 2022 (mock panel).
- [ ] Complete credential/profile lifecycle, cold boot, negative policy and recovery checks.
- [x] Run the packaged native baseline and job-limit queries/updates on the disposable Server 2022 VM.
- [ ] Verify the intended service policy, sustained resource enforcement and additional supported Windows host configurations.
- [ ] Connect a real test panel using trusted TLS from both the panel backend and browser.
- [ ] Verify the server-creation/manual-provisioning workflow and installation callback in that panel version.
- [ ] Start the bundled demo from the panel, see readiness, send a command, then stop/restart/kill it.
- [ ] Confirm subusers receive only their permitted power/console actions and lose access immediately after revocation.
- [ ] Exercise file listing/editing through the real panel frontend and confirm unsupported operations give clear errors.
- [ ] Provision one real Windows game server, validate its working directory, dependencies, port binding, output regex and stdin stop behavior.
- [ ] Observe CPU/job-memory behavior under realistic load; confirm root-only statistics are understood.
- [x] Register a non-administrator service and verify actual SCM stop/restart with separate game identity on Server 2022.
- [ ] Verify host reboot, profile recovery and certificate renewal.
- [ ] Verify firewall/NAT and reach the actual game from another machine.
- [ ] Decide which game-specific adapters are required for RCON/log-file-only consoles.

## Unsupported features

Do not enable backups, restore, transfer, SFTP, automatic egg installation/reinstall, signed downloads/uploads or Rust Server Wiper operations on this node. This release has no network/account isolation, disk quota, automatic daemon restart or game updater. Optional game crash recovery and persistent console history are documented below.

Failures during startup are reported in recent console output. A game that never matches readiness remains starting. A daemon shutdown kills managed jobs, so plan service updates as game-server downtime.

## Alpha 17 service provisioning

The new installer/module passed a disposable Server 2022 SCM demo test using the alpha 16 runtime binaries: account creation, rights/ACLs, injected registration-failure rollback, actual non-administrator service identity, separate game-account tree, API console, SCM stop and restart/game stop. The final alpha 17 package passed all 11 service checks, including expanded start-failure/full-DACL rollback. An independent audit confirmed no test users, rights, tasks, services or processes remain; one loaded profile is retained. See [evidence](evidence/server2022-alpha17-service.md). No real panel, TLS, reboot, WFP traffic or game-server result is claimed.

## Alpha 18 service credential bridge

Local tests cover real duplex pipe framing/peer PIDs, UTF-8 round trip, rejected zero/negative/oversize/truncated/invalid-UTF-8 frames and pure plan rejection of running/mismatched services, wrong pinned game SID, wrong targets, missing files/profiles and malformed UUIDs. Plans use mocked account/service lookup and change no users/tasks/rights. Injected scheduler/right backends also cover nonterminating registration errors, partial task creation, preserved preexisting batch rights and explicit cleanup failures without host mutation. Full development suite: 231 cases/subtests, zero skipped; vet passed.

The development scripts passed a Server 2022 15-check fixture using alpha 17 runtime binaries: public create/inspect/replace/remove, accidental-overwrite and wrong-identity rejection, plus actual SCM demo identity/console/stop/restart. The final checksum-verified alpha 18 package then passed all 18 checks, adding bad-service-password cleanup, lock contention and preservation of preexisting batch rights. An independent audit found no residual test accounts, rights, tasks, services, mocks or owned processes; one final-fixture profile is retained loaded. See [evidence](evidence/server2022-alpha18-credentials.md). No cold reboot, actual account-password reset, real panel/game or WFP traffic result is claimed.

## Alpha 19 game crash recovery

The local suite adds policy/default validation, actual process/startup crashes and bounded loops, clean/disabled exits, queued/manual stop/kill, suspension/shutdown and active-launch cancellation, fresh-panel suspension rejection, failed preflight and rolling attempt expiry. Full suite: 258 cases/subtests, zero skipped; vet and clean-source release build passed. The checksum-verified final package passes 14 portable native cases and all 22 Server 2022 SCM/credential/recovery checks. See [evidence and harness/profile cleanup limits](evidence/server2022-alpha19-recovery.md). Reboot tests are deferred by operator instruction and remain unverified. Persistent state/logs, daemon restart policy, real game/panel and network isolation remain open.

## Alpha 20 persistent console logs

New local tests cover bounded rotation/history, rejected aliases and duplicate writers, corrupt/torn records, invalid UTF-8/truncation, backpressure, write/close/locked-rotation errors, concurrent close and history restored into a fresh manager. Config/identity/managed-layout tests enforce a separate private store. A test-only helper gives Windows PowerShell children their own module path instead of inheriting incompatible PowerShell 7 module paths from a developer host. Full suite: 294 cases/subtests, zero skipped; vet, clean-source release build and packaged negative checks pass. The checksum-verified package passes 14 native cases and all 25 SCM/credential/recovery/logging checks on Server 2022, including actual game-token read denial, bounded rotation and restored history after service restart. See [evidence, fixture correction and cleanup limits](evidence/server2022-alpha20-console.md). No reboot test is performed. Persistent game/recovery state, daemon diagnostics, real panel/game and WFP gates remain open.

## Alpha 21 Windows service diagnostics

Local injected-handler and HTTP/HTTPS fixtures verify truthful SCM readiness, pending/interrogate/stop/shutdown/startup cancellation, sink failures and secret-safe failure stages. The full 317-case suite passes with zero skips; vet, clean-source release build and packaged negative checks pass. The exact package passes 14 native and all 29 Server 2022 SCM/credential/recovery/logging/events checks, including real non-administrator lifecycle events and a safe malformed-configuration startup failure. See [evidence and cleanup limits](evidence/server2022-alpha21-service-events.md). See [operator guide](SERVICE-EVENTS.md). Reboot tests remain deferred.

## Alpha 22 persistent recovery budgets

Local storage/config/node fixtures cover durable reservations, repeated reopen, corruption/alias/ownership rejection, sticky write failures, bounded close, expired/future restoration, fail-closed sync cleanup and manual cancellation during a blocked save. All 345 local cases/subtests pass with zero skips; vet, clean release build and packaged negative checks pass. The exact package passes 14 native and all 33 Server 2022 service/persistence checks, including SCM/forced-termination preservation, actual private budget-file denial, restored exhaustion and corrupt-state startup refusal. See [evidence and cleanup limits](evidence/server2022-alpha22-recovery-state.md). No game/timer auto-restore or reboot result is claimed. See [operator guide](RECOVERY-STATE.md).

## Alpha 23 approved egg text files

Contract/config/Windows fixtures verify approved prefix rewriting, encoding/schema/bounds, protected identities and observed links, full-batch preflight, guard cleanup, cancellation and actual launch. All 394 local cases/subtests pass with zero skips; vet, clean release build and packaged negative checks pass. The exact package passes 14 native and all 35 Server 2022 service/persistence/text checks, including missing-file refusal and actual non-administrator UTF-8 edits with preserved SDDL. See [evidence and cleanup limits](evidence/server2022-alpha23-egg-text.md). Other parsers, full egg compatibility and interrupted-operation recovery remain open; reboot testing remains deferred. See [operator guide](EGG-TEXT-CONFIG.md).

## Alpha 24 approved JSON egg files

Local fixtures cover nested/array/wildcard paths, scalar types and preserved large numbers, literal conditions, malformed/duplicate/deep/oversize input, bounded growth/encoding, cancellation, mixed-batch preflight and actual native launch. All 425 local cases/subtests pass with zero skips; vet, clean release build and packaged negative checks pass. The exact package passes 14 native and all 37 combined Server 2022 checks, including malformed mixed-batch refusal and actual typed JSON edits with retained precision/SDDL. See [evidence and cleanup limits](evidence/server2022-alpha24-egg-json.md). The remaining parsers, full egg compatibility and interrupted-operation recovery remain open. See [JSON guide](EGG-JSON-CONFIG.md). No reboot test is performed.

## Alpha 25 approved properties egg files

Local fixtures cover full-key conditions, escapes/Unicode/continuations, stable output, literal environment/cycle values, encoding/size/count/cancellation limits, mixed-batch preservation and actual native launch. A pinned dependency and its complete BSD notice are packaged. All 446 local cases/subtests pass with zero skips; vet, clean release build and packaged checks pass. The exact package passes 14 native and all 39 combined Server 2022 checks, including third-file refusal without earlier edits and actual Unicode/continuation/literal/conditional rewrites with retained SDDL. See [evidence and cleanup limits](evidence/server2022-alpha25-egg-properties.md). YAML/INI/XML, full parser parity and interrupted-operation recovery remain open; reboot testing stays deferred. See [properties guide](EGG-PROPERTIES-CONFIG.md).

## Alpha 26 approved INI egg files

Local fixtures cover default/section/bracketed paths, dotted keys, exact section ownership, scalar/legacy values, duplicates, Unicode, literal interpolation/cycles, Windows paths/quoting, stable output, unsupported rules/encoding/shapes, input/result/encoding bounds, cancellation, mixed-batch preservation and native launch. The pinned upstream dependency and Apache license are packaged. All 483 local cases/subtests pass with zero skips; vet, clean release build and packaged checks pass. The exact package passes 14 native and all 41 combined Server 2022 checks, including fourth-file refusal preserving earlier files and actual section/Unicode/literal/path/quote rewrites with retained SDDL. See [evidence and cleanup limits](evidence/server2022-alpha26-egg-ini.md). See [INI guide](EGG-INI-CONFIG.md). YAML/XML and broader parser/lifecycle parity remain open; reboot tests remain deferred.

## Alpha 27 approved YAML egg files

Local fixtures cover YAML/yml paths, scalar types, arrays/wildcards/literal conditions, missing containers, preserved numeric spellings, multiline strings, Unicode, literal interpolation, stable quoting, duplicate/non-string keys, tags/aliases/merges/documents, encoding/depth/node/key/size/output limits, cancellation, mixed-batch preservation and native launch. The pinned upstream dependency and complete dual-license notice are packaged. All 525 local cases/subtests pass with zero skips; vet, clean release build and packaged checks pass. The exact package passes 14 native and all 43 combined Server 2022 checks, including fifth-file refusal preserving earlier files and actual typed/precise/conditional/literal/Unicode rewrites with retained SDDL. See [evidence and cleanup limits](evidence/server2022-alpha27-egg-yaml.md). See [YAML guide](EGG-YAML-CONFIG.md). XML and broader parser/lifecycle parity remain open; reboot tests remain deferred.

## Alpha 28 XML and opt-in VM reboot fixture

Local XML fixtures cover paths/attributes/wildcards/root creation, Unicode/escaping/literals, strict shapes/DTD/entity/namespace rules, bounds/mutation/output/cancellation, mixed-batch preservation and native launch. The combined fixture adds sixth-file refusal and actual XML edits. Reboot preparation retains a separate fixture and schedules unattended post-boot service/history/budget/cold-credential checks and cleanup. All 558 local cases/subtests, vet, clean release build and packaged checks pass. The exact package passes 14 native and all 45 combined service checks. A separate fixture passes 40 preparatory checks and all seven actual post-boot checks: delayed non-admin service, offline games/timers, history, budget, cold credential game launch and tree stop, with changed boot time, pinned SSH reconnect and unattended/independent cleanup. See [evidence and limits](evidence/server2022-alpha28-xml-reboot.md). Preparation alone proves no reboot gate. See [XML](EGG-XML-CONFIG.md) and [reboot scope](REBOOT-VALIDATION.md).

## Alpha 29 opt-in running intent restoration

Local checks cover dual snapshot initialization/transitions, corruption/missing-stop-slot/link/writer refusal, configuration prerequisites, manager restart/readiness gating, budget/cancellation, fresh panel suspension, manual/clean stops, stale exit writes and emergency kill during blocked storage. SCM/reboot fixtures add private-state denial, durable running/stopped intent, corrupt startup refusal and automatic distinct-account post-boot launch. The clean release passes all 584 local cases/subtests and vet; its exact package passes 14 native and 49 combined Server 2022 checks. A separate fixture repeats 44 preparatory checks and passes all seven actual post-boot checks, including automatic game restoration, cold distinct-account launch and budget reservation. Pinned SSH reconnect and unattended/independent cleanup pass. See [exact evidence and limits](evidence/server2022-alpha29-running-restoration.md). See [restoration semantics and limits](RUNNING-RESTORATION.md).
