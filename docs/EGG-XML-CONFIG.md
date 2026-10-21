# Approved XML egg configuration

Alpha 28 adds `parser: "xml"` to the [approved egg file workflow](EGG-TEXT-CONFIG.md). Text, JSON, properties, INI, YAML and XML share the same startup batch. Targets must already exist and be approved by exact relative filename. Guarded handles, protected identities and permission-preserving writes apply.

```json
{
  "file": "server.xml",
  "parser": "xml",
  "replace": [
    {"match": "server.port", "replace_with": 28015},
    {"match": "server.worlds.world.mode", "replace_with": "normal"},
    {"match": "server.worlds.world", "replace_with": "[enabled='true']"}
  ]
}
```

Enable egg process mode and add `server.xml` to local `egg_config_files`. Paths include the root and use dot-separated element names. Exact paths create missing elements along the first matching branch, then update all existing full-path matches. One non-root wildcard segment selects existing children only, including a terminal wildcard. Root mismatches, XPath predicates, indices, namespace prefixes and literal dotted element names are unsupported.

This follows the [audited upstream XML element/attribute convention](https://github.com/pterodactyl/wings/blob/747bc4393daf5d95fdf61d76382f5e23749cffe9/parser/parser.go) with stricter restrictions. A replacement `[name='value']` sets an attribute on selected elements and preserves their children/text. Other scalar replacements set leaf text; string/number/boolean/null become text, with null `<nil>`. Legacy `value` is a fallback. Nonempty conditions and unresolved `{{...}}` templates are rejected. There is no regex condition, arbitrary XPath or environment expansion; `${NAME}` remains literal.

The existing file may be empty: the first rule creates its root. An empty file without a creation rule fails. A deeper path cannot replace existing non-whitespace text. Text replacement on a node with children fails rather than creating mixed content. Names use ASCII letters/underscore followed by letters/digits/underscore/hyphen, up to 256 bytes. Attribute name `xmlns` is forbidden.

The standard-library streaming XML decoder accepts UTF-8 XML with ordinary attributes, text/CDATA and predefined/numeric character references. Reject DTDs/directives, external or unknown entities, processing instructions other than one initial XML declaration, namespaces, duplicate attributes, multiple roots, mixed non-whitespace text/children, invalid XML characters and unsupported names. No external resource resolver or filesystem loader is installed. Comments, declarations, CDATA style and formatting normalize away. Output uses escaped UTF-8 text/attributes, two-space indentation and explicit end tags. Unrelated element/attribute data and sibling order remain. A bounded reparse compares the entire data tree before writes.

Shared limits remain 16 files, 128 KiB remote entries, 128 rules/file, 8 KiB match/value scalars and 1 MiB input/output. XML adds depth 64, 100,000 elements/attributes, 128 attributes/element, 64 KiB input attribute values, 32 path segments and at most 4,096 matches at any traversal step. A conservative mutation-byte budget prevents wildcard expansion from constructing excessive text/attributes. Streaming encoding checks the output cap, including escaping/indentation. Cancellation is checked per token, traversal/update and encoder write. UTF-8 BOM input is explicitly rejected. No third-party XML dependency is added.

All files render before any is changed, so malformed XML blocks launch without applying earlier files. Once in-place writing begins, I/O failure/cancellation may leave partial changes; later launch failure does not undo edits. There is no atomic batch, rollback journal or interrupted-operation recovery. The trusted-private-node and filesystem/ACL/mapping-race limits remain. This completes an initial subset for each parser family, not full egg parity: broader dialects, templates, conditions and XML namespaces/mixed content remain open.

Local fixtures exercise paths/creation, attributes/wildcards, Unicode/escaping/literals, stable output, invalid rules/input, DTD/entity/namespace/mixed-content refusal, structure/match/mutation/output limits, cancellation, mixed-batch preservation and native launch. The service fixture adds sixth-file refusal and actual XML edits with retained SDDL. All 558 local cases/subtests, 14 native cases and 45 combined Server 2022 checks pass. See [evidence and remaining gates](evidence/server2022-alpha28-xml-reboot.md).
