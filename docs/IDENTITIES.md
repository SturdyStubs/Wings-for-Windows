# Experimental Windows game identities

Alpha 6 added an opt-in identity launch path for M2; alpha 7 strengthens its filesystem audit, and alpha 8 adds daemon process/token and declared code/dependency checks. It is not a completed isolation implementation. The default continues using the daemon account; existing profiles do not change accounts automatically. Use disposable trusted-node deployments until the remaining isolation and live validation gates pass.

## Profile configuration

```json
"run_as": {
  "sid": "S-1-5-21-111111111-222222222-333333333-1001",
  "credential_target": "WingsWindows/11111111-1111-4111-8111-111111111111"
}
```

This fragment belongs inside one schema-v1 profile. Replace the SID with the actual local game account SID. The credential target must match that profile's full lowercase server UUID. Each configured game identity must have a distinct SID; shared game accounts, built-in account SIDs and credential references belonging to other servers are rejected.

The profile contains a reference, not a password. At each start the daemon reads that generic credential from its own Windows account's Credential Manager set, performs a local batch logon, compares the resulting SID with the pinned SID and rejects the daemon's own account. Tokens containing Administrators or other explicitly checked privileged built-in groups are rejected even when the group is disabled or deny-only. This does not enumerate every possible privileged custom group or user right.

The token is restricted with `DISABLE_MAX_PRIVILEGE`, which retains Windows' traverse privilege. The process starts through `CreateProcessAsUserW`, suspended, with explicit inherited pipe handles. It joins the Job Object before resuming. Any credential, SID, token, ACL, process creation, job assignment or resume failure aborts startup. Alpha 14 also requires [launch access](LAUNCH-ACCESS.md) for executable reads/execute, declared dependency reads and list/traverse/create permissions on the existing working and temporary directories. The checks do not change ACLs or grant access. There is no fallback to the daemon account.

