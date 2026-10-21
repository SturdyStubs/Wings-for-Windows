# Approved egg JSON configuration

Alpha 24 adds `parser: "json"` to the existing [approved egg configuration workflow](EGG-TEXT-CONFIG.md). JSON and text targets can share one batch: all files are opened/rendered before any is changed. The existing exact local approval, link/code protections and in-place permission-preserving writes apply to both.

## Example

Local profile:

```json
"use_egg_process_configuration": true,
"egg_config_files": ["server.cfg", "server.json"]
```

Panel process_configuration.configs entry:

```json
{
  "file": "server.json",
  "parser": "json",
  "replace": [
    {"match": "network.port", "replace_with": "28015"},
    {"match": "network.enabled", "replace_with": true},
    {"match": "worlds[0].name", "replace_with": "Main"},
    {"match": "worlds.*.mode", "if_value": "old", "replace_with": "new"}
  ]
}
```

The existing target must contain valid UTF-8 JSON with an object root. Missing files, malformed input, duplicate keys (including nested objects), trailing data, BOM/UTF-16 input and unsupported rules prevent game launch. Text targets earlier in the same batch stay unchanged when JSON preflight fails. Startup, manual restart and automatic recovery use fresh panel entries; syncing a running game does not immediately edit files.

## Paths and conditions

Supported paths are dot-separated object keys, `name[index]` array entries and one internal `*` segment over existing object/array children. A missing wildcard prefix does nothing. Root/terminal or multiple wildcards, multiple bracket indices in one segment and escaped literal-dot/bracket/wildcard keys are unsupported. Explicit array indices are canonical decimal 0..1023. Existing indices can be changed; a missing array can be created only at index zero. Missing object containers can be created. Existing scalar/null containers are not silently converted into objects/arrays to satisfy a deeper path.

A nonempty literal `if_value` compares the existing scalar's plain text (strings without quotes; other values encoded as JSON). An existing mismatch leaves that target unchanged. As in the pinned contract, a missing target is still created even when a literal condition is supplied. Empty/null conditions are unconditional. Regex conditions and unresolved `{{...}}` templates are explicitly rejected.

The subset follows [the pinned upstream JSON helper](https://github.com/pterodactyl/wings/blob/747bc4393daf5d95fdf61d76382f5e23749cffe9/parser/helpers.go), with stricter bounds and explicit failures for unsupported shapes. This is partial Windows egg compatibility, not a claim that every upstream path/template works.

## Scalar types and formatting

Replacement values must be scalars; legacy `value` remains a fallback when `replace_with` is absent. To match the upstream scalar coercion:

| Replacement | Stored JSON |
| --- | --- |
| Boolean `true` / `false` | Boolean |
| Integer or integer-looking string within signed 64-bit range | Number |
| String `"true"` | String |
| Decimal/exponent spelling such as `1.5` | String `"1.5"` |
| Null | String `"<nil>"` |

Untouched JSON numbers retain their original spelling/precision through `json.Number`; large integers are not converted to floating point. JSON object ordering and whitespace can change. Output uses four-space indentation, sorted object keys and no required trailing newline. Comments/JSON5 are unsupported. File contents and replacement values are not included in parser errors or console success messages.

## Bounds and failures

Shared limits remain 16 targets, 128 KiB remote entries, 128 rules/file, 8 KiB match/value scalars and 1 MiB input/output per file. JSON adds at most 32 dotted segments/64 traversal steps, depth 64, 100,000 input/output values/containers and 4,096 wildcard children. A conservative mutation-size budget rejects excessive growth before building it; bounded encoding stops before output exceeds 1 MiB. Cancellation is checked during parse, mutation and encoding. These limits may reject a near-limit document even when a less conservative rewrite could fit.

Semantic/input validation precedes all writes, but storage writes remain in place. I/O failure or cancellation during writing can leave partial edits, and a later native launch-check failure does not undo successful edits. Retain backups and inspect failures; no rollback journal, atomic batch or interrupted-install guarantee is provided. The existing trusted-private-node, writable-mapping/ACL race and isolation limits remain. Alpha 25 adds [properties](EGG-PROPERTIES-CONFIG.md); YAML, INI and XML are still open; reboot/profile recovery testing remains deferred.

Local tests cover type/precision preservation, literal conditions, nested creation, arrays/wildcards, malformed/duplicate/shape/encoding rejection, bounded expansion/encoding, cancellation, mixed-batch preflight and actual native launch. The exact package passes 14 native and 37 combined Server 2022 checks, including malformed mixed-batch refusal and typed/precise JSON edits with retained SDDL. See [evidence and cleanup limits](evidence/server2022-alpha24-egg-json.md).
