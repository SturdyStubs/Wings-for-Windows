# Server 2022 alpha 56 pseudoconsole verification

Observed October 24, 2026 UTC and America/Phoenix. This batch implements an optional Windows console adapter and retains a failed real-game diagnostic honestly. It does not certify a complete real-game lifecycle. [Machine-readable results](server2022-alpha56-pseudoconsole-results.json).

## Clean artifact and native checks

- Immutable release/probe source: `c108831b9b22a49ed65be4d988263a52c4c5a501`, source dirty false. Console implementation was introduced in `f981a04`; the final commit additionally sanitizes diagnostic reporting/refuses occupied test ports.
- Package: `wings-windows-native-0.1.0-alpha.56-windows-amd64.zip`.
- Package SHA256: `c8ca6046ce19fddae4431bb0af700e63f2e914722f2b5f587ad1ddee7b9f6a6b`.
- Daemon SHA256: `77a44c701c695e4ae141a8b5da5ef7c395f2b27c47a514d09b11d0469247d825`.
- Clean Windows amd64 release: **1,580 local cases/subtests**, zero skips, vet and packaged default-invocation smoke.
- Exact package on Server 2022 build 20348: **21 native runtime cases/subtests**, zero failures/skips, including seven new console cases; one portable config case verifies both flag values. Native portable tests run as administrator; retained-service proof below uses the existing separate non-administrator identities.

The native fixture calls GetConsoleMode, ReadConsoleW and WriteConsoleW, proving a console API path distinct from anonymous stdio. Tests cover native console transport, command echo/graceful fixture stop, restricted-token creation, descendant termination, kill without an output consumer, bounded output/truncation/consumer close and setup-failure cleanup. Initial tests exposed redirected parent handles; explicit invalid standard-handle placeholders with STARTF_USESTDHANDLES fix that case before the clean release.

## Retained service and real panel

Runtime-only alpha55-to56 upgrade preserves existing configuration bytes and executable ACLs, with protected backups. Public operator-managed update stays 200/applied false and actual outgoing panel sync returns 204.

Separately add only `pseudoconsole: true` to the existing approved demo profile, preserving all other bytes, credentials, SFTP flags and ACLs. The daemon remains the dedicated WingsRuntime account and launches the demo under its existing pinned separate game identity, restricted-token/launch guards and Job Object. The demo reaches Running through the console adapter.

The unchanged alpha52 Client probe passes **19 real Client API/panel-issued-token groups** with the adapter enabled: authenticated WSS stats, Client/WSS commands with actual echoed markers, stop/start observed over WSS, file edits/rename/copy, archive/plain TAR workflows, actual 64 KiB signed HTTP transfers and replay refusal. This is retained-service console proof, not an actual OpenTTD panel/game run or browser rendering.

Known files are deleted through Client API, then three exact empty directories are independently removed nonrecursively. Final audit: one profile, console flag enabled, valid prior marker/file-manager data, zero fixtures/stages/active pulls, exact daemon hash, protected config/key ACLs, only the planned profile addition, outgoing panel sync 204 and demo/service/tunnel Running, unsuspended.

## Real-game investigation: failed, still open

Official [OpenTTD 15.3 Windows amd64](https://www.openttd.org/downloads/openttd-releases/15.3) and [OpenGFX 8.0](https://www.openttd.org/downloads/opengfx-releases/8.0) archives match their published SHA256 values (recorded in JSON); the extracted EXE also matches the original ZIP member. Game assets are not bundled or committed.

An isolated **local-workstation** native-runtime diagnostic starts the actual game with a loopback-only target and observes a TCP listener. The official Windows GUI build does not provide console output/echo through the tested launcher, the UDP identity check is not satisfied, and quit does not produce graceful termination. The owned Job Object is forcibly terminated with successful native cleanup. The diagnostic correctly returns failure. A listening port is not game certification.

OpenTTD's platform CreateConsole/CRT initialization and a supported console build or game-specific control adapter need further investigation. No OpenTTD panel server/profile/account is created and no public port is opened. Save/reload, player join, game installer/callbacks, separate-identity game execution and deletion are not verified. The VM has approximately 2.4 GiB RAM, so a lightweight game was selected before attempting a larger server distribution.

## Remaining work and scheduling

This advances broad console-adapter issue #26; full real-game issue #27 stays open. No accepted task is split out: **22 accepted / 23 open milestone tasks out of 45**, all six milestones open. A measured delivery forecast is not available. The earlier weeks-to-months wording was a rough effort assessment, not a minimum calendar wait; automation reduces operator workload but does not replace compatibility evidence. [Completion assessment](../COMPLETION-STATUS.md).

Fixed terminal size, raw VT/redraw behavior, lossy bounded output, conhost accounting outside root metrics and cooperative native cleanup are documented in [scope/limits](../PSEUDOCONSOLE.md). No GUI automation, full tenant isolation, complete game compatibility or new reboot is claimed.
