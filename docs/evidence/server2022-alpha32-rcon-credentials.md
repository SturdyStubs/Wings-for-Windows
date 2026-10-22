# Server 2022 alpha 32 RCON credential validation

Validation date: October 21, 2026 (America/Phoenix; UTC records are October 22). The exact clean-source archive was transferred over fingerprint-pinned SSH to the disposable Server 2022 VM. Native logs, combined service results and an independent read-only cleanup audit were retrieved.

| Field | Result |
| --- | --- |
| Host | Windows Server 2022 Standard Evaluation x64, build 20348; PowerShell 5.1.20348.558 |
| Clean package source | `4254bf917727d160fd947772e5bbc9c8b511c8b1` |
| ZIP SHA256 | `ed32844a2300cbdbffc69da9f4f904d4d3c1afc3b44c2c27f3c5c555d30a2366` |
| Runtime-test SHA256 | `428f07aab4df4bb585932c92c4424841c8bdec66dcd0a221d6289151212a9251` |
| Demo SHA256 | `3167bf1f38f9a7a148be3cc9e44f1af0da8ca0193d5ec3e0f973496cabd7e7c0` |
| Local suite | 672 cases/subtests, zero skipped; vet and clean release build pass |
| Packaged Server 2022 baseline | 14 cases/subtests, zero failed/skipped; default runner invocation |
| Native baseline UTC | 2026-10-22T04:26:48.6155119Z through 2026-10-22T04:26:51.1355980Z |
| Combined service fixture | All 66 checks pass; all six cleanup flags true |
| Independent cleanup audit | Pass; one retained/loaded fixture profile, no forced unload/delete |

[Sanitized JSON results](server2022-alpha32-rcon-credentials-results.json) retain every check, source/package binding, fixture hash and cleanup counts. Nine setup/credential/Event Log scripts match the immutable archive. No node config, credentials, transient pipe contents or cleanup secrets are committed. The harmless demo password is public fixture data.

## Seven new managed-service checks

The existing 59 combined service checks run before a dedicated RCON credential phase:

1. Provision through the public service-account helper. Refuse replacement of a missing entry and accidental overwrite; inspect exact target metadata without returning the password.
2. Reject an unconfigured server UUID before provisioning another target.
3. Remove the fixture service environment reference, configure only the exact RCON vault target and verify real API/RCON authentication and reply redaction under the daemon service identity.
4. Ask the distinct game process to inspect that target in its own account context. It reports unavailable, not present. This is a metadata-resolution probe; no password read is attempted in the game, and it is not a general hostile-isolation certification.
5. Deliberately replace the vault password with a wrong fixture value, restart and verify authentication rejection, then replace it with the correct value while stopped.
6. Remove/inspect the RCON entry and independently inspect the game-account credential to confirm its separate namespace remains present.
7. Set a matching public fixture environment value but keep the vault-only configuration with its entry removed. Verify delivery still fails instead of falling back; emergency kill remains available.

Local tests cover exact reference selection, cross-server/game-target refusal, Unicode, create/replace/remove rules, malformed marker/NUL/surrogate data, scoped metadata, namespace preservation and native per-command lookup/rotation/removal. Planner tests cover RCON store/inspect/remove, conflicting references, wrong targets and missing worker files. Packaged CLI checks cover empty/invalid UUIDs, conflicting modes, exact target metadata and idempotent missing removal without panel contact.

A storage-boundary test exposed an inherited 5120-byte allowance that exceeded Windows' documented 2560-byte credential blob limit. Game and RCON writers/planning now reject values above 1280 UTF-16 code units before the Windows write; the shared reader/cleanup bounds match. The RCON UTF-8 transport bound remains separate. Tests assert the early storage-limit error rather than accepting a later RPC failure.

## Cleanup and scope

Both credentials are confirmed removed through the daemon-context helper before service/account deletion. RCON cleanup responsibility is retained conservatively even if a worker response is interrupted. Existing task/pipe/right cleanup is reused and failure keeps affected service/account state for inspection. The six successful cleanup flags include explicit RCON removal.

The independent audit finds zero matching accounts, explicit logon rights, owned processes, credential tasks, service, mock or Event Log source keys. Historical events remain. Fixture `scm-fc378be1` retains one loaded game profile; no forced unload/delete is used. Earlier fixtures/profiles and the [alpha 19 disposable-credential limitation](server2022-alpha19-recovery.md) remain documented. Cleanup is not forensic erasure of process memory or historical profile data.

This proves the harmless loopback demo/service workflow, not a real panel/TLS/game deployment. Password provisioning does not update the game configuration, attest listener ownership or change account passwords. Same-account games, administrators, compromised daemon code, memory disclosure and broader account/profile/password/backup lifecycle remain outside the guarantee. No new VM reboot was performed; [alpha 29 evidence](server2022-alpha29-running-restoration.md) remains version-specific. M1/M2/M3 and native create/install/reinstall/delete remain open. See [setup and limits](../RCON-CREDENTIALS.md).
