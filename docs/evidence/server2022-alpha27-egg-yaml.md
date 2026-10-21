# Server 2022 alpha 27 approved YAML egg configuration

Validation date: October 21, 2026 (America/Phoenix). The exact clean-source archive was transferred over fingerprint-pinned SSH. Native JSON/full logs and combined service results were retrieved. Windows was not rebooted; reboot tests remain explicitly deferred. Forced termination of the exact fixture daemon is a separate process test.

| Field | Result |
| --- | --- |
| Host | Windows Server 2022 Standard Evaluation x64, build 20348; PowerShell 5.1.20348.558 |
| Clean package source | `956b8198319b080e8148c441f6e634117b1c9634` |
| ZIP SHA256 | `0ed87d6aaf9a6c12211fc8b34e66056ce845b3e58158b88533241616a11f7efd` |
| Runtime-test SHA256 | `2f3218d6621b07a78a09aacbf4badffe923154e0233a4e218499028560acb09f` |
| Demo SHA256 | `400e8ae9ba0faa780562aada52714a9c8e0fac5f7c28c2a36cd1aa6228afa14c` |
| Local suite | 525 cases/subtests, zero skipped; vet and clean release build passed |
| Packaged Server 2022 baseline | 14 cases/subtests, zero failed/skipped; both runner arguments omitted |
| Native baseline UTC | 2026-10-21T22:49:10.1706023Z through 2026-10-21T22:49:12.7399089Z |
| Combined service/persistence/text/JSON/properties/INI/YAML fixture | All 43 checks passed |

[Sanitized JSON results](server2022-alpha27-egg-yaml-results.json) retain every check and the independent cleanup audit. Seven setup/credential/event-source scripts match archive bytes. The mock was rebuilt for five approved targets. The [developer fixture](../../tools/service-provisioning-test/README.md) ran with all previous flags plus `-TestEggYAML`.

## New YAML checks

The disposable local profile approves server.cfg, server.json, server.properties, server.ini and server.yml and enables egg process mode. The loopback mock provides all five rule sets. The daemon runs under a dedicated non-administrator SCM identity; the harmless demo uses a separate game account.

1. After the four existing parser refusal checks, provide valid earlier files and malformed YAML in file five. Request start and verify fifth-file preflight failure, offline state and unchanged contents of all four earlier targets.
2. Replace only the YAML fixture with valid CRLF input and record its full SDDL. Start the game and compare the complete expected normalized YAML: numeric port, boolean enabled flag, unchanged host, large integer 9007199254740993, exact decimal-exponent spelling 1.2300e+3, conditional wildcard change only on the matching world, Unicode name, literal `${USERNAME}` references and unrelated values. Full SDDL is identical. Prior text/JSON/properties/INI content and permission checks also pass.

The previous 41 checks remain passing, including account/credential lifecycle, installer rollback, actual identities/console, private file denial, bounded crash recovery, persistent budgets/logs, service restart, forced daemon termination, corrupt-budget startup refusal and secret-safe service events. This is a loopback mock panel and harmless demo, not real panel/game certification.

Local tests also exercise both parser names, JSON paths/arrays/wildcards/conditions, missing containers, replacement coercion, integers beyond unsigned 64-bit range, decimal exponents, multiline string data, Unicode/emoji, quoted ambiguous values, literal environment/cycle values, stable rewrites, unsupported rules, duplicates/non-string keys, malformed/core/custom tags, aliases/anchors/merges/multiple documents, encoding/NUL/depth/node/key/size limits, capped output, cancellation, mixed-batch preservation and actual native launch. The former unsupported-YAML contract fixture now uses unsupported XML. An initial mixed-batch test helper read the earlier text as YAML instead of staging valid YAML; the helper was corrected before the clean build. No production limit was relaxed.

The dependency `gopkg.in/yaml.v3 v3.0.1` matches the audited upstream pin and is checksummed. Node validation never expands aliases or coerces mapping keys. Only bounded byte-buffer decoding and capped stream encoding are used. Numeric lexemes are not converted through floating point. The encoded result is reparsed and compared as complete JSON-compatible raw data before any writes. The release includes the complete MIT/Apache license/copyright notices and YAML guide. Packaged negative/default-runner checks pass without Go on PATH, including default arguments from the package root and an unrelated working directory.

## Cleanup and limits

The exact disposable game credential, temporary accounts/rights, service, mock and Event Log source were removed. An independent read-only audit finds zero matching users, rights, owned processes, credential tasks, service, mock or source keys. Historical events remain. One loaded profile is retained without forced unload/deletion. Earlier fixture files/profiles and the [alpha 19 disposable-credential limitation](server2022-alpha19-recovery.md) remain as previously documented.

YAML support is a documented subset: a single mapping-root document, string keys, supported scalar types, bounded paths and literal conditions. Aliases/anchors/merge keys, YAML-specific numeric forms and unsupported tags are refused. Formatting/comments normalize. XML, broader YAML/INI dialects, regex conditions, node templates and broader JSON paths remain open. Semantic errors precede all writes, but I/O failure/cancellation during in-place writes can leave partial edits; there is no atomic batch or rollback journal. Library decoding is bounded by input but not interrupted mid-call. Real panel/TLS/game, native install/reinstall/create/delete, interrupted-operation recovery, WFP traffic, hostile isolation and reboot/profile recovery remain unverified. The broad parser task and M3 remain open.
