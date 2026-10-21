# Server 2022 alpha 26 approved INI egg configuration

Validation date: October 21, 2026 (America/Phoenix). The exact clean-source archive was transferred over fingerprint-pinned SSH. Native JSON/full logs and combined service results were retrieved. Windows was not rebooted; reboot tests remain explicitly deferred. Forced termination of the exact fixture daemon is a separate process test.

| Field | Result |
| --- | --- |
| Host | Windows Server 2022 Standard Evaluation x64, build 20348; PowerShell 5.1.20348.558 |
| Clean package source | `a315b4f52ee8c019e761ca1f70aa107eea4a5857` |
| ZIP SHA256 | `0ba8a9af46a9af542a49a84409a5967a64ccefc49042168942e91a786208e563` |
| Runtime-test SHA256 | `2f3218d6621b07a78a09aacbf4badffe923154e0233a4e218499028560acb09f` |
| Demo SHA256 | `a8b780deedc2e72e829bed0a3e8efdc2afa41a53539bb199c3b6cf4dd6ef55fa` |
| Local suite | 483 cases/subtests, zero skipped; vet and clean release build passed |
| Packaged Server 2022 baseline | 14 cases/subtests, zero failed/skipped; both runner arguments omitted |
| Native baseline UTC | 2026-10-21T22:29:52.5201731Z through 2026-10-21T22:29:55.0608228Z |
| Combined service/persistence/text/JSON/properties/INI fixture | All 41 checks passed |

[Sanitized JSON results](server2022-alpha26-egg-ini-results.json) retain every check and the independent cleanup audit. Seven setup/credential/event-source scripts match archive bytes. The mock was rebuilt for four approved targets. The [developer fixture](../../tools/service-provisioning-test/README.md) ran with all previous flags plus `-TestEggINI`.

## New INI checks

The disposable local profile approves server.cfg, server.json, server.properties and server.ini and enables egg process mode. The loopback mock provides all four rule sets. The daemon runs under a dedicated non-administrator SCM identity; the harmless demo uses a separate game account.

1. After existing missing-text/malformed-JSON/malformed-properties refusal checks, provide valid earlier files and malformed INI in file four. Request start and verify fourth-file preflight failure, offline state and byte-for-byte preservation of all three earlier targets.
2. Replace only the INI fixture with valid CRLF input and record its full SDDL. Start the game and verify a default-section port, added boolean text, exact child-section update without changing the parent's same-named key, bracketed dotted game section/key, Unicode name, literal `${USERNAME}` references, a Windows path ending in a backslash, a quoted value with spaces/comment symbols and identical SDDL. Prior JSON/text/properties content and permission checks also pass.

The previous 39 checks remain passing, including account/credential lifecycle, installer rollback, actual identities/console, private file denial, bounded crash recovery, persistent budgets/logs, service restart, forced daemon termination, corrupt-budget startup refusal and secret-safe service events. This is a loopback mock panel and harmless demo, not real panel/game certification.

Local tests also exercise first-dot paths/dotted keys, exact dotted-section ownership, scalar/legacy/null values, missing sections/keys, duplicate merging/last-value order, Unicode/emoji, literal interpolation/cycles, Windows paths/quoting, stable rewriting, unsupported conditions/rules, malformed/encoding/shapes, input/result structure and size limits, capped encoding, cancellation, mixed-batch preservation and actual native launch. A preflight rejects multiline quoted values before the library parse. The bounded custom encoder reparses its result and compares all raw section/key/value data before any write.

The dependency `gopkg.in/ini.v1 v1.67.0` matches the audited upstream pin and is checksummed. Byte-buffer loading and raw `Value()` reads avoid environment mapping, interpolation and inherited-key lookup. Continuations are disabled to preserve Windows backslashes. The library writer/globals are not used; comments and formatting normalize as documented. The release includes the complete Apache-2.0 license and INI guide. Packaged negative/default-runner checks pass without Go on PATH, including default arguments from the package root and an unrelated working directory.

## Cleanup and limits

The exact disposable game credential, temporary accounts/rights, service, mock and Event Log source were removed. An independent read-only audit finds zero matching users, rights, owned processes, credential tasks, service, mock or source keys. Historical events remain. One loaded profile is retained without forced unload/deletion. Earlier fixture files/profiles and the [alpha 19 disposable-credential limitation](server2022-alpha19-recovery.md) remain as previously documented.

INI support is a documented subset: single-line raw values and ordinary keys, with exact section updates. Nonempty conditions, multiline/dialect/array/shadow support and broader parser parity remain open. YAML/XML, regex conditions, node templates and broader JSON paths remain open. Semantic errors precede all writes, but I/O failure/cancellation during in-place writes can leave partial edits; there is no atomic batch or rollback journal. Real panel/TLS/game, native install/reinstall/create/delete, interrupted-operation recovery, WFP traffic, hostile isolation and reboot/profile recovery remain unverified. The broad parser task and M3 remain open.
