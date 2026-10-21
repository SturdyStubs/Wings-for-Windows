# Windows profile schema v1

Profiles are local operator approval for a server's executable, arguments and prerequisites. The panel cannot edit these requirements or select a different executable. The daemon must be restarted after editing profiles.

```json
{
  "schema_version": 1,
  "executable": "C:\\Games\\Example\\Dedicated.exe",
  "arguments": ["--port", "{{SERVER_PORT}}"],
  "allowed_environment": [],
  "ready_pattern": "^SERVER_READY$",
  "stop_command": "stop",
  "stop_timeout_seconds": 20,
  "requirements": {
    "architecture": "amd64",
    "minimum_windows_build": 20348,
    "required_files": ["C:\\Games\\Example\\runtime.dll"]
  }
}
```

This is an illustrative profile, not a supported game recipe. Set requirements using the game's actual vendor documentation. The build number in this example is not a declaration of host certification.

## Migration and validation

Existing profiles with omitted/zero `schema_version` normalize to v1. Explicit v1 is recommended for new configurations. Other versions fail configuration loading rather than being silently interpreted. Unknown fields still fail strict JSON decoding. Argument, environment, readiness, stop and memory behavior remains as described in [setup](SETUP.md).

| Requirement | Meaning |
| --- | --- |
| `architecture` | Optional `amd64` or `386`, matched against the executable PE machine. Omitted accepts either. The daemon itself requires Windows amd64. A matching 386 header does not prove WOW64 or game dependencies are installed. |
| `minimum_windows_build` | Optional integer 0–1,000,000. Zero/omitted disables the minimum check. Compares with the running Windows build, not the marketing release name or Server/Desktop edition. |
| `required_files` | Optional list of up to 64 absolute local file paths that must open for reading as regular files. No variable expansion, search paths, relative paths, UNC or device namespaces. |

The executable must be a readable Windows PE application, with an executable header and optional header, not a DLL/object file. Unsupported machines fail. These checks run even when no optional requirements are declared. Windows process creation remains the final authority on whether an image can run.

Dependency files are checked for readable regular-file presence. Their extensions, versions, exports, hashes, signatures and transitive dependencies are not inspected. Preflight never loads a DLL or executes a helper to discover dependencies. Optional frameworks, registry-installed runtimes, licenses, drivers, networking and headless game behavior need separate checks and live validation.

## Optional network policy

`network_policy: {"mode": "block_connections"}` selects an experimental SID-scoped WFP block path and requires `run_as`. Omitted/empty mode or `unmanaged` preserves existing behavior. Unknown modes fail validation. See [network policy limits and verification](NETWORK-POLICY.md); no allocation allowlist or live filtering certification is supplied.

## Running preflight

```powershell
.\wings-windows.exe --config C:\WingsWindows\config.json --check
.\wings-windows.exe --config C:\WingsWindows\config.json --check-profiles
```

`--check` validates JSON and configuration rules. `--check-profiles` additionally checks every local profile in UUID order, prints pass/failure details and exits with status 1 if any profile fails. It does not contact the panel, create server directories, launch an executable or change files. It does not validate remote egg settings or TLS connectivity. Use one check flag at a time.

Run checks using the daemon's eventual service account: an administrator's file access can differ. The profile preflight also runs before each actual start and before the manual installation-success callback. Startup preflight failures appear in console history and leave the game offline. Failed installation preflight sends no installation-success callback. A missing requirement is not installed automatically.

Checks are snapshots. Files can change afterward; no authenticity or isolation guarantee is implied. Profiles without `run_as` run as the daemon account. An experimental [identity reference](IDENTITIES.md) can select a pinned local account with restricted-token launch and an ACL guard; complete isolation and separate-account deployment validation remain open. Keep the [current trusted-node restriction and planned isolation model](ISOLATION.md) in mind when approving executables and paths.

## Optional crash recovery

Profile v1 accepts `crash_recovery` with explicit `enabled`, bounded `max_restarts`, `window_seconds` and `delay_seconds`. It is disabled by default and applies only to unexpected nonzero game exits while this daemon remains active. Optional `persist_history` requires global `console_logs` and retains consumed retry reservations across daemon restarts without starting games or resuming timers; see [persistent budgets](RECOVERY-STATE.md). See [behavior and limits](CRASH-RECOVERY.md).
