# Server 2022 alpha 53 read-only SFTP verification

Observed October 24, 2026 UTC and America/Phoenix. The opt-in read-only loopback subsystem authenticates against the real panel. This does not certify public SFTP or writes. [Machine-readable results](server2022-alpha53-sftp-results.json).

## Clean artifact and tests

- Immutable runtime/probe source: `f019de388cfc2721d7f7e87dcaab0135eed973e1`, source dirty false.
- Package: `wings-windows-native-0.1.0-alpha.53-windows-amd64.zip`.
- Package SHA256: `20bf5a289bde2025e32eba29d09a0d87bd8af25b079f7d540b8480993f1e0631`.
- Daemon SHA256: `75de5e471f5305743702c22d318f57bdf2012e2cc965f5d64735bec537be8817`.
- Clean Windows amd64 release: **1,472 local cases/subtests**, zero skips, vet and default-invocation packaged host smoke pass.
- Exact Server 2022 build 20348 package: **14 native runtime**, **32 portable SFTP backend** and **34 SSH/SFTP protocol** cases/subtests, zero failures/skips. Executable/probe bindings are recorded in JSON. Portable tests run as administrator; retained daemon runs as the existing non-administrator service identity.

## Implementation and live proof

The [documented opt-in](../SFTP.md) requires an explicit loopback listener and an operator-provisioned persistent Ed25519 key beside protected native configuration. Missing/invalid keys or unavailable listeners prevent readiness. Local tests exercise key guards, password/public-key callbacks, final authenticated identity, permissions, shared file/directory handle budgets, pending-connection limits, refusal of mutations/extra sessions/exec/forwarding, revocation, denylist changes, native link/alias refusal and conflicting-writer locks.

Runtime-only alpha52-to53 upgrade preserves existing config bytes and executable ACLs with protected backups. Public operator-managed update stays 200/applied false and real outgoing panel sync returns 204. SFTP is then enabled separately with only the planned JSON addition; the persistent host key receives the protected configuration ACL. Actual listener is `127.0.0.1:2022`, with no public firewall or tunnel change. Demo readiness is independently checked after asynchronous startup.

The [live SFTP probe](../../tools/live-sftp-test/README.md) passes **18 checks**. A temporary Client key, held only in memory/stdin, registers one ephemeral SSH public key on the existing panel account. Real node-authenticated remote SFTP authorization resolves the approved demo/user; actual SSH/SFTP public-key authentication and root listing succeed. Existing install marker is unchanged; a **9 MiB binary download** matches SHA256. Uploads, mutations, unsafe paths, exec, forwarding and an unregistered key are refused.

An authenticated scoped node deauthorization stops an already-open SFTP read; a fresh panel-authorized session succeeds. Removing the ephemeral panel key is confirmed through Client API, and new SSH authentication using it fails. This is not actual panel subuser removal, and panel key deletion alone does not revoke existing sessions. Real-panel password login is not tested here; local callback/contract coverage is separate.

The unchanged alpha52 [Client probe](../../tools/live-client-test/README.md) passes **19 actual panel API/token groups** against this alpha53 daemon: files/archives/plain TAR, real signed 64 KiB transfers and replay refusal, trusted WSS auth/stats/commands and Client power. **Sixteen tar.gz/ZIP**, **ten upload** and **ten WSS** synthetic/operator regressions also pass. These identities remain distinguished from real panel-issued tokens.

## Cleanup and independent audit

Temporary panel key and SFTP fixture are removed. Client cleanup deletes only its known files; independent on-node validation removes its three exact empty directories using nonrecursive deletion. Final audit confirms zero fixture roots/active pulls, one approved profile, unchanged prior marker/file-manager data, exact daemon hash, protected configuration/key ACLs and only the planned SFTP config addition. Demo/service/tunnel are Running and unsuspended.

Two developer-instrumentation corrections are retained in JSON: the first portable SSH log retrieval used a relative .NET path; rerun with an absolute path passes unchanged tests. The first audit counted the pull-response wrapper instead of its downloads array; the corrected actual list is empty. No production code changed for either correction. The immediate enable report observed offline while asynchronous startup was pending; subsequent live and final checks confirm Running.

## Remaining gates

SFTP writes/public deployment, actual panel subuser removal, browser rendering, quotas/reservations, durable recovery and complete real-game/isolation certification remain open. Public 9 MiB HTTP uploads still return proxy 413; this does not affect the loopback SFTP download proof. No new reboot is claimed. Earlier Application lifecycle checks remain alpha51 evidence.

Accept only the scoped read-only loopback deliverable [#51](https://github.com/SturdyStubs/Wings-for-Windows/issues/51); broader [#30](https://github.com/SturdyStubs/Wings-for-Windows/issues/30) and all six milestones remain open. M4 becomes **5/10 accepted deliverables**, 45 total project deliverables; counts describe issue granularity, not engineering completion. HTTP inventory remains 28/38 with partial semantics because SFTP is a separate subsystem.
