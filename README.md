# Wings Windows Native

Run native Windows game-server executables through a Pterodactyl panel. This is a separate Go daemon implementing a **subset of the Wings protocol**, with Windows Job Objects replacing the Docker process runtime.

**Version: 0.1.0-alpha.1. Experimental, for a trusted operator and disposable test servers.** Local Windows process and protocol tests pass; a live panel, Windows Server host and real game server have not yet been verified together. This is not an official Pterodactyl release or a drop-in replacement for upstream Wings.

The [Wings parity audit](docs/WINGS-PARITY-AUDIT.md) found additional compatibility defects in this alpha, including the file-listing URL/schema, missing egg file restrictions and emergency-kill locking. The implemented helpers below do not establish complete panel compatibility; correct these defects before relying on those workflows.

## What works

| Feature | Implementation |
| --- | --- |
| Native executables | Locally approved `.exe` path and an argument array for each server UUID. No shell expansion. |
| Panel discovery | Fetches this node's assigned servers with the node credentials; only registered local profiles can run. |
| Power controls | Start, stop, restart and forced kill. Suspended servers cannot start. |
| Process trees | Creates the process suspended, assigns it to a Job Object, then resumes it. Killing/exiting the root or closing the daemon kills descendants. |
| Resource limits | Optional job memory limit and panel CPU limit; CPU `100` means approximately one core. |
| Console | Combined stdout/stderr, bounded recent logs, single-line stdin commands and authenticated WebSocket events. |
| Status | Offline, starting, running after a configured readiness regex, and stopping. |
| Statistics | Root-process CPU, working-set memory and uptime. Network and disk accounting are unavailable and identified as such in `native_metrics`. |
| File API helpers | Confined listing, reading and writing of text files up to 8 MiB; listing URL/schema currently differ from Wings, so real panel file browsing is not supported yet. |
| Windows service | SCM stop/shutdown support and a service registration script. |

There is **no Docker dependency**. Windows runs the actual Windows executable. The panel itself still runs separately in its supported environment.

## Boundaries

Job Objects manage processes; they **do not isolate files, credentials, accounts or the network like containers**. Every game runs as the daemon's Windows account. A game, native plugin or executable under that account could read the node token and other servers' files. Use this on a private node where you trust all code and all people with access to server files/console. It is unsuitable for public or mutually untrusted hosting.

Linux egg installation scripts, automatic installs/updates, SFTP, signed upload/download URLs, backups, transfers, scheduled tasks that need unsupported endpoints, RCON, disk quotas and automatic crash restart are not implemented. Unsupported operations return HTTP 501. File editing works through the panel API, but its upload/download and SFTP features do not. Games that require RCON or a real interactive console instead of stdin/stdout need an additional adapter.

The Rust Server Wiper extension in this repository is **not yet validated against this daemon**; required backup/delete/restore operations are currently unsupported. Keep existing Linux nodes on upstream Wings.

## Build or download

Build from a Windows x64 terminal with **Go 1.26 or newer**:

```powershell
cd wings-windows
.\scripts\Build.ps1
```

If Go is not on PATH, pass `-GoExecutable C:\path\to\go.exe`. The script runs the Windows integration tests and `go vet`, then creates:

```text
dist/wings-windows-native-0.1.0-alpha.1-windows-amd64.zip
dist/wings-windows-native-0.1.0-alpha.1-windows-amd64.zip.sha256
```

The archive includes the daemon, a harmless console demo executable, configuration example, documentation and service script. Docker, Node, PHP and a C compiler are not needed to build it. Native integration tests compile a temporary demo executable using Go.

## Try it with your panel

Follow the [setup guide](docs/SETUP.md) for the full procedure. The first test should use the bundled demo executable rather than a production game server.

1. Extract the archive into `C:\WingsWindows`. Create a dedicated Windows node in your test panel and copy its UUID, token ID and token from its configuration page.
2. Create a test server assigned to that node. Its initial Linux installation request will fail with an unsupported-operation response; this alpha uses manual provisioning. Record the **full server UUID**, not its short identifier.
3. Copy `configs\config.example.json` to `config.json`. Set the panel URL, credentials, TLS paths, data directory and profile key to the real values. Keep the demo executable profile for the first test.
4. Run the commands below. Acknowledge installation only after the profile and executable are provisioned. Run `--mark-installed` with the daemon stopped; it makes an explicit panel installation-success callback without running an installer.

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

- [Setup, profiles, TLS and Windows service](docs/SETUP.md)
- [Supported API and architecture](docs/PROTOCOL.md)
- [Verification results and live test checklist](docs/VERIFICATION.md)
- [Wings parity audit and implementation priorities](docs/WINGS-PARITY-AUDIT.md)
- [Third-party notices](THIRD-PARTY-NOTICES.md)

Protocol reference: [upstream Wings](https://github.com/pterodactyl/wings/tree/747bc43), snapshot `747bc43` from October 13, 2026. This implementation uses its API conventions; the upstream Linux runtime remains separate. MIT licensed; see [LICENSE](LICENSE).
