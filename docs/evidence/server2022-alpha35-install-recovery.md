# Server 2022 alpha 35 installer receipt validation

Validation date: October 22, 2026. The exact clean-source archive was transferred over fingerprint-pinned SSH to the disposable Windows Server VM. The packaged native runner, full combined service fixture and independent read-only cleanup audit passed.

| Field | Result |
| --- | --- |
| Host | Windows Server 2022 Standard Evaluation x64, build 20348; PowerShell 5.1.20348.558 |
| Clean package source | `bb9ca3701733441c5bb8fe508ecee2e7b67b4213` |
| ZIP SHA256 | `0a180656823b84f439041da4224025baff9c5c2a22ccfe07ff1fb7e075bb0caa` |
| Runtime-test SHA256 | `428f07aab4df4bb585932c92c4424841c8bdec66dcd0a221d6289151212a9251` |
| Demo SHA256 | `5e33d81224837992597ae390aeb62bf9d053a271105c4f6861e3d3927c2975e9` |
| Local suite | 761 cases/subtests, zero skipped; vet and clean release build pass |
| Packaged Server 2022 baseline | 14 cases/subtests, zero failed/skipped; default runner invocation |
| Native baseline UTC | 2026-10-22T16:44:45.7701669Z through 2026-10-22T16:44:48.2828840Z |
| Combined service fixture | All 89 checks pass; all six cleanup flags true |
| Independent cleanup audit | Pass; one retained/loaded fixture profile, no forced unload/delete |

[Sanitized JSON results](server2022-alpha35-install-recovery-results.json) retain every check, exact package/source binding, fixture hash and cleanup counts. Nine setup/credential/Event Log scripts match the immutable archive. No node configuration, credentials or raw private provisioning output is committed.

## Ten new managed-service checks

The previous 79 checks remain in the combined suite. The new phase enables `installer.persist_state` and:

1. Rejects a mock callback before recording it. Successful setup retains its result and exact attempt ID.
2. Rejects reinstall while that result is unresolved.
3. Restarts with callback delivery available again; the result survives without automatic retry or execution.
4. Rejects a stale attempt, explicitly retries notification, preserves a changed data marker and refuses duplicate acknowledgement. Accepted callback count increases exactly once.
5. Restarts again and retains the acknowledgement without retry.
6. Starts the distinct game account and verifies actual access denial when it probes the private receipt file.
7. Launches a hanging installer with a child, forcibly terminates the actual daemon process and independently confirms child termination through Job Object cleanup.
8. Restarts, restores the interrupted attempt and rejects game start and normal result notification for its unknown outcome.
9. Explicitly reports the interrupted attempt as failed, accepts the failed callback and preserves another changed marker without running setup again.
10. Corrupts the newest receipt slot while stopped, verifies service startup refusal, restores its original bytes and verifies readiness/handle release.

This is actual daemon-process termination, not a VM reboot. All tests use the packaged harmless demo and authenticated loopback mock. Reconciliation is an extra native route; upstream method/path coverage remains 16/38.

## Local checks

Tests cover durable result/acknowledgement transitions, exclusive writers, conflicting mutation and hardlinks; torn, empty, missing, oversized, checksum/UUID/sequence/attempt/phase/unknown-field/trailing data; rejected false outcomes and attempt changes; restart retry without marker changes; explicit interrupted failure and blocked starts; initial/result/acknowledgement write failures without unconfirmed callbacks; private-storage requirements; HTTP authentication, action, stale/duplicate/disabled gates and no execution.

The clean package passes existing CLI/config/host-runner negative and default-invocation checks without Go. The recovery guide is packaged. No automatic callback retry or historical-attempt ledger is provided.

## Cleanup and limits

Credential helpers confirm game and RCON credential removal before account/service deletion. Independent inspection finds no current fixture accounts, explicit rights, owned processes, bootstrap tasks, service registration, mock process or managed Event Log source. One profile remains retained/loaded and historical events remain. Previous fixture/profile retention limits, including alpha 19, are unchanged; no forced profile removal was attempted.

A callback accepted before a crash or failed local acknowledgement can remain pending locally. Delivery is uncertain, not exactly once; inspect panel state before retrying. Checksums detect damage and depend on private ACLs for protection. Corrupt state fails initial sync rather than silently using an older result. Reporting interrupted failure does not roll back partial data.

No actual panel, real game, public TLS, WFP traffic or new reboot was tested. Account/ACL provisioning, binary promotion, general transactional recovery and live certification remain open. See the [operator guide](../INSTALL-RECOVERY.md).
