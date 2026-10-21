# Server 2022 alpha 24 approved JSON egg configuration

Validation date: October 21, 2026 (America/Phoenix). The exact clean-source archive was transferred over fingerprint-pinned SSH and the native JSON/full log and combined service results were retrieved. Windows was not rebooted; reboot tests remain explicitly deferred. The existing forced termination of the exact fixture daemon is a separate process test.

| Field | Result |
| --- | --- |
| Host | Windows Server 2022 Standard Evaluation x64, build 20348; PowerShell 5.1.20348.558 |
| Clean package source | `c54ccb2620e10edba39738f6d131e124195ee35b` |
| ZIP SHA256 | `9423a0aeacab8ab077ad003795924900ebbb7ea8552d1432bd2374dcac17c701` |
| Runtime-test SHA256 | `2f3218d6621b07a78a09aacbf4badffe923154e0233a4e218499028560acb09f` |
| Demo SHA256 | `3437dc8de1bc27e5f775ddef919aa2388badc5fffb761ae132e7acf6f22d2c94` |
| Local suite | 425 cases/subtests, zero skipped; vet and clean release build passed |
| Packaged Server 2022 baseline | 14 cases/subtests, zero failed/skipped; both runner arguments omitted |
| Native baseline UTC | 2026-10-21T16:43:16.6139387Z through 16:43:19.1642101Z |
| Combined service/persistence/text/JSON fixture | All 37 checks passed |

[Sanitized JSON results](server2022-alpha24-egg-json-results.json) retain every check and the independent cleanup audit. Seven setup/credential/event-source scripts match archive bytes. The mock was rebuilt for the two approved target entries. The [developer fixture](../../tools/service-provisioning-test/README.md) ran with all previous flags plus `-TestEggJSON`.

## New JSON checks

The disposable local profile approves server.cfg and server.json and enables egg process mode. The mock panel provides text rules plus JSON rules for a nested numeric port, boolean enabled flag, Unicode name and conditional wildcard updates. The daemon runs under a dedicated non-administrator SCM identity; the harmless demo uses a separate game account.

1. After the existing missing-text-file refusal, provision the text file but leave JSON malformed. Request game start and observe file-two preflight failure, offline state and byte-for-byte preservation of the earlier text target. No partial semantic batch is applied.
2. Replace only the fixture JSON with valid input, record its full SDDL and start the game. Verify a numeric port, actual boolean type, preserved unrelated nested host/value, conditional wildcard change only on the old matching child, Unicode name, unchanged textual large integer 9007199254740993 and identical SDDL. The text-file content/ACL checks also pass in this mixed batch.

The previous 35 checks remain passing with both parsers enabled, including account/credential lifecycle, installer rollback, actual identities and console, private file denial, bounded crash recovery, persistent budgets/logs, service restart, forced daemon termination, corrupt-budget startup refusal and secret-safe service events. This remains a loopback mock panel and harmless demo, not real panel/game certification.

Local tests additionally exercise scalar coercion and untouched number precision, literal conditions, missing object/array-zero creation, existing array indices, object/array wildcards, unsupported path/rule rejection, duplicate/malformed/non-object input, encoding, depth/node/size bounds, conservative mutation accounting, capped encoding, cancellation, mixed-batch preflight and actual native launch. An initial output-bound fixture fit under 1 MiB; it was corrected to use deeper indentation and verify bounded-encoder rejection. No production limit was relaxed. Packaged negative/default-runner checks pass without Go on PATH.

## Cleanup and limits

The exact disposable game credential, temporary accounts/rights, service, mock and Event Log source were removed. An independent read-only audit finds zero matching users, rights, owned processes, credential tasks, service, mock or source keys. Historical events remain. One loaded profile is retained without forced unload/deletion. Earlier fixture files/profiles and the [alpha 19 disposable-credential limitation](server2022-alpha19-recovery.md) remain as previously documented.

JSON support is a documented subset: object roots, bounded nested paths/explicit array indices, one wildcard level and literal conditions. Regex conditions, node templates, more complex paths, YAML/INI/properties/XML and full egg compatibility remain open. Input/semantic errors precede all writes, but I/O failure/cancellation during in-place writes can leave partial edits; there is no atomic batch or rollback journal. Real panel/TLS/game, native install/reinstall/create/delete, interrupted-operation recovery, WFP traffic, hostile isolation and reboot/profile recovery remain unverified. The broad parser task and M3 remain open.
