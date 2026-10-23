# First Windows node and demo egg

Use a disposable, trusted test node. The native daemon runs Windows executables; unchanged Linux eggs and Docker images are not supported. This guide prepares a harmless console demo before a game-specific Windows recipe. A real panel/node/game integration run remains an open validation gate. Alpha 34 adds opt-in [manual-approval creation](PENDING-CREATION.md): retain the panel record first, then bind its full UUID to an approved Windows profile. Automatic installation/start during creation remains unavailable.

## 1. Prepare the Windows host

Use Windows Server 2022 amd64 for the first test, matching our fixture host. Extract the release into `C:\WingsWindows\bin`, create empty `private`, `data` and optional `logs` directories, and run packaged `scripts\Test-Host.ps1`. That offline test does not connect a panel.

Create a separate enabled non-administrator local game account and retain its password privately. Record its SID with `Get-LocalUser -Name YOUR_GAME_ACCOUNT | Select-Object Name,SID`. The daemon will use a different dedicated service account. Follow [managed service setup](SERVICE-PROVISIONING.md) and [credential provisioning](SERVICE-CREDENTIALS.md); installation by Administrator does not mean games should run as Administrator.

## 2. Create the panel's node record

In panel administration, create a location and a dedicated Windows test node. Use a hostname/IP the **panel backend and browser** can reach, the daemon's configured listen port, and valid trusted TLS for remote connections. A private VM address is usable only when the relevant clients can route to it. Do not expose unencrypted node credentials on a public connection. The loopback HTTP exception is exclusively for same-host fixtures.

Copy the node UUID, token ID and token from the panel's generated node configuration into our JSON fields `node_uuid`, `token_id`, `token`; set `panel_url` to the panel URL. Do not run the upstream Linux auto-deploy command or copy its YAML directly into this daemon. Use `configs/config.bootstrap.example.json` as the zero-profile native JSON starting point and keep credentials/TLS private keys under `private`.

Create a primary allocation using the Windows host's reachable address and an unused test port. The demo does not open a game port; the allocation exercises panel metadata only. Real games will additionally need their own documented TCP/UDP firewall and routing rules. The panel's SFTP setting does not make SFTP available in this daemon.

## 3. Bootstrap the daemon and create the demo record

Import [`egg-windows-native-demo.json`](../configs/eggs/egg-windows-native-demo.json) using your panel's egg import UI (a dedicated Windows test nest on panels that use nests). Its PTDL_v2 structure follows the reviewed panel import format. The image is a required compatibility placeholder, never pulled by this daemon. Its Linux script deliberately fails; never assign it to a stock Linux node.

For a new empty node, set `allow_pending_creation: true` and `profiles: {}` in private configuration. Run `--check`, then start the daemon temporarily in the foreground:

```powershell
C:\WingsWindows\bin\wings-windows.exe --config C:\WingsWindows\private\config.json
```

There are no approved executables in this bootstrap configuration. Use the current setup account only for this temporary handshake; stop it before adding execution profiles and move to the dedicated service identity. Keep node TLS/credentials private and the initial data/log directories empty.

Create the panel server with this egg, Windows node/allocation, 128 MiB memory and a small CPU limit. **Disable Start on completion** (`start_on_completion: false`). Leave the server's skip-install-script flag false if you intend to exercise the approved native setup recipe later. Creation should return 202 and retain the record while it waits for approval, without running the remote Linux script or sending an installation-success callback. The panel may show Installing during this wait.

Record the **full UUID**, not the short identifier. The daemon's authenticated GET response has `native_creation.stage: awaiting_profile`; power/files/install remain blocked. Stop the foreground daemon with Ctrl+C. No server data directory should have been created, so initial managed provisioning can now configure the new UUID and game identity. This source-derived workflow still needs a real panel session for certification; if creation fails, retain diagnostics without repeatedly creating records or patching the database.

## 4. Approve the UUID with a local native profile

Copy [`profile-windows-native-demo.json`](../configs/profile-windows-native-demo.json) as the value under `profiles.FULL_SERVER_UUID` in the node JSON. Replace its `run_as.sid` with the game account SID and `credential_target` with `WingsWindows/FULL_SERVER_UUID`. The profile launches the packaged demo from `bin` and approves that same executable's `install success` mode. Panel startup text is descriptive; the local argument array is authoritative.

Before starting the daemon, run:

```powershell
C:\WingsWindows\bin\wings-windows.exe --config C:\WingsWindows\private\config.json --check
C:\WingsWindows\bin\wings-windows.exe --config C:\WingsWindows\private\config.json --check-profiles
```

Complete managed service installation, store the game's credential in the **daemon account's** credential store with `Set-ServiceGameCredential.ps1`, then start the service. Initial managed setup requires empty server data/logs; do not rerun it over a populated installation. For a later UUID on an existing populated node, create its dedicated identity using the [account helper](GAME-ACCOUNTS.md) before profile approval, then use the [incremental game permission workflow](GAME-PERMISSIONS.md). Local profiles are loaded at daemon startup, so adding/changing a profile requires a restart. Confirm the node becomes reachable and the UUID appears in its assigned-server discovery.

## 5. Validate through the panel

Invoke **Reinstall** on the demo server. The approved native recipe should write `data\FULL_SERVER_UUID\install.marker`, emit installer output and finish with a successful panel callback, leaving the server offline. Start it, wait for `SERVER_READY`, send a harmless `hello` console command and look for `ECHO:hello`; stop it and confirm it remains offline. Test console reconnection and file browsing of the marker. SFTP, backups, transfers and deletion are still unavailable. Record the panel version, exact release/source revision and each result without tokens or passwords.

Once this works, select a game that provides a Windows dedicated-server binary and document its distribution/license, dependencies, noninteractive startup, readiness, console/RCON and shutdown behavior. Build a separate egg and approved local profile for it. The current native setup runner initializes data around operator-provisioned protected binaries; it cannot yet download/promote a whole game installation. See [installer scope](NATIVE-INSTALLERS.md) and the [roadmap](../ROADMAP.md).
