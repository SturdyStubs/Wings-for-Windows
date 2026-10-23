# Live archive probe

Build `go build -trimpath -o dist/live-archive-test.exe ./tools/live-archive-test` and run on the Windows node:

```powershell
.\live-archive-test.exe C:\WingsWindows\private\config.json https://windows.vanity.gg APPROVED-DEMO-UUID --allow-owned-fixture
```

Reads the node configuration locally without printing secrets. Creates one exclusive random demo directory and known files, including a 9 MiB random binary. Master-authenticated public HTTPS requests create tar.gz archives, then a synthetic scoped JWT downloads one over HTTPS. Checks metadata, gzip/tar structure, Unicode/empty directories/files and exact byte integrity; repeats compression to verify prior archive preservation and refuses traversal, overlap and a live conflicting writer.

Cleanup removes only known fixture files, validated returned archive leaf names and empty owned directories through `os.Root`. Unexpected content is retained and causes failure. There is no recursive fallback. This does not use a real browser/panel-issued download token, extract files on the node, test quotas or certify a real game.
