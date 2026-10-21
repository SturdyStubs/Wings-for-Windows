# Server 2022 alpha 28 XML and actual VM reboot validation

Validation date: October 21, 2026 (America/Phoenix). The operator explicitly authorized reboot testing, superseding the prior deferral. The exact clean-source archive was transferred over fingerprint-pinned SSH. Native logs, normal service results, preparation/post-boot results and an independent cleanup audit were retrieved. Only the disposable Server 2022 VM was rebooted.

| Field | Result |
| --- | --- |
| Host | Windows Server 2022 Standard Evaluation x64, build 20348; PowerShell 5.1.20348.558 |
| Clean package source | `c680cb2e0028a551f952f83475130d8a5d3080c3` |
| ZIP SHA256 | `c2a1a39929ad7dae20367a5de1c819eed2bc33a9e99514034d2524d6ea16a79f` |
| Runtime-test SHA256 | `2f3218d6621b07a78a09aacbf4badffe923154e0233a4e218499028560acb09f` |
| Demo SHA256 | `1982fcb53166a9a17a6c085a34c5e3ef40af49ba37de33401fc058d2100ccb27` |
| Local suite | 558 cases/subtests, zero skipped; vet and clean release build passed |
| Packaged Server 2022 baseline | 14 cases/subtests, zero failed/skipped; both runner arguments omitted |
| Native baseline UTC | 2026-10-21T23:01:17.9518350Z through 2026-10-21T23:01:20.5364312Z |
| Combined service/six-parser fixture | All 45 checks passed, followed by independent cleanup audit |
| Separate reboot preparation | 40 preparatory checks passed; retained resources explicitly marked reboot_prepared |
| Actual post-boot fixture | All 7 unattended checks passed; pinned SSH reconnected |
| Boot time before / after UTC | 2026-10-21T02:56:06.8275410Z / 2026-10-21T23:04:00.1119860Z |

[Sanitized JSON results](server2022-alpha28-xml-reboot-results.json) retain every check, source/hash binding, SSH/control preflight and both cleanup audits. The 40 preparation checks are a repeated subset of the combined service checks, not 40 additional distinct features. The mock was rebuilt for six approved targets. Seven setup/credential/event-source scripts match archive bytes; checked-out reboot fixture source hashes are recorded separately.

## XML service checks

The disposable profile approves text, JSON, properties, INI, YAML and XML targets. The loopback mock provides all six rule sets. The daemon runs under a dedicated non-administrator SCM identity and the harmless demo under a distinct game SID.

1. After existing parser refusal checks, leave the first five targets valid and XML malformed. Request start and observe sixth-file preflight failure/offline state with all five earlier contents unchanged.
2. Replace XML with valid input, record full SDDL, start the game and verify numeric/scalar text, all repeated world matches, attribute updates that retain child elements/IDs, Unicode and escaped ampersand/angle brackets, literal `${USERNAME}`, unrelated attributes and identical SDDL. Earlier five-parser content/permission checks also pass.

Local tests cover exact/wildcard paths, empty-root creation, attribute notation, stable round trips, XML character rules, DTD/entity/namespace/processing-instruction/duplicate-attribute/mixed-content refusal, node/depth/match limits, conservative wildcard mutation budget, escaping-driven output growth, cancellation, mixed-batch preservation and native launch. The former unsupported-XML schema fixture now uses unsupported TOML. An ignored upload helper initially reused its fixture-name variable for a source filename; validation refused that run before creating accounts/service/root. The helper was corrected before the 45-check run. No production bound was relaxed.

## Actual reboot and cold profiles

A fresh separate fixture completed its 40 pre-boot checks and left a unique persisted console marker, two consumed retry reservations and an offline game. Preparation configured only its own temporary service for delayed automatic start, a SYSTEM at-startup loopback mock and a SYSTEM resume task delayed three minutes. SSH automatic startup, task principals/delay, source hash, service mode and encrypted-state DACL were verified before the explicitly authorized shutdown/restart request.

Temporary cleanup credentials used machine DPAPI inside a directory accessible only to Administrators and SYSTEM. Neither plaintext/encrypted credential data nor node configuration is included in this evidence. The game and daemon could not read that directory. The resume task required no SSH or interactive login and used the cleanup credential only after the cold game-launch checks.

After the actual boot-time change, pinned SSH reconnected. Seven checks passed: real reboot; automatic delayed-start service running under its non-administrator SID; games and recovery timers remaining offline; restored console marker/history; restored two-attempt recovery budget; manual launch through the stored credential with distinct game root/child SIDs after cold profile loading; and complete game-tree stop. The task then removed the exact game credential, service, mock, both reboot tasks, Event Log source, temporary accounts/rights and encrypted cleanup-state file. Its report was retrieved after unattended completion.

## Cleanup and remaining gates

The combined independent read-only audit across both current fixtures finds zero users, explicit rights, owned processes, credential/reboot tasks, service, mock, Event Log source keys or encrypted cleanup-state files. Historical events remain. Two fixture profiles remain: the normal fixture profile is now unloaded after the reboot; the post-boot fixture profile remains loaded. No profile was force-unloaded/deleted. Earlier fixture files/profiles and the [alpha 19 disposable-credential limitation](server2022-alpha19-recovery.md) remain as documented.

This verifies one delayed-start mock/demo deployment through a real VM reboot; installation still defaults to Manual. It does not implement automatic game/timer restoration, certify immediate-start dependency races, power-loss/interrupted operations, password rotation, real panel/TLS/game workflows, WFP traffic or hostile isolation. Broader reboot recovery and M1/M2/M3 remain open.

All six parser families now have initial supported subsets, not full egg compatibility. XML namespaces/mixed content, broader dialects, conditions/regex, node templates and complex paths remain open. Semantic errors precede all writes; in-place I/O failure/cancellation may leave partial edits and later launch failure does not undo changes. There is no atomic batch or rollback journal. See [XML scope](../EGG-XML-CONFIG.md) and [reboot workflow](../REBOOT-VALIDATION.md).
