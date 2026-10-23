# Server 2022 alpha 39 staging inspection and quarantine validation

Validation date: October 22, 2026. The exact clean-source archive was transferred over fingerprint-pinned SSH to the disposable Windows Server VM. Packaged native tests, the combined managed-service fixture and independent read-only cleanup inspection passed.

| Field | Result |
| --- | --- |
| Host | Windows Server 2022 Standard Evaluation x64, build 20348; PowerShell 5.1.20348.558 |
| Clean package source | `589aad88e5c32c42dc241cdc1327b0af04cce460` |
| ZIP SHA256 | `cf989742540ec6db4627e523e0bbc9dcaca21e6f99fe89230de949ba4a99f31f` |
| Runtime-test SHA256 | `428f07aab4df4bb585932c92c4424841c8bdec66dcd0a221d6289151212a9251` |
| Demo SHA256 | `db25cffe567948bab46e447cf72caac0ec8b6833f2b771bd501b909cdcf8f7ba` |
| Local suite | 833 cases/subtests, zero skipped; vet and clean release build pass |
| Packaged Server 2022 baseline | 14 cases/subtests, zero failed/skipped; default runner invocation |
| Native baseline UTC | 2026-10-23T04:14:33.9888744Z through 2026-10-23T04:14:36.5200553Z |
| Combined service fixture | All 114 checks pass; all seven cleanup flags true |
| Independent cleanup audit | Pass; 1 retained profiles, 1 loaded; no forced unload/delete |

[Sanitized JSON results](server2022-alpha39-stage-recovery-results.json) retain every check, source/package binding, fixture hash and cleanup counts. Seventeen setup/credential/Event Log/permission/account/import/recovery scripts match the immutable archive. Node configuration, account SIDs, passwords and raw private output are excluded.

## Seven new managed-service checks

The earlier 107 checks remain. The fixture creates a **simulated interrupted-copy tree**, with partial bytes, an administrator-only root and a daemon-readable file, while the service is stopped. This is not a real helper-process crash or reboot. The immutable package's public command:

1. Inspects the exact stage and reports its content/ACL fingerprint and partial-file/readable-entry counts.
2. Confirms that unfinished staging blocks ordinary code import.
3. Runs fingerprint-approved WhatIf and preserves stage bytes/ACLs without creating quarantine.
4. Refuses mutation while the shared maintenance lock is held.
5. Changes partial bytes and refuses the old fingerprint.
6. Reinspects, quarantines the selected tree with bytes preserved and independently confirms only SYSTEM/Administrators remain on the quarantine parent, root and file. No stage remains in the code parent.
7. Refuses replay and preserves the quarantined bytes.

The normal verified import, profile approval, permissions, credential storage, native setup and root/child game launch then succeed. Actual dependency write denial and cross-server data denial remain in the combined suite. The fixture retains quarantine for inspection. No extra upstream route is implemented; coverage remains 16/38.

## Local and package checks

Seventeen new scenarios plus their parent test cover no-mutation inspection, successful quarantine, partially applied ACLs, exact stage names, configured and normalized path references, existing destinations, foreign/writable ACLs, hardlinks/junctions, changed contents/ACLs/service identity, empty staging, move failure and destination races. Failed moves retain bytes; existing destinations are not overwritten. Unit tests use real temporary content/sharing/ACL inspection with injected node context and a test-operator grant to stand in for the public command's required elevated Administrators token. Production ACL sealing is verified separately on Server 2022.

Clean package checks validate checksums/source binding, CLI/configuration controls, host-runner negative/default invocations without Go and inclusion of the recovery scripts and operator guide.

## Cleanup and limits

Credential removal is confirmed before account/service deletion. The fixture restores shared root/data-parent/code ACLs and the original configuration, removes the third account's added batch right and deletes its owned account. Independent inspection checks all three identities and finds zero remaining accounts, explicit rights, owned processes, credential tasks, service registrations, mock processes or managed Event Log sources. 1 profiles remain retained, 1 loaded, along with fixture data/logs, retained administrator-only quarantine and historical events. Independent post-cleanup readback verifies the quarantined bytes and ACLs and confirms no staging blocker remains under executable code storage. Earlier evidence retention limits, including alpha 19, remain unchanged.

Recovery inspects and explicitly quarantines one supported stage using an approved source content/ACL fingerprint. It seals the tree and moves it without deleting or publishing files. A failed or interrupted seal/move can retain bytes with stricter ACLs or leave completed quarantine without a success response. There is no durable journal, automatic retry, deletion, promotion resume, UUID inference or verification of a published destination after a crashed import. Configured path bindings, unsafe names/ACLs/owners, links and existing quarantine destinations are refused. Checks are bounded snapshots on a trusted stopped node, not hostile-administrator confinement. No actual helper crash, real panel/game, public TLS, WFP traffic or new reboot was tested. See the [operator guide](../GAME-FILE-RECOVERY.md).
