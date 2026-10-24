# Alpha 51 real panel Application and Client API verification

Observed October 24, 2026 UTC (October 23 America/Phoenix). User-authorized temporary Application and Client keys enabled actual panel API integration rather than operator-minted JWT simulations. Neither key nor any node token/JWT/signed URL is stored in this report or repository. [Machine-readable results](server2022-alpha51-real-panel-apis-results.json).

## Bindings and scope

- Retained runtime: `0.1.0-alpha.51`, unchanged clean source `e1d29fb79025f69a873d7a8c75a90b85f4cd4473`.
- Actual daemon SHA256: `0e70a483a7a2b5a4302418caa1252421d00ad89a0fb2b59147868b52be789180`, matching the earlier alpha 51 clean package.
- Developer probe source: `358b332589a5ee3708b9a67b8700162e3e127633`. Exact source/binary hashes are recorded in JSON. The Client helper was built before committing its final source, so its Go metadata records the preceding revision with modified=true; it is not represented as a new clean runtime release.
- Local helper checks: **17 Go URL guard cases/subtests**, **four Python no-network refusal checks**, Go vet. The earlier clean runtime's 1,360 cases were not rerun or relabeled as new evidence.
- Target: existing Windows Test node ID 4 / server ID 13, approved demo UUID `58e8c7f9-4923-43b3-b431-90b2bf4edf4f`, trusted panel/node HTTPS and Server 2022 build 20348. No other node/server is mutated.

## Application API: 12 checks

The [Application helper](../../tools/live-panel-test/README.md) reads a key privately, validates the specified node/server and uses only that node's in-memory configuration token for direct native verification. Actual panel suspension returns 204, syncs the native suspended policy and stops the demo. Native start returns 400 while suspended. Actual panel unsuspension returns 204 and clears the native suspended policy.

Application-initiated reinstall returns 204, invokes the approved harmless native installer, rewrites `install.marker`, succeeds and sends a real callback that clears the panel Installing state. Native installation reports complete/successful with no callback error or pending notification. This deployment has persistent installation receipts disabled, so no durable acknowledgement/reboot result is claimed. Native power restores Running and a unique command echoes; final panel/native states are unsuspended/running. The Application key is correctly refused with 403 at a Client endpoint.

## Client API and panel-issued tokens: 18 checks

The [Client helper](../../tools/live-client-test/README.md) requires only the actual Client key; it does not mint JWTs or use the node master token. It verifies:

- Access matches the approved demo; panel resource utilization includes `disk_bytes`.
- A JWT from the real panel's websocket endpoint authenticates over trusted WSS with the panel Origin, and console stats are received.
- Panel file-list/create-folder/create/edit/read, rename and copy preserve expected text.
- A real panel-issued upload link accepts 65,536 random binary bytes; the real panel-issued download link returns the same SHA256 without a master bearer. Replay of each consumed link is refused (observed 404).
- Panel compress, move and extract round-trip the expected fixture text.
- Client power stop/start reaches Offline/Running over authenticated WSS. Commands sent through Client API and WSS each return their unique expected echo.
- Cleanup deletes only known fixture filenames through Client API and verifies that just the two expected empty child directories remain. A final start request is accepted; independent audit confirms actual Running afterward.

The first helper attempt refused the panel's explicit `:443` socket URL because it compared literal strings to the default-port spelling. It stopped before creating fixture data or power mutations. The helper now compares secure host/path and equivalent default/443 port; negative guards cover wrong host/port, userinfo, fragments, unexpected query fields and insecure schemes. The corrected run passes all 18 checks; runtime code did not change.

## Independent cleanup and final state

The Client helper intentionally avoids the API's recursive directory-delete action. On the VM, validate the exact random fixture name and directory/reparse types, then remove only the three empty directories with `.NET Directory.Delete(path, false)`. Independent audit confirms zero client/disk/extract/upload/pull fixture roots, one retained profile, earlier file-manager data present, valid installed marker, unchanged/protected native configuration, and demo/service/tunnel Running. No account, user, key, node, egg or server records are created/deleted. The expected panel activity records and rewritten demo marker remain.

## Remaining gates and GitHub progress

This materially advances real-panel verification for M1/M3/M4: panel-driven suspension/reinstall, Client file workflows, actual panel-issued signed transfers and WSS now pass. It does **not** certify rendered browser UI, real panel subuser removal, all per-operation permissions, a real Windows game, persistent installer recovery or a new reboot. The 9 MiB public-upload proxy body limit remains a separate pending gate; this run uses a small binary payload.

Tracking issues #17, #23, #28 and #29 remain open for their broader acceptance requirements. Scoped upload/download deliverables #49/#50 remain completed with this additional real-panel evidence. No new accepted deliverable is split out; all milestones remain open and counts are unchanged. See [milestone review](../MILESTONE-REVIEW.md).
