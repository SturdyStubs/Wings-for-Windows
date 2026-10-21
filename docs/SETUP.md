# Setup and operator guide

## Requirements

- Windows x64. Development was exercised on this Windows workstation. Windows Server 2022/2025 are intended targets and still require host verification.
- A disposable Pterodactyl node and server. The protocol was compared with upstream Wings commit `747bc43`, not validated with a live panel. Panel 1.x and 2.x compatibility must each be checked before claiming support.
- A Windows executable that runs without a desktop session and accepts stdin/stdout, or the bundled demo executable. GUI-only programs and RCON-only game consoles need an adapter.
- A hostname and trusted TLS certificate for the node, reachable by the panel backend and the user's browser. Configure DNS, NAT and firewall rules yourself for the API and each game port. The daemon does not bind allocations, enforce network isolation or open ports.

Do not replace an existing production Wings installation with this alpha.

## Configure the node

The JSON file is deliberately separate from upstream Wings' YAML. Copy its `uuid`, `token_id`, `token` and panel URL into the corresponding JSON fields. There is no automatic conversion. `node_uuid` identifies the configured node for the operator; remote authorization is bound to the panel-issued token ID and secret.

| Field | Meaning |
| --- | --- |
| `listen` | API bind address. Public binds require TLS. Example `0.0.0.0:8080`. |
| `panel_url` | Canonical panel base URL, usually `https://panel.example.com`. Used for remote API requests and the JWT issuer. |
| `node_uuid`, `token_id`, `token` | Node credentials from the panel; token must be at least 32 characters. Never use a client/application API key here. |
| `data_directory` | Absolute path for per-server working directories. |
| `timezone` | Optional IANA timezone for derived `TZ`; defaults to `UTC`. Games may interpret timezone environment settings differently on Windows. |
| `tls.cert`, `tls.key` | Absolute PEM certificate-chain and private-key paths. Both are required for a public listener. |
| `allow_insecure_loopback` | Development exception for literal `127.0.0.1` or `::1` HTTP. Does not permit remote HTTP or arbitrary hostnames. |
| `allow_cors_private_network` | Optional boolean, default false. Enables the private-network CORS response header for browser access to a private node. Actual requests still require authentication. |
| `allowed_origins` | Additional exact browser origins, without trailing slashes. The panel origin is already allowed. No wildcard. |
| `profiles` | Map of full server UUID to locally approved execution profile. Restart the daemon after editing. |

You can leave `token` blank and set `WINGS_NATIVE_TOKEN` in the daemon's environment instead. Child executables receive a newly constructed environment, so that variable is not inherited. This is not a security boundary against code running as the same account.

Restrict the configuration, service executable and TLS private key ACLs to administrators and the dedicated daemon account. Give that account read/execute permissions on the games and write access to their data directories. Do not place the node configuration or TLS key inside a server's file-editor directory.

`--check` validates JSON structure and configuration rules without contacting the panel, checking certificate trust, opening the listener or launching an executable. It does not prove that paths exist or that the game works.

## Manually provision a server

1. Create a server in the panel assigned to this node. The alpha deliberately rejects automatic egg installation. The panel can show an installation failure until the manual acknowledgement below.
2. Record the full server UUID from its admin page/API. Create a corresponding profile in `config.json` and provision the executable and its dependencies on disk. Server data belongs under `C:\WingsWindows\data\<UUID>` if using the example data directory.
3. Set panel egg variables needed by your argument template. Add only those names to `allowed_environment` in the local profile.
4. Stop any running daemon, run `--check` and `--check-profiles`, then `--mark-installed <UUID>`. This fetches the node's assigned servers, runs local executable/OS/dependency preflight and explicitly reports manual installation success. It never runs a Linux installer or downloads game files.
5. Start the daemon and verify the server from the panel console.

The acknowledgement checks the executable and explicitly declared requirements. It remains an operator statement, not an automatic dependency resolver or a completeness check on all game files. Repeating it sends another installation-success callback. If your panel's server-creation transaction rolls back after the rejected installation request, create/retain the server through that panel's supported administrative workflow first; there is no automatic panel database patch.

## Profiles

