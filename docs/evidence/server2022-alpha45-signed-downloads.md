# Server 2022 alpha 45 signed download validation

Recorded October 23, 2026 (America/Phoenix). This implements bounded one-use file downloads, not uploads, resume or complete file-management parity.

| Field | Result |
| --- | --- |
| Clean package source | `ac1b3f18c4b20b0faaacf1a5303623172e026498` |
| ZIP SHA256 | `c49ea4781def0ff7ced83652029e270d12e2cf0184c73f991ee50d64c4e1e826` |
| Deployed daemon SHA256 | `e7f734bc012a75fbed99a7be257e6bf19ac152b524b648c907e94cd412d00a10` |
| Local suite | 1,027 cases/subtests; zero skipped; vet and clean release pass |
| Packaged native Server 2022 baseline | 14 cases/subtests, zero failed/skipped |
| Portable signed-download API suite | 42 cases/subtests, zero failed/skipped |
| Live HTTPS download groups | 10 pass, including a 9 MiB binary and owned fixture cleanup |
| Live WSS regression | All 10 synthetic token/permission/revocation checks rerun and pass |
| Native baseline UTC | 2026-10-23T19:55:19.8717320Z through 2026-10-23T19:55:22.3881533Z |
| Live download UTC | 2026-10-23T19:56:54Z |
| Host | Microsoft Windows Server 2022 Standard Evaluation, build 20348; PowerShell 5.1.20348.558 |

[Sanitized results](server2022-alpha45-signed-downloads-results.json) bind the clean package/source, runtime/demo/API/claim-fixture hashes and developer live probe executable/source. The package was fingerprint-pinned SSH transferred, hash checked before extraction and exercised through its default Test-Host runner. Developer API/live executables and their fixture are separate from the runtime ZIP. The reusable live probe was added after the immutable runtime build; this does not change its source binding.

## Regression coverage

The 42 focused cases/subtests exercise the independently sourced panel/Wings claim schema, binary/empty/9 MiB files, safe Unicode attachment headers and exact byte counts, one-use replay and concurrent double use, bounded receipt reclamation/capacity, same-jti/new-unique links, query/body/method/Range and signed-path guards, algorithms/signatures/issuer/scope/time/user/unique/jti validity, boot cutoff, missing/directory resources, denylist and actual junction/hardlink refusal, ordinary source locks, sparse oversize and transfer capacity. Source reads remain pinned against new writes/renames during transfer. Midstream tests verify user/jti revocation, expiry, cancellation, policy refresh and shutdown abort the response after a bounded chunk; transport deadline tests verify expiry-capped write-idle renewal. Case-insensitive user revocation is tested for both downloads and existing WebSocket validation.

The full 1,027-case local suite reruns prior protocol/runtime regressions. Native baseline and focused portable tests pass on Server 2022 without Go installed on the VM. The older combined mock SCM suite was not rerun.

## Live deployment and transfer

The running demo was stopped gracefully and the retained dedicated service upgraded from the clean package with protected rollback backups. The exact executable SDDL and native configuration hash are preserved. This is a runtime-only update, not replacement of every installed package file. Existing accounts, credentials, profiles, data and supervised tunnel remain. Service/demo resume; operator-managed acknowledgement still returns applied false and outgoing real-panel sync returns 204.

The [live probe](../../tools/live-download-test/README.md) requires an explicit owned-fixture flag; omission is refused with exit 2 before configuration reads. It creates only an exclusive random directory and three known files under the approved demo data root. It signs five-minute JWTs for a synthetic user using node credentials read locally without displaying them. Trusted public HTTPS is used without certificate bypass, authenticated redirect following or printing active URLs.

Ten groups pass:

1. A 9,437,184-byte binary streams with correct SHA256, length and attachment headers.
2. The same unique token is rejected on replay.
3. A new unique ID with the same panel-style jti is accepted.
4. Extra query path values do not override the signed file.
5. Range is refused and the same still-unused link succeeds for a complete transfer.
6. Wrong-scope token is rejected.
7. An empty file downloads correctly.
8. An unused link is refused after synthetic-user revocation using mixed UUID casing.
9. A fresh later-issued link succeeds after revocation.
10. Only the three known files and exclusive fixture directory are removed, with no recursive fallback.

All ten live WebSocket protocol checks then rerun successfully against alpha 45. A separate read-only inspection confirms zero download-probe roots, earlier install marker/file-manager data preserved, daemon running as the dedicated account, service/tunnel task Running and demo running. No existing user files were selected for cleanup.

## Limits

These are **operator-minted JWTs**, not URLs requested from an actual panel user/browser session. The panel Download button, actual subuser membership removal and browser reconnect/UI remain open gates. No claim of success for those workflows is inferred from source review or synthetic tokens. The runtime test schema matches the reviewed panel; older token schemas without user UUID are intentionally rejected.

Files over 1 GiB, Range/resume/HEAD/conditional requests, uploads, SFTP, slow-client/maximum-size live transfers and durable same-second restart replay prevention remain unsupported or unverified. Single-use receipts/revocations are in memory, and already buffered bytes cannot be recalled. One GiB is a transfer bound, not storage accounting. No real game, new reboot or hostile filesystem/process isolation certification is claimed. External proxy logs can record query credentials independently; no node key, JWT or active signed URL is published here.

HTTP inventory is **22/38 with partial behavior**; M4 stays open. See [operator protocol/limits](../FILE-DOWNLOADS.md).
