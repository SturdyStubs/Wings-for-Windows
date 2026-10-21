# Approved YAML egg configuration

Alpha 27 adds `parser: "yaml"` and the `yml` alias to the [approved egg file workflow](EGG-TEXT-CONFIG.md). Text, JSON, properties, INI and YAML can share a startup batch. Files must already exist and be approved by exact relative filename in the local profile. Existing guarded handles, protected identities and permission-preserving writes apply.

## Example

Local profile:

```json
"use_egg_process_configuration": true,
"egg_config_files": ["server.yml"]
```

Panel process_configuration.configs entry:

```json
{
  "file": "server.yml",
  "parser": "yaml",
  "replace": [
    {"match": "network.port", "replace_with": "28015"},
    {"match": "network.enabled", "replace_with": true},
    {"match": "worlds.*.mode", "if_value": "old", "replace_with": "new"}
  ]
}
```

Paths, wildcard updates, scalar coercion and literal conditions use the existing [JSON subset](EGG-JSON-CONFIG.md). Support includes nested object keys, canonical array indices 0..1023 and one internal wildcard over existing object/array children. Missing objects can be created; missing arrays can be created only at index zero. Existing mismatched containers fail rather than silently changing type. A literal condition leaves existing unequal values alone; missing targets are still created, as in the JSON contract. Regex conditions and unresolved `{{...}}` values are rejected. Legacy `value` is a fallback.

## YAML subset

Follow the [audited upstream YAML-to-JSON workflow](https://github.com/pterodactyl/wings/blob/747bc4393daf5d95fdf61d76382f5e23749cffe9/parser/parser.go), with explicit restrictions and stricter bounds. Accept one document with a mapping root, string mapping keys, sequences and string/boolean/null/JSON-compatible decimal numeric values. String block scalars are accepted and their decoded newlines are preserved. Core scalar tags must have a supported spelling. Numbers must use JSON syntax: octal, hex, leading plus/zero, underscores, nonfinite values and other YAML-specific numeric forms are rejected. Untouched accepted numeric spellings, including large integers and decimal exponents, are retained without floating-point conversion.

Reject duplicate keys, numeric/complex keys, anchors, aliases, merge keys, custom/binary/timestamp tags and multiple documents. Anchors and aliases are refused even when unused or harmless. Parse into a node tree and validate it directly; no alias expansion or typed interface decoding occurs. No file includes, environment interpolation or node-template lookup occurs. `${NAME}` and similar strings remain literal. Quote dates and other timestamp-like values explicitly if they are intended as text.

Replacements follow JSON coercion, rather than YAML type inference: booleans remain booleans; integer-looking scalars within signed 64-bit range become numbers; decimal/exponent replacements become strings; null replacements become string `<nil>`. Untouched YAML nulls remain null. A replacement string `"01"` becomes integer `1`, while an input numeric `01` is rejected as non-JSON syntax. See the JSON guide for the full table.

Output uses sorted mapping keys and two-space indentation. All string keys/values are double-quoted to preserve ambiguous booleans, dates, numeric-looking text and literal interpolation values. Comments, original order/style, document directives and formatting normalize. The emitter may add a core numeric tag when needed for a large integer. A bounded reparse compares the complete JSON-compatible raw data before writing; formatting changes must not alter values. Keep backups and confirm the target game's YAML dialect before enabling rules.

## Bounds and failures

The pinned/checksummed `gopkg.in/yaml.v3 v3.0.1` matches the audited upstream dependency. Its complete MIT/Apache license and copyright notices are packaged in `licenses/yaml-MIT-Apache.txt`. Only byte-buffer node decoding and capped stream encoding are used; no library globals are modified.

Shared limits: 16 files, 128 KiB remote entries, 128 rules/file, 8 KiB match/value scalars and 1 MiB input/output. YAML adds depth 64, 100,000 nodes including keys, 8 KiB mapping keys, and the JSON path/wildcard bounds. Input and result structures are both checked. A conservative 1 MiB JSON representation/mutation budget applies before encoding; some near-limit YAML documents may therefore be rejected even if a different encoding could fit. UTF-8 without BOM/NUL is required; decoded NUL strings are rejected too. Underlying YAML syntax restrictions also apply.

Cancellation is checked before/after the bounded library decode, during tree validation/mutations/output construction and on encoder writes. The library decode itself is not interrupted mid-call. Encoding stops before exceeding the output cap. Parser errors do not include configuration contents or replacement values.

Every target is rendered before any file changes. Malformed YAML prevents earlier text/JSON/properties/INI edits and game launch. In-place I/O failure or cancellation after writes begin may leave partial changes; later launch failures do not undo edits. There is no atomic batch, rollback journal or interrupted-operation recovery. Trusted-private-node and existing filesystem/ACL/mapping-race limits remain. XML, broader YAML dialects/templates/conditions, real panel/game certification and isolation remain open. Reboot testing remains deferred.

## Verification

Local fixtures cover typed/scalar updates, arrays/wildcards/literal conditions, missing containers, Unicode/emoji, large integers/exponents, multiline strings, literal environment/cycles, quoted ambiguity, stable rewriting, unsupported rules/tags/aliases/documents, duplicates, encoding/depth/node/key/size limits, capped output, cancellation, mixed-batch preservation and actual native launch. The disposable service fixture adds fifth-file refusal and actual typed/precise YAML edits with retained SDDL. Final packaged Server 2022 evidence is recorded separately in [verification](VERIFICATION.md).