See [profile schema v1 and preflight](PROFILES.md) for migration, requirement fields and limits. Existing profiles without `schema_version` use v1 automatically. `--check` validates configuration only; `--check-profiles` additionally inspects local files and the Windows host, without panel requests or process execution.

```json
{
  "schema_version": 1,
  "executable": "C:\\WingsWindows\\games\\ExampleDedicated.exe",
  "arguments": ["--port", "{{SERVER_PORT}}", "--name", "{{SERVER_NAME}}"],
  "allowed_environment": ["SERVER_PORT", "SERVER_NAME"],
  "stop_command": "quit",
  "ready_pattern": "^Listening on port ",
  "stop_timeout_seconds": 30,
  "memory_mb": 4096
}
```

This is a profile fragment for a hypothetical stdio server, not a verified game's command line. Use the executable vendor's actual arguments and console behavior.

Each argument is substituted independently and escaped for Windows process creation. `{{NAME}}` can reference an allowlisted egg environment variable or a derived `SERVER_IP`, `SERVER_PORT`, `SERVER_MEMORY`, `STARTUP` or `TZ` value; missing values fail startup. Derived values take precedence over egg variables. There is no `cmd.exe`, PowerShell, shell interpolation or executable-path substitution. Reserved environment names such as `PATH`, `TEMP` and `SystemRoot` cannot be overridden through the panel. SystemRoot/WINDIR, a minimal executable/System32 PATH and per-server TEMP/TMP are supplied locally. Some games need additional DLL locations or variables; profile support for those should be added explicitly rather than inheriting the daemon's entire environment.

Choose a readiness regex that appears only once the game can accept connections. A process that stays alive without matching remains `starting`; there is no automatic readiness timeout yet. Output is read from stdout/stderr, not from a game-specific log file. Very long lines are emitted in bounded chunks. ANSI color sequences may affect regex matching; account for them or disable colored game logs.

Stopping sends one configured stdin line, waits the configured number of seconds, then kills the job if needed. An empty stop command immediately kills the job. Input has a bounded queue and a two-second write deadline at the API level; a command that times out might still reach a slow reader later, so do not automatically retry destructive commands. No Ctrl+C/CTRL_BREAK or RCON adapter is included.

The smaller positive value of `memory_mb` and the panel memory limit becomes the job memory cap; zero means no local cap. The panel CPU limit uses one core as 100. CPU and memory statistics cover the root process, while limits apply to the job. There is no disk or network quota. Binding the panel's allocated IP/ports is the executable's responsibility through its arguments/configuration; games sharing a host can conflict if misconfigured.

## Optional game identities

See [experimental identity setup](IDENTITIES.md) before enabling `run_as`. It requires a pinned non-built-in local account SID, a per-server credential stored as the daemon account, service token privileges and suitable ACLs. The opt-in [managed service installer](SERVICE-PROVISIONING.md) creates a service account and initial rights/ACLs; game accounts and game credentials still require separate provisioning. Use the [credential lifecycle tools](CREDENTIAL-LIFECYCLE.md) for exact-target inspection, deliberate rotation and removal; they do not create/delete Windows users or reset passwords. Default profiles retain the daemon account. Full isolation and real game deployment remain unverified; Server 2022 SCM demo launch has passed.

Use [`--check-identity <UUID>`](IDENTITY-DIAGNOSTICS.md) as the eventual daemon identity for an explicit logon/token/ACL diagnostic after provisioning. It does not launch a game or contact the panel, and successful preflight does not replace actual service/game validation.

## Egg process settings and live limits

Set `use_egg_process_configuration: true` on a local Windows profile to use the panel's `process_configuration`. The locally approved executable, argument array and environment allowlist remain authoritative. Omit `ready_pattern` and `stop_command` in this mode; they are replaced by the egg's process settings. Existing profiles default to `false` and keep their local settings.

Supported readiness uses `startup.done` with 1–64 literal strings or `regex:` patterns, plus `startup.strip_ansi`. Invalid `regex:` expressions fall back to literal matching as upstream does. Supported stop strategies are `type: command` with a single stdin line, or `type: signal` with `SIGKILL` for explicit forced termination.

`SIGINT`, `SIGTERM`, Docker-native `stop`, automatic config-file rewrites (`configs`) and user-interaction adapters are not implemented. Profiles opting into an unsupported process configuration fail explicitly; they do not silently run Linux behavior or substitute a forced stop. Node initialization can therefore fail if a registered opted-in profile has unsupported settings. `--check` cannot validate remote egg settings without contacting the panel.

