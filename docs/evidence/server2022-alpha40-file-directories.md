# Server 2022 alpha 40 directory-creation validation

Validation date: October 22, 2026. The immutable clean-source package was transferred over fingerprint-pinned SSH and its SHA-256 checked before extraction. The default packaged native runner, portable directory API suite, non-administrator SCM fixture and independent cleanup audit pass.

| Field | Result |
| --- | --- |
| Host | Microsoft Windows Server 2022 Standard Evaluation, build 20348; PowerShell 5.1.20348.558 |
| Clean package source | `166eae87abb1800701d219ba864e765421ae3444` |
| ZIP SHA256 | `6f6ac3b25d6b7d95402b3fab5eb5e169ba6b1be88bef71dd78ae9902aaf5dd96` |
| Runtime-test SHA256 | `428f07aab4df4bb585932c92c4424841c8bdec66dcd0a221d6289151212a9251` |
| Demo SHA256 | `ad08ac61d48fbcf4eefd217cafd8d6abf8d51478cfa00a1301cf66fe88f7bdd4` |
| Portable API-test SHA256 | `6214ce76a23cf4290802627eb9177b22f57b6f78216b36fc83eb4a48a681e378` |
| Local suite | 871 cases/subtests, zero skipped; vet and clean release build passed |
| Packaged native VM baseline | 14 cases/subtests, zero failed/skipped; default runner invocation |
| Native baseline UTC | 2026-10-23T05:03:40.5752416Z through 2026-10-23T05:03:43.0455087Z |
| Portable API directory suite | All 38 cases/subtests pass, zero failed/skipped |
| Managed-service fixture | All 23 checks pass, including five new directory checks; four cleanup flags true |
| Independent cleanup | No remaining fixture accounts, explicit rights, owned processes, credential tasks, service, mock or Event Log source |

[Sanitized JSON results](server2022-alpha40-file-directories-results.json) record each service check, exact source/binary/fixture hashes and cleanup counts. Node configuration, account SIDs, passwords and private raw output are excluded. The portable API executable and JSON contract fixture are developer evidence, separate from the runtime ZIP.

## What was exercised

The native runner verifies the matching packaged harmless runtime fixtures. The directory suite runs without Go on Server 2022 and uses the panel/upstream create-directory request fixture. It tests nested/repeat creation, listing/writing inside new directories, authentication/method/size handling, malformed input, Windows device/ADS/traversal aliases, bounds, denylist preflight and sync refresh, existing-file preservation, real junctions, Unicode, a path beyond MAX_PATH, handle release and cancellation.

The SCM fixture retains the basic service/account/credential/lifecycle checks and adds:

1. Anonymous create returns 401 without making a directory.
2. The non-administrator daemon creates missing parents, accepts replay, lists the result and writes a file.
3. Case-folded and nested denylist matches refuse creation without leaving earlier parents.
4. Invalid paths/device names and a regular-file collision preserve existing data.
5. The actual distinct game identity reads the daemon-written file and writes its own file through inherited directory DACLs; the API reads it back.

The first portable API upload omitted its JSON contract sidecar, causing one fixture-loading failure while 37 other checks passed. Staging/workdir was corrected and all 38 passed with production code unchanged. The preliminary log hash is retained in the JSON. The earlier 114-check alpha 39 combined fixture was not rerun in this batch; this release uses the focused 23-check basic-plus-directories SCM fixture.

## Cleanup and limits

The service fixture confirms credential removal, service deletion, mock termination and account/right removal. A separate read-only inspection verifies zero remaining accounts, explicit LSA rights, owned processes, credential tasks, service registrations, mock processes or managed Event Log sources. **1 profile remains retained, 1 loaded**; no forced profile unload/deletion was attempted. Fixture data, newly created folders and logs are retained for inspection. Earlier fixture retention limits remain unchanged.

No real panel, game distribution, public TLS workflow, Windows 2025 host, new reboot, WFP traffic, SFTP or disk-quota enforcement was tested. Windows path checks and scoped handles do not establish hostile-process or hostile-administrator isolation. Later cancellation/I/O/policy changes can retain created parents; creation has no destructive rollback. HTTP inventory is now **17/38 with partial behavior**, and M3/M4 plus the live gates remain open. See [operator/API scope](../FILE-DIRECTORIES.md).
