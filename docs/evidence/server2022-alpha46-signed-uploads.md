# Server 2022 alpha 46 multipart upload validation

Recorded October 23, 2026 (America/Phoenix). Bounded native uploads are implemented and exercised. **The public 9 MiB upload remains blocked by a proxy 413**; this is not a claim that the public large-upload workflow is complete.

| Field | Result |
| --- | --- |
| Clean package source | `b60a46aa0b93783845cab81d43dac013ca4a1356` |
| ZIP SHA256 | `daeea085736c9653d6b046dcf611831dc8170e50bea8b828402bb7d5409b55be` |
| Deployed daemon SHA256 | `10678353a404887f899a387f63fcaaf67b1c281a494b5b7015f5a34628998926` |
| Local suite | 1,079 cases/subtests, zero skipped; vet and clean release pass |
| Packaged native Server 2022 baseline | 14 cases/subtests, zero failed/skipped |
| Portable upload API suite | 52 cases/subtests, zero failed/skipped |
| Live upload/service/cleanup groups | 10 pass, including public small multipart and retained-service 9 MiB integrity |
| Public 9 MiB upload | HTTP 413; proxy body-size configuration and retest pending |
| Live download regression | All 10 synthetic signed-download checks pass, including public 9 MiB integrity |
| Live WSS regression | All 10 synthetic origin/token/permission/revocation checks pass |
| Host | Windows Server 2022 Standard Evaluation, build 20348; PowerShell 5.1.20348.558 |

[Sanitized results](server2022-alpha46-signed-uploads-results.json) bind the source, clean ZIP, daemon, runtime/demo/API executables, independently sourced claims fixture, live probe and timestamps. The exact ZIP was transferred through fingerprint-pinned SSH, hash checked before extraction, and tested with its default packaged host runner. Portable developer tests/probes are separate from the runtime ZIP. No release tag or public binary publication is implied.

## Coverage and deployment

The 52 upload cases/subtests cover the reviewed panel/Wings JWT and multipart schema, binary/empty/multiple files and Unicode, streamed 9 MiB and exact 100 MiB success, one-byte-over-limit refusal, raw filename traversal/ADS/device/path bounds, existing and late collisions, case-insensitive duplicates, malformed/truncated bodies, unsupported fields and oversized headers, file-count/body/epilogue bounds, unknown servers, restricted paths and actual Windows junction parents. Interruption occurs after at least 64 KiB is read and a temporary upload exists: cancellation, revocation and shutdown remove unpublished data. Concurrent replay admits only one request; upload/download receipts are shared and capacity refusal preserves an unused token.

The full local suite reruns prior runtime/protocol regressions. Packaged native and portable tests run on Server 2022 without Go installed. A local staging-helper typo after successful native validation mistakenly called upload while fetching the report and raised TypeError. The helper was corrected and resumed the same extracted package; the original host report and all 52 portable test results are retained. Production code was unchanged. This was a harness failure, not a hidden failed product test or a new source build.

The demo was stopped gracefully before upgrading the retained dedicated service from alpha 45. Protected rollback copies remain in the operator vault. The deployed executable hash matches the clean package, its exact SDDL is preserved, and the native configuration is byte-identical. Existing accounts, credentials, profile and tunnel remain. This is a runtime-only upgrade. The demo resumes running; operator-managed configuration acknowledgement returns 200/applied false, and outgoing real-panel sync returns 204.

## Live checks and independent cleanup

The opt-in [upload probe](../../tools/live-upload-test/README.md) runs on the VM, reading protected credentials locally and minting five-minute tokens for a fresh synthetic user. Without its owned-fixture flag it exits 2 before configuration access. It creates an exclusive random directory under the approved demo root; no existing user files are selected for cleanup. HTTPS certificate validation and redirect refusal remain enabled. Ten groups pass:

1. Public binary/empty multipart upload and exact bytes.
2. One-use replay refusal.
3. Existing destination preservation on collision.
4. Wrong-scope refusal.
5. Raw traversal filename refusal.
6. Retained non-administrator service receives a 9,437,184-byte binary over loopback with matching SHA256.
7. Mixed-case synthetic user revocation invalidates an unused upload link.
8. A later fresh link succeeds after revocation.
9. No upload temporary files remain.
10. Only known fixture files and the exclusive empty fixture directory are removed, with no recursive fallback.

The additional public 9 MiB attempt returns 413. The probe explicitly records `public_large_upload_verified: false` and `public_proxy_size_gate_pending: true`; its other checks passing does not remove that gate. In the dedicated `windows.vanity.gg` HTTPS Nginx server block, add `client_max_body_size 101m;`, validate with `sudo nginx -t`, then reload using `sudo systemctl reload nginx`. Rerun the public probe afterward. The 100 MiB native boundary was verified by local/portable tests; no 100 MiB live-service or public success is claimed.

Download and WSS probes then rerun against alpha 46. A separate read-only audit confirms zero upload/download probe directories, the earlier install marker and file-manager data still present, one retained profile, service running as `.\WingsRuntime`, and service/demo/tunnel Running. There is no new direct distinct-game-account read test for uploaded data; inherited permissions and the retained service context are the verified scope.

## Remaining gates

Operator-minted requests are not actual panel-issued browser URLs. Actual upload UI, browser overwrite handling, real subuser removal, public large uploads, slow-client live limits, SFTP, quotas/storage reservation, configurable larger limits, overwrite/resume, durable staging recovery and broader file semantics remain open. Multi-file publication is not transactional; a later publication error can retain earlier newly published files. No new real game, reboot, combined mock SCM suite or hostile isolation certification is claimed. External proxy logs may contain token queries; no credentials or active signed URLs are included here.

Scoped task [#50](https://github.com/SturdyStubs/Wings-for-Windows/issues/50) is accepted with these explicit limits. Broad [#28](https://github.com/SturdyStubs/Wings-for-Windows/issues/28), live gate [#17](https://github.com/SturdyStubs/Wings-for-Windows/issues/17) and all milestones stay open. HTTP inventory is 23/38 with partial behavior, not a parity percentage. See [upload protocol and limits](../FILE-UPLOADS.md).
