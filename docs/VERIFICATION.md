# Verification and release status

Version `0.1.0-alpha.16`, verified locally October 20, 2026 with Go 1.26.9 on Windows x64 (Windows NT build 26200). No production panel, game server, service registration, local user account, existing host ACL, logon right or firewall configuration was changed. Identity tests write/read/delete one unique temporary fake generic credential without account logon, and adjust ACLs only on disposable test fixtures.

The [alpha 1 audit](WINGS-PARITY-AUDIT.md) identified contract mismatches missed by the original tests. Alpha 2 adds upstream/panel-derived metadata fixtures and canonical route assertions, plus regression checks for the fixes below. Passing these tests is still not proof that a complete live panel/game deployment works. Current outstanding work is tracked in [ROADMAP.md](../ROADMAP.md).

## Automated checks

The release build runs `go test -count=1 -timeout 90s ./...` and `go vet ./...`. Tests launch harmless compiled fixture executables in temporary directories and use a mock panel API. They do not start Docker or contact a live panel.

Alpha 16 local run: **192 test cases/subtests passed, zero skipped**; `go vet ./...` passed. The release build repeats these checks and records counts/source revision in BUILD-INFO.json.

Coverage includes:

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
- [ ] Repeat with the intended SCM/service identity and complete credential/profile lifecycle, negative policy and recovery checks.
- [x] Run the packaged native baseline and job-limit queries/updates on the disposable Server 2022 VM.
- [ ] Verify the intended service policy, sustained resource enforcement and additional supported Windows host configurations.
- [ ] Connect a real test panel using trusted TLS from both the panel backend and browser.
- [ ] Verify the server-creation/manual-provisioning workflow and installation callback in that panel version.
- [ ] Start the bundled demo from the panel, see readiness, send a command, then stop/restart/kill it.
- [ ] Confirm subusers receive only their permitted power/console actions and lose access immediately after revocation.
- [ ] Exercise file listing/editing through the real panel frontend and confirm unsupported operations give clear errors.
- [ ] Provision one real Windows game server, validate its working directory, dependencies, port binding, output regex and stdin stop behavior.
- [ ] Observe CPU/job-memory behavior under realistic load; confirm root-only statistics are understood.
- [ ] Register the service using an unprivileged account, test SCM stop, host reboot and certificate renewal.
- [ ] Verify firewall/NAT and reach the actual game from another machine.
- [ ] Decide which game-specific adapters are required for RCON/log-file-only consoles.

## Unsupported features

Do not enable backups, restore, transfer, SFTP, automatic egg installation/reinstall, signed downloads/uploads or Rust Server Wiper operations on this node. This release has no network/account isolation, disk quota, crash auto-restart, persistent console history, service Event Log integration or game updater.

Failures during startup are reported in recent console output. A game that never matches readiness remains starting. A daemon shutdown kills managed jobs, so plan service updates as game-server downtime.
