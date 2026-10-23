# GitHub milestone review — October 23, 2026

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
