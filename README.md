# Wings for Windows

Run native Windows game-server executables through a Pterodactyl panel. This is a separate Go daemon implementing a **subset of the Wings protocol**, with Windows Job Objects replacing the Docker process runtime.

**Version: 0.1.0-alpha.24. Experimental, for a trusted operator and disposable test servers.** Local Windows process and protocol tests pass; a live panel, Windows Server host and real game server have not yet been verified together. This is not an official Pterodactyl release or a drop-in replacement for upstream Wings.

Follow the [progress tracker](ROADMAP.md), [GitHub milestones](https://github.com/SturdyStubs/Wings-for-Windows/milestones), [changelog](CHANGELOG.md) and [original parity audit](docs/WINGS-PARITY-AUDIT.md). The current release adds [approved JSON egg configuration](docs/EGG-JSON-CONFIG.md), alongside [text-file rewriting](docs/EGG-TEXT-CONFIG.md) before game launch. Existing features include [persistent retry budgets](docs/RECOVERY-STATE.md), [console history](docs/CONSOLE-LOGS.md) and [managed service setup](docs/SERVICE-PROVISIONING.md). Reboot testing remains deferred; real panel/game compatibility and full isolation gates remain open.

<!-- progress:start -->

## Project progress

[Milestone progress bars](https://github.com/SturdyStubs/Wings-for-Windows/milestones) update as their task issues close. The badges below refresh automatically through Shields.io; GitHub/image caches may delay updates.

| Milestone | Live issue progress | Tracking |
| --- | --- | --- |
| M1: Core protocol compatibility | [![M1 closed tasks](https://img.shields.io/github/milestones/progress/SturdyStubs/Wings-for-Windows/1?label=tasks&cacheSeconds=300)](https://github.com/SturdyStubs/Wings-for-Windows/milestone/1) | [Details #1](https://github.com/SturdyStubs/Wings-for-Windows/issues/1) |
| M2: Isolation and threat model | [![M2 closed tasks](https://img.shields.io/github/milestones/progress/SturdyStubs/Wings-for-Windows/2?label=tasks&cacheSeconds=300)](https://github.com/SturdyStubs/Wings-for-Windows/milestone/2) | [Details #2](https://github.com/SturdyStubs/Wings-for-Windows/issues/2) |
| M3: Provisioning and recovery | [![M3 closed tasks](https://img.shields.io/github/milestones/progress/SturdyStubs/Wings-for-Windows/3?label=tasks&cacheSeconds=300)](https://github.com/SturdyStubs/Wings-for-Windows/milestone/3) | [Details #3](https://github.com/SturdyStubs/Wings-for-Windows/issues/3) |
| M4: Files and SFTP | [![M4 closed tasks](https://img.shields.io/github/milestones/progress/SturdyStubs/Wings-for-Windows/4?label=tasks&cacheSeconds=300)](https://github.com/SturdyStubs/Wings-for-Windows/milestone/4) | [Details #4](https://github.com/SturdyStubs/Wings-for-Windows/issues/4) |
| M5: Backups and transfers | [![M5 closed tasks](https://img.shields.io/github/milestones/progress/SturdyStubs/Wings-for-Windows/5?label=tasks&cacheSeconds=300)](https://github.com/SturdyStubs/Wings-for-Windows/milestone/5) | [Details #5](https://github.com/SturdyStubs/Wings-for-Windows/issues/5) |
| M6: Operations and validation | [![M6 closed tasks](https://img.shields.io/github/milestones/progress/SturdyStubs/Wings-for-Windows/6?label=tasks&cacheSeconds=300)](https://github.com/SturdyStubs/Wings-for-Windows/milestone/6) | [Details #6](https://github.com/SturdyStubs/Wings-for-Windows/issues/6) |

**Completion means all six milestones are closed after their acceptance checks, including live panel/game/service validation.** Task counts are not estimates of engineering effort or universal Linux egg compatibility. The current release remains experimental.

GitHub Issues are the source of truth; ROADMAP.md records implementation scope and verification. Run `scripts/Update-Progress.ps1` to regenerate this section. Use `-Initialize` to add missing planned milestones/tasks; rerunning it never automatically re-closes existing issues.
<!-- progress:end -->

Alpha 23 verification: **394 local cases/subtests**, **14 packaged native cases** and **35 Server 2022 service/persistence/text checks** pass. See the [evidence and remaining gates](docs/evidence/server2022-alpha23-egg-text.md).

## What works

| Feature | Implementation |
| --- | --- |
| Native executables | Versioned local profiles, PE architecture checks, optional minimum Windows build and dependency-file requirements before startup or installation acknowledgement. Argument arrays without shell expansion. |
| Game identities | Optional per-server credential reference and pinned local SID, restricted-token launch, daemon process/token checks and bounded recursive/ancestor ACL audit with code/dependency protection, link rejection, required launch permissions and a bounded startup path guard. Real separate-account batch launch passed on Server 2022; SCM demo launch passes; full isolation remains open. |
| Network policy | Optional account-scoped WFP connection blocking, requiring `run_as`; installation/rollback paths tested with an injected backend. Live filtering is unverified. Default is unmanaged. |
| Panel discovery | Fetches this node's assigned servers with the node credentials; only registered local profiles can run. |
| Power controls | Start, stop, restart and forced kill; bounded asynchronous lock waits with `wait_seconds`. Kill cancels queued/active starts and restarts and interrupts graceful shutdown. Suspended start/restart requests return 400. |
| Process trees | Creates the process suspended, assigns it to a Job Object, then resumes it. Killing/exiting the root or closing the daemon kills descendants. |
| Resource limits | Job memory and panel CPU limits update on sync without a restart; local memory caps remain authoritative. CPU `100` means approximately one core. |
| Console | Combined stdout/stderr, bounded recent logs, single-line stdin commands and authenticated WebSocket events. |
| Status | Offline, starting, running after local readiness or opted-in egg literal/regex matches with ANSI stripping, and stopping. |
| Statistics | Root-process CPU, working-set memory and uptime. Network and disk accounting are unavailable and identified as such in `native_metrics`. |
| File API | Wings-compatible listing URL/metadata, MIME/length/download headers, confined reads/writes up to 8 MiB and case-insensitive egg denylist checks. Contract tests pass; live panel verification remains open. |
| Egg configuration | Opt-in text prefixes and bounded JSON paths/arrays/wildcards, exact file approval and protected-path checks. Other parsers remain open. |
| Startup variables | Derived `SERVER_IP`, `SERVER_PORT`, `SERVER_MEMORY`, `STARTUP` and `TZ`, alongside explicitly allowed egg variables. |
| Game crash recovery | Opt-in delayed restart after unexpected nonzero exits, bounded attempt window, fresh security checks and operator cancellation. Optional persistent retry budget; games remain offline after daemon restart. |
| Persistent console logs | Optional rotated per-server JSONL, restored history, bounded queue and explicit drop/error counts. |
| Windows service | Listener-ready SCM state, optional secret-safe Application lifecycle/failure events, stop/shutdown, dedicated account/initial rights/ACL setup and daemon-context game-credential create/inspect/replace/remove. |

There is **no Docker dependency**. Windows runs the actual Windows executable. The panel itself still runs separately in its supported environment.

## Boundaries

Job Objects manage processes; they **do not isolate files, credentials, accounts or the network like containers**. Profiles without `run_as` still run as the daemon's Windows account and can expose credentials and other servers' files. The experimental [identity path](docs/IDENTITIES.md) adds distinct-account checks and restricted tokens, with a passing separate-account batch-task probe on Server 2022. SCM demo launch passes; remaining filesystem/network isolation gates are open. Use this on a private node where you trust all code and all people with access to server files/console. It is unsuitable for public or mutually untrusted hosting.

Linux egg installation scripts, automatic installs/updates, SFTP, signed upload/download URLs, backups, transfers, scheduled tasks that need unsupported endpoints, RCON, disk quotas and automatic daemon restart are not implemented. Unsupported operations return HTTP 501. File editing works through the panel API, but its upload/download and SFTP features do not. Games that require RCON or a real interactive console instead of stdin/stdout need an additional adapter.

The separate Rust Server Wiper extension is **not yet validated against this daemon**; required backup/delete/restore operations are currently unsupported. Keep existing Linux nodes on upstream Wings. Existing Linux eggs require a Linux execution backend; native Windows games need Windows-specific profiles and installers.

## Build or download

Build from a Windows x64 terminal with **Go 1.26 or newer**:

```powershell
git clone https://github.com/SturdyStubs/Wings-for-Windows.git
cd Wings-for-Windows
.\scripts\Build.ps1
```

If Go is not on PATH, pass `-GoExecutable C:\path\to\go.exe`. The script runs the Windows integration tests and `go vet`, then creates:

```text
dist/wings-windows-native-0.1.0-alpha.24-windows-amd64.zip
dist/wings-windows-native-0.1.0-alpha.24-windows-amd64.zip.sha256
```

The archive includes the daemon, a harmless console demo executable, compiled runtime tests, a host-report script, configuration example, documentation and service script. Docker, Node, PHP and a C compiler are not needed to build it. Native integration tests compile a temporary demo executable using Go.

## Test a Windows VM first

No Go or panel setup is needed on the VM. From a clean extracted package, run:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\Test-Host.ps1 -OutputDirectory C:\WingsEvidence\baseline
```

This launches harmless fixtures and writes a JSON host report plus the test log. It checks package hashes and requires every listed runtime test to pass with zero skips. Use a new output directory each run. Passing this baseline does not certify separate game accounts, a Windows service, panel integration or WFP filtering. See the [VM workflow](docs/WINDOWS-SERVER-TESTING.md).

## Try it with your panel

Follow the [setup guide](docs/SETUP.md) for the full procedure. The first test should use the bundled demo executable rather than a production game server.

1. Extract the archive into `C:\WingsWindows`. Create a dedicated Windows node in your test panel and copy its UUID, token ID and token from its configuration page.
2. Create a test server assigned to that node. Its initial Linux installation request will fail with an unsupported-operation response; this alpha uses manual provisioning. Record the **full server UUID**, not its short identifier.
3. Copy `configs\config.example.json` to `config.json`. Set the panel URL, credentials, TLS paths, data directory and profile key to the real values. Keep the demo executable profile for the first test.
4. Run the commands below. Acknowledge installation only after the profile and executable are provisioned. Run `--check-profiles` to check local requirements, then run `--mark-installed` with the daemon stopped; it repeats preflight and makes an explicit panel installation-success callback without running an installer.

```powershell
cd C:\WingsWindows
.\wings-windows.exe --config C:\WingsWindows\config.json --check
.\wings-windows.exe --config C:\WingsWindows\config.json --mark-installed YOUR-FULL-SERVER-UUID
.\wings-windows.exe --config C:\WingsWindows\config.json
```

5. Open that server in the panel. Start it, confirm `SERVER_READY` and a running status, send `hello` and check for `ECHO:hello`. Stop, restart and kill it. Complete the [live verification checklist](docs/VERIFICATION.md) before adding a real game.

Use TLS with a certificate trusted by both the panel host and the browser. The panel's configured node hostname, port and HTTPS setting must point to this daemon. The daemon permits HTTP only on an explicitly enabled literal loopback address; this does not provide connectivity from a separate panel host.

To add a real server, install its Windows files yourself, add a profile using the full panel server UUID, set the executable, argument list, allowed variables, readiness regex and stop command, then restart the daemon. The working directory is always `<data_directory>\<server UUID>`. Editing a panel startup command never changes the locally approved executable or argument template.

## Documentation

- [Roadmap and current milestone](ROADMAP.md)
- [Version history](CHANGELOG.md)
- [Setup, profiles, TLS and Windows service](docs/SETUP.md)
- [Windows Server VM validation and evidence](docs/WINDOWS-SERVER-TESTING.md)
- [Profile schema and preflight checks](docs/PROFILES.md)
- [Isolation design and acceptance gates](docs/ISOLATION.md)
- [Service-account credential provisioning](docs/SERVICE-CREDENTIALS.md)
- [Experimental game identities and credential provisioning](docs/IDENTITIES.md)
- [Credential inspection, rotation and retirement](docs/CREDENTIAL-LIFECYCLE.md)
- [Identity deployment diagnostic](docs/IDENTITY-DIAGNOSTICS.md)
- [Experimental network policy and validation gates](docs/NETWORK-POLICY.md)
- [WFP installation/cleanup diagnostic](docs/NETWORK-DIAGNOSTICS.md)
- [Supported API and architecture](docs/PROTOCOL.md)
- [Verification results and live test checklist](docs/VERIFICATION.md)
- [Wings parity audit and implementation priorities](docs/WINGS-PARITY-AUDIT.md)
- [Third-party notices](THIRD-PARTY-NOTICES.md)

Protocol reference: [upstream Wings](https://github.com/pterodactyl/wings/tree/747bc43), snapshot `747bc43` from October 13, 2026. This implementation uses its API conventions; the upstream Linux runtime remains separate. MIT licensed; see [LICENSE](LICENSE).

Alpha 17 final-package SCM validation: **11 checks passed** on Server 2022, including registration/start failure rollback. [Evidence and cleanup limits](docs/evidence/server2022-alpha17-service.md). Real panel/game, reboot/profile recovery and WFP gates remain open.
