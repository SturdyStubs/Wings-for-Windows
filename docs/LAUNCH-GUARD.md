# Startup filesystem guard

Alpha 13 adds a bounded startup guard to profiles using `run_as`. It reduces replacement opportunities between the protected-path audit and Windows opening the process image. Shared-daemon profiles retain their previous behavior and receive no isolation guarantee.

## What is held

After opening/checking the restricted token and daemon access, startup opens the executable, working directory, temporary directory, declared dependencies, every nonrecursive protected rule and each ancestor through the local drive root. The rules include node secrets, daemon binaries and all configured profile executables/declared dependencies. Recursive other-server data trees are audited without retaining a handle per entry.

Components are opened with `GENERIC_READ`, `FILE_SHARE_READ`, `FILE_FLAG_BACKUP_SEMANTICS` and `FILE_FLAG_OPEN_REPARSE_POINT`. Read access also requests directory listing on directories; the daemon must have that access. Attribute-only handles were insufficient to enforce write sharing in local regression tests. See Microsoft's [CreateFile access and sharing contract](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilew).

Handles have no inheritable security attributes. Observed reparse points, non-single-link regular files, ambiguous/nonlocal paths, missing components and sharing conflicts fail startup. Exact path duplicates are merged without case folding. At most 1,024 component handles can be retained per start; exceeding the bound fails and rolls back. Cancellation is checked between opens and before success; synchronous Windows calls cannot be interrupted mid-call.

The handles remain open across the ACL audit, optional WFP setup, suspended process creation, Job Object assignment and thread resumption. They close when startup returns, including on errors. The snapshot audit separately uses read/list access with read/write sharing and no delete sharing; this still allows existing log writers while making its own rename pin effective.

## Operational consequences

Stop installers, editors and updaters holding write/delete handles on protected files or directories before starting an identity-isolated server. An incompatible existing handle causes a launch error identifying the component; there is no unguarded retry. Ordinary file reads and creating/writing independent child files remain possible. Guarding shared declared code can temporarily prevent an updater used by another configured profile.

`--check-identity` continues to audit credentials, tokens and ACL snapshots. It does not acquire this startup guard, pin the working directory or certify sharing compatibility. A successful diagnostic can therefore be followed by a launch rejection due to an active writer or the handle bound.

## Remaining limits and verification

This is not an atomic filesystem snapshot or a permanent boundary. It does not lock recursive data contents, freeze ACLs, invalidate earlier writable mappings/privileged handles, infer adjacent/transitive DLLs, or retain paths for delayed loading after resume. Changes to hardlinks, case/namespace behavior, mount policy and other hostile race cases still need separate validation. The separate alpha 14 [launch-permissions check](LAUNCH-ACCESS.md) verifies required grants on the working and temporary directories, without testing writes or descendant permissions.

Local Windows tests exercise real write/rename conflicts after the snapshot audit returns, directory writer rejection, ordinary reads/child creation, pre-existing writers, missing-path rollback, idempotent close, cancellation, handle bounds, hardlinks and junction ancestors. Harmless fixtures launch with pins held, including a restricted token derived from the test identity. No second account, Windows Server/service deployment or adversarial race campaign has been certified. M2 remains open.
