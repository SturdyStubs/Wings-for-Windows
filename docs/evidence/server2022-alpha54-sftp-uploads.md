# Server 2022 alpha 54 new-file SFTP upload verification

Observed October 24, 2026 UTC and America/Phoenix. An additional explicit opt-in permits staged new-file uploads through the loopback SSH/SFTP subsystem. This is partial progress on full SFTP #30, not public SFTP or overwrite certification. [Machine-readable results](server2022-alpha54-sftp-uploads-results.json).

## Artifact and tests

- Immutable runtime/probe revision: `c54bafa6ffe903835471e167635893d32132f077`, source dirty false.
- Package: `wings-windows-native-0.1.0-alpha.54-windows-amd64.zip`.
- Package SHA256: `42d64098d9f186d1db19f45563542ae1bda00c8e110854563f5cd4baa2a5d5ca`.
- Daemon SHA256: `a6289f886ddc07159082d320ef38405f41e797e36591731a704905fd0a4dc324`.
- Clean Windows amd64 release passes **1,515 local cases/subtests**, zero skips, vet and packaged default-invocation host smoke.
- Exact Server 2022 build 20348 package passes **14 native runtime / 74 SFTP backend and integrated-protocol / 34 SSH-SFTP subsystem / 13 configuration** cases, zero failures/skips. Portable tests run as administrator; the retained daemon and approved game use their existing separate non-administrator identities.

## Scope and protocol behavior

`allow_uploads` defaults false inside the existing SFTP object. It adds no HTTP route and does not change the loopback-only requirement. Require `file.create` and write/create OPEN flags. Existing destinations, append/update and other mutations remain refused. Client read/write-create OPEN is accepted for writes, while READ through an unpublished upload handle is denied. Windows inherited ACLs remain authoritative; supplied create attributes are not applied.

A random exclusive native stage remains unpublished until CLOSE. Track bounded merged byte coverage so random-order/overlapping packets work but holes cannot publish. On clean completion, recheck current policy/parent identity/authorization, validate the owned stage, flush and perform native no-replace publication. Destination collisions preserve the existing file. Any write error, transfer-error notification, cancellation, revocation, suspension or policy failure aborts publication and attempts owned-stage cleanup. A two-minute context triggers cleanup automatically; native I/O is cooperative.

Tests cover a 9 MiB reverse-packet binary, empty/overlapping/gap-filled files, exact 100 MiB success and cumulative/offset overflow, fragmentation/packet bounds, wrong flags/permissions/default opt-out, links/aliases/denylist changes, publication races, cancellation/revocation/suspension/shutdown, shared HTTP capacity, and actual encrypted SSH CLOSE versus abrupt-disconnect behavior. Protocol tests use mock authentication; real-panel proof is separate below.

## Real panel and retained-service checks

Runtime-only alpha53-to54 upgrade preserves native configuration bytes and executable ACLs; public operator-managed update remains 200/applied false and real outgoing panel sync returns 204. With uploads still disabled, the updated real-panel probe passes **18 read-only checks**, including upload refusal. The existing persistent host-key fingerprint matches alpha53.

Enable only `sftp.allow_uploads: true` on the disposable test VM, preserving all other native configuration bytes, protected ACLs and the existing host-key file. Bind remains `127.0.0.1:2022`; no public SSH forwarding/firewall change is made. Demo becomes Running before proceeding.

Upload-mode probe passes **23 real-panel public-key checks**. It registers/removes only an ephemeral SSH public key on the existing Client account. A **9 MiB out-of-order SFTP upload** is absent at the final name before CLOSE, then reads back with matching SHA256. Empty upload succeeds. Abrupt SSH disconnect discards its partial stage; scoped node deauthorization prevents another open upload from being published. Existing-file uploads, delete/rename/chmod/mkdir, unsafe paths, exec/forwarding and unregistered/removed keys are refused. Read-only download integrity, fresh authorization after revocation and key cleanup also pass.

The unchanged alpha52 Client probe passes **19 actual Client API/panel-issued-token groups**, including file/archive/plain TAR workflows, real 64 KiB signed HTTP transfer/replay refusal, WSS auth/stats/commands and power. The existing HTTP upload probe passes **10 operator/synthetic regression groups**, including retained-service 9 MiB integrity and cleanup; public 9 MiB HTTP still returns proxy 413. Earlier broader tar.gz/ZIP/WSS regression reports remain alpha53 evidence, not relabeled as rerun here.

## Cleanup and final audit

Both SFTP probes remove their ephemeral panel keys and exact known fixture files. Client cleanup removes known files through the panel, followed by three independently checked empty directories removed nonrecursively on-node. Audit confirms zero fixture roots, unpublished SFTP stages and active remote pulls; one profile; valid prior install marker/file-manager data; exact daemon hash; protected config/key ACLs; only the planned upload flag addition; and demo/service/tunnel Running, unsuspended. Real outgoing panel sync again returns 204.

Developer instrumentation corrections are recorded in JSON. Before the clean build, a nonexistent `os.Root.ReadDir` probe call was replaced with directory open/ReadDir. The initial encoded final-audit command exceeded the Windows SSH command-line limit before execution; uploading and invoking the same script by path succeeded. Neither required a change to the deployed runtime.

## Remaining gates

No overwrite/append/resume, implicit directories or other SFTP mutations, public SFTP, quota/reservation, durable stage recovery, full browser/client matrix, actual panel subuser removal or real-game/isolation certification. Live password login is not tested. A clean CLOSE of a contiguous prefix is a file of that length; SFTP does not declare the client's intended length. A lost CLOSE reply can leave a complete published file; reload before retrying. Abrupt process/host failure can retain stages. No new reboot is claimed.

Full SFTP [#30](https://github.com/SturdyStubs/Wings-for-Windows/issues/30) and all six milestones stay open. No new accepted issue is split out: **M4 remains 5/10**, 45 project deliverables. [Setup and limits](../SFTP.md).
