# Real Application API demo probe

This developer probe is deliberately limited to the established Windows Native Demo UUID `58e8c7f9-4923-43b3-b431-90b2bf4edf4f`. It requires an explicit lifecycle flag, HTTPS endpoints, matching Application API server/node records, and an initially running, unsuspended demo with an approved native installer. It does not create/delete servers, users, nodes, keys or eggs.

```powershell
python tools/live-panel-test/probe.py https://panel.vanity.gg https://windows.vanity.gg 4 13 --allow-demo-lifecycle
```

Python 3 standard library only. The terminal prompts for a temporary Application API key with input hidden. Noninteractive automation can supply the key on stdin; never put it in command arguments or a committed file. The key needs read access to the selected server/node, node-configuration access and server lifecycle writes. The panel-provided node credential is held in memory and used only at the explicitly supplied matching node HTTPS endpoint. Redirects are refused; normal TLS verification remains enabled.

Checks actual panel suspension/unsuspension, refused suspended start, Application-initiated native reinstall, marker rewrite, real callback clearing Installing, and native start/unique console echo. Native power/console verification is explicitly distinguished from Client API requests. The helper attempts to restore the demo to running/unsuspended even after an ambiguous lifecycle request failure. Inspect its restoration result; it cannot guarantee recovery during an outage. Reinstall intentionally reruns the existing approved harmless recipe and replaces its `install.marker`.

Reports contain only selected checks/status/times, not API keys, node tokens, raw responses or console contents. A separate Client key/probe is required for real file/console user workflows. Browser visuals, actual subuser removal, real games and persistent installer receipts remain separate validation gates.
