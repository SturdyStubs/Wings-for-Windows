# GitHub milestone review

Current review: October 24, 2026. Descriptions reviewed through alpha 54; the latest SFTP upload scope is recorded below. Scoped read-only loopback SFTP [#51](https://github.com/SturdyStubs/Wings-for-Windows/issues/51) is accepted with [exact evidence](evidence/server2022-alpha53-sftp.md); full SFTP #30 remains open. M4 is 5/10 accepted tasks, 45 total project deliverables. M1 11/12, M2 2/5, M3 2/7, M5 0/4 and M6 2/7 remain unchanged. No milestone is closed and task counts are not effort percentages.

## Historical alpha 43 review

All six milestone descriptions were refreshed against the current alpha 43 evidence. No milestone was closed. The original broad deliverables remain open until their entire acceptance scope passes.

Six completed, narrowly defined deliverables were added and closed with acceptance evidence. This changes issue granularity from 35 to 41 deliverables; it is not a claim of six newly implemented whole features or a project completion percentage. The same tasks are added to progress-plan.json so initialization remains consistent.

| Milestone | Accepted tasks | Remaining focus |
| --- | --- | --- |
| [M1](https://github.com/SturdyStubs/Wings-for-Windows/milestone/1) | 11/12 | Real panel subuser removal, browser file UI and reconnect |
| [M2](https://github.com/SturdyStubs/Wings-for-Windows/milestone/2) | 2/5 | WFP, cross-server/adversarial isolation and host/game matrix |
| [M3](https://github.com/SturdyStubs/Wings-for-Windows/milestone/3) | 2/7 | Automatic provisioning, deletion, broader recovery and real game lifecycle |
| [M4](https://github.com/SturdyStubs/Wings-for-Windows/milestone/4) | 1/6 | Deletion, transfers, archives/fetch, SFTP and storage quotas |
| [M5](https://github.com/SturdyStubs/Wings-for-Windows/milestone/5) | 0/4 | Backups, restore and native transfers remain unimplemented |
| [M6](https://github.com/SturdyStubs/Wings-for-Windows/milestone/6) | 2/7 | Boot/outage matrix, remote apply/rotation, metrics/limits and operations |

## Accepted bounded issues

- [#42](https://github.com/SturdyStubs/Wings-for-Windows/issues/42): M1: Verify live HTTPS node authentication and demo browser console
- [#43](https://github.com/SturdyStubs/Wings-for-Windows/issues/43): M2: Verify managed non-administrator service and separate demo identity
- [#44](https://github.com/SturdyStubs/Wings-for-Windows/issues/44): M3: Verify manually approved demo creation and live install callback lifecycle
- [#45](https://github.com/SturdyStubs/Wings-for-Windows/issues/45): M4: Implement and verify bounded directory, rename and regular-file copy operations
- [#46](https://github.com/SturdyStubs/Wings-for-Windows/issues/46): M6: Implement pinned reverse SSH supervision and verify child-exit reconnection
- [#47](https://github.com/SturdyStubs/Wings-for-Windows/issues/47): M6: Support explicit operator-managed configuration update acknowledgements

README badges query GitHub milestone counts automatically; cached badges can lag. Counts here are a dated snapshot. No speculative completion date was added. Newly discovered or extended acceptance work can change denominators. The original live gate #17 remains open despite its narrow completed child issue.

The same batch adds [live WSS protocol evidence](evidence/server2022-alpha43-live-websocket.md), distinguishing operator-minted synthetic-user JWTs from actual panel subuser login/removal. Runtime alpha 43 is unchanged.

## Alpha 44 follow-up

Accepted [#48](https://github.com/SturdyStubs/Wings-for-Windows/issues/48) for bounded pinned-handle file/directory deletion after local, portable Server 2022 and isolated live-service checks. This adds one scoped deliverable: current total is 42; M4 is 2/7 and remains open. The earlier table is the alpha 43 review snapshot. Full delete semantics, browser controls, SFTP/accounting and server-record deletion remain open. See [evidence](evidence/server2022-alpha44-file-delete.md).

## Alpha 45 follow-up

Accepted [#49](https://github.com/SturdyStubs/Wings-for-Windows/issues/49) for bounded one-use signed file downloads after local, portable Server 2022, live HTTPS and WSS regression checks. Current total is 43 deliverables; M4 is 3/8 and stays open. Operator-minted live tokens do not close actual panel-issued browser download or subuser-removal gates. Uploads/resume, SFTP and accounting remain open. See [evidence](evidence/server2022-alpha45-signed-downloads.md).


## Alpha 46 follow-up

Accepted [#50](https://github.com/SturdyStubs/Wings-for-Windows/issues/50) for bounded one-use multipart uploads after local/portable Server 2022, retained-service large-file and public small-file checks. Current total is 44 deliverables; M4 is 4/9 and stays open. Public 9 MiB uploads currently return proxy 413; configure the dedicated HTTPS Nginx body limit and retest. Actual browser-issued uploads, subuser removal, broader semantics, SFTP and accounting remain open. The scoped task does not certify those gates. See [evidence](evidence/server2022-alpha46-signed-uploads.md).


## Alpha 47 partial archive progress

Bounded tar.gz creation now passes 1,117 local / 14 native / 38 portable cases and eleven live archive checks, including a public HTTPS create/download round trip with 9 MiB binary integrity. Upload and WSS regressions also pass. Issue #29 stays open for extraction, malicious-archive extraction checks, guarded remote fetch and broader workflow work. No new accepted task is split out: counts remain M4 4/9 and 44 total deliverables. Actual browser Compress and the separate public large-upload proxy gate remain open. See [evidence](evidence/server2022-alpha47-archive-creation.md).


## Alpha 48 partial archive progress

Bounded single-member tar.gz extraction into new destinations passes 1,187 local / 14 native / 70 portable cases and thirteen live create/extract/cleanup checks, plus upload/WSS regressions. Initial Server 2022 sharing failures were fixed before service upgrade; exact failures/correction are retained in the evidence. Issue #29 stays open for merge/overwrite/additional formats, remote fetch, progress UI and remaining validation. No new accepted task is split out: M4 stays 4/9 with 44 total deliverables. Actual browser Extract and the public large-upload proxy gate remain open. See [evidence](evidence/server2022-alpha48-archive-extraction.md).


## Alpha 49 remote-fetch progress

Opt-in direct public HTTPS pulls now support foreground/background operation, active progress and scoped cancellation. The clean release passes 1,277 local / 14 native / 87 portable pull cases and eleven live checks, plus upload/WSS regressions. Only the dedicated test node was explicitly enabled; existing configuration bytes are preserved except that field. Browser workflows, broader archive/fetch behavior, quotas and durable recovery keep #29 open. Counts remain M4 4/9 and 44 deliverables overall. The separate public large-upload proxy gate remains open. See [evidence](evidence/server2022-alpha49-remote-pulls.md).

## Alpha 50 ZIP extraction progress

Bounded ZIP Store/Deflate extraction now shares the tar.gz safety/publication workflow. The clean release passes 1,340 local / 14 native / 133 portable extraction cases and sixteen public HTTPS live groups, plus upload/WSS regressions. The initial probe encoding/cleanup failure and corrected rerun are preserved in [evidence](evidence/server2022-alpha50-zip-extraction.md). Issue #29 remains open for broader archive/fetch/workflow/recovery work. No new accepted issue is split out: M4 remains 4/9, 44 deliverables overall, all milestones open. Actual browser Extract and the public large-upload proxy gate remain open.

## Alpha 51 disk measurement progress

Cached logical data-root usage now replaces constant zero in approved server/console stats. Clean release: 1,360 local / 14 native / 19 scanner-cache / one API case; six live disk groups verify exact growth and baseline restoration, with extraction/upload/WSS regressions passing. [Evidence](evidence/server2022-alpha51-disk-usage.md). This advances #31 but does not complete accounting or enforce quotas. No new accepted task is split out: M4 4/9, 44 deliverables overall, all milestones open. Browser dashboard, full quotas, SFTP and the public large-upload proxy gate remain open.

## Alpha 51 real panel API follow-up

Twelve real Application and eighteen Client/panel-issued-token checks pass on the unchanged alpha 51 runtime. Actual panel-driven suspension/reinstall/callbacks, file edits/mutations/archives, signed upload/download with replay refusal, WSS auth/stats/commands and Client power are now verified beyond synthetic identities. Exact owned-file deletion and nonrecursive empty-directory cleanup pass; demo/service/tunnel Running. [Evidence](evidence/server2022-alpha51-real-panel-apis.md). Issues #17/#23/#28/#29 stay open for browser rendering, actual panel subuser removal, broader semantics/recovery and real-game gates. Completed scoped #49/#50 gain actual panel-token evidence. No new task is split out: M1 11/12, M3 2/7, M4 4/9; all six milestones remain open. Public large uploads still require the separate proxy fix/retest.

## Alpha 52 plain TAR and real Client progress

Plain TAR now shares the guarded tar.gz/ZIP Extract workflow. Clean release: 1,394 local / 14 native / 150 portable extraction cases; nineteen actual Client/panel-issued-token groups include signed TAR upload/extract, with tar.gz/ZIP/upload/WSS regressions passing. [Evidence](evidence/server2022-alpha52-plain-tar.md). This advances #29 without closing broader archive/fetch/recovery requirements. Browser visuals, actual subuser removal, real-game certification, quotas and public large uploads remain open. No accepted task is split out: all milestones/counts remain unchanged.


## Alpha 54 new-file SFTP uploads

Additional explicit upload opt-in passes 1,515 local, 14 native, 74 backend/integrated-protocol, 34 SSH subsystem and 13 config cases. Eighteen real-panel read-only and 23 upload checks prove default refusal, 9 MiB out-of-order upload integrity, publication only after CLOSE, disconnect cleanup and revoked-upload nonpublication. Nineteen actual Client API/token groups and ten HTTP upload regressions pass; final audit confirms protected key/config, only planned flag addition, no fixtures/stages and Running demo/service/tunnel. [Evidence](evidence/server2022-alpha54-sftp-uploads.md). Full SFTP #30 remains open for overwrite/append/other mutations, public deployment and remaining live/client gates. No new accepted task: M4 stays 5/10, 45 project deliverables and all milestones open.
