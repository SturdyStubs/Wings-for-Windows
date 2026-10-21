# Server 2022 alpha 25 approved properties egg configuration

Validation date: October 21, 2026 (America/Phoenix). The exact clean-source archive was transferred over fingerprint-pinned SSH. Native JSON/full logs and combined service results were retrieved. Windows was not rebooted; reboot tests remain explicitly deferred. Forced termination of the exact fixture daemon is a separate process test.

| Field | Result |
| --- | --- |
| Host | Windows Server 2022 Standard Evaluation x64, build 20348; PowerShell 5.1.20348.558 |
| Clean package source | `99a84f1263535c0c559c7b8da1244e596d3cb49e` |
| ZIP SHA256 | `dd10028da4498fa54c4c0f004fe93467359e95b899af9b5e3400eda59888dc93` |
| Runtime-test SHA256 | `2f3218d6621b07a78a09aacbf4badffe923154e0233a4e218499028560acb09f` |
| Demo SHA256 | `5ca42bcb6de014e9c60b7517de3544970be43a606944e754b4f8113ff837ad2a` |
| Local suite | 446 cases/subtests, zero skipped; vet and clean release build passed |
| Packaged Server 2022 baseline | 14 cases/subtests, zero failed/skipped; both runner arguments omitted |
| Native baseline UTC | 2026-10-21T17:13:19.6922897Z through 2026-10-21T17:13:22.2692035Z |
| Combined service/persistence/text/JSON/properties fixture | All 39 checks passed |

[Sanitized JSON results](server2022-alpha25-egg-properties-results.json) retain every check and the independent cleanup audit. Seven setup/credential/event-source scripts match archive bytes. The mock was rebuilt for three approved targets. The [developer fixture](../../tools/service-provisioning-test/README.md) ran with all previous flags plus `-TestEggProperties`.

## New properties checks

The disposable local profile approves server.cfg, server.json and server.properties and enables egg process mode. The loopback mock provides all three rule sets. The daemon runs under a dedicated non-administrator SCM identity; the harmless demo uses a separate game account.

1. After existing missing-text and malformed-JSON refusal checks, provide valid earlier files and malformed properties Unicode in file three. Request game start and verify third-file preflight failure, offline state and byte-for-byte preservation of both earlier targets.
2. Replace the properties fixture with valid CRLF input and record its full SDDL. Start the game and verify the port update, Unicode/emoji Java escapes, decoded continuation, preserved initial header, unchanged literal `${USERNAME}` references, newly added literal reference, unchanged nonmatching condition, skipped missing conditional key, scalar boolean text and identical SDDL. Existing typed JSON/precision and text/ACL checks also pass.

The previous 37 checks remain passing, including account/credential lifecycle, installer rollback, actual identities/console, private file denial, bounded crash recovery, persistent budgets/logs, service restart, forced daemon termination, corrupt-budget startup refusal and secret-safe service events. This is a loopback mock panel and harmless demo, not real panel/game certification.

Local tests additionally exercise escaped keys, key/value and split-Unicode continuations, BMP/supplementary Unicode, stable repeated rewriting, duplicate last-value behavior, Windows-path header comments, literal environment references/cycles, malformed Unicode/surrogates, raw/escaped NUL, encoding/size/count limits, bounded encoding, cancellation, mixed-batch preservation and actual native launch. CRLF continuations and valid surrogate pairs are normalized before the library parse; a bounded Java-style encoder handles output. An initial output-limit fixture exceeded the input limit; it was corrected to fit the input bound and exceed the escaped-output bound. No production limit was relaxed.

The dependency `github.com/magiconair/properties v1.8.10` is pinned/checksummed. Environment/property expansion is disabled for parsing, reads and writes. Only bounded byte-buffer loading is used. The release contains its complete BSD-2-Clause notice and the properties guide. Packaged negative/default-runner checks pass without Go on PATH, including default arguments from the package root and an unrelated working directory.

## Cleanup and limits

The exact disposable game credential, temporary accounts/rights, service, mock and Event Log source were removed. An independent read-only audit finds zero matching users, rights, owned processes, credential tasks, service, mock or source keys. Historical events remain. One loaded profile is retained without forced unload/deletion. Earlier fixture files/profiles and the [alpha 19 disposable-credential limitation](server2022-alpha19-recovery.md) remain as previously documented.

Properties support is a documented subset: exact keys, scalar replacements and literal conditions. Formatting is normalized and only initial comments are retained. YAML/INI/XML, regex conditions, node templates, broader JSON paths and full parser parity remain open. Semantic errors precede all writes, but I/O failure/cancellation during in-place writes can leave partial edits; there is no atomic batch or rollback journal. Real panel/TLS/game, native install/reinstall/create/delete, interrupted-operation recovery, WFP traffic, hostile isolation and reboot/profile recovery remain unverified. The broad parser task and M3 remain open.
