# Live extraction probe

Build `go build -trimpath -o dist/live-extract-test.exe ./tools/live-extract-test`, then run **on the node**:

```powershell
.\live-extract-test.exe C:\WingsWindows\private\config.json https://windows.vanity.gg APPROVED-DEMO-UUID --allow-owned-fixture
```

Reads protected node credentials locally without displaying them. Creates an exclusive random fixture, a 9 MiB random Unicode-named source file, empty entries and a sentinel. Uses public authenticated HTTPS to create an archive and extract it into a fresh destination; checks exact hashes, repeat preservation, locked source refusal, traversal/link/checksum rejection before publication and retained source/sentinel bytes. Invalid traversal is bounded to another location inside the same owned fixture if a regression were present; it never intentionally selects unrelated user files.

Cleanup removes only known owned files, a validated returned archive filename and empty fixture directories through `os.Root`. Unexpected contents remain and cause failure, with no recursive fallback. No actual panel browser action, game, reboot, quota or durable recovery certification is implied.

Alpha 50 extends this probe with public HTTPS ZIP extraction of a SHA256-verified 9 MiB Unicode binary, repeat collision preservation and traversal refusal before publication. The same opt-in and exact owned cleanup rules apply. Browser-issued actions and real game installation remain unverified.
