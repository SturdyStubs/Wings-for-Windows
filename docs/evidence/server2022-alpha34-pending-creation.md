# Server 2022 alpha 34 pending creation validation

Validation date: October 22, 2026. The exact clean-source archive was transferred over fingerprint-pinned SSH to the disposable Windows Server VM. The packaged native runner, full combined service fixture and an independent read-only cleanup inspection passed.

| Field | Result |
| --- | --- |
| Host | Windows Server 2022 Standard Evaluation x64, build 20348; PowerShell 5.1.20348.558 |
| Clean package source | `7b8cdfd6789ab18ff58cf5af17a49af8d9fac431` |
| ZIP SHA256 | `f670ea1f4ae5a2aec80d880454ec3350acff37aa7b01f6502b72fdbb3df5ba96` |
| Runtime-test SHA256 | `428f07aab4df4bb585932c92c4424841c8bdec66dcd0a221d6289151212a9251` |
| Demo SHA256 | `c01888a1a08492e7cd1448829964c1a50b198dbe756d4faee833fce81bb234c2` |
| Local suite | 731 cases/subtests, zero skipped; vet and clean release build pass |
| Packaged Server 2022 baseline | 14 cases/subtests, zero failed/skipped; default runner invocation |
| Native baseline UTC | 2026-10-22T15:51:23.3349030Z through 2026-10-22T15:51:25.8541398Z |
| Combined service fixture | All 79 checks pass; all six cleanup flags true |
| Independent cleanup audit | Pass; one retained/loaded fixture profile, no forced unload/delete |

[Sanitized JSON results](server2022-alpha34-pending-creation-results.json) retain every check, package/source binding, fixture hash and cleanup counts. Nine setup/credential/Event Log scripts match the immutable archive. No node configuration, credentials or raw private provisioning output is committed.

## Six new combined service checks

The preceding 73 account/parser/recovery/log/RCON/native-installer checks remain in the suite. A dedicated phase then:

1. Enables manual-approval creation and adds a fresh UUID to the authenticated loopback mock's assigned list. POST creation returns success; GET reports awaiting_profile, no execution approval and no data directory.
2. Replays the pending request and verifies exactly one listed record.
3. Rejects automatic start with 422 and pending install with 409.
4. Restarts the SCM daemon and reconstructs the pending UUID from panel discovery, still without its data directory.
5. Temporarily removes all local profiles and starts the same managed daemon with manual approval enabled. The original assigned UUID now waits for approval; no installation callback is sent.
6. Restores the original local profile/configuration, restarts and successfully invokes native reinstall with its completion callback.

The brand-new pending UUID receives no game account, credential or ACL, and is never launched. The final handoff uses the original pre-provisioned fixture UUID/account/data. Local tests separately cover a new manager transitioning from no profile to a UUID-specific approved profile. This does not certify fresh account/permission provisioning on a populated managed node.

## Contract and local checks

The request fixture follows upstream Wings 747bc43 installer.ServerDetails and the reviewed panel Services/Daemon/ServerClient::create fields. A mock test reproduces the panel action's persist-before-request/delete-on-error behavior and verifies successful creation preserves its record. Actual PHP panel creation/import has not been run.

Local tests cover default-disabled behavior, UUID v4, unsupported automatic start, fresh assignment lookup and bearer credentials, missing/mismatched assignment, generic panel failure, closure, capacity, concurrent/idempotent pending requests, restart reconstruction/removal, no unapproved data/execution, local-profile handoff and HTTP authentication/body/operation gates. The zero-profile packaged bootstrap example validates with the real config loader and packaged --check CLI.

Manual approval deliberately differs from upstream's automatic creation/installation. Pending state is reconstructed from the panel, not journaled separately. Ordinary GET state is a cached snapshot; a create replay always rechecks assignment. Approval and restart do not send installation success automatically; native reinstall remains an operator action. Pending WebSocket sessions remain unavailable.

## Cleanup and limits

The daemon-context helpers confirm game and RCON credential removal before service/account deletion. The service, accounts, explicit rights, bootstrap tasks, owned processes, mock process and managed Event Log source are absent in independent inspection. One game profile remains retained/loaded; no forced unload/delete was attempted. Historical events and previous batch retention limits, including alpha 19, remain unchanged.

No real panel, actual game, public TLS, WFP traffic or new reboot was tested. Automatic create/install/start, game-account/ACL provisioning, protected binary promotion, server deletion and durable callback/interrupted-install reconciliation remain open. See [manual-approval scope](../PENDING-CREATION.md) and [first-node walkthrough](../PANEL-WINDOWS-SETUP.md).
