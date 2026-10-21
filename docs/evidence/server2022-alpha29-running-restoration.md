# Server 2022 alpha 29 running restoration validation

Validation date: October 21, 2026 (America/Phoenix). The exact clean-source archive was transferred over fingerprint-pinned SSH to the disposable Windows Server 2022 VM. The operator authorized reboot testing. Native logs, normal service results, separate preparation/post-boot reports and independent cleanup audits were retrieved.

| Field | Result |
| --- | --- |
| Host | Windows Server 2022 Standard Evaluation x64, build 20348; PowerShell 5.1.20348.558 |
| Clean package source | `df86b8a6adcce01eb27e1d042ace1f8b828d682d` |
| ZIP SHA256 | `a996189a81a897d8b957ddc92a04e06480523f6cf195e2d9af015c1e1746c2a8` |
| Runtime-test SHA256 | `2f3218d6621b07a78a09aacbf4badffe923154e0233a4e218499028560acb09f` |
| Demo SHA256 | `a20344dd5a06993400655b4f817bf5178d1807acd10eb8cefe96b554b0d46145` |
| Local suite | 584 cases/subtests, zero skipped; vet and clean release build passed |
| Packaged Server 2022 baseline | 14 cases/subtests, zero failed/skipped; default runner arguments |
| Native baseline UTC | 2026-10-21T23:24:16.1501339Z through 2026-10-21T23:24:18.9363065Z |
| Combined service fixture | All 49 checks passed, followed by independent cleanup audit |
| Separate reboot preparation | All 44 checks passed; retained fixture marked reboot_prepared |
| Actual post-boot fixture | All 7 unattended checks passed; pinned SSH reconnected |
| Boot time before / after UTC | 2026-10-21T23:04:00.1119860Z / 2026-10-21T23:28:07.0844380Z |

[Sanitized JSON results](server2022-alpha29-running-restoration-results.json) retain every check and source/hash binding, SSH/control preflight and both cleanup audits. The 44 preparation checks repeat a subset of the 49 combined checks; they are not 44 additional features. Seven setup/credential/Event Log scripts match the immutable archive. Checked-out reboot fixture hashes are recorded separately. No credentials, node configuration or encrypted cleanup state are committed.

## Intent storage and service checks

The opt-in profile requires private logs and enabled persistent crash recovery. Four additional combined checks verify actual game-token denial of intent-file reads, automatic running restoration after SCM restart, a durable manual stop across another SCM restart, and refusal of corrupt intent at startup. The fixture alone raises its restart limit to five, keeping production defaults unchanged.

Local tests cover initialized stopped snapshots, UUID/checksum/sequence binding, conflicting writers and links, corruption, missing latest stop slots, configuration prerequisites, readiness gating, fresh panel suspension, budget limits, explicit cancellation, manual/clean stops and stale clean-exit writes. A missing-stop-slot test exposed an initial-snapshot rollback risk during development; both slots now initialize stopped before running intent can be written. Storage-failure and blocked-write tests verify that emergency kill still kills the game tree before waiting for durable confirmation.

## Actual automatic restoration after boot

Preparation leaves a running game, running intent, a unique console marker and three consumed restart reservations. Only the temporary service is set to delayed automatic start. SYSTEM startup tasks launch the loopback mock and resume validation after three minutes. Before reboot, SSH automatic start, task principals/delay, resume source hash, delayed service mode and Administrators/SYSTEM-only cleanup-state DACL were checked. Temporary cleanup credentials are protected by machine DPAPI and are used only after cold game-launch checks.

After the real boot-time change and pinned SSH reconnect, all seven unattended checks pass: boot change, delayed automatic non-administrator service identity, automatic game restoration, console history, retry budget, cold stored-credential game root/child identities and complete game-tree stop. Restoration consumes the fourth reservation; the loaded budget was three. The game starts through fresh launch guards, without interactive login or a credential-helper launch warming its profile first. Post-boot stop verifies complete game-tree termination; the separate SCM check verifies stopped intent across restart. This creates a new process; it does not reattach to a surviving PID.

## Cleanup and scope

The normal fixture reports all five cleanup flags true; the post-boot fixture reports all six true. An independent combined read-only audit finds zero matching accounts, explicit rights, owned processes, credential/reboot tasks, service, mock, Event Log source keys or encrypted cleanup-state files. Historical events remain. Profiles: scm-3cb33f87: 1 retained, 0 loaded; scm-24b3baad: 1 retained, 1 loaded. No profile is force-unloaded/deleted. Earlier fixture files/profiles and the [alpha 19 disposable-credential limitation](server2022-alpha19-recovery.md) remain documented.

Restoration remains disabled by default. Installation still defaults to Manual service startup. This proves one delayed-start mock/demo deployment, not real panel/TLS/game workflows, immediate-start dependency races, power-loss durability, password rotation, native installation/create/delete recovery, WFP traffic or full isolation. An abrupt exit or failed/unconfirmed intent write can retain prior intent; process exit, intent snapshots and retry-budget files are not one atomic transaction. Same-account games cannot be isolated from their daemon identity. M1/M2/M3 remain open. See [operator controls and limits](../RUNNING-RESTORATION.md).
