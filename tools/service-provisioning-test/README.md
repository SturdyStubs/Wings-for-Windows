# Developer-only service provisioning fixture

Run only on a disposable, checkpointed Windows Server VM as Administrator. This is privileged test infrastructure, not a production setup/bootstrap tool. It deliberately creates temporary local users, grants their rights, replaces DACLs in a fresh owned fixture tree and registers the real `WingsWindowsNative` service. It refuses an existing service or matching account/root. Never point it at real node configuration or a populated installation.

The loopback mock panel serves only the fixture server; the packaged daemon/demo perform the real SCM and separate-account lifecycle. The bootstrap worker uses a temporary password-backed task and SID-restricted named pipe; no plaintext password file/CLI argument is used. Temporary daemon batch rights exist only for this test bootstrap.

Build the mock in this repository with Windows amd64 Go:

```powershell
go build -o C:\FixtureStaging\mock-panel.exe ./tools/service-provisioning-test
```

Put `mock-panel.exe`, `bootstrap.ps1`, and `Test-ServiceProvisioning.ps1` in a new staging directory. Copy the tested package's `scripts\Install-Service.ps1` and `scripts\NodeSetup.psm1` into that same staging directory. Keep the verified, extracted release separate. Example elevated invocation (use a fresh random suffix/UUID each time):

```powershell
$suffix = ([guid]::NewGuid().ToString('N')).Substring(0,8)
C:\FixtureStaging\Test-ServiceProvisioning.ps1 `
  -Staging C:\FixtureStaging `
  -Package C:\ExtractedRelease `
  -Root ("C:\WingsWindowsTesting\scm-"+$suffix) `
  -GameName ('WwScmG_'+$suffix) -DaemonName ('WwScmD_'+$suffix) `
  -ServerID ([guid]::NewGuid().ToString())
```

Inspect the returned JSON and `<Root>\scm-report.json`: require `passed=true`, all eleven checks passed, and every cleanup flag true. The script reports failures in JSON; do not interpret PowerShell process exit 0 alone as success. Independently audit temporary users/rights/service/tasks/processes after execution. On a residual service it retains account/rights state for inspection. Cleanup is best effort and not crash-safe; no force-unload/profile deletion is attempted. Retained files/profile state require separate review after stopping all owned processes.

The executed alpha 17 fixture checks are documented in [VM evidence](../../docs/evidence/server2022-alpha17-service.md). This retained harness adds root/account-name guards and preserves accounts/rights if the service cannot be removed; those cleanup-failure branches are not claimed as live verified. The eleven successful check bodies match the executed fixture. The tool is not included as a supported operator utility in the release package. Real panel/TLS/game, reboot/cold-profile recovery, WFP and hostile isolation remain separate gates.
