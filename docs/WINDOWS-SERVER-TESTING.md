# Windows Server VM validation

Start with a fresh disposable Windows Server 2022 VM and a VM checkpoint. The package's first baseline needs no Go, Docker, panel configuration, game accounts or service installation. It launches harmless demo processes under the account invoking the script. Later phases deliberately exercise credentials, service setup and network policy separately.

## 1. Copy and verify the package

Build from a clean checkout on the development machine using `scripts/Build.ps1`, then copy the resulting ZIP and `.sha256` file into the VM. Use the package and checksum from that trusted build. A checksum detects accidental mismatch; a co-located checksum is not a publisher signature.

In Windows PowerShell inside the VM, use the actual downloaded ZIP path:

```powershell
$archive = 'C:\Users\Administrator\Downloads\wings-windows-native-0.1.0-alpha.15-windows-amd64.zip'
$expected = ((Get-Content -LiteralPath ($archive + '.sha256') -Raw).Trim() -split '\s+')[0]
$actual = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant()
if ($actual -ne $expected) { throw 'Archive checksum mismatch.' }
Expand-Archive -LiteralPath $archive -DestinationPath C:\WingsWindows
Set-Location C:\WingsWindows
```

Extract into a new directory. The package contains `BUILD-INFO.json`, `HOST-TESTS.json`, the daemon, the demo, compiled runtime tests and scripts/docs. The host runner refuses dirty-source packages or mismatched build/test-manifest revisions. Self-built validation packages therefore need a committed, clean checkout.

## 2. Run the portable baseline

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\Test-Host.ps1 -OutputDirectory C:\WingsEvidence\baseline
```

The execution-policy option applies to this PowerShell process; it changes no machine-wide policy. Run the verified package's local script. Administrator elevation is not requested by the runner. Record the invoking identity; an interactive administrator result does not establish behavior under the eventual daemon service account.

The script checks both executable hashes, then runs the complete compiled `internal/native` suite using the bundled demo. It requires successful exit, every exact top-level manifest test, and zero failed/skipped tests. No Go executable, source checkout or network connection is needed. Use a new output directory each time; existing evidence is never overwritten.

The baseline currently covers 14 cases/subtests: native process creation with Job Object assignment, console/Unicode/quoting, graceful and forced stop, descendant cleanup, bounded blocked console input, actual job-limit queries/updates, cancelled launch, pinned image startup and a restricted-token child derived from the invoking identity. The network cleanup-order test uses a fake lease; it installs no filters. Invalid-identity testing fails before real credential lookup/logon.

The suite creates/deletes temporary fixture files and starts/kills harmless processes. It does not read node configuration, contact a panel, create accounts, change host ACLs/logon rights, register a service or install WFP rules. These are runtime fixture tests, not a real game-server workflow or hostile isolation audit.

## 3. Save and interpret evidence

The output directory contains:

- `host-report.json`: version/source revision, OS edition/build/architecture, invoking SID/admin-token/session metadata, binary hashes, UTC run times, exact coverage counts and exit/failure details.
- `runtime-tests.log`: complete verbose test output, including failure diagnostics, once the runtime suite starts.

Package/revision/hash/output-directory rejection occurs before any test execution and can fail before a report is created. Later metadata/test failures produce `passed: false`; do not treat a report's existence as success. The script exits nonzero on failure. Preserve both files and the package's BUILD-INFO/HOST-TESTS manifests with the test record. Reports/logs can reveal local SIDs and filesystem paths; they contain no configured node secrets because this workflow reads no node config.

On success, `runtime_fixture_verified` becomes true. `panel_tested`, `distinct_account_tested`, `service_tested` and `network_filtering_tested` stay false. The `host.windows_server` field records the actual OS classification; the same suite also runs on a client Windows workstation and never converts that result into Server certification.

## 4. Validate distinct game identities

After the baseline passes, checkpoint the VM again. Follow [identity provisioning](IDENTITIES.md) to create a dedicated daemon identity and distinct non-administrator game identity, grant the intended logon rights, protect binaries/secrets and provision writable data/`.tmp` directories. These actions remain manual; the baseline script does none of them.

Store the credential as the eventual daemon identity and run its [identity diagnostic](IDENTITY-DIAGNOSTICS.md). Save the ten-stage JSON report and first failure. Passing requires the configured SID, batch logon, daemon guards, forbidden filesystem grants, [required launch permissions](LAUNCH-ACCESS.md) and conservative service privileges. This still launches no game. Do not publish a filled node configuration or credentials as evidence.

## 5. Validate panel, service and network separately

Follow [setup](SETUP.md) with the bundled demo first. Record panel version, trusted TLS connectivity, installation acknowledgement, power/console/subuser/file workflows and the actual child identity. Then register/test the intended service account, SCM stop/restart, VM reboot and certificate lifecycle. A baseline run in an interactive session does not test SCM behavior.

For configured `block_connections`, stop workloads using the dedicated account and deliberately run the [WFP installation/cleanup diagnostic](NETWORK-DIAGNOSTICS.md). Unlike the baseline it temporarily changes WFP state. A successful object report is separate from actual IPv4/IPv6 TCP/UDP/loopback traffic, unrelated-account preservation and crash/restart validation. Do not infer those results from the fake-lease runtime test.

## Current evidence matrix

| Gate | Recorded status |
| --- | --- |
| Windows 11 x64 build 26200, PowerShell 5.1 portable native fixture run | Local pass: 14 cases/subtests, zero skipped; Go absent from PATH. |
| Windows Server 2022 VM native baseline | Pending VM execution. |
| Distinct-account Windows Server logon/launch and ACL behavior | Pending. |
| Windows Server SCM/reboot and trusted-TLS panel workflows | Pending. |
| Real WFP installation, traffic and crash behavior | Pending. |
| Real game's complete panel lifecycle and multi-server load | Pending. |

Attach host reports/results to the relevant [M2 matrix issue](https://github.com/SturdyStubs/Wings-for-Windows/issues/21) and [deployment gate](https://github.com/SturdyStubs/Wings-for-Windows/issues/41). Close only the exercised acceptance gates after reviewing the evidence; a baseline pass alone completes neither milestone.
