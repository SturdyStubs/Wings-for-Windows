# Approved INI egg configuration

Alpha 26 adds `parser: "ini"` to the [approved egg file workflow](EGG-TEXT-CONFIG.md). Text, JSON, properties and INI can share a startup batch. All files must already exist and be approved by exact relative name in the local profile. The existing guarded handles, protected identities and permission-preserving writes apply.

## Example

Local profile:

```json
"use_egg_process_configuration": true,
"egg_config_files": ["GameUserSettings.ini"]
```

Panel process_configuration.configs entry:

```json
{
  "file": "GameUserSettings.ini",
  "parser": "ini",
  "replace": [
    {"match": "port", "replace_with": 28015},
    {"match": "network.enabled", "replace_with": true},
    {"match": "[/Script.Game.Settings].Server.Name", "replace_with": "Windows Café"}
  ]
}
```

A plain match addresses the default section. The first unbracketed dot separates section and key; remaining dots belong to the key. `network.inner.key` addresses key `inner.key` in section `network`. Bracket dotted section names: `[parent.child].shared` addresses key `shared` in section `parent.child`. Brackets are path syntax and are not part of the section name. `DEFAULT.key` explicitly addresses the default section.

Names are case-sensitive, following the pinned library's default mode; the default section uses its reserved `DEFAULT` name. Missing sections/keys are created, and repeated rules apply in order. Updates target the exact section's own key; they never follow parent-section key inheritance. String, number, boolean and null replacements become text, with null represented as `<nil>`. Legacy `value` is a fallback when `replace_with` is absent.

## Supported subset and normalization

The [upstream INI path convention](https://github.com/pterodactyl/wings/blob/747bc4393daf5d95fdf61d76382f5e23749cffe9/parser/parser.go) guides the match syntax. This is not full parser parity. Nonempty `if_value` is rejected rather than ignored. Empty/null conditions are accepted as unconditional. Unresolved `{{...}}` values are rejected. No regex evaluation or node template lookup occurs.

Only single-line values and ordinary unquoted keys are supported. Reject quoted/auto-increment/boolean keys, bracket/control characters in names, ambiguous quotes, multiline values and NULs. Keys containing `=`, `:`, `;`, `#`, single/double quotes or backticks are unsupported. Leading/trailing name whitespace is unsupported. Section names may contain dots, slashes and internal spaces.

Input CRLF/CR normalizes to LF. Backslashes remain literal, including a Windows path ending in a backslash: continuation processing is disabled. This deliberately differs from upstream's default continuation behavior. Multiline quoted values are refused before library parsing, avoiding repeated multiline concatenation. Use a different supported parser or adjust the configuration explicitly if the game requires continuations or repeated array/shadow keys.

Repeated sections merge and duplicate keys retain the last value in the first key's position. Output normalizes all formatting to `key=value` with LF, retains section/key order and removes comments. Ordinary values remain unquoted; values requiring protection use the library's backtick/triple-double-quote syntax. A bounded reparse verifies that serialization preserves every section, key and raw value before writing. Confirm the target game's dialect accepts this quoting before enabling a rule, and retain backups.

## Literal values and bounds

The pinned/checksummed `gopkg.in/ini.v1 v1.67.0` matches the audited upstream dependency. Only byte-buffer loading and raw `Value()` reads are used. No environment mapper, interpolated `String()` lookup, inherited `GetKey()` read, filesystem loader or library writer is used. `${NAME}` and `%(other)s` remain literal, including cycles. No library globals are modified; the custom encoder avoids the library writer's full-buffer alignment expansion. The complete Apache-2.0 license is packaged in `licenses/ini-Apache-2.0.txt`.

Shared limits: 16 approved files, 128 KiB remote entries, 128 rules per file, 8 KiB match/value scalars and 1 MiB per input/output. INI additionally allows at most 8,192 LF-normalized newline delimiters, 512 sections including the default section, 4,096 total keys, 8 KiB names and 64 KiB decoded values. UTF-8 without BOM/NUL is required. Input and result structures are checked, output is capped while encoding, and cancellation is checked around the bounded library parse and during normalization/updates/encoding.

All targets are rendered before any file changes; malformed INI blocks launch without applying earlier text/JSON/properties changes. In-place I/O failure or cancellation after writes begin may leave partial changes; later launch failures do not undo edits. There is no atomic batch, rollback journal or interrupted-operation recovery. Trusted-private-node and existing filesystem/ACL/mapping-race limits remain. Alpha 27 adds [YAML](EGG-YAML-CONFIG.md); Alpha 28 adds [XML](EGG-XML-CONFIG.md); broader dialects/templates/conditions, real panel/game certification and isolation remain open. The operator has now authorized [reboot testing](REBOOT-VALIDATION.md); exact results are recorded separately.

## Verification

Local fixtures exercise path/scalar semantics, exact dotted sections, duplicates, Unicode, literal interpolation/cycles, Windows paths/quotes, stable output, malformed/encoding/shape/size/count limits, output growth, cancellation, mixed-batch preservation and actual native launch. The disposable service fixture adds fourth-file refusal and actual INI edits with retained SDDL. All 483 local cases/subtests, 14 packaged native cases and 41 combined Server 2022 checks pass. See [evidence and cleanup limits](evidence/server2022-alpha26-egg-ini.md).
