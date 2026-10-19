# Experimental account network policy

Alpha 11 adds an opt-in WFP installation path for M2. **Actual WFP rule installation and blocked/allowed live traffic are not yet verified on a provisioned Windows Server host.** The automated tests use an injected backend, not host firewall changes. Keep the trusted-node restriction and M2 gates open.

## Configuration

Inside a local profile with a dedicated `run_as` identity:

```json
"network_policy": { "mode": "block_connections" }
```

Omitted/empty mode or `unmanaged` installs no rules. `block_connections` requires a valid dedicated identity reference; unknown modes, including allocation allowlists, fail validation. This mode is a connection-blocking foundation, not a usable allocation-limited game policy. A game using it should not be expected to accept clients or contact update/authentication services.

After credential/SID/token/access checks and before any child process is created, the daemon opens a dynamic Windows Filtering Platform session and installs four hard block rules scoped to the game account SID:

- IPv4 and IPv6 ALE authorization for outgoing connects.
- IPv4 and IPv6 ALE authorization for receive/accept.

The `ALE_USER_ID` condition uses a security descriptor granting `FWP_ACTRL_MATCH_FILTER` to exactly the pinned account. The daemon refuses its own SID. The scope is the dedicated account, including children retaining that identity, rather than an EXE filename. Other existing processes using that account can also be affected while the session is active. Do not share game identities with unrelated workloads.

These layers authorize TCP connects/accepts, first UDP sends/receives and certain ICMP exchanges. They do not create a network namespace or prevent all local socket binding/listening. Raw/redirected/service-proxied traffic, multicast/broadcast, loopback interactions, existing flows, alternate credentials and other Windows network paths require explicit testing before stronger claims. There is no IP/port allowlist, inbound allocation mapping, DNS exception or restricted outbound destination feature in this release.

Microsoft documents [ALE layers](https://learn.microsoft.com/en-us/windows/win32/fwp/ale-layers), [user-scoped filtering](https://learn.microsoft.com/en-us/windows/win32/fwp/permitting-and-blocking-applications-and-users) and [dynamic sessions](https://learn.microsoft.com/en-us/windows/win32/api/fwpmu/nf-fwpmu-fwpmengineopen0). Bindings use pinned [tailscale/wf](https://github.com/tailscale/wf/tree/6fbb0a674ee6); runtime licenses are included in the release.

## Installation and cleanup

WFP management requires host permissions and a running filtering engine. The daemon grants no additional rights and the service installer does not provision WFP access. The existing unprivileged service setup can therefore reject this mode. Failure to open the engine, add the sublayer or add any rule aborts launch; there is no unmanaged fallback. Partial installation closes the dynamic session, and rollback errors are returned. Cancellation is checked before opening, between rule additions and after installation; synchronous native calls are not interruptible mid-call.

Each launch owns its dynamic session. The process remains subject to suspended creation and Job Object assignment before resume. On root exit, cleanup terminates remaining job descendants, captures up to 4,096 current process handles, waits for the job's active count and captured process handles to finish, then releases the session. The cleanup wait is bounded at five seconds. If process enumeration/open/wait/query fails or exceeds the bound, the daemon closes the kill-on-close job but retains the block session until daemon exit and emits a cleanup warning.

Dynamic objects are removed when their session/owning process ends; no persistent firewall rule is installed. Unexpected daemon/filter-engine termination, concurrent descendant creation and cleanup ordering need deployment/adversarial validation. This is not a durable network boundary across daemon or filtering-engine crashes. Retained sessions can keep an account blocked until maintenance/restart. These limits are why M2 remains incomplete.

The policy is local profile configuration; changing it requires restarting the daemon. Panel sync does not mutate it. `--check` validates the selected mode/reference. `--check-profiles` and `--check-identity` do not install filters or prove WFP access; the identity report continues to record `network_policy_tested: false`.

## Installation diagnostic

[`--check-network-policy <UUID>`](NETWORK-DIAGNOSTICS.md) explicitly installs the selected account block after identity preflight, verifies its owned objects, closes it and checks removal through an independent observer session. It can affect other processes using the dedicated account while active. Unlike the existing read-only checks, it mutates WFP state temporarily. It launches no game and makes no traffic probe. This tool has not itself been live-certified on Windows Server.

## Verification

Local tests verify an independently pinned set of four WFP layer GUIDs, block action, account-condition descriptor and nonpersistent rules. An injected engine tests each partial-install failure, cancellation, rollback errors and idempotent session close. A real Windows Job Object/child fixture with a fake network lease verifies lease release after captured descendants exit. No test installs WFP filters, changes Windows Firewall rules or authenticates a new game account.

Before enabling this mode outside disposable testing, use a provisioned dedicated SID and service account to verify real installation/readback, TCP/UDP IPv4/IPv6 and loopback blocking, children, unrelated-account traffic, rollback, daemon/filter-engine failure and service restart/cleanup. Allocation-limited game networking and the complete network threat model remain future M2/M6 work.
