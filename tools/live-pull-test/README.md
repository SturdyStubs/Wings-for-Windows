# Live guarded remote-pull probe

Build `go build -trimpath -o dist/live-pull-test.exe ./tools/live-pull-test` on Windows, then run on the disposable node:

```powershell
.\live-pull-test.exe C:\WingsWindows\private\config.json https://windows.vanity.gg APPROVED-DEMO-UUID --allow-owned-fixture
```

Requires `allow_remote_downloads: true` and reads protected credentials only locally. Creates an exclusive random directory with a 9 MiB random file and a sparse 100 MiB cancellation source. Five-minute synthetic signed download URLs provide controlled public HTTPS sources through the existing node proxy. Foreground and background pulls verify hashes, progress/list shape, collision preservation, private/HTTP source refusal, early cancellation and cleanup. This exercises real public DNS/TLS and the non-administrator daemon; it does not test a browser or third-party game installation.

No token, source URL or credentials are output. Proxy access logs can independently contain query tokens. Cleanup removes only known fixture files and the empty owned directory through `os.Root`; bounded retries permit a cancelled source's read handle to close. Unexpected contents remain and fail cleanup. No recursive fallback or private-network allowlist is used.
