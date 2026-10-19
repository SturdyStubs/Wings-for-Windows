# Experimental Windows game identities

Alpha 6 adds an opt-in identity launch path for M2. It is not a completed isolation implementation. The default continues using the daemon account; existing profiles do not change accounts automatically. Use disposable trusted-node deployments until the remaining isolation and live validation gates pass.

## Profile configuration

```json
"run_as": {
  "sid": "S-1-5-21-111111111-222222222-333333333-1001",
  "credential_target": "WingsWindows/11111111-1111-4111-8111-111111111111"
}
```

This fragment belongs inside one schema-v1 profile. Replace the SID with the actual local game account SID. The credential target must match that profile's full lowercase server UUID. Each configured game identity must have a distinct SID; shared game accounts, built-in account SIDs and credential references belonging to other servers are rejected.

The profile contains a reference, not a password. At each start the daemon reads that generic credential from its own Windows account's Credential Manager set, performs a local batch logon, compares the resulting SID with the pinned SID and rejects the daemon's own account. Tokens containing Administrators or other explicitly checked privileged built-in groups are rejected even when the group is disabled or deny-only. This does not enumerate every possible privileged custom group or user right.

The token is restricted with `DISABLE_MAX_PRIVILEGE`, which retains Windows' traverse privilege. The process starts through `CreateProcessAsUserW`, suspended, with explicit inherited pipe handles. It joins the Job Object before resuming. Any credential, SID, token, ACL, process creation, job assignment or resume failure aborts startup. There is no fallback to the daemon account.

Microsoft documents [batch logon](https://learn.microsoft.com/windows/win32/api/winbase/nf-winbase-logonuserw), [credential-set lookup](https://learn.microsoft.com/windows/win32/api/wincred/nf-wincred-credreadw), [restricted tokens](https://learn.microsoft.com/en-us/windows/win32/secauthz/restricted-tokens) and [CreateProcessAsUser privilege requirements](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-createprocessasusera). In particular, a separate-account launch may require `SeAssignPrimaryTokenPrivilege` and `SeIncreaseQuotaPrivilege` on the daemon account. The daemon does not grant or elevate these rights. The existing service installer does not configure them.

## Operator provisioning

Provision distinct non-administrator local game accounts and their passwords through your host's administrative workflow. Assign only the required batch-logon right and file permissions. The project does not create/delete accounts, grant logon rights or mutate ACLs automatically. Interactive/RDP access is not required for game accounts and should not be granted as part of provisioning. No interactive desktop or Windows user profile is loaded during launch; games needing a loaded profile need further support.

Store a credential **as the Windows identity that will run the daemon**, using a secure provisioning session for that account:

```powershell
$gameCredential = Get-Credential -UserName '.\WingsGameRust'
.\scripts\Store-GameCredential.ps1 -ServerUUID 11111111-1111-4111-8111-111111111111 -Credential $gameCredential
```

The helper requires `.\local-account`, writes only `WingsWindows/<UUID>` and clears its temporary unmanaged password buffer. It changes no user accounts, rights or ACLs. Credentials stored by an administrator in their own account are not the daemon account's credential set. Provisioning a noninteractive service account's credential set needs an appropriate host-administration workflow; the helper does not solve service-profile initialization.

Do not put passwords in JSON, arguments, panel variables or logs. The runtime copies the credential password directly into UTF-16 memory for logon and clears its buffers afterward. This is best-effort memory cleanup, not a guarantee against administrator inspection or memory dumps. Stored credentials are not deleted automatically. Rotation updates the same target; the next start reads it again. Remove unused targets through Credential Manager when retiring a server.

## Read-only ACL guard

Before launching with `run_as`, the daemon checks the restricted token's effective DACL grants using `AccessCheck`. It rejects forbidden grants on:

- The loaded configuration file and configured TLS private key: content read, write, delete and ownership/DACL changes.
- The daemon executable: write/delete and ownership/DACL changes.
- The immediate parent directories of those paths: write, child replacement/deletion and ownership/DACL changes.
- Other configured servers' root directories: listing/content access, write, delete and ownership/DACL changes.

An unreadable/missing security descriptor fails the check. The daemon records its configuration source path during loading; it does not serialize that path into exported config. The guard does not repair ACLs. Ensure the game account can read/execute its game files and write its own root/temp directory while the daemon can still manage those files.

These checks inspect current descriptors on named paths. They do not recursively audit every existing child, every ancestor, shared executable/dependency tree, alternate stream, hardlink or reparse target. Preventing root listing does not prevent traversal to a known child whose ACL grants access. Path replacement, inheritance and TOCTOU races need further work. The guard is a rejection check for obvious exposure, not proof of cross-server filesystem confinement.

There is no network namespace, allocation firewall enforcement, mount policy, process-handle audit, account lifecycle recovery or virtualization backend. Mixing legacy shared-account games with `run_as` profiles does not protect credentials from the legacy games. The [isolation plan](ISOLATION.md) and all remaining M2 gates still apply.

## Verification and rollout gate

Local tests cover actual restricted-token process launch, child privilege queries, stdin/stdout, Job Object termination, SID/daemon-account rejection, absent credentials, config binding/uniqueness and effective DACL grants. A helper round-trip uses a unique temporary fake generic credential, deletes it afterward and never calls account logon.

Successful separate-account batch logon and process launch have **not** been validated with a provisioned game account on Windows Server. No existing user accounts, service rights, firewall rules or ACLs were changed by development verification. Before relying on this path, record the host/service account configuration, successful distinct-SID child launch and attempted credential/cross-server/process access under that identity. Keep issues #19–21 open until their full enforcement and live acceptance checks pass.

`--check` validates reference syntax and per-server uniqueness. `--check-profiles` checks executable/dependency requirements; it does not read credentials, attempt account logon, test launch privileges or run the token ACL guard. Those checks happen on actual start. Manual `--mark-installed` still acknowledges provisioned files only and does not certify identity readiness.
