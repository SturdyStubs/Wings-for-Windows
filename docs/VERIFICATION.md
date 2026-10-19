# Verification and release status

Version `0.1.0-alpha.1`, verified locally October 18, 2026 with Go 1.26.9 on Windows x64 (Windows NT build 26200). No production panel, game server, service registration or firewall configuration was changed.

The subsequent [source parity audit](WINGS-PARITY-AUDIT.md) identified real contract mismatches that these tests do not catch, including directory-listing URL/schema differences. Passing the current suite is not proof that the panel's file browser or other complete workflows work. Upstream/panel-derived contract fixtures are required next.

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
