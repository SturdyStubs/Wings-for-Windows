# One-use streamed uploads

Alpha 46 implements the public Wings/panel contract:

```http
POST /upload/file?token=SHORT_LIVED_JWT&directory=/existing/directory
Content-Type: multipart/form-data; boundary=...

files: a binary file with its filename
```

The panel authorizes its user, signs a 15-minute `file-upload` JWT, and sends multipart fields named `files`. Multiple files may share one request; the current panel sends one file per request with limited concurrency. Success is HTTP 200. No node master bearer is needed or accepted as a substitute for the signed token.

The contract fixture follows reviewed Wings `router/router_server_files.go::postServerUploadFiles`, `router/tokens/upload.go`, and panel `FileUploadController.php` / `NodeJwt.php` / `resources/scripts/api/server/files/queries.ts`. HS256, exact panel issuer, issued/expiry times (maximum 16-minute lifetime), scope, server/user UUIDs, jti and unique_id are required. Not-before validation applies. Audience is decoded but not independently enforced. The directory is an unsigned query parameter within the signed, approved server; upstream does not sign a destination path for uploads. Unapproved/pending profiles have no file access.

## Limits and preservation

- **100 MiB aggregate file data**, at most 16 files and four concurrent upload requests. Total HTTP body is capped at 100 MiB plus 256 KiB multipart overhead. These are fixed native limits; the panel's configured upload limit does not change them in operator-managed mode.
- Existing parents only. Validate directory and combined destination against Windows path/component bounds. Validate the raw multipart filename, rather than silently removing path components: refuse traversal, backslashes, ADS, reserved devices, trailing dots/spaces and invalid names.
- Each part requires a `files` disposition and a plain filename, at most 16 header values / 8 KiB header text, and no transfer encoding. Unsupported fields, duplicate disposition headers and case-insensitive duplicate filenames fail. Go's multipart parser has its own bounded header parsing allocation before these stricter checks; no fixed 64 KiB total-memory guarantee is claimed.
- Requested/canonical paths, all ancestors and current denylist policy are checked. Observed reparse parents are refused. The parent handle remains pinned; publication rechecks its canonical location. Temporary files inherit the checked parent's permissions and are opened exclusively with native handle-relative creation.
- File bytes stream in 64 KiB chunks to random `.wings-upload-*.tmp` files and are flushed. The request, including its bounded trailing data, must complete before publication. No final filename replaces an existing file, directory, link or hardlink; a late destination collision also fails without replacing it.

Unlike upstream's direct overwrite-and-delete-on-oversize behavior, this implementation deliberately preserves existing destinations. Rename or remove an existing file explicitly before uploading a replacement. This difference can affect the panel's overwrite workflow.

Unpublished temporary files are deleted by their exact owned handles on ordinary failure; cleanup never recursively removes an arbitrary path. A cleanup failure returns 500 and can leave an owned temporary file. A process crash can leave temporary files; there is no durable upload recovery journal. Publication of multiple files is **not transactional**: a late publication failure can retain earlier successfully published new files. Reload the listing after a failed batch. Disk quotas, free-space reservations, overwrite, resume, directory uploads and automatic parent creation remain open.

## Authorization during transfer

Uploads and downloads share the atomic, expiry-pruned 4,096-entry unique_id receipt cache. Once a valid multipart request starts after token/directory/capacity preflight, its token is consumed even if subsequent parsing, collision, cancellation or storage fails. Retry with a new URL. The jti may repeat across fresh links, as in the panel's issuer. Receipts and revocations remain in memory; the second-granularity boot cutoff is not a durable same-second restart replay guarantee.

Expiry, revocation, current denylist, cancellation, shutdown and parent identity are rechecked between chunks and before publication. The upload route renews the read-idle deadline to at most 30 seconds, capped by JWT expiry, and replaces the global absolute write deadline with the expiry ceiling while receiving data. Buffered bytes cannot be recalled; a blocked read may last until its deadline. A short response-write deadline is restored after receiving. Cancellation/revocation during transfer removes unpublished stages rather than publishing partial final files.

Typical statuses: 401 invalid/revoked token, 404 unapproved server or used link, 403 restricted/unavailable parent, 409 collision, 413 body/file/count limit, 422 invalid filename/directory, 503 capacity/shutdown, 507 storage exhausted, 400 malformed multipart. Wrong methods return 405. Generic errors omit credentials and paths.

## Reverse proxy and verification

Nginx must forward `/upload/file` and its query/body to the daemon. Set `client_max_body_size 101m;` in the dedicated node server block to permit the native 100 MiB plus framing ceiling, then validate `sudo nginx -t` before reloading. A smaller proxy limit can return 413 before the daemon receives anything. Proxy request buffering may retain the body on the VPS; its disk limits and timeouts remain operator concerns. Protect or redact query strings in access logs because signed URLs are credentials. The HTTP tunnel does not carry game UDP or SFTP.

Local/portable tests cover the contract, 100 MiB boundary, binary/empty/multiple files, raw names, parser bounds, collision preservation, truncation, real junction parents, staged cancellation/revocation/shutdown cleanup, replay, shared receipts and capacity. The opt-in [live probe](../tools/live-upload-test/README.md) distinguishes retained-service uploads from public proxy limits. Clean release and VM verification are pending. Synthetic signed requests do not establish actual panel-issued browser upload or real subuser-removal behavior.
