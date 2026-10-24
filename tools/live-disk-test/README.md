# Live disk measurement probe

Build `go build -trimpath -o dist/live-disk-test.exe ./tools/live-disk-test`, then run on the dedicated Windows test node:

```powershell
.\live-disk-test.exe C:\WingsWindows\private\config.json https://windows.vanity.gg APPROVED-DEMO-UUID --allow-owned-fixture
```

Reads credentials only on the node, sends master-authenticated HTTPS requests to the explicit operator-controlled endpoint and never prints credentials or response bodies. Requires a disposable approved demo whose data is otherwise quiet. Creates an exclusive random `disk-probe-*` directory, checks an exact 9 MiB + three-byte change (including a hidden file and an empty file), then removes only known files and the empty directory and waits for the baseline to return. Unexpected contents are preserved and fail cleanup; no recursive fallback.

Each refresh wait is at most 48 seconds; normal execution takes about a minute because production caches are deliberately not bypassed. No files outside the owned directory are changed. This tests API measurements under the retained service, not browser dashboard rendering, real-game installation or quota enforcement.
