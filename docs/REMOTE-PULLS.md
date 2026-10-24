# Guarded remote-file downloads

Alpha 49 implements the panel/Wings remote pull routes for approved native servers. Enable deliberately in the protected native configuration, then restart the daemon:

```json
"allow_remote_downloads": true
```

The default is false: all three routes return 501 until enabled. The system capability `files_remote_pull_https` appears only when enabled. Node master authentication remains required; the panel enforces its user's permissions before making that request. No pending profile gains access. Downloaded files are data only: this does not install, execute or approve game code.

```http
POST /api/servers/SERVER_UUID/files/pull
{"root":"/existing","url":"https://example.com/file.bin","file_name":"file.bin","use_header":false,"foreground":false}

GET /api/servers/SERVER_UUID/files/pull
DELETE /api/servers/SERVER_UUID/files/pull/DOWNLOAD_ID
```

Background POST returns 202 with `{"identifier":"UUID"}`. GET lists active transfers as `{"downloads":[{"Identifier":"UUID","Progress":0.5}]}`, matching the reviewed upstream's capitalized fields. Progress is 0–1 for known lengths; an unknown-length transfer reports zero until it disappears. Completed/failed/cancelled jobs are removed after resource cleanup. DELETE returns 204 for missing/already-finished jobs and cancels only a job belonging to this server. Capacity remains occupied until cancellation cleanup finishes. Foreground POST instead returns 200 with file metadata, or a bounded generic error, and follows request cancellation.

The request fixture follows reviewed Wings `postServerPullRemoteFile`, `getServerPullingFiles`, `deleteServerPullRemoteFile`, `router/downloader/downloader.go`, and panel `app/Services/Daemon/FilesClient.php::pull`. Deprecated `directory` is accepted when `root` is empty. `use_header` gives a valid raw Content-Disposition filename precedence; otherwise use `file_name` or the URL basename. Unsafe raw filenames are refused rather than stripped. No new progress WebSocket event is invented: active polling is supported. Terminal background logs contain only the job ID and generic result, not the URL or detailed error. A vanished job is not proof of successful publication; check the file or use foreground mode for a direct result.

## Network guard

- Only HTTPS on port 443, with ordinary certificate/hostname verification and TLS 1.2 or newer. No URL userinfo, fragments, alternate ports, HTTP/FTP/file schemes or private-network exceptions.
- No redirects, including public-to-public redirects. Submit a direct final URL. No environment proxy, ambient node bearer/cookies or automatic content decompression. Reject non-200 responses and unexpected Content-Encoding.
- DNS is resolved immediately before connection with a five-second lookup deadline. Refuse empty, failed, oversized or mixed public/private answer sets. Every returned address must pass the conservative policy; connection attempts use the validated numeric IPs directly, without a second hostname resolution. Up to four addresses are attempted with bounded dial timeouts.
- Refuse loopback/private/link-local/multicast/unspecified, shared carrier NAT, documentation, benchmark, reserved and selected relay/protocol ranges. IPv6 is restricted to the current global-unicast allocation with conservative special-range exclusions; mapped IPv4 is checked as IPv4. This may reject legitimate special-purpose global destinations. It is not a complete network isolation boundary against a hostile process or operator-controlled public forwarding service.
- Two-minute total job context/client timeout, ten-second TLS/response-header timeouts and 16 KiB response-header cap. No retry/resume is performed. URLs up to 8,192 bytes may contain query credentials; node API/progress/log errors do not echo them. External source/proxy access logs can independently record signed query strings.

## Filesystem, lifecycle and bounds

Require an existing confined destination directory, valid bounded Windows names and current requested/canonical ancestor denylist policy. A raw filename header containing traversal, drive/ADS/device aliases or another invalid name fails. Six active jobs globally, three per server, and **100 MiB per file**. Unknown-length/chunked and empty responses are accepted; known lengths must match. Oversized lengths fail before staging; an oversized stream can write at most one extra byte before refusal.

Stream through 64 KiB chunks into an exclusive random `.wings-pull-*.tmp` in the checked parent. Recheck cancellation, shutdown, parent identity and source-independent destination/stage policy between chunks and before publication. Flush and publish with native handle-relative no-replace rename. Existing/late-created destinations remain intact. No untrusted path is recursively removed; ordinary failure/cancellation deletes the exact owned stage by handle. Cleanup failure can retain a stage and returns 500 in foreground mode. Crash/reboot recovery is not durable, and a lost success response can leave a complete file.

The cancel function exists before registration/start, so early cancellation cannot miss a not-yet-started job. Admission/counts/removal are serialized. Graceful API close cancels active pulls and waits for cleanup; native disk I/O can still block independently. Background jobs outlive their initiating HTTP request; foreground jobs are cancelled when that request ends. Jobs and progress are in memory only. File size caps do not enforce panel disk allocations or reserve space.

Typical statuses: 501 disabled; 401 invalid node bearer; 202 background accepted; 200 foreground metadata/list; 204 cancel acknowledgement; 422 unsafe URL/name/root; 403 restricted/unavailable parent; 409 destination collision; 413 file limit; 408 foreground cancellation/deadline; 503 capacity/shutdown; 507 disk exhaustion; 400 refused network/HTTP/body failure. Generic errors intentionally omit source URLs and private paths.

## Verification and open work

Focused tests cover contract/metadata/binary/empty/unknown-length downloads, filename headers, URL/IP/DNS pinning, proxy/TLS/redirect configuration, limits, truncation/read failures, collisions, active progress/cancellation, cross-server cancellation refusal, shutdown/request cleanup, dynamic policy and opt-in configuration. Full clean build and Server 2022/live verification are pending. The [live probe](../tools/live-pull-test/README.md) exercises controlled public HTTPS sources on the node without outputting signed URLs.

Private-network allowlists, arbitrary ports/schemes, redirect support, larger/configurable limits, terminal job history, automatic retries/resume, overwrite, quota enforcement, durable recovery and actual browser Pull UI certification remain open. The separate public-upload Nginx body limit does not affect these small JSON requests; it still affects browser uploads of large files.
