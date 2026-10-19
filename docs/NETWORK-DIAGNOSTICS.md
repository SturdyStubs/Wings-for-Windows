# WFP installation and cleanup diagnostic

Alpha 12 adds an **explicitly mutating** diagnostic for a disposable provisioned host:

```powershell
.\wings-windows.exe --config C:\WingsWindows\config.json --check-network-policy 11111111-1111-4111-8111-111111111111
```

The selected profile must explicitly configure `network_policy.mode: block_connections` and a dedicated `run_as` identity. The full [identity preflight](IDENTITY-DIAGNOSTICS.md), including one batch logon, must pass first. Missing credentials, shared/unknown profiles or an unselected policy prevent any WFP session from opening.

**This command temporarily installs the account block.** Stop all workloads using that dedicated account before running it: the rules can affect other processes with the same SID, even though this diagnostic launches no game. It is not a read-only status command. Run it as the eventual daemon identity on a disposable host with the required privileges, WFP permissions and filtering-engine availability.

## Stages

The JSON report contains `server_uuid`, overall `passed`, the identity report and, if preflight succeeds, a policy verification report with these stages:

1. `install`: create the same dynamic session/sublayer and four account-scoped block rules as startup.
2. `readback`: enumerate rules and sublayers, then verify this session's exact rule IDs, count, layers, action/flags/weight, sublayer and single-SID matcher. Unexpected/missing/altered objects fail verification. Unrelated filters are not changed or included in the report.
3. `session_close`: close this dynamic session even after cancellation or readback failure.
4. `independent_cleanup_readback`: open a separate observer session, enumerate current objects and verify that the owned rule IDs and sublayer are absent, then close the observer.

The policy report includes owned `rule_ids` and `sublayer_id` for troubleshooting. `cleanup_verified` is true only if session close and the independent inspection succeed. Readback failures can coexist with successful cleanup; overall `passed` remains false. Unknown cleanup state is never reported as success. Missing stages are `not_run`.

Both `traffic_tested` and `game_launched` remain false even on success. The identity report also retains its own game/panel/network exclusions. The command verifies object installation/readback/removal, not packet behavior, allocation restrictions or a game workflow.

Exit status is zero only when every required stage passes; failure exits 1. Invalid configuration/CLI arguments can fail on stderr before JSON is available. Only one CLI operation can be selected. `--check`, `--check-profiles`, `--credential-status` and `--check-identity` retain their prior semantics and do not install network rules.

## Cleanup and failure limits

Cleanup is attempted after an installed session even if the context is cancelled or inspection fails. It is scoped to the dynamic session; there is no wildcard rule deletion. Observer/open/enum/close errors produce failure and no claim of verified cleanup. Partial installation uses startup's existing rollback path; if rollback fails, no subsequent stages can certify removal.

The command uses a 30-second context for preflight/install/readback checks. Native WFP/logon calls and the final cleanup attempts are synchronous and cannot be interrupted mid-call. Cleanup deliberately continues after cancellation. A hung native call can therefore exceed that time. The diagnostic is not a crash-proof transaction or recovery journal; dynamic-session/daemon/filter-engine failure behavior still requires deployment validation.

The pinned WFP binding enumerates current system objects. Unsupported unrelated rule representations or insufficient read permissions may make inspection fail even when installation was possible. This is reported as unverified rather than bypassed. No service/WFP rights or firewall settings are granted automatically.

## Evidence and remaining tests

Development tests use an injected backend for success, altered rule contents, stale objects, denied inspection, close failure, denied observer creation and cancellation. They verify that unrelated sentinel filters remain untouched, cleanup is still attempted once and no traffic/game success is claimed. Node/CLI missing-credential tests stop before WFP and create no server roots.

**Real WFP installation/readback/cleanup has not been executed by these automated tests.** Save a successful report from the intended Windows Server/service configuration before recording that gate as complete. Separately test TCP/UDP IPv4/IPv6/loopback traffic, children, unrelated-account traffic, allocation policy and crash/restart behavior as described in [network policy](NETWORK-POLICY.md). M2 remains open.
