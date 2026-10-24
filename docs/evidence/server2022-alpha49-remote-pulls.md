# Server 2022 alpha 49 remote-pull validation

Recorded October 23, 2026 (America/Phoenix; VM observations are October 24 UTC). This verifies bounded opt-in direct public HTTPS downloads, not automatic game installation or complete remote-transfer parity.

| Field | Result |
| --- | --- |
| Clean runtime package source | `67c594700749dca6fe84b35b2dc121c74ccbbee5` |
| ZIP SHA256 | `4c6ec163f56b276eb5d51bf3d7992e496f9717eac7f0814913881b97ec2ba132` |
| Deployed daemon SHA256 | `26940f578e5ea23cdff509642c6bebdf94c90d770873a4567121915998de73a9` |
| Local suite | 1,277 cases/subtests, zero skipped; vet and clean release pass |
| Packaged native Server 2022 baseline | 14 cases/subtests, zero failed/skipped |
| Portable remote-pull API suite | 87 cases/subtests, zero failed/skipped |
| Final live pull/cleanup groups | 11 pass through public HTTPS and retained non-administrator service |
| Source / early-cancellation fixture | 9,437,184 random bytes / 104,857,600 sparse bytes |
| Upload / WSS regressions | Ten checks each pass; public large upload remains proxy 413 |
| Host | Windows Server 2022 Standard Evaluation, build 20348; PowerShell 5.1.20348.558 |

[Sanitized results](server2022-alpha49-remote-pulls-results.json) bind clean source/package, daemon/runtime/demo/API executables, contract fixture, live probe source/executable, UTC timestamps, explicit opt-in and independent cleanup. The exact ZIP was fingerprint-pinned SSH transferred, hash checked before extraction and exercised with its default host runner. Portable developer tests and live tools are separate from the ZIP. The live probe's cleanup wait was updated after the immutable runtime build; runtime source/tests are unchanged. No GitHub binary release is implied.

## Coverage

The 87 focused cases/subtests follow the reviewed panel/Wings POST/GET/DELETE contract and independently test public/special IPv4/IPv6 filtering, unsafe schemes/ports/credentials/fragments, mixed/failed/empty/oversized DNS answers and validated numeric dialing. Production transport checks enforce TLS verification, no environment proxy, no redirects, bounded headers and disabled decompression/keepalive reuse. Resolver/dial seams and fake HTTP responses are test-only; they are not operator-configurable bypasses.

Filesystem/protocol cases cover binary/Unicode metadata, empty and unknown-length responses, exact 100 MiB success and streaming overflow, filename headers, HTTP/encoding/length/read failures, existing/late destinations, denied paths/stages and parent/path bounds. Background tests check identifiers, active progress, early cancellation, cross-server cancellation refusal and shutdown cleanup. Foreground request cancellation and dynamic policy cleanup also pass. Three local configuration cases/subtests verify the explicit default-off flag round trip. All prior checks rerun in the full local suite; the older combined mock SCM fixture was not rerun.

## Runtime deployment and explicit configuration change

The demo was stopped gracefully for the runtime-only alpha 48-to-49 upgrade, with protected rollback backups. Executable hash/SDDL, original configuration bytes, identities and credentials were preserved in that phase. Operator-managed update acknowledgement returns 200/applied false and outgoing real-panel sync returns 204.

The live test then deliberately enabled `allow_remote_downloads` on this dedicated test node. GET pull returned 501 before opt-in. A candidate configuration passed the new daemon's check; the change inserted only `"allow_remote_downloads":true,` into the existing JSON object, preserving every other original byte. The configuration ACL is unchanged, protected original/atomic backups remain, the service was restarted and the demo resumed. The enabled capability and empty active list were checked. An independent final comparison removes that one inserted marker and exactly matches the protected backup; this is not a claim that the final complete configuration hash stayed unchanged.

The first operator helper used a null backup argument with PowerShell 5.1 `File.Replace`, which became an invalid path and failed before configuration mutation. Service/demo were restored. A tiny owned-file reproduction identified the binding issue; using an explicit protected backup path fixed the helper. This changed no runtime code and did not weaken the network policy. Rollback to an older daemon must also restore a configuration without the new field.

## Live behavior and cleanup

The [opt-in live probe](../../tools/live-pull-test/README.md) reads credentials on the VM only. It creates an exclusive random `pull-probe-*` directory containing known source files. Five-minute synthetic signed download URLs expose controlled sources through the node's existing trusted public HTTPS proxy. Pulls exercise actual public DNS/TLS and the non-administrator daemon without private-address exceptions. The probe never prints credentials or URLs; external proxy logs can independently contain signed query strings. Missing its fixture flag, it exits 2 before configuration access.

Eleven groups pass:

1. Foreground public HTTPS pull returns metadata and a 9 MiB binary with matching SHA256.
2. Existing destination collision preserves the earlier file.
3. Literal loopback HTTPS is refused.
4. DNS-resolved localhost is refused.
5. HTTP source is refused.
6. Background acceptance, active-list schema and eventual binary integrity pass.
7. Immediate cancellation against a sparse 100 MiB source leaves no final destination.
8. Repeating cancellation remains 204.
9. No temporary pull file remains.
10. The original source binary remains intact.
11. Known files and the exclusive empty fixture directory are removed nonrecursively.

The first run passed the ten functional checks but its 15-second cleanup window expired on a source read handle after an early HTTP response refusal. No destination stage remained. Once that source handle closed, a separately bounded audit verified exactly one known ordinary source file in the owned fixture, then removed only that file and empty directory. The probe's retry window was extended to 45 seconds; the rerun passes all eleven checks with runtime code unchanged. Initial failed cleanup and exact recovery results are retained in the JSON evidence rather than relabeled as a pass.

Ten upload and ten synthetic WSS regression checks pass afterward. A separate read-only audit confirms zero pull/upload probe roots, zero active pulls, earlier install marker/file-manager data present, one retained profile, explicit opt-in true, and service/demo/tunnel Running. The service remains `.\WingsRuntime`. No direct distinct-game-account output read, actual browser Pull session, real game or new reboot is claimed.

## Remaining limits

Only public HTTPS/443 without redirects is supported. No private-network allowlist, arbitrary scheme/port, overwrite, retries/resume, configurable larger limit, terminal job history, progress WebSocket event, quota/reservation or durable job/stage recovery. Unknown-length progress remains zero while active. A disappeared job is not proof of publication; foreground mode or checking the final file provides the result. Downloaded files are not executed or promoted into protected game code.

Browser workflows, broader archive formats/merge behavior, progress UI, durable recovery, quotas and hostile isolation certification remain open. The separate public 9 MiB upload still returns proxy 413; small JSON pull requests are unaffected. [Issue #29](https://github.com/SturdyStubs/Wings-for-Windows/issues/29) and all milestones remain open. Accepted task counts stay M4 4/9, 44 overall deliverables. HTTP inventory is 28/38 with partial semantics, not a completion percentage. See [operator scope](../REMOTE-PULLS.md).
