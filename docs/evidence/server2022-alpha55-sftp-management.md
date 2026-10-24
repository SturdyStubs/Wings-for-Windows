# Server 2022 alpha 55 SFTP file management verification

Observed October 24, 2026 UTC and America/Phoenix. An independent opt-in exposes bounded mkdir, no-replace rename/move, regular-file deletion and empty-directory removal. This advances full SFTP #30 without completing overwrite/append or public deployment. [Machine-readable results](server2022-alpha55-sftp-management-results.json).

## Artifact and tests

- Immutable runtime/probe source: `e8c31a185f9487780cd7e1fc97b7cb4c42413da0`, source dirty false.
- Package: `wings-windows-native-0.1.0-alpha.55-windows-amd64.zip`.
- Package SHA256: `a9604f07d7bda9d8c829a5ad154707b0b6d0a6255c3d350186c81b34d30b1107`.
- Daemon SHA256: `4a2b20c99a557c81c6d1349669141b39516059558e01a4e90240da8667a35e29`.
- Clean Windows amd64 release: **1,572 local cases/subtests**, zero skips, vet and packaged default-invocation native smoke.
- Exact Server 2022 build 20348 package: **14 native runtime / 130 SFTP backend and integrated-protocol / 34 SSH subsystem / 14 configuration** cases, zero failures/skips. Portable tests run as administrator; the retained daemon/game use their existing separate non-administrator identities.

## Implemented scope

The independent `sftp.allow_file_management` flag defaults false. Require `file.create` for mkdir, `file.update` for rename/move, and `file.delete` for REMOVE/RMDIR. Creation uses one new directory under an existing pinned parent. Rename uses existing HTTP preparation/native no-replace helpers and shared rename contention lock, including case-only names and bounded nonempty directories. Removal shares the HTTP delete contention lock and deletes the exact pinned entry; REMOVE is regular-file-only and RMDIR is empty-directory-only. No recursive fallback or implicit parent creation exists.

Reject root mutation, unsafe paths, observed aliases/links/multiple hardlinks, wrong types, collisions, restricted source/destination/descendants, unsupported operations and missing sources. Four global command slots, cooperative ten-second context, 1000 rename-tree entries and 32 child levels bound work. A replaced immutable file-policy identity during preparation refuses mutation, followed by current authorization/policy checks. This does not certify an atomic hostile-directory snapshot or administrator isolation. Native filesystem calls are cooperative.

Local/portable tests cover successful file and directory workflows, wrong types/nonempty folders, individual permission gates, default opt-out, revocation/cancellation/suspension/shutdown, alias/link/denylist/collision protection, exact no-op/case-only rename, tree bounds, shared HTTP locks/capacity and actual encrypted SFTP commands with mock authentication. Initial Unicode tree-limit fixture names collapsed under NTFS case folding; unique ASCII counters correctly exercise the unchanged production bound. Corrected before the clean build; no runtime cap change was needed.

## Real panel and deployment

Runtime-only alpha54-to55 upgrade preserves config bytes and executable ACLs with protected backups. Public operator-managed update remains 200/applied false and actual outgoing panel sync returns 204. Before enabling management, the updated real-panel upload probe passes **23 checks**, including continued mkdir/rename/remove refusal while uploads work.

Enable only `sftp.allow_file_management: true` on the dedicated VM, preserving all other native configuration bytes, protected ACLs, the existing upload flag and persistent host key/fingerprint. Binding remains `127.0.0.1:2022`; demo readiness is confirmed. No public SSH tunnel/firewall change is made.

The real-panel management probe passes **31 checks**. It registers/removes only an ephemeral public key on the existing Client account, authenticates through the real panel remote API, creates a folder and file, renames the file, moves the nonempty folder, verifies contents, preserves a colliding existing destination, refuses wrong-type/nonempty deletion, removes the regular file and then the empty folder. Scoped node revocation denies subsequent mkdir and an unpublished upload. Existing 9 MiB SFTP upload/download integrity, publication only at CLOSE, disconnect cleanup, unregistered/removed-key refusal and key/fixture cleanup also pass.

The unchanged alpha52 Client probe passes **19 actual Client API/panel-issued-token groups**, covering files/archive/plain TAR, real signed HTTP transfer/replay refusal, trusted WSS auth/stats/commands and power. Earlier broader HTTP upload/archive/synthetic-WSS evidence remains version-specific; no claim those probes were rerun here. Public large HTTP uploads remain a known prior proxy 413 gate.

## Cleanup and audit

Both SFTP probes remove their own panel keys and exact owned fixtures. Client cleanup removes only known files, followed by three independently validated empty directories removed nonrecursively on-node. Final audit finds zero fixture roots, SFTP stages and active remote pulls, one profile, intact prior marker/file-manager data, exact daemon hash, protected config/key ACLs, only the planned management flag addition, and demo/service/tunnel Running, unsuspended. Outgoing actual panel sync again returns 204.

## Remaining work

Overwrite/append/resume, POSIX replacement rename, attributes/links, public SFTP and broad client/load certification remain open. So do quotas/reservations, durable staging recovery, full hostile-isolation coverage, actual panel subuser removal, live password login, browser rendering and a complete real-game lifecycle. No new reboot is claimed. Native I/O can exceed cancellation deadlines; late mkdir failure may retain a newly created folder with foreign contents. Reload after a lost mutation reply before retrying.

No additional accepted task is split out. **M4 stays 5/10; the project has 22 accepted milestone tasks and 23 open out of 45**, all six milestones open. These counts are not engineering percentages. [Practical remaining-work assessment](../COMPLETION-STATUS.md) | [SFTP setup and limits](../SFTP.md).
