# Live signed-download probe

Developer tool for an operator-approved disposable demo on a trusted Windows node. It reads the native config locally, creates an exclusive random download-probe directory under the specified approved server and tests public trusted HTTPS using synthetic short-lived JWTs. No real panel user is created or modified, and no node token/JWT/active signed URL is printed.

```powershell
go build -trimpath -o live-download-test.exe ./tools/live-download-test
.\live-download-test.exe C:\WingsWindows\private\config.json https://YOUR_NODE_HOST YOUR_APPROVED_DEMO_UUID --allow-owned-fixture
```

Build on the developer machine and transfer the executable securely if Go is absent on the VM. The explicit flag authorizes three test files in a newly created random directory and temporary daemon-memory revocation of its synthetic user. Run only against an operator-controlled HTTPS endpoint and trusted managed data root. The host/path/config arguments are operator inputs, not automatic authorization discovery.

Checks include 9 MiB binary hash/length/attachment integrity, one use, same-jti/new-unique links, ignored query path overrides, Range refusal without token consumption, wrong scope, empty file, case-normalized synthetic-user revocation and a fresh post-revocation link. Cleanup removes only the three exact known test files and their exact exclusive directory through os.Root; no recursive fallback removes unknown children. Failure emits sanitized partial checks and nonzero status; inspect any retained owned fixture rather than deleting unrelated data.

This does not request a URL from an actual panel user session, test browser downloads, remove a real subuser, certify isolation or test concurrent/slow/maximum-size transfers. Synthetic revocation entries remain in memory until daemon restart. Proxy/access logs can independently record credential-bearing query strings even though the tool never prints them; protect/redact those logs. The runtime endpoint has its own comprehensive scoped/streaming tests.
