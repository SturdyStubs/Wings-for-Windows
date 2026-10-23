# One-use signed file downloads

Alpha 45 adds the panel/upstream public route:

```http
GET /download/file?token=SHORT_LIVED_JWT
```

The panel authorizes its user and creates the signed URL. The browser presents that URL directly to the node without a node master bearer token. The route trusts the node-signed JWT, not the presence of an Authorization header. It never grants an unapproved/pending local profile access to files. File reading through the older master-authenticated `/files/contents` remains limited to 8 MiB; this endpoint streams binary files up to **1 GiB**.

The independent fixture follows reviewed Wings `router/router_download.go::getDownloadFile`, `router/tokens/file.go`, and panel `app/Http/Controllers/Api/Client/Servers/FileController.php::download` / `app/Services/Nodes/NodeJwt.php::issue`. Required claims are HS256, exact panel issuer, issued/expiry times, file-download scope, server/user UUIDs, nonempty jti, unique_id and signed file_path. The reviewed panel issues a fresh unique_id with a potentially reused identified-by jti; one-use tracking is keyed by **unique_id**, not jti. Standard not-before validation applies. Audience is decoded but not independently enforced; signature, issuer, server and file binding are enforced. Tokens from older schemas without a user UUID are rejected.

## Authorization and bounds

- One token query value, at most 8,192 bytes; bounded claim/path strings and a maximum 16-minute issued-to-expiry lifetime (the reviewed panel uses 15 minutes).
- Current approved local server and a non-root Windows-valid signed file path; query file/server overrides do not change the signed resource.
- Requested/canonical path and ancestor denylist checks; regular files only. Observed links/reparse points, multi-hardlinks and directories are refused.
- A native read-sharing-only source handle pins ordinary writes/deletion during transfer. The checked parent/root remain open. Existing conflicting writers return unavailable/locked instead of producing a moving ordinary-file snapshot.
- Eight concurrent transfers and 4,096 retained one-use receipts. Expired receipts are reclaimed; unexpired receipts are never evicted to admit more links. Capacity failure returns 503.

The body uses 64 KiB chunks without loading the whole file. Content-Length is the opened size, Content-Type is application/octet-stream and Content-Disposition uses a safely encoded attachment filename. Cache-Control is no-store and Referrer-Policy is no-referrer. Range/resume, HEAD, conditional downloads, directory/archive creation and files over 1 GiB are not supported. Request a new signed URL for a new transfer or interrupted download.

## Streaming and one-use semantics

Signature/time/scope/revocation checks precede file access. Tokens are consumed atomically only after file/path preflight and capacity admission succeed. Failed preflight or unsupported Range does not burn a link. Concurrent use of the same unique_id admits at most one transfer. Once a transfer begins, its link remains consumed even on cancellation, connection failure, partial data or revocation. Error/replay responses do not echo the token, URL, claims or private file path.

Expiry, server/user or jti revocation, current canonical-path denylist policy, request cancellation and daemon shutdown are checked between reads/writes. User UUID casing is normalized for revocation in both downloads and WebSockets. Each chunk renews the connection write-idle deadline to at most 30 seconds and never past JWT expiry, replacing the ordinary server-wide 30-second absolute write deadline for this route. Already buffered/in-flight bytes cannot be recalled; this is bounded checking, not instantaneous removal of bytes already delivered.

After headers/body begin, failure aborts the HTTP response instead of appending JSON to file bytes or reporting a complete transfer. Clients must detect a truncated Content-Length and obtain a fresh link. Replay receipts and revocation maps are in memory, not a durable journal. Boot-time rejection uses the existing second-granularity issued-at cutoff; no durable same-second restart replay guarantee is claimed.

## Operations and remaining gaps

Typical statuses: 401 invalid/expired/revoked token; 404 missing/unapproved/directory/already-used resource; 403 restricted or observed linked data; 413 oversize; 416 Range request; 423 unavailable/locked source; 503 transfer/receipt capacity. Wrong methods return 405. Generic resource errors intentionally reveal little. Nginx must pass `/download/file` and its query through to the daemon; the test topology's existing root proxy does this. No TLS verification bypass or special public listener is needed.

A signed URL is a short-lived credential. Protect or redact proxy/access logs that include query strings and do not paste active URLs in issues/chat. Node errors do not log token values, but an external proxy can independently log the full request. The existing tunnel carries this HTTP body, not game UDP or SFTP.

Tests cover binary/empty/large files, the actual claim schema, single use and concurrent replay, same-jti/new-unique links, invalid algorithms/signatures/claims, boot/expiry guards, invalid/root/alias paths, denylist and actual junction/hardlink refusal, locked sources, sparse oversize, token/cache capacity, chunk-time revocation/cancellation/policy/shutdown/expiry, source pinning, write deadlines and case-normalized revocation. Trusted private-node limits still apply: sharing/path snapshots do not prove hostile mapping/namespace/ACL isolation. Actual browser-issued panel links, panel subuser removal, resume, uploads, SFTP and full file-management parity remain separate gates.
