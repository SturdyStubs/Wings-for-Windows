# Verification and release status

Version `0.1.0-alpha.5`, verified locally October 18, 2026 with Go 1.26.9 on Windows x64 (Windows NT build 26200). No production panel, game server, service registration or firewall configuration was changed.

The [alpha 1 audit](WINGS-PARITY-AUDIT.md) identified contract mismatches missed by the original tests. Alpha 2 adds upstream/panel-derived metadata fixtures and canonical route assertions, plus regression checks for the fixes below. Passing these tests is still not proof that a complete live panel/game deployment works. Current outstanding work is tracked in [ROADMAP.md](../ROADMAP.md).

## Automated checks

The release build runs `go test -count=1 -timeout 90s ./...` and `go vet ./...`. Tests launch harmless compiled fixture executables in temporary directories and use a mock panel API. They do not start Docker or contact a live panel.

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
- GitHub milestones and README badge endpoints are checked against actual repository counts. Progress initialization is idempotent; closing/reopening issues remains authoritative.

The build script produces the Windows amd64 daemon, demo executable, documentation, dependency licenses and a ZIP checksum. Go race-detector testing is not included; no compatible C toolchain is configured on this host.

## Required live checks

These remain open and limit the release to an experimental private node:

- [ ] Install on a disposable Windows Server 2022/2025 host and confirm job limits under that host's policy.
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
