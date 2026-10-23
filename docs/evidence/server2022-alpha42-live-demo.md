# Alpha 42 first live demo lifecycle

Observed October 22, 2026 (America/Phoenix). Test UTC: `2026-10-23T06:51:12.7653474Z` through `2026-10-23T06:51:16.0414949Z`.

The operator imported the Windows Native Demo egg and created a server record in the real remote panel. Its assigned UUID was independently confirmed as `awaiting_profile` by the daemon before approval. The initial data directory was empty and there was no existing managed service.

## Provisioning

The temporary zero-profile Administrator daemon was stopped. A dedicated non-administrator local game account and a separate non-administrator daemon account were created with randomly generated credentials. The operator-only recovery directory contains DPAPI-encrypted PSCredential exports; passwords were not printed or placed in source control. The earlier bootstrap configuration backup is also protected there. The approved demo/installer profile uses the fixed packaged demo executable, exact game SID and per-server credential target.

The packaged initial service installer applied the node permissions and rights and registered the daemon with manual startup. The credential helper stored the game credential in the daemon account's Credential Manager through its password-backed task and removed that temporary task. The service starts and remains Running as the dedicated daemon account. Neither dedicated account is a direct local Administrators member. No WFP confinement or comprehensive effective-group/domain policy audit is claimed.

## Live checks

All requests below were direct authenticated node API calls over the public trusted HTTPS endpoint and reverse SSH tunnel. They were not button clicks or browser WebSocket tests.

| Check | Result |
| --- | --- |
| Reinstall request | Accepted; approved native installer completes successfully |
| Real panel installation callback | Daemon reports no callback error after the real panel response |
| Installation marker | Read successfully through the file contents endpoint |
| Start | Demo reaches running state |
| Console | `hello` command produces `ECHO:hello` in fetched logs |
| Stop | Demo reaches offline state |
| Post-test process/task inspection | No demo processes or credential provisioning tasks remain |

The daemon service, dedicated accounts, approved profile, credentials and demo data are deliberately retained for the next browser test. This is a deployment, not a fixture cleanup. The demo is offline. Service startup remains Manual; the SSH tunnel remains in the operator's foreground session. The generated recovery credentials are local to the VM and encrypted for the provisioning identity; they were not exported to the repository or chat.

## Remaining gates

Panel UI health/installation status needs operator confirmation. Browser JWT/WebSocket authentication, console streaming/reconnection, subuser revocation, panel file UI, automatic configuration updates, automatic tunnel startup/reconnection, permanent service startup/reboot recovery, real-game installation/gameplay and player routing remain open. The harmless demo is not a real Windows game. No milestone is complete on the strength of these direct API results.

See [the clean alpha 42 package/source binding](server2022-alpha42-file-copy.md) and [first HTTPS handshake](server2022-alpha42-live-handshake.md). No node tokens, generated passwords, private keys, server UUID or account SIDs are published here.
