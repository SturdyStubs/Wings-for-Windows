# Identity deployment diagnostics

Run one explicit identity diagnostic as the Windows account that will run the daemon:

```powershell
.\wings-windows.exe --config C:\WingsWindows\config.json --check-identity 11111111-1111-4111-8111-111111111111
```

This command loads and validates local configuration, checks the chosen profile's files, reads its credential, **attempts one batch logon** and performs the existing token/process/filesystem checks. It does not contact the panel, create server directories, launch an executable, grant privileges, modify ACLs or change firewall policy. A failed real password can contribute to Windows account lockout/audit events. Use it deliberately after provisioning; it is not a background retry or monitoring loop.

Use `--check` for configuration-only validation, `--check-profiles` for local executable/dependency checks without logon, or `--credential-status <UUID>` for credential metadata without logon. Only one CLI operation can be selected at a time. UUID lookup is case-insensitive; the report uses lowercase.

## Report and exit status

The command emits one JSON object and exits zero only when all reported checks pass. Failed diagnostics exit 1. Configuration loading or flag errors also exit nonzero and may be reported on stderr before a structured report is available.

The report contains the server UUID, daemon version, host OS/architecture, `passed` and an ordered `checks` array. Each check has `name`, `status` (`passed`, `failed` or `not_run`) and an error on failure. Evaluation stops at the first failure; later stages are never silently accepted.

| Check | What it verifies |
| --- | --- |
| `profile` | A configured local profile with `run_as`, a valid SID/credential reference and the correct server binding. Shared-account profiles fail this diagnostic. |
| `profile_files` | The existing read-only executable PE/architecture, minimum Windows build and declared dependency checks, using the daemon account. |
| `protected_path_rules` | The same protected-path rules used by startup can be constructed, without creating a manager or syncing panel servers. |
| `identity_reference` | The identity reference syntax is valid. |
| `credential_presence` | The exact generic credential exists in the current account's credential set. Presence alone does not validate its password. |
| `account_token` | One local batch logon succeeds, the token matches the pinned SID, is distinct from the daemon account, passes privileged-group rejection and can be restricted. The token is closed before the command returns. |
| `daemon_access` | The restricted token passes the launch-time daemon process and primary-token DACL guards. |
| `filesystem_access` | The restricted token passes the same bounded protected-file/ancestor/other-server-tree audit used at startup. All audited paths must already exist. |
| `service_privileges` | The current daemon token contains `SeAssignPrimaryTokenPrivilege` and `SeIncreaseQuotaPrivilege`, without enabling or granting them. |

The service-privilege check is deliberately conservative. Windows can waive some privilege requirements for particular assignable/restricted tokens; absence here means this tool cannot certify the intended separate-account service setup. Presence is not proof that Windows process creation will succeed. Actual launch still depends on token/session policy, image/working-directory access, dependencies and Job Object creation/assignment. See Microsoft's [CreateProcessAsUser requirements](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-createprocessasusera).

Every report includes `game_launched: false`, `panel_tested: false` and `network_policy_tested: false`. These remain false even when `passed` is true. A successful report establishes only the checks above at that moment; it does not certify the game, Windows Server, account recovery or complete isolation. In particular, working-directory writes, transitive DLL loading, live filesystem races and network enforcement require additional verification.

## Deployment workflow

1. Provision a disposable host, daemon service account and distinct non-administrator game account using the [identity guide](IDENTITIES.md). Record the Windows edition/build and eventual daemon identity.
2. Install the executable/dependencies, create the writable server root and apply the intended host permissions. The diagnostic never provisions missing directories or changes permissions.
3. Store the credential as the daemon identity and confirm its [metadata](CREDENTIAL-LIFECYCLE.md). Verify the profile pins the actual local account SID.
4. Run this diagnostic in that same Windows identity. Correct the first reported failure, then rerun deliberately. Do not treat another administrator's successful report as validation of the service identity.
5. Save the JSON report with the host/configuration version, then perform an actual disposable game launch, verify the child SID and run the panel/console/file/service checks in [verification](VERIFICATION.md).

The overall diagnostic uses a 20-second context shared across preflight and ACL scanning. Windows credential/logon calls are synchronous and cannot be interrupted mid-call; cancellation is observed between stages and filesystem operations. No automatic logon retries occur. A diagnostic from an interactive provisioning session is not proof that the eventual SCM logon/session/profile has the same behavior.

## Local evidence

Local automated tests cover invalid/shared/unknown profiles, missing executables, missing credentials, cancellation, explicit `not_run` states and absence of server-directory creation. They use random nonexistent credential targets to avoid account logon. Existing restricted-token process/ACL fixtures remain separate runtime checks. No successfully provisioned second Windows account or live Windows Server/panel deployment was validated by these tests. M2 live gates remain open.
