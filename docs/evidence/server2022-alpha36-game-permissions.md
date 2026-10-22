# Server 2022 alpha 36 incremental game permissions validation

Validation date: October 22, 2026. The exact clean-source archive was transferred over fingerprint-pinned SSH to the disposable Windows Server VM. Packaged native tests, the combined managed-service fixture and independent read-only cleanup inspection passed.

| Field | Result |
| --- | --- |
| Host | Windows Server 2022 Standard Evaluation x64, build 20348; PowerShell 5.1.20348.558 |
| Clean package source | `07a54bf8dd364cabb684289ead06154ad08aa1aa` |
| ZIP SHA256 | `3a8a0f766d8e023b72b37f85222c7caf3472c7b460b83899767159b93e025c66` |
| Runtime-test SHA256 | `428f07aab4df4bb585932c92c4424841c8bdec66dcd0a221d6289151212a9251` |
| Demo SHA256 | `02a801d924cead8c01ee1bffa666a21af23912a81c3e304b5ccaca9cc77f1861` |
| Local suite | 774 cases/subtests, zero skipped; vet and clean release build pass |
| Packaged Server 2022 baseline | 14 cases/subtests, zero failed/skipped; default runner invocation |
| Native baseline UTC | 2026-10-22T18:06:18.1941026Z through 2026-10-22T18:06:20.7262376Z |
| Combined service fixture | All 96 checks pass; all seven cleanup flags true |
| Independent cleanup audit | Pass; 1 retained profiles, 1 loaded; no forced unload/delete |

[Sanitized JSON results](server2022-alpha36-game-permissions-results.json) retain every check, source/package binding, fixture hash and cleanup counts. Eleven setup/credential/Event Log/permission scripts match the immutable archive. Node configuration, account SIDs, passwords and raw private output are excluded.

## Seven new managed-service checks

The earlier 89 checks remain. With the daemon stopped and older data/logs populated, the fixture creates a third dedicated local account and approves a fresh UUID/profile. It runs the immutable package's public permission command and:

1. Verifies WhatIf changes no shared DACL, batch right or target directory.
2. Applies real permissions and confirms older populated data/log content and full ACL snapshots remain unchanged.
3. Replays against the empty new target and preserves the existing batch right.
4. Stores the new credential in the daemon context and successfully runs the approved installer for the new UUID, with an acknowledged callback and data marker.
5. Starts the game and independently resolves both root and child process ownership to the new game SID.
6. Sends a probe under that actual identity and confirms access to the original server's marker is denied.
7. Kills the job tree, confirms both processes exited and verifies permission preparation rejects the now-populated target.

All executables are harmless demo fixtures; the panel is an authenticated loopback mock. The new helper is operator-only. Upstream route coverage remains 16/38.

## Local and package checks

Local tests cover read-only planning, application/replay, injected mid-application rollback, preservation of preexisting rights, unexpected ACL identities, deny-batch rights, populated targets, external code, duplicate SIDs, hardlinks, junctions and stale ACL plans. Unit identity/LSA calls are injected and mutate only disposable ACL trees; actual account/LSA behavior is exercised on the VM. The non-elevated unit host retains a test-operator grant on new directories to stand in for the public command's required elevated Administrators token.

Clean package checks validate checksums/source binding, CLI/configuration controls, host-runner negative/default invocations without Go and inclusion of the new commands and operator guide.

## Cleanup and limits

Credential removal is confirmed before account/service deletion. The fixture restores shared root/data-parent/code ACLs and the original configuration, removes the third account's added batch right and deletes its owned account. Independent inspection checks all three identities and finds zero remaining accounts, explicit rights, owned processes, credential tasks, service registrations, mock processes or managed Event Log sources. 1 profiles remain retained, 1 loaded, along with fixture data/logs and historical events. Earlier evidence retention limits, including alpha 19, remain unchanged.

Preparation only supports precreated accounts, existing approved code and empty new target data on a stopped trusted managed node. It is not populated-target repair, automatic account/password/credential provisioning, binary distribution or durable rollback after power loss. Filesystem checks are bounded snapshots; concurrent hostile writers and full isolation remain open. No actual panel, real game, public TLS, WFP traffic or new reboot was tested. See the [operator guide](../GAME-PERMISSIONS.md).
