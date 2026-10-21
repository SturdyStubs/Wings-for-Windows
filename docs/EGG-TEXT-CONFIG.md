# Approved egg text configuration files

Alpha 23 adds a bounded subset of egg configuration rewriting: the plain-text `file` parser. It runs before a game starts, using fresh panel process configuration. Local execution mode remains unchanged.

## Authorize files locally

Add exact, case-sensitive, root-relative filenames to the local profile and enable egg process configuration:

```json
"use_egg_process_configuration": true,
"egg_config_files": ["server.cfg", "cfg/network.cfg"]
```

Approval permits the trusted panel to replace content in these files as the daemon. It does not grant arbitrary node-file access. Use forward slashes; absolute paths, traversal, backslashes, streams, wildcards, device names, control characters and trailing-dot/space aliases are rejected. At most 16 distinct files may be approved; case-folded duplicates are rejected. Approval without egg process mode is invalid.

Files and parent directories must already exist inside this server's data root. No file/directory is created by the parser. Missing, locked, linked, oversize or invalidly encoded files prevent game launch. The daemon must have write access. File DACLs/ownership are preserved by writing the existing handle. Do not grant additional game permissions to node-private paths.

Example panel process_configuration entry:

```json
{
  "file": "server.cfg",
  "parser": "file",
  "replace": [
    {"match": "server.port", "replace_with": "server.port 28015"},
    {"match": "server.name", "replace_with": "server.name \"Windows Server\""}
  ]
}
```

The process configuration must still provide supported startup readiness and command/SIGKILL stop settings. A file not listed locally, duplicate target, unsupported parser or malformed rule rejects the process configuration. Initial discovery/sync validates the schema; actual file preflight and editing occur on start/restart, including automatic recovery. Syncing a running server does not rewrite its files immediately.

## Text behavior and limits

The schema and prefix behavior are based on [the pinned upstream parser](https://github.com/pterodactyl/wings/blob/747bc4393daf5d95fdf61d76382f5e23749cffe9/parser/parser.go). A matching prefix replaces the entire line. Missing prefixes are not appended. When several rules match a line, their replacements are concatenated in order, followed by one LF. CRLF input becomes LF; nonempty final lines receive an LF. Unmatched lines are retained.

String, number, boolean and null replacement scalars are supported; null becomes the literal `<nil>`. Legacy `value` is used only when `replace_with` is absent. No shell evaluation or node-environment substitution occurs. Unresolved `{{...}}` values and nonempty `if_value` conditions are explicitly rejected; the latter is not a supported conditional text parser. Unknown fields are rejected. The egg file denylist continues to protect frontend/API operations; explicit local parser approval is a separate authority to edit the target at startup.

Limits: 16 files, 128 KiB combined remote entries, 128 replacements per file, 8 KiB match/value scalars, 1 MiB per input/output file and 64 directory components. Input must be valid UTF-8 without BOM or NUL. UTF-16/BOM files need an explicit operator conversion. Result size is checked before any write. JSON, YAML, INI, properties and XML parsers remain unsupported.

## Safety and failure behavior

Every target is opened without following observed reparse points; all ancestor directory handles are retained during the batch. Regular single-link files are required. File identity checks protect node configuration, TLS material and every configured executable/dependency, including alternate spellings of the same file. Existing handles deny new conflicting writers and replacement while held. No host paths, file contents or replacement values appear in parser errors; an error identifies the plan's one-based file index.

The daemon opens and renders all targets before changing the first, so schema/input/preflight errors do not partially apply a batch. Changed files are then written and synced in place. An I/O error or cancellation during this phase can leave completed or partially written files; startup fails and requires inspection. Readers may observe an in-progress file. This is not an atomic multi-file transaction, rollback journal, interrupted-install recovery or defense against concurrent administrative ACL changes/preexisting writable mappings. Retain backups before enabling a new egg's rules.

Power cancellation is checked during rendering/between writes and again before native launch. Slow OS file I/O can delay the power action; emergency kill can still cancel launch because file work does not hold the server state lock. Profile/executable preflight precedes editing; native credential/access/network checks still run before process creation. A later launch-check failure can leave the approved configuration changes applied. Windows-native execution still does not support arbitrary Linux eggs or installation scripts. Use a trusted private node.

## Validation

Local contract and Windows fixtures cover prefix/overlap order, legacy/scalar values, UTF-8/LF behavior, schema and size bounds, cancellation, full-batch preflight, code/alias/lock rejection, released directory guards and actual game launch after successful editing. The exact package passes 14 native and 35 Server 2022 service/persistence/text checks, including missing-file refusal and actual non-administrator edits with preserved SDDL. See [evidence and cleanup limits](evidence/server2022-alpha23-egg-text.md). Reboot tests remain deferred; the remaining parsers and installer workflows stay open.
