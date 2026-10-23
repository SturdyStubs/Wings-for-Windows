# Server 2022 alpha 37 game account creation validation

Validation date: October 22, 2026. The exact clean-source archive was transferred over fingerprint-pinned SSH to the disposable Windows Server VM. Packaged native tests, the combined managed-service fixture and independent read-only cleanup inspection passed.

| Field | Result |
| --- | --- |
| Host | Windows Server 2022 Standard Evaluation x64, build 20348; PowerShell 5.1.20348.558 |
| Clean package source | `109176ed8b74ae8bc376fd83c348319d2b76787c` |
| ZIP SHA256 | `14b4de5631b4399ae5bdee0ac7442acf8e6757f5ff0f7a044f71bb37c0e1fb55` |
| Runtime-test SHA256 | `428f07aab4df4bb585932c92c4424841c8bdec66dcd0a221d6289151212a9251` |
| Demo SHA256 | `1bd5f4d324afdbe5787ab0581ff67ad7d5673fd78f848ff9cd3c2b77eedfcd4b` |
| Local suite | 791 cases/subtests, zero skipped; vet and clean release build pass |
| Packaged Server 2022 baseline | 14 cases/subtests, zero failed/skipped; default runner invocation |
| Native baseline UTC | 2026-10-23T01:21:31.1017038Z through 2026-10-23T01:21:33.6065158Z |
| Combined service fixture | All 101 checks pass; all seven cleanup flags true |
| Independent cleanup audit | Pass; 2 retained profiles, 2 loaded; no forced unload/delete |

[Sanitized JSON results](server2022-alpha37-game-accounts-results.json) retain every check, source/package binding, fixture hash and cleanup counts. Thirteen setup/credential/Event Log/permission/account scripts match the immutable archive. Node configuration, account SIDs, passwords and raw private output are excluded.

## Five new managed-service checks

The earlier 96 checks remain. The third game account is now created by the immutable package's public account command instead of fixture code. The new checks:

1. Run WhatIf without a password and confirm no account, UUID data or configuration mutation.
2. Hold the shared credential-provisioning lock and confirm creation is refused without leaving an account.
3. Create the real account, independently verify enabled/non-expiring/self-change-disabled flags, returned SID/UUID credential target, no explicit rights and unchanged configuration.
4. Refuse a second request for the same account name while preserving its SID.
5. Refuse an already-approved UUID without creating the proposed account.

The later permission/credential/installer/game checks use that exact account and password, resolve root and child process ownership to its SID, verify another server's data is denied and confirm populated older data/logs remain unchanged. This exercises real Windows account creation and the complete manual handoff; it does not claim automatic panel-driven provisioning.

All executables are harmless demos and the panel is an authenticated loopback mock. No new upstream route is implemented; coverage remains 16/38.

## Local and package checks

Sixteen new unit scenarios plus their parent test cover read-only planning, creation metadata, existing names/profiles/data, invalid names/UUIDs, empty/oversized SecureString passwords, create failure, dedicated-account validation failure, mismatched SIDs, daemon identity conflict, rollback failure and stale service/account plans. Account operations are injected locally; only the disposable Server 2022 fixture creates real users. Failed validation removes only the freshly returned SID; preexisting accounts are never adopted, reset or removed.

Clean package checks validate checksums/source binding, CLI/configuration controls, host-runner negative/default invocations without Go and inclusion of the account scripts and operator guide.

## Preliminary VM finding and correction

The first clean candidate (`1126c9e`, ZIP `26148d5d48ede56fa7945342b27dfe6821685bdec617040cbbf84934f50477ca`) failed when Windows rejected its 54-character account description. `New-LocalUser` permits at most 48 characters; the rejection happened before account creation. The fix shortens the prefix while retaining the full UUID, with a regression that exercises the production wrapper's native arguments, description bound and password flags. The failed report remains marked failed in the sanitized JSON. Its six cleanup flags and independent audit passed. The corrected archive was rebuilt and the complete suite rerun.

## Cleanup and limits

Credential removal is confirmed before account/service deletion. The fixture restores shared root/data-parent/code ACLs and the original configuration, removes the third account's added batch right and deletes its owned account. Independent inspection checks all three identities and finds zero remaining accounts, explicit rights, owned processes, credential tasks, service registrations, mock processes or managed Event Log sources. 2 profiles remain retained, 2 loaded, along with fixture data/logs and historical events. Earlier evidence retention limits, including alpha 19, remain unchanged.

The account command supports an unused UUID/name on a stopped existing managed node, accepts SecureString passwords and returns metadata without rewriting profiles. Permissions and credentials are separate explicit steps. Existing account password rotation, deletion, automatic panel-driven provisioning, binary distribution and durable rollback after power loss remain open. Filesystem checks are bounded snapshots; concurrent hostile writers and full isolation remain open. No actual panel, real game, public TLS, WFP traffic or new reboot was tested. See the [operator guide](../GAME-ACCOUNTS.md).
