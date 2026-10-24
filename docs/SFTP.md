# Read-only loopback SFTP

Alpha 53 introduces a deliberately limited SSH/SFTP subsystem. It is disabled unless the operator adds `sftp` to the native JSON configuration. This is not public-hosting or full Wings SFTP certification.

```json
"sftp": {
  "listen": "127.0.0.1:2022",
  "host_key": "C:\\WingsWindows\\private\\sftp-host-key"
}
```

The listener must use an explicit loopback IP and port 1-65535, distinct from HTTP. The persistent, unencrypted Ed25519 private key must be a separate canonical absolute file beside the protected native configuration, outside the server data directory on the same drive. Do not put it in a game directory or source control. Provision the key before restarting the daemon; an unavailable/invalid key or occupied listener prevents readiness. The daemon never silently generates a replacement key.

One provisioning option is Windows OpenSSH `ssh-keygen -t ed25519 -f C:\WingsWindows\private\sftp-host-key`. Use an empty passphrase for unattended loading. Apply the protected native configuration file's ACL to the key: administrators/SYSTEM and the daemon identity may read it; game identities and ordinary users must not read or replace it. Confirm the parent directory is also protected. Native identity checks additionally deny game access to this key.

For the disposable test node, the opt-in probe can generate an exclusive new key with `--generate-host-key`; see [probe instructions](../tools/live-sftp-test/README.md). Back up the key securely and compare its public SHA256 fingerprint before trusting a client connection. A file copy of a private key grants no authorization to game data but permits server impersonation.

## Authentication and access

Use the panel SFTP username (`username.<eight-character-server-prefix>`), with a panel password or a public key registered on the user's panel account. Every SSH authentication delegates to `POST /api/remote/sftp/auth` using the native node credentials. The returned server must match exactly one locally approved profile. Unknown/ambiguous/unapproved/suspended servers are refused.

`file.sftp` is required for login, `file.read` for directory listing/stat/realpath, and `file.read-content` for opening file contents; panel wildcard permissions are recognized. All mutations are refused, including upload, truncate, mkdir, rename, delete, attributes and links. SSH shell, command execution, forwarding and extra session channels are refused.

Reads use pinned Windows handles inside the server data root. Paths and current case-insensitive egg denylists are checked at open and during reads. Reparse points, multiple hardlinks, alternate streams, reserved Windows names and conflicting writer handles are refused. A read handle prevents ordinary concurrent writes/deletion until closed; actively written game logs may consequently be unavailable. This first version favors stable reads over live-tail compatibility. Directory listing filters inaccessible entries and rejects more than 1,000 raw entries, including filtered entries.

The existing authenticated `/api/deauthorize-user` hook invalidates older sessions for the specified user/server. Suspension, shutdown and removal from the registered manager also deny subsequent operations. A one-second check closes revoked transports, and reads check authorization on every request. Fresh sessions must authenticate again with the panel. Deleting a panel SSH key prevents new authentication; that panel action alone does not notify existing sessions to disconnect. Actual panel subuser removal still requires a separate live test.

## Bounds and remaining work

- Sixteen total connections, including handshakes; four per observed IP and four per authenticated user. A tunnel makes its users share the observed IP bound.
- Fifteen-second handshake deadline, three auth attempts, five-second panel-auth context, ten-minute maximum session and sixty-second operation-renewed idle deadline.
- One SFTP session per SSH transport, sixteen shared file/directory handles, 32 KiB outbound data packets, 64 KiB maximum backend read request and 1 GiB maximum opened regular file.
- Bounded 16 KiB panel auth input/response and host-key file; at most 128 returned permissions. Shutdown closes listeners/transports and waits with a bounded deadline.

Timeouts are cooperative; native filesystem calls are not forcibly interruptible. These limits do not constitute NTFS quotas, bandwidth quotas or isolation from privileged host administrators. Password guessing protection depends on the panel in addition to local handshake limits.

The HTTP reverse proxy does not carry SSH traffic. No public SFTP firewall/tunnel/DNS changes are included. Keep this listener local for testing; public deployment, uploads/mutations, resumable writes, quota reservations, durable recovery, browser configuration and mutually untrusted tenants remain open. Remove the `sftp` object and restart to disable it, keeping the protected host key for a future re-enable.
