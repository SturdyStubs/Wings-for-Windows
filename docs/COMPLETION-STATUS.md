# What remains before completion

Assessment: October 24, 2026. GitHub currently has **22 accepted milestone tasks out of 45**, with **23 open**; all six milestones remain open. These issue counts are not a measure of remaining engineering hours. Small accepted deliverables and large unfinished subsystems have different costs. See [live milestones](https://github.com/SturdyStubs/Wings-for-Windows/milestones) and [exact verification history](VERIFICATION.md).

## Practical private Windows game hosting

The harmless native demo already installs/reinstalls through the panel, starts/stops, echoes console commands and exposes files, transfers and statistics under separate service/game identities. This validates the bridge, not a real game. The next practical acceptance needs a complete Windows game egg and protected executable/dependency installation, node allocations/network reachability, real console/readiness/stop behavior, restart/failure recovery and manual or automatic deletion/cleanup. Browser/subuser and external file-client workflows still need their own proof. Advanced backup/transfer features are a separate scope.

## Full Windows feature parity

| Remaining area | Completion requires |
| --- | --- |
| Game provisioning/lifecycle | Automatic approved binary/account provisioning, server deletion, interrupted install/recovery and an entire real game lifecycle. |
| Files/SFTP | Existing-file updates/append/resume, public SSH deployment and client matrix, complete locked/path/link/disk-full tests and durable staging recovery. Bounded reads/new-file uploads and optional management are partial implementations. |
| Storage | Quota/reservation enforcement across API, archives, installers and continuously running game writes; logical disk measurement alone does not enforce limits. |
| Backups/restores/transfers | Local/S3 backup integrity/exclusions, safe restore/locking/callbacks, node transfer/cancel/rollback and booting a restored real game. |
| Isolation/network | Real WFP traffic/allocation enforcement, broader cross-server/alias/race and credential tests; current support is for a trusted private operator. |
| Operations/certification | Aggregate metrics, remote apply/token rotation/durable delivery, broader cold-boot/outage/upgrade/certificate lifecycle, CI and supported game/load matrix. |
| Panel workflows | Rendered browser files/reconnect, actual panel subuser removal and real password/client coverage beyond the API probes. |

A useful private preview is closer than full production parity. Full parity is still a **weeks-to-months engineering and validation effort**, not a few final alpha releases; this is an assessment, not a calendar commitment. A credible release date needs the first real-game lifecycle and a narrower supported-host/game matrix. Scope and difficult isolation/recovery failures can extend it.

Native Windows feature parity does not imply unchanged Linux Docker egg compatibility. Linux binaries/images/scripts require a Linux backend; native EXE execution cannot translate them. Completion is evidence against defined Windows workflows, not a claim that every egg runs unchanged.
