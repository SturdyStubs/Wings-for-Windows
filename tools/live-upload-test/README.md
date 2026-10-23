# Live upload probe

Build with `go build -trimpath -o dist/live-upload-test.exe ./tools/live-upload-test` and run **on the node**:

```powershell
.\live-upload-test.exe C:\WingsWindows\private\config.json https://windows.vanity.gg APPROVED-DEMO-UUID --allow-owned-fixture
```

Requires an approved disposable demo, operator access to its protected configuration and data, and a running daemon with scoped uploads. Mints synthetic user tokens locally; never outputs credentials or signed URLs. Exercises HTTPS multipart binary/empty/large uploads, replay, overwrite refusal, traversal, scope and synthetic user revocation. A separate loopback 9 MiB upload checks the retained service even if the external proxy returns 413. The report distinguishes that proxy gate from an end-to-end large upload success.

Creates an exclusive random `upload-probe-*` directory and removes only its known files and the empty directory through `os.Root`. Unexpected contents are retained and cause failure; no recursive fallback. No real panel browser session, game, account removal, or reboot is tested. Protect proxy logs that include token query strings.
