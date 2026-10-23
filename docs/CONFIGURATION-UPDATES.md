# Operator-managed node configuration

Alpha 43 implements the upstream `POST /api/update` response for an explicit operator-managed node. It does not implement remote native configuration editing or token rotation.

Enable this in the protected native JSON and restart the daemon:

```json
"ignore_panel_config_updates": true
```

With this flag enabled, an authenticated POST with one JSON object up to 64 KiB returns:

```json
{"applied":false}
```

This follows upstream Wings' `IgnorePanelConfigUpdates` branch in `router/router_system.go` at reviewed revision `747bc4393daf5d95fdf61d76382f5e23749cffe9`. The reviewed panel `app/Services/Daemon/NodeClient.php` posts generated node configuration to that endpoint; its client treats a successful HTTP response as success. A panel save can therefore complete without the unsupported-route error, while the response accurately states that native configuration was not applied. This implementation additionally requires a bounded valid object even in ignore mode.

## What stays operator-managed

All request fields are ignored, including tokens, node identity, panel URL, listener/TLS settings, upload/disk/SFTP values and Linux data paths. The daemon does not echo the request or write disk. Profiles, executable approval, accounts, local privileges, game files and SSH tunnel are unaffected. The system capability `configuration_updates_operator_managed` advertises the enabled mode.

Change Windows settings locally, validate them with `--check`, then restart deliberately. Panel inventory settings can differ from the native listener and paths when a proxy is used. In the test topology, the panel connects to HTTPS/443 while the daemon listens on loopback HTTP/8080; preserving that difference is intentional.

**Do not reset/rotate the panel node master key expecting this endpoint to install the new token.** An accepted response has `applied: false`; the requested credentials are not adopted. Coordinate the native JSON's token and token ID manually with the panel and restart. A mismatched token can make the next panel request fail until corrected. Existing browser tokens are not revoked by this acknowledgement route. The remaining credential/runtime-update work stays open.

## Responses and permissions

- Valid authenticated object in enabled mode: 200 with exactly `applied: false`.
- Enabled mode, missing/null/array/scalar/malformed/multiple JSON: 400; body over 64 KiB: 413.
- Flag absent/false: 501, explaining that remote configuration is unsupported.
- Missing/incorrect bearer token: 401 before parsing or acknowledgement.
- Other methods remain unsupported.

The service needs no write permission on its private configuration directory. Do not loosen those ACLs to support this mode. Request-body strings are never included in errors. No concurrent mutation or persistence is attempted, so acknowledgement cannot partially change credentials or the transport. This is partial route compatibility, not full Wings configuration-update parity. Enabling the policy is an operator choice and does not certify panel versions that interpret the response differently.
