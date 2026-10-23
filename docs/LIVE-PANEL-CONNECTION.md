# Connecting a private Windows VM to a remote panel

The control-plane topology has now been deployed on a test VM: authenticated public HTTPS, an operator-reported browser demo console and controlled SSH-child reconnection pass. See [live operations evidence](evidence/server2022-alpha42-live-operations.md). Reboot recovery, complete browser workflows and real-game connectivity remain unverified. The following deployment sequence is a template; it is not an automatic installer. The current development VM uses a private Hyper-V address. DNS alone cannot make that address reachable from a remote panel VPS. The first live target is a dedicated test node and the harmless Windows demo, before a real game's install/control/client-connection workflow.

## Proposed control-plane route

```text
panel backend and browser
       -> https / wss on dedicated node hostname, port 443
       -> VPS HTTPS reverse proxy
       -> VPS loopback-only SSH forwarded port
       -> encrypted reverse SSH connection initiated by Windows VM
       -> daemon bound to Windows loopback
```

The native daemon's existing insecure-loopback exception would be used only for the final same-host hop. The public connection needs a trusted certificate at the VPS proxy. Do not expose that loopback HTTP listener directly on a public or LAN interface. Actual system/file requests, operator-reported console streaming and controlled SSH-child recovery have passed on the test deployment. Full origin/token-revocation, browser reconnect, outage and boot checks remain open.

## Required operator details

- VPS SSH address, login username and an authorized access method.
- A dedicated node hostname controlled by the operator.
- The VPS's actual public origin IP, web-server/proxy layout and certificate provisioning method.
- Authenticated panel administration access to create a dedicated test location/node/allocation.

Keep node tokens, private keys and local production configuration outside this repository. The existing panel and its other nodes should remain untouched; reserve a separate hostname, proxy configuration, loopback tunnel port, Windows installation root and game identity for this test.

## DNS and deployment order

1. Create an A record for the chosen node hostname pointing to the **VPS public IPv4**, initially DNS-only. Use the origin IP from the VPS provider, not a Cloudflare edge address. Add AAAA only if that IPv6 endpoint is actually configured.
2. Inspect the VPS configuration and choose an unused loopback port before changing proxy or SSH settings.
3. Configure and validate a dedicated HTTPS/WebSocket proxy and its trusted certificate, preserving existing sites.
4. Establish a fingerprint-pinned reverse SSH forward restricted to the selected loopback port; check the daemon remains loopback-only. Define an explicit stop/cleanup procedure and verify disconnection behavior.
5. Create the dedicated panel node with that hostname and HTTPS/443, then privately bind its generated credentials to the native bootstrap JSON.
6. Follow [the node/demo walkthrough](PANEL-WINDOWS-SETUP.md), retain manual profile approval and use the dedicated non-administrator service/game identities.
7. Verify real panel discovery, install state/callbacks, console/power, file actions and subuser revocation. Record failures and cleanup before advancing the live gate.

The SSH HTTP/WebSocket route covers the **node API and console**, not arbitrary game/player UDP/TCP ports or SFTP. Actual game connectivity needs a separate approved network route. SFTP is currently unimplemented.

DNS field reference: [Cloudflare subdomain records](https://developers.cloudflare.com/dns/manage-dns-records/how-to/create-subdomain/). Exact proxy/tunnel commands depend on the inspected host configuration and are intentionally not represented as deployed here.

## Persistent Windows tunnel supervisor

Use [`scripts/Start-NodeTunnel.ps1`](../scripts/Start-NodeTunnel.ps1) as a separately deployed supervisor. For example:

```powershell
powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -File C:\ProgramData\WingsWindowsTunnel\Start-NodeTunnel.ps1 -HostName YOUR_VPS_HOST -UserName wings-tunnel -IdentityFile C:\ProgramData\WingsWindowsTunnel\identity -KnownHostsFile C:\ProgramData\WingsWindowsTunnel\known_hosts
```

Protect the entire directory and key against game/ordinary-user access before running it. Copy a public host key already verified by the operator into the dedicated known-hosts file; never disable host-key checking. The SSH key must be unencrypted for unattended batch use and restricted on the VPS to its intended listener. The supervisor does not provision keys, accounts, ACLs, VPS access, HTTPS or task registration. It intentionally writes no unbounded connection logs; inspect task state and authenticated HTTPS together for health.

For unattended startup, register a task under the intended protected identity with an AtStartup trigger, zero execution-time limit, IgnoreNew multiple-instance policy and restart-on-failure settings. The test deployment uses SYSTEM and a directory accessible only to Administrators/SYSTEM; that is privileged transport infrastructure, separate from the non-administrator daemon/game accounts. The wrapper retries every ten seconds after SSH exits and uses keepalive detection. A stopped task stops supervision; inspect for a remaining SSH child if manually decommissioning it. Do not start a second forward on the same VPS port. Preserve an operator connection while replacing an existing manual tunnel.

Test a controlled child exit and check that a replacement child restores authenticated HTTPS before relying on it. AtStartup and Automatic service settings alone do not prove reboot recovery. This tunnel carries only API/console TCP, not game UDP or SFTP.
