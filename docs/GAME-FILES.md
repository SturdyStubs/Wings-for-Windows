# Import approved game files into protected code storage

Alpha 38 adds `scripts/Import-ServiceGameFiles.ps1` for an existing managed node. It imports operator-approved local files into a **new** `bin\games\FULL-UUID` directory while `WingsWindowsNative` is stopped. Run it as an administrator in **Windows PowerShell 5.1**, before adding that UUID's local profile. It never overwrites an existing destination, rewrites profiles, changes existing code/data ACLs, downloads software or runs an installer.

Prepare a local source directory outside the managed installation. Obtain the Windows executable and dependencies through the game's supported distribution process. Review their provenance/license and expected hashes. The manifest is an integrity checklist; hashes alone do not establish publisher trust or game compatibility.

Create a UTF-8 manifest outside the managed installation:

```json
{
  "schema_version": 1,
  "files": [
    { "path": "server.exe", "sha256": "REPLACE_WITH_64_HEX_DIGITS" },
    { "path": "deps/runtime.dll", "sha256": "REPLACE_WITH_64_HEX_DIGITS" }
  ]
}
```

Every entry has exactly `path` and `sha256`. Only listed files are copied; other source files are ignored. Paths are relative, case-insensitively unique, at most 16 components/220 characters, using ordinary ASCII Windows names with internal spaces, dots, underscores or hyphens. Leading dots, trailing spaces/dots, reserved device names, traversal, streams, rooted paths, links and multi-hardlink files are refused. The source cannot contain the installation or be inside it. The manifest must be outside the installation too.

```powershell
Stop-Service WingsWindowsNative
$root = 'C:\WingsWindows'
$server = 'YOUR-FULL-SERVER-UUID'
$manifest = 'C:\ApprovedGame\manifest.json'
# Pin the hash of the manifest you reviewed; keep that expected value stable.
$manifestHash = (Get-FileHash -LiteralPath $manifest -Algorithm SHA256).Hash
$options = @{
    InstallationRoot = $root
    ServerUUID = $server
    SourceDirectory = 'C:\ApprovedGame\files'
    ManifestPath = $manifest
    ManifestSHA256 = $manifestHash
}
& "$root\bin\scripts\Import-ServiceGameFiles.ps1" @options -WhatIf
& "$root\bin\scripts\Import-ServiceGameFiles.ps1" @options
```

WhatIf verifies the stopped managed service, existing identities/code-parent ACLs, manifest hash and every listed source hash without creating directories. Bounds are 1 MiB of manifest text, 1–1000 files and 8 GiB total source bytes. Larger game distributions need a future provisioning path; this is not a general package manager.

The real command shares the credential-provisioning lock and holds node configuration against ordinary writes/replacement. Source streams stay open without write/delete sharing from verification through copy. It creates a random staging directory under the protected code parent, initially accessible only to SYSTEM and Administrators, copies with exclusive new-file creation, flushes and verifies the staged bytes, then publishes by a same-parent directory move. An existing destination makes promotion fail rather than merging or overwriting it.

Published files have protected SYSTEM/Administrators full control and daemon read/execute access. No game account gets write access. The shared `games` parent permits traversal/read by current configured identities; the new subtree initially grants only the daemon. Later, the [incremental permission helper](GAME-PERMISSIONS.md) grants the new identity read/execute after explicit profile approval. This is the existing trusted-node permission model, not complete code or process isolation.

## Complete the handoff

Create the dedicated account with the [account helper](GAME-ACCOUNTS.md) before approving its profile. Account creation and file import may be performed in either order, while that UUID is still unapproved. Set the approved profile's executable/installer paths under the returned destination, declare required dependency files, run `--check-profiles`, then prepare permissions and daemon-context credentials. Start the service, invoke the approved native data/setup installer and launch manually. The importer does not inspect PE architecture or discover dependencies; normal profile preflight performs declared checks before installation/launch.

## Failure and recovery limits

Failures clean up only files/directories created in this attempt, using non-recursive deletion. Existing destinations and older server data/logs are preserved. A new shared parent is removed only if empty; unexpected contents or cleanup errors are reported for inspection. No unverified partial destination is published during an ordinary copy failure.

Keep writers stopped during maintenance. These are bounded path/ACL snapshots and ordinary file-sharing protections, not confinement against hostile administrators, loader/mapping races or concurrent privileged changes. There is no durable crash journal: a crash before promotion can retain a protected `.stage-*` directory; a crash after the directory move may leave a complete destination even if the caller saw no success. Inspect hashes/state before retrying. New imports and incremental game-permission preparation refuse leftover `.stage-*` code until it is inspected; no automatic stale-stage deletion is provided. Updates, replacements, stale-stage recovery, automatic downloads and panel-driven provisioning remain open. Tests use a harmless demo and loopback panel mock, not a real game or live panel.
