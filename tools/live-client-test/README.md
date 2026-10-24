# Real Client API and panel-issued-token probe

Build `go build -trimpath -o dist/live-client-test.exe ./tools/live-client-test`. This developer probe is limited to the established disposable Windows Native Demo UUID `58e8c7f9-4923-43b3-b431-90b2bf4edf4f` and initially running state. Requires explicit fixture/power opt-in and a Client API key with the demo's file/console/power permissions. It never requires the Application key or node master token.

To prompt without echoing or putting the key in process arguments, use a Python 3 wrapper from the repository root:

```powershell
python -c "import getpass,subprocess; r=subprocess.run(['dist/live-client-test.exe','https://panel.vanity.gg','https://windows.vanity.gg','--allow-demo-fixture-and-power'],input=getpass.getpass('Temporary Client key: ')+'\n',text=True); raise SystemExit(r.returncode)"
```

The key is read from stdin. HTTPS redirects are refused, TLS stays verified and returned signed URLs/socket addresses must match the explicit node host, secure scheme, expected path and default/443 port. Both omitted and explicit `:443` are accepted. Unknown hosts/ports, userinfo, fragments and unexpected token query fields fail. JWTs/signed URLs remain in process memory and are never printed.

Exercises actual panel Client resource stats, file create/edit/read, rename/copy, compress/move/extract and delete; real panel-issued 64 KiB binary upload/download links and their replay refusal; real panel-issued WebSocket auth/stats, Client and WebSocket commands with unique echoed markers, and Client stop/start observed over WSS. This is API automation, not browser UI rendering or an actual panel subuser-removal test. It uses the existing approved harmless executable and never creates accounts, keys, nodes or servers.

Creates an exclusive random `client-probe-*` directory with `source` and `extracted` children. Cleanup deletes only the known fixture filenames through Client API, validates that only the two empty directories remain, and reports the fixture name. It deliberately does not invoke the API's recursive directory-delete path. The operator must then remove those three **empty** directories on the node with nonrecursive deletion (for example, `.NET Directory.Delete(path, false)` after validating the exact fixture path/types), and independently verify no fixtures remain. Unexpected contents are preserved and fail cleanup. The report's passed flag does not include that separate on-node directory cleanup; the combined verification evidence does.

Power failures attempt a final start request; inspect final node state independently because an accepted start is not proof of readiness. No larger public-upload claim is made: the existing proxy body limit remains a separate gate. No actual key/token appears in this package.

Alpha 52 extends this probe with a second real panel-signed upload containing a plain TAR, extracts it through Client API, verifies the expected file contents, and includes only those known extra fixture files in cleanup. This current probe therefore requires alpha 52 or later.
