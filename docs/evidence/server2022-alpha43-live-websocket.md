# Alpha 43 live WebSocket permissions and synthetic revocation

Observed October 23, 2026 (America/Phoenix), UTC `2026-10-23T16:38:52Z`. This uses the retained alpha 43 daemon from [clean-release/deployment evidence](server2022-alpha43-config-policy.md), the real demo game process and the public trusted WSS/Nginx/reverse-SSH route. The daemon/runtime was not changed or restarted in this batch.

A local invocation without the explicit mutation-consent arguments is refused with exit code 2 before reading node credentials.

The new [operator probe](../../tools/live-websocket-test/README.md) was built, vetted and run on Server 2022. It reads node credentials locally without printing them and signs five-minute JWTs for a randomly generated synthetic user/unique JTIs. Public WSS certificate verification remains enabled. The explicit mutation flag authorizes only memory revocation for those synthetic identities; no real panel subuser/account was created, changed or removed.

## Results

All ten checks pass:

1. Disallowed Origin is refused at WebSocket upgrade with HTTP 403.
2. Wrong JWT issuer is rejected.
3. A JWT for another server is rejected.
4. Expired JWT is rejected.
5. An allowed Origin and valid synthetic token authenticate successfully.
6. A token with only websocket.connect cannot send console commands.
7. The same token cannot stop the game.
8. Revoking its unique JTI expires its active session; replay is rejected.
9. Revoking the synthetic user expires its active session; replay is rejected.
10. A fresh token issued after user revocation authenticates successfully.

[Sanitized results](server2022-alpha43-live-websocket-results.json) bind the probe executable/source SHA256 and post-check. After the probe, the demo remains running and the forbidden command's echo is absent from fetched logs. The dedicated daemon service and supervised tunnel task remain Running.

An earlier development copy of the same probe passed the same ten checks before the committed harness added an explicit revocation-consent flag and HTTP response cleanup. The final recorded run uses the committed source and rebuilt executable. No runtime package or new release was created, and the prior 945-case runtime suite was not unnecessarily rerun for this standalone tool/documentation batch.

## Limits

Tokens are operator-minted protocol fixtures, **not tokens obtained through a real panel subuser login**. This proves live transport and daemon permission/revocation behavior for this narrow set; it does not prove that panel membership removal emits the correct request or invalidates an actual subuser/browser session. M1's real subuser removal and browser file controls remain open. The operator's prior browser-console report remains separate evidence.

This does not exercise all permissions, browser reconnect, token refresh UI, exact expiry warnings, JTI-less tokens, revocation persistence through restart, real-game behavior or comprehensive isolation. Synthetic revocation entries remain in daemon memory until restart and have no matching real panel user. No node token, JWT, real user UUID, private key or credential file is published.