Sync now applies supported CPU/memory changes to active jobs. The smaller positive panel/local memory limit remains in force; zero removes the corresponding cap when no local cap exists. The game keeps the same process and uptime. If an update is rejected, the API reports failure and does not display the requested value as applied. If a partial update cannot be rolled back, the job is terminated rather than left with unknown enforcement. Future allocations are subject to the new memory cap; a lower cap is not a guarantee of immediate termination of an already large process.

## TLS and connectivity

Use the same node hostname in the panel, certificate and browser connection. Both the API client on the panel and the browser must trust its certificate. Certificate renewal requires replacing the PEM files and restarting the daemon. Self-signed certificates require explicitly installed trust; certificate validation is not disabled.

For a local-only development panel, set `listen` to `127.0.0.1:8080`, clear both TLS paths and enable `allow_insecure_loopback`. A loopback panel URL may use HTTP under the same explicit exception. A remote panel cannot reach your Windows loopback listener. A browser served over HTTPS may also reject an insecure WebSocket URL.

No automatic Windows Firewall or NAT changes are made. Expose only the required ports to appropriate networks. Native game executables share the account and host network.

## Register as a Windows service

For a new node, use the [managed service setup guide](SERVICE-PROVISIONING.md) to create the service account and initial permissions. The manual registration example below assumes those permissions were provisioned separately.

First validate the foreground run. Create a dedicated **non-administrator** account with **Log on as a service**, grant its required directory/file ACLs and obtain its credential. Do not use LocalSystem or an administrator account. This still does not isolate one server from another.

In an elevated PowerShell terminal:

```powershell
$credential = Get-Credential '.\WingsRuntime'
C:\WingsWindows\scripts\Install-Service.ps1 `
  -Executable C:\WingsWindows\wings-windows.exe `
  -Configuration C:\WingsWindows\config.json `
  -Credential $credential
Start-Service WingsWindowsNative
Get-Service WingsWindowsNative
```

The script validates the configuration and registers the service with manual startup. Without `-ConfigurePermissions`, it does not grant account rights or adjust ACLs. The opt-in managed mode grants scoped rights and initial ACLs; neither mode modifies firewall rules or enables automatic recovery. Set automatic startup only after testing host reboots. The service runs the same daemon and kills its managed jobs on shutdown; it does not adopt previously running external game processes.

A disposable Server 2022 SCM test passed using separate non-administrator daemon/game accounts, a loopback mock panel and demo processes. Cold reboot, real panel/game and profile recovery remain unverified. Foreground daemon diagnostics go to stderr; persistent daemon/service/Event Log integration remains future work. Game console history can now be stored separately through [console_logs](CONSOLE-LOGS.md).

To update, stop the service, replace the executable and restart it. Stopping the daemon terminates managed games; arrange an appropriate maintenance window. To uninstall the service, stop it and use Windows' `sc.exe delete WingsWindowsNative`. Game files and configuration remain on disk.

## Troubleshooting

| Symptom | Check |
| --- | --- |
| Node appears offline | Node hostname/port/TLS, certificate trust from panel host, node token, firewall and foreground daemon logs. |
| Unknown local server profile | Full UUID, profile spelling and whether the server belongs to this node. Restart after editing configuration. |
| Installation failed | Expected until manually provisioning and explicitly running `--mark-installed`. Linux installers are unsupported. |
| Start accepted but server stays offline | Recent console logs: missing executable/DLL/argument, panel sync failure, suspension or job assignment failure. |
| Stays starting | Readiness regex must match stdout/stderr; a log-file-only game needs an adapter. |
| Console is blank | Game may require a real console, RCON or log-file tailing. Use the bundled demo to distinguish runtime from game issues. |
| Console token rejected | Refresh the panel page for a fresh token after daemon restart; check clock, issuer, origin and permissions. |
| Memory/disk/network values seem incomplete | Root-process working set only; disk/network measurements and quotas are unavailable. |
| Game ports unreachable | Check the actual game's bind arguments, conflicts, network routing and firewall. The daemon does not bind allocations for the game. |
