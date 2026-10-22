# Server 2022 alpha 30 Source RCON validation

Validation date: October 21, 2026 (America/Phoenix; UTC records are October 22). The exact clean-source archive was transferred over fingerprint-pinned SSH to the disposable Server 2022 VM. Default native-test logs, combined service results and an independent read-only cleanup audit were retrieved.

| Field | Result |
| --- | --- |
| Host | Windows Server 2022 Standard Evaluation x64, build 20348; PowerShell 5.1.20348.558 |
| Clean package source | `3dff830626d69e5e04c3d50ca1fcd15cb8b87bd9` |
| ZIP SHA256 | `a0bd4ac81d0af0e5dfcc7bf66a9b6155f83b308d986d3948797857ead63102b2` |
| Runtime-test SHA256 | `2f3218d6621b07a78a09aacbf4badffe923154e0233a4e218499028560acb09f` |
| Demo SHA256 | `dd982a844f81f785272776035e1e2a42d9b85055b6496e0c36cccc711759a6ec` |
| Local suite | 619 cases/subtests, zero skipped; vet and clean release build pass |
| Packaged Server 2022 baseline | 14 cases/subtests, zero failed/skipped; default runner invocation |
| Native baseline UTC | 2026-10-22T03:39:25.8195416Z through 2026-10-22T03:39:28.4429298Z |
| Combined service fixture | All 54 checks pass; all five cleanup flags true |
| Independent cleanup audit | Pass; one retained/loaded fixture profile, no forced unload/delete |

[Sanitized JSON results](server2022-alpha30-source-rcon-results.json) retain every check, package/source binding, fixture hash and cleanup counts. Seven setup/credential/Event Log scripts match immutable archive bytes. No node configuration, credentials or cleanup state is committed. The fixed public demo password is test data, not a production secret.

## New RCON checks

The normal 49-check service fixture is followed by five additional checks with a Source TCP demo running under the separate game SID:

1. Change only the disposable local profile to a loopback RCON endpoint and supply the public fixture password to the daemon service environment. Send a command through the existing authenticated panel API and verify its `[RCON]` reply.
2. Ask the harmless demo to return its fixture password and verify the assembled reply is redacted before console publication.
3. Verify both game root and child run under the distinct game SID.
4. Send the normal power stop, verify the demo received that stop through RCON and confirm the root/child tree is gone.
5. Restart the fixture service with a wrong fixture password, verify the API rejects command delivery without executing it, then confirm emergency kill still works.

Local tests independently cover literal packet bytes, fragmented TCP reads, authentication with/without an empty prelude, empty and split replies, UTF-8/password text spanning packets, authentication/ID/type/padding/size errors, truncation, missing completion, total timeout, packet/byte/line budgets, cancellation and no resend. Native lifecycle tests cover console publication, host-secret non-inheritance, concurrent-command rejection, kill interrupting a stalled exchange, graceful stops and forced-stop fallback after authentication failure. Egg RCON stop commands are limited to the protocol packet budget before launch.

## Cleanup and scope

The independent audit finds zero matching accounts, explicit logon rights, owned processes, credential tasks, services, mocks or Event Log source keys. Historical lifecycle/failure events remain. The current fixture `scm-1c5807f0` retains one loaded game profile; no profile is force-unloaded/deleted. Earlier fixtures/profiles and the [alpha 19 disposable-credential limitation](server2022-alpha19-recovery.md) remain documented.

This verifies the harmless demo and loopback mock, not an actual Source game or real panel/TLS workflow. The client requires the documented empty response-value completion mirror; Rust WebRCON, BattlEye/UDP and incompatible engine dialects remain unsupported. Readiness still uses stdout. Environment references do not provide a credential vault or secure service-environment provisioning, and loopback does not attest listener ownership. Private/trusted-node and same-account limitations remain.

No VM reboot was performed in this batch; the separate [alpha 29 reboot evidence](server2022-alpha29-running-restoration.md) remains historical rather than relabeled as alpha 30. M1/M2/M3 and native create/install/reinstall/delete remain open. Failed responses may leave command execution unconfirmed; no automatic retry/stdin fallback is used. See [operator setup and limits](../SOURCE-RCON.md).
