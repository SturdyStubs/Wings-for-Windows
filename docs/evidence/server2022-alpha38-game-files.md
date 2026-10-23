# Server 2022 alpha 38 protected game-file import validation

Validation date: October 22, 2026. The exact clean-source archive was transferred over fingerprint-pinned SSH to the disposable Windows Server VM. Packaged native tests, the combined managed-service fixture and independent read-only cleanup inspection passed.

| Field | Result |
| --- | --- |
| Host | Windows Server 2022 Standard Evaluation x64, build 20348; PowerShell 5.1.20348.558 |
| Clean package source | `06324e09b8679893f25f353540bd7370ee5378e0` |
| ZIP SHA256 | `8e7262eca617b0df5dcf552993f6ced0fccc14da798c56508861dcaa0c4afa0c` |
| Runtime-test SHA256 | `428f07aab4df4bb585932c92c4424841c8bdec66dcd0a221d6289151212a9251` |
| Demo SHA256 | `d560b3dd381bbdd39882ecb58c1cde3d45fb561f3d1142f98634cf0d92580046` |
| Local suite | 815 cases/subtests, zero skipped; vet and clean release build pass |
| Packaged Server 2022 baseline | 14 cases/subtests, zero failed/skipped; default runner invocation |
| Native baseline UTC | 2026-10-23T03:56:04.9685252Z through 2026-10-23T03:56:07.5947131Z |
| Combined service fixture | All 107 checks pass; all seven cleanup flags true |
| Independent cleanup audit | Pass; 1 retained profiles, 1 loaded; no forced unload/delete |

[Sanitized JSON results](server2022-alpha38-game-files-results.json) retain every check, source/package binding, fixture hash and cleanup counts. Fifteen setup/credential/Event Log/permission/account/import scripts match the immutable archive. Node configuration, account SIDs, passwords and raw private output are excluded.

## Six new managed-service checks

The earlier 101 checks remain. Before adding the new UUID profile, the fixture creates an external local source directory and a pinned manifest for the packaged harmless executable and a dependency file. It runs the immutable package's public importer and:

1. Verifies WhatIf creates no code directory.
2. Rejects an incorrect manifest hash before publication.
3. Corrupts a source file and rejects its hash mismatch before publication, then restores the approved bytes.
4. Imports both files, verifies final content and protected daemon read-only ACLs, and preserves configuration, older populated data/logs and parent ACL snapshots.
5. Refuses replay against the existing destination without changing the executable hash.
6. After profile/permission/credential approval, successfully installs and starts the imported executable under the actual dedicated game SID, then confirms that identity cannot open the non-executable imported dependency for writing. Root/child ownership and other-server data denial are also verified by the combined suite.

The write probe opens an ordinary dependency file without truncation; executable image locking cannot explain the observed denial. It writes no bytes even if access is unexpectedly allowed. All executables are harmless demos and the panel is an authenticated loopback mock. No new upstream route is implemented; coverage remains 16/38.

## Local and package checks

Twenty-two importer scenarios plus their parent test cover no-mutation planning, successful publication/content/ACLs, manifest/source mismatches, schema/size/file-count bounds, unknown fields, duplicate case-insensitive names, traversal/device aliases, existing destinations/profiles/staging, hardlinks/junctions, partial-copy and promotion failures, stale service identity, pinned-source write refusal and destination-race preservation. An additional permission-planner case refuses unfinished staging before granting game access.

Unit tests use real temporary files/sharing checks with injected service/account/ACL wrappers. Their non-elevated host retains a test-operator grant on new directories to stand in for the public command's elevated Administrators token. The disposable VM uses real production permission application. Clean package checks validate checksums/source binding, CLI/configuration controls, host-runner negative/default invocations without Go and inclusion of the import scripts and operator guide.

## Cleanup and limits

Credential removal is confirmed before account/service deletion. The fixture restores shared root/data-parent/code ACLs and the original configuration, removes the third account's added batch right and deletes its owned account. Independent inspection checks all three identities and finds zero remaining accounts, explicit rights, owned processes, credential tasks, service registrations, mock processes or managed Event Log sources. 1 profiles remain retained, 1 loaded, along with fixture data/logs and historical events. Earlier evidence retention limits, including alpha 19, remain unchanged.

The importer supports reviewed local files for a new destination before profile approval. It does not download software, overwrite/update existing code or execute an installer. A crash before promotion may retain protected staging; a crash after directory move may leave a complete destination without a success response. New imports and permission preparation refuse unfinished staging until inspection. There is no durable recovery journal or automatic stale-stage deletion. Filesystem checks are bounded snapshots; concurrent hostile writers and full isolation remain open. No actual panel, real game, public TLS, WFP traffic or new reboot was tested. See the [operator guide](../GAME-FILES.md).
