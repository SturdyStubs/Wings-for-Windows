# Server 2022 alpha 33 native setup validation

Validation date: October 22, 2026. The exact clean-source archive was transferred over fingerprint-pinned SSH to the disposable Windows Server VM. Native reports, combined service results and independent cleanup inspection were retrieved.

| Field | Result |
| --- | --- |
| Host | Windows Server 2022 Standard Evaluation x64, build 20348; PowerShell 5.1.20348.558 |
| Clean package source | `00ac2d40af68eaf6e53bb5708c6a3280af20fd74` |
| ZIP SHA256 | `73520ea097034b208052842991c0a969ce4d3810724b5a7efd2afa0be3c07ac1` |
| Runtime-test SHA256 | `428f07aab4df4bb585932c92c4424841c8bdec66dcd0a221d6289151212a9251` |
| Demo SHA256 | `afe03bfb1510ffa5b1c12088857511e1016c61cc751a1526934bec0be0ea3649` |
| Local suite | 707 cases/subtests, zero skipped; vet and clean release build pass |
| Packaged Server 2022 baseline | 14 cases/subtests, zero failed/skipped; default runner invocation |
| Native baseline UTC | 2026-10-22T14:12:21.9293486Z through 2026-10-22T14:12:24.4294993Z |
| Combined service fixture | All 73 checks pass; all six cleanup flags true |
| Independent cleanup audit | Pass for both preliminary and corrected fixtures; retained profiles were not forcibly unloaded/deleted |

[Sanitized JSON results](server2022-alpha33-native-installers-results.json) retain every check, source/package binding, fixture hash and cleanup counts. Nine setup/credential/Event Log scripts match the immutable archive. No node config, credentials, raw console dumps or provisioning secrets are committed.

## Seven new managed-service checks

The previous 66 service/account/parser/recovery/log/RCON checks remain in the combined fixture. The new phase uses only approved `wings-windows-demo.exe install` modes:

1. Execute data setup through the install route as the game identity, verify `install.marker`, offline state and successful/non-reinstall callback.
2. Start the game and invoke reinstall; confirm it stops and callback has `reinstall: true`, leaving the game offline.
3. Run a nonzero-exit installer and verify local failure and unsuccessful callback.
4. Run a hanging installer with a child and independently resolve the child's actual SID to the distinct configured game account.
5. Reject simultaneous reinstall and game start requests with HTTP 409.
6. Kill installation, verify cancellation/failure callback and confirm the child process is gone.
7. Apply a one-second installer timeout and verify the timeout state and unsuccessful callback.

Local tests additionally cover exact callback boolean keys/authentication, event names, disabled recipes, skip-egg-scripts, missing game/installer preflight, fresh suspension and sync cancellation, daemon shutdown, callback failure without automatic rerun, bounded long/flood output, config limits and protected installer paths. HTTP tests confirm request fields cannot override executable approval. Packaged CLI checks verify both game/installer preflight, installer-only failure and absence of directory creation. Demo egg JSON/fields match the sample profile and reviewed panel import format; actual import has not been tested.

## Preliminary VM finding and correction

The first clean candidate (`542441c`) passed setup but timed out during reinstall. The installer busy gate also blocked its own lifecycle-owned stop command. A one-second local fixture stop timeout had masked this by forcing termination before installer timeout; the VM's longer normal stop exposed it. The correction permits internal approved stop delivery while still rejecting public commands and adds an explicit `STOPPING` assertion. The exact corrected package was rebuilt and all 73 combined checks passed.

The preliminary run failed and is preserved as such in the JSON, with all six cleanup flags and a separate passing audit. Both disposable fixtures have zero remaining service registrations, local accounts, explicit rights, owned processes, bootstrap tasks, mocks or managed Event Log source. One profile per fixture remains retained/loaded, and historical events remain; no forced profile removal was attempted. Earlier evidence retention limits, including alpha 19, remain unchanged.

## What this does not establish

Game binaries are pre-provisioned protected code. This validates data/setup execution under the existing game identity, not arbitrary downloads, privileged installs, protected binary promotion or a real game's install/update recipe. Networking is unmanaged; WFP traffic enforcement and hostile multi-tenant isolation remain open. No reboot was performed in this batch.

No live panel or actual game was used. The reviewed panel 2.0.0-beta.1 removes a new server record after an unsupported daemon create request: native server creation is the next integration gap. The demo egg/profile and [node guide](../PANEL-WINDOWS-SETUP.md) prepare that workflow without claiming it already works. Creation/deletion, durable interrupted-installer state, callback reconciliation and partial-data rollback remain open. Installer success and callback failure are separately reported; no durable notification retry is provided.
