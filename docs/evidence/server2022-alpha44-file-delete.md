# Server 2022 alpha 44 bounded deletion validation

Recorded October 23, 2026 (America/Phoenix). This batch implements file-manager deletion; it does not implement server-record/account deletion, recycle-bin recovery or a transactional rollback.

| Field | Result |
| --- | --- |
| Clean package source | `57bbad0e41dc9d92aeecb91ba56493de63910e20` |
| ZIP SHA256 | `9943271dafe04954d4d6271e9e643a3984343882172a02552914fb593e7b5264` |
| Deployed daemon SHA256 | `9f998dd5b3df480ad617db34da8300d4cffed529bbe1090ca7fbe21ed5e86970` |
| Local suite | 985 cases/subtests, zero skipped; vet and clean release build pass |
| Packaged native Server 2022 baseline | 14 cases/subtests, zero failed/skipped |
| Portable delete API suite | 40 cases/subtests, zero failed/skipped |
| Native baseline UTC | 2026-10-23T16:50:52.0343450Z through 2026-10-23T16:50:54.5680723Z |
| Live tests UTC | 2026-10-23T16:51:15.8508820Z |
| Host | Microsoft Windows Server 2022 Standard Evaluation, build 20348; PowerShell 5.1.20348.558 |
| Retained service | Dedicated non-administrator daemon; running demo restored after upgrade |

[Sanitized results](server2022-alpha44-file-delete-results.json) bind source/package/executable/fixture hashes. The clean ZIP was transferred over fingerprint-pinned SSH, hash checked before extraction and exercised through the default host runner. The portable API executable and its independent JSON sidecar are developer evidence, separate from the distributable ZIP.

## Windows regression coverage

Forty focused cases/subtests cover the upstream/panel contract and empty 204, nested file/directory removal, missing replay, Unicode/case/empty folders, body/path/root/count/overlap/auth/method guards, full-batch ancestor/source/descendant denylist refusal, actual junction/hardlink rejection, exact 1,024-entry budget and overflow, deep trees, sharing locks and readonly files. They also verify source replacement refusal, policy refresh after preparation, preflight and mid-batch cancellation, request-lock contention, and preservation of an unscanned child created after preparation.

All planned entries are validated and pinned before any native deletion. Children are deleted through exact handles before parents. Tests show a late child causes nonempty-parent refusal after earlier scanned children may already be deleted; that partial outcome is deliberate and documented. No recursive path-delete fallback is used.

## Retained-service live checks

The existing running demo was stopped gracefully. The previous executable, configuration and build metadata were backed up in the operator-only directory. The runtime was upgraded from the clean package, executable hash verified and exact executable SDDL preserved. The existing non-administrator service/game identities, credentials, approved profile and data were retained; service and demo resumed. This is a runtime-only upgrade, not replacement of every package file. Authenticated system/update requests still work and outgoing real-panel sync returns 204.

Eight live checks over public trusted HTTPS pass against a newly generated dedicated test root under the demo data directory:

1. Anonymous deletion returns 401 with its sentinel intact.
2. An invalid traversal in a batch returns 422 before an earlier valid sentinel is deleted.
3. An ordinarily locked file returns 423 before an earlier sentinel is deleted.
4. A new hardlink is refused with 403 and both names intact; only that exact test-created link is then removed by the operator probe.
5. A nested directory tree is removed while its sibling sentinel remains.
6. A file and empty directory are deleted in one batch.
7. Replaying missing paths returns 204.
8. The exact random fixture root is deleted through the new API after an explicit absolute-path/prefix check.

No existing user/game files were selected for live deletion. An independent read-only inspection confirms zero remaining deletion-probe roots, the earlier installation marker and file-manager fixtures still present, service Running and supervised tunnel task Running. The demo remains running. Its accounts, real data, protected credentials, service and tunnel are intentionally retained; this is not a complete deployment cleanup.

## Limits

The previous combined mock SCM suite, browser file UI, real panel subuser removal, real games, new reboot and broader hostile-filesystem/isolation checks were not rerun. Local full regressions include earlier behaviors; the focused portable and actual retained-service checks are the new VM evidence. Native deletion is irreversible and nontransactional. Readonly/native failures, late writes/children, changed policy, cancellation or process loss can leave earlier deletions completed. There is no journal, recursive re-scan, attribute reset or force-close behavior. See [operator bounds and error semantics](../FILE-DELETE.md).

HTTP inventory is **21/38 with partial behavior**. M4 remains open for broader semantics, scoped transfers, archives/fetch, SFTP, permissions/accounting and browser validation. Server-record deletion is still unimplemented and remains a separate M3 requirement.
