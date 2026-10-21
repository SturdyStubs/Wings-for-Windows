# Required game identity permissions

Alpha 14 checks that an opt-in `run_as` identity has the filesystem grants needed by the configured launch. The protected-path audit checks what the game must not access; this check separately verifies access it must have. Neither check changes permissions.

## Required grants

| Configured object | Required effective DACL grants |
| --- | --- |
| Game executable | `FILE_GENERIC_READ` and `FILE_GENERIC_EXECUTE`. |
| Each `requirements.required_files` entry | `FILE_GENERIC_READ`; these entries can be data files, so execute access is not assumed. |
| Server working directory | List contents, traverse, read attributes, create files and create subdirectories. |
| Server `.tmp` directory | The same directory grants; this is the directory passed as `TEMP` and `TMP`. |

The directory creation bits are `FILE_ADD_FILE` and `FILE_ADD_SUBDIRECTORY` (aliases of file write/append bits). See Microsoft's [file access constants](https://learn.microsoft.com/en-us/windows/win32/fileio/file-access-rights-constants) and [AccessCheck contract](https://learn.microsoft.com/en-us/windows/win32/api/securitybaseapi/nf-securitybaseapi-accesscheck).

The checker duplicates the selected restricted token for effective access evaluation. It opens clean local components without following observed reparse points, inspects handle-derived security descriptors and evaluates the leaf's grant using the existing conservative ACL evaluator. Unsupported generic/complex applicable ACEs fail explicitly. Non-single-link files, wrong object types, absent objects and cancellation fail the check. Ancestors are opened by the daemon for inspection; their DACLs are not treated as inherited permission for the leaf.

Explicit directory traverse permission is required as a conservative setup rule even though the retained Windows traverse privilege can bypass some traversal checks. The daemon itself must also have the read/list/security-descriptor access needed for inspection. Configured profiles remain limited to 64 declared dependencies; the internal checker separately rejects more than 256 inputs.

## Startup and diagnostics

Startup creates the configured server root/`.tmp` through the existing daemon flow, acquires the [startup guard](LAUNCH-GUARD.md), audits forbidden access, then checks required grants. A failure keeps the server offline and occurs before WFP policy installation or game process creation. No fallback to shared-account execution or permission repair occurs.

`--check-identity <UUID>` runs the same checker as `launch_access` after `filesystem_access` and before `service_privileges`. The diagnostic never creates missing directories. Provision the root and `.tmp`, apply the intended ACLs and store the game credential before running it as the eventual daemon account. A failed earlier stage leaves this stage `not_run`. JSON now contains ten ordered checks; consumers should locate checks by name. Network-policy diagnostics inherit this additional identity prerequisite.

The checker performs no test writes, impersonation, process launch or additional logon. The identity diagnostic still attempts its existing one batch logon before reaching access checks. Errors identify the object/purpose and missing access mask where available; they contain no credential values.

## Limits and local evidence

Directory creation grants do not establish access to existing children or prove that inherited ACLs will permit reopening/modifying newly created files. Save files, existing logs, file deletion/rename, loader dependencies outside the declaration, EFS/integrity policy, game-specific access and changes after inspection still need validation. Passing DACL checks is not proof of successful image loading or a complete isolation boundary.

Tests use a restricting SID on the existing test token and ACLs on disposable fixtures. They cover missing execute/read/create grants, explicit denial, missing/wrong-type paths, cancellation and input bounds. Actual Windows read/execute file opens and allowed/denied temporary-file creation under that token agree with preflight results. Impersonation occurs only in the test and is reverted before assertions. No second Windows account or live Windows Server/service/game deployment has been certified; M2 remains open.
