# Live WebSocket protocol probe

Operator-only developer tool for a disposable Windows Native Demo on an already authenticated test node. It reads the protected native JSON on the node, mints short-lived synthetic-user JWTs locally and checks trusted public WSS directly. It never prints node tokens or JWTs. The public node endpoint must be operator-controlled; a supplied URL is not discovered or authorized by the tool. Run with the node's explicit trusted HTTPS origin and approved disposable demo UUID.

```powershell
go build -trimpath -o live-websocket-test.exe ./tools/live-websocket-test
.\live-websocket-test.exe C:\WingsWindows\private\config.json https://YOUR_NODE_HOST YOUR_DEMO_UUID --allow-synthetic-revocation
```

Compile on a developer host, then transfer the executable over an authenticated channel if the node has no Go installation. Keep configuration/keys on the node and outside the repository. The flag explicitly authorizes short-lived daemon-memory revocation entries for the randomly generated synthetic user/JTIs; no panel account is created, changed or removed.

Ten checks cover disallowed origin, wrong issuer/server/expiration, accepted synthetic auth, missing console/power permissions, JTI revocation/replay, synthetic-user revocation/replay, and acceptance of a later-issued token. Console and stop requests are deliberately attempted with no permission: use only a disposable demo because a permission regression could execute those requests. The tool does not install or launch a game. It does not accept certificates insecurely, follow HTTP redirects for authenticated requests or echo error bodies. Failures produce sanitized partial results and nonzero exit status.

This is direct WSS protocol testing with operator-minted JWTs, not browser inspection or proof that real panel subuser removal emits the correct revocation request. Complete that separate panel UI gate before closing M1. Results do not certify malicious filesystem/process isolation, all permissions, token-expiry warning timing, persistent revocation after daemon restart or long outages. The synthetic revoked-user entries live in daemon memory until restart; they have no matching panel account.
