# Approved properties egg configuration

Alpha 25 adds `parser: "properties"` to the existing [approved egg file workflow](EGG-TEXT-CONFIG.md). Properties, text and JSON can share one startup batch. Exact local file approval, existing-target requirements, guarded handles, protected node/code identities and permission-preserving writes remain unchanged.

## Example

Local profile:

```json
"use_egg_process_configuration": true,
"egg_config_files": ["server.properties"]
```

Panel process_configuration.configs entry:

```json
{
  "file": "server.properties",
  "parser": "properties",
  "replace": [
    {"match": "server-port", "replace_with": 25565},
    {"match": "motd", "replace_with": "Windows Café"},
    {"match": "difficulty", "if_value": "easy", "replace_with": "normal"}
  ]
}
```

Matching uses the entire decoded property key, not a line prefix or dotted JSON path. Keys are case-sensitive. String, number, boolean and null replacements become text; null becomes `<nil>`. Legacy `value` remains a fallback when `replace_with` is absent. Unconditional rules add missing keys. A nonempty literal `if_value` changes only an existing equal value; missing/mismatched keys are left alone. Unlike JSON's missing-target condition behavior, a properties condition does not create a missing key. `regex:` in a properties condition is ordinary literal text, not a regex instruction. Unresolved `{{...}}` replacement templates are rejected.

## Parsing and encoding

Input must be UTF-8 without BOM. Java-style equals/colon/whitespace separators, escaped keys, comments, Unicode escapes and physical line continuations are supported. CRLF/CR normalize to LF; continued lines are joined before decoding escapes, including escapes split over continuations. Valid UTF-16 surrogate pairs are decoded without losing supplementary Unicode characters. Malformed/unpaired Unicode escapes in data and decoded NUL keys/values are rejected. Double-escaped text remains literal. Latin-1/UTF-16 input requires explicit operator conversion.

The parser uses pinned [magiconair/properties v1.8.10](https://github.com/magiconair/properties/tree/v1.8.10), with **expansion disabled**. `${NAME}` references remain literal in both input and replacements; daemon environment variables, other properties and circular references are not expanded. Only the byte-buffer loader is used: it does not load referenced paths or URLs. This intentionally differs from an expanding loader and avoids writing daemon environment values into game files.

Output uses `key=value` and LF. Keys/values use bounded Java-style escaping for backslashes, separators, leading whitespace, controls and Unicode. Non-ASCII characters become `\uXXXX` escapes; supplementary characters use surrogate pairs. Initial blank/comment lines (`#`/`!`, including leading whitespace) are retained. Other comments, original whitespace/separators and duplicate declarations are normalized: duplicates use the last value in the first key's position, and new keys append in rule order. The retained header remains UTF-8. Output is stable on repeated rewrites; no timestamp is added.

For a literal Windows path in the properties source, escape backslashes, for example `log-path=C:\\Games\\Logs`. Unknown escape prefixes follow the properties parser's rules; do not treat an unescaped Windows path as raw text. Keep backups before enabling a new egg's rules.

## Bounds and failures

Shared limits remain 16 approved files, 128 KiB remote entries, 128 rules per file, 8 KiB rule keys/values and 1 MiB input/output. Properties adds 65,536 physical LF-normalized lines, 16,384 decoded keys, 8 KiB decoded keys and 64 KiB decoded values. Normalization/encoding check cancellation; the bounded byte-buffer library parse is checked before/after. The encoder checks the output limit as it writes and never expands `${...}` values.

Every file in the mixed batch is rendered before any is changed, so malformed properties prevent earlier text/JSON edits and game launch. Once in-place writing begins, I/O failure/cancellation can leave partial edits; later native launch-check failure does not undo changes. This is not an atomic batch, rollback journal or interrupted-install recovery. The trusted-private-node, shared-account, writable-mapping/ACL race and isolation limits remain. YAML, INI, XML, node templates and broader parser parity stay open. Reboot/profile recovery testing remains deferred.

The new dependency is pinned/checksummed and its BSD-2-Clause notice is included in `licenses/properties-BSD.txt` in release archives. Local tests cover conditional updates, escaped keys/continuations/Unicode, stable output, literal environment/cycle values, encoding/size/count/cancellation errors, mixed-batch preflight and actual native launch. Exact packaged Server 2022 results are recorded in [verification](VERIFICATION.md).
