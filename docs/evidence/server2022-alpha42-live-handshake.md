# Alpha 42 first live node handshake

Observed October 22, 2026 (America/Phoenix); VM observation UTC: `2026-10-23T06:45:09.9173291Z`.

The operator configured a dedicated DNS-only public node hostname, a VPS Nginx HTTPS/WebSocket reverse proxy and a loopback-only reverse SSH listener. Certbot reported successful certificate issuance and scheduled renewal. The node uses the alpha 42 package/source recorded in [release evidence](server2022-alpha42-file-copy.md). Its private native JSON was derived from the operator-provided panel node YAML without displaying credentials. It binds HTTP only to Windows loopback, with an exact HTTPS panel origin and zero approved executable profiles.

## Observations

- Operator supplied unauthenticated HTTP and public HTTPS responses: both return 401 with the daemon JSON error schema. HTTPS succeeded without bypassing certificate validation.
- Agent independently requested authenticated `GET /api/system` directly on Windows loopback and through the public HTTPS endpoint: both return 200 using the node token read locally on the VM, without printing it.
- Agent requested authenticated `GET /api/servers` through public HTTPS: 200. An independent loopback check confirms an empty JSON array; zero assigned local/pending servers and zero approved profiles. Pending creation is enabled.
- Panel settings save still displays an automatic daemon-update warning. Source inspection confirms that the daemon does not implement the upstream runtime configuration-update route. This warning is not proof of a failed system endpoint; the exact panel-side failed request/status has not been captured.

## Remaining gates

The daemon runs temporarily in the operator's foreground Administrator session with no execution profiles. The reverse tunnel is also manual. Permanent non-administrator service provisioning, tunnel reconnection/startup, actual panel UI health, demo egg import/create/install/power/console/files, subuser revocation and a real game remain unverified. A successful authenticated system request is only a first live network/authentication handshake, not complete panel compatibility. Game-player routing and SFTP are not provided by this HTTP tunnel. Certificate renewal is configured but renewal success has not been tested.

No tokens, account credentials, node UUIDs or private key contents are recorded. The existing alpha 42 mock/service and cleanup evidence remains separate from this deployment.