Microsoft documents [batch logon](https://learn.microsoft.com/windows/win32/api/winbase/nf-winbase-logonuserw), [credential-set lookup](https://learn.microsoft.com/windows/win32/api/wincred/nf-wincred-credreadw), [restricted tokens](https://learn.microsoft.com/en-us/windows/win32/secauthz/restricted-tokens) and [CreateProcessAsUser privilege requirements](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-createprocessasusera). In particular, a separate-account launch may require `SeAssignPrimaryTokenPrivilege` and `SeIncreaseQuotaPrivilege` on the daemon account. The daemon does not grant or elevate these rights. The existing service installer does not configure them.

## Operator provisioning

Provision distinct non-administrator local game accounts and their passwords through your host's administrative workflow. Assign only the required batch-logon right and file permissions. The project does not create/delete accounts, grant logon rights or mutate ACLs automatically. Interactive/RDP access is not required for game accounts and should not be granted as part of provisioning. No interactive desktop or Windows user profile is loaded during launch; games needing a loaded profile need further support.

Store a credential **as the Windows identity that will run the daemon**, using a secure provisioning session for that account:

```powershell
$gameCredential = Get-Credential -UserName '.\WingsGameRust'
.\scripts\Store-GameCredential.ps1 -ServerUUID 11111111-1111-4111-8111-111111111111 -Credential $gameCredential
```

The helper requires `.\local-account`, writes only `WingsWindows/<UUID>` and clears its temporary unmanaged password buffer. It changes no user accounts, rights or ACLs. Credentials stored by an administrator in their own account are not the daemon account's credential set. Provisioning a noninteractive service account's credential set needs an appropriate host-administration workflow; the helper does not solve service-profile initialization.

Do not put passwords in JSON, arguments, panel variables or logs. The runtime copies the credential password directly into UTF-16 memory for logon and clears its buffers afterward. This is best-effort memory cleanup, not a guarantee against administrator inspection or memory dumps. Stored credentials are not deleted automatically. The helper refuses accidental overwrite; pass `-Replace` for an existing target and the same account name when rotating. The next start reads the updated target. `--credential-status <UUID>` emits metadata without the password; `--remove-credential <UUID>` removes exactly that target and succeeds harmlessly when already missing. Neither command requires node config or attempts account logon. See [credential lifecycle](CREDENTIAL-LIFECYCLE.md) for maintenance, removal semantics and recovery limits.

## Read-only ACL guard

Before launching with `run_as`, the daemon checks the restricted token's effective DACL grants using `AccessCheck`. It rejects forbidden grants on:

- The loaded configuration file and configured TLS private key: content read, write, delete and ownership/DACL changes.
- The daemon executable and **every configured game executable and declared `requirements.required_files` dependency**: write/delete and ownership/DACL changes. Reads of game code/dependencies remain allowed.
- The immediate parent directories of those paths: write, child replacement/deletion and ownership/DACL changes.
- Other configured servers' root directories **and existing descendants**: listing/content access, write, delete and ownership/DACL changes. Every existing child is checked even when the root denies access.

An unreadable/missing security descriptor fails the check. Applicable raw generic permission bits and complex/conditional/object ACE types are unsupported and abort the audit; ordinary inheritance-only entries are skipped for the current object. The guard never treats unsupported permission semantics as a lack of access. The daemon records its configuration source path during loading; it does not serialize that path into exported config. The guard does not repair ACLs. Ensure the game account can read/execute its game files and write its own root/temp directory while the daemon can still manage those files. Executables and declared dependencies must be in immutable locations for game accounts: writable code files or replaceable immediate parent directories reject launch. This can require separating executable installation directories from writable server data. There is no exception for game self-updaters.

The audit also checks every ancestor up to the local drive root for deletion, delete-child and ownership/DACL changes. It opens path components with `FILE_FLAG_OPEN_REPARSE_POINT`, reads metadata and security descriptors from handles, and rejects any observed reparse point or regular file whose hardlink count is not exactly one. Junctions, symlinks, mounted folders and existing deduplicated/hardlinked files in audited paths are intentionally unsupported. UNC/device namespaces, alternate-stream syntax, trailing-dot/space aliases and common reserved device names are rejected.

Component handles request read data/directory-list access so the delete-sharing restriction is effective while that path is checked; existing write access remains shareable to avoid excluding active log writers. The daemon must have read/list access as well as permission to inspect DACLs. This reduces replacement opportunities but does not make the whole scan atomic. ACL/reparse edits, files created after enumeration, subsequent changes, alternate streams, undeclared executable/dependency content and other filesystem race cases remain open. Declared executable/dependency files are checked, but adjacent DLLs, transitive loader paths and complete shared-directory/mount policies are not inferred. Audit handles close after each audit scope; the audit does not enforce a permanent filesystem boundary. Alpha 13 additionally retains a separate [startup guard](LAUNCH-GUARD.md) for nonrecursive protected paths, the executable, working directory and their ancestors across audit, network setup and process creation/resume. Conflicting writers fail startup. Recursive other-server contents remain snapshots; ACL edits, mapped writes and runtime loader inputs remain outside this guard.

The combined audit is capped at 100,000 protected entry visits and 64 path components, uses 128-entry directory batches and observes the startup context between filesystem operations. Exceeding a bound, cancellation, sharing conflicts, missing paths or unreadable metadata/descriptors aborts startup; the scan is never silently truncated. The current startup deadline is 20 seconds, shared with panel fetch and other preflight work. Large or rapidly changing trees may therefore need a future policy design before use.

These checks are rejection checks of current local filesystem state, not proof of complete cross-server confinement. Microsoft's documentation describes [reparse-point opening](https://learn.microsoft.com/en-us/windows/win32/fileio/reparse-points-and-file-operations), [handle metadata](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getfileinformationbyhandle) and [hardlink counts](https://learn.microsoft.com/en-us/windows/desktop/api/fileapi/ns-fileapi-by_handle_file_information).

## Daemon process and token guard

Before `run_as` launch, a second read-only DACL audit checks the running daemon process and its primary token using the restricted game token. It rejects process memory read/write/operation, thread/process creation, handle duplication, termination, suspension, quota/information changes and ownership/DACL changes. It separately rejects daemon-token assignment, duplication, impersonation, adjustment and ownership/DACL changes.

The guard evaluates handle-derived security descriptors and rejects unreadable descriptors and unsupported applicable ACL entries. It changes no process or token DACL. Harmless query/synchronization rights alone are not treated as memory or impersonation access. See Microsoft's [process access rights](https://learn.microsoft.com/en-us/windows/win32/procthread/process-security-and-access-rights) and [token access rights](https://learn.microsoft.com/en-us/windows/win32/secauthz/access-rights-for-access-token-objects).

This is a launch-time descriptor check, not a system-wide handle audit. Previously acquired handles, other privileged helper processes, subsequent permission changes and a complete inter-process threat model still need validation. No untrusted-hosting guarantee follows from it.

There is no network namespace, allocation allowlist, complete mount/loader policy, account provisioning/password-change recovery or virtualization backend. The optional [WFP connection-blocking path](NETWORK-POLICY.md) is experimental; live filtering and complete network policy remain unverified. Mixing legacy shared-account games with `run_as` profiles does not protect credentials from the legacy games. The [isolation plan](ISOLATION.md) and all remaining M2 gates still apply.

## Verification and rollout gate

Local tests cover actual restricted-token process launch, child privilege queries, stdin/stdout, Job Object termination, SID/daemon-account rejection, absent credentials, config binding/uniqueness and effective DACL grants. A helper round-trip uses a unique temporary fake generic credential, deletes it afterward and never calls account logon. A restricting-token fixture with temporary ACLs verifies that an exposed child behind a denied root fails recursive audit. Actual NTFS hardlinks/junctions, ancestor checks, cancellation, scan limits, existing writer compatibility and rename rejection while a handle is pinned are covered locally. Alpha 8 also checks real `OpenProcess(PROCESS_VM_READ)` and daemon `OpenProcessToken(TOKEN_DUPLICATE)` denial under OS-thread-confined restricted-token impersonation, with reversion before assertions. This uses a restricted version of the test identity, not a new account.

Successful separate-account batch logon and process launch have **not** been validated with a provisioned game account on Windows Server. Only disposable fixture ACLs and a temporary fake generic credential were changed by development verification; no existing user accounts, host ACLs, service rights or firewall rules were changed. Before relying on this path, record the host/service account configuration, successful distinct-SID child launch and attempted credential/cross-server/process access under that identity. Keep issues #19–21 open until their full enforcement and live acceptance checks pass.

`--check-identity <UUID>` runs the [explicit identity deployment diagnostic](IDENTITY-DIAGNOSTICS.md), including one batch logon and the launch-time access guards without creating a game. It emits stage-by-stage JSON and reports unperformed game/panel/network validation explicitly.

`--check` validates reference syntax and per-server uniqueness. `--check-profiles` checks executable/dependency requirements; it does not read credentials, attempt account logon, test launch privileges or run the token ACL guard. Those checks happen on actual start. Manual `--mark-installed` still acknowledges provisioned files only and does not certify identity readiness.
