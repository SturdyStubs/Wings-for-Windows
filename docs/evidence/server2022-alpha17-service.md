# Server 2022 alpha 17 service provisioning evidence

Validation date: October 20, 2026 (America/Phoenix); baseline UTC start/finish: 2026-10-21T04:43:05.6376295Z / 2026-10-21T04:43:08.0766600Z. The agent used fingerprint-verified SSH and retrieved the JSON reports/full native log. No credentials or full host/user SIDs are published.

| Field | Result |
| --- | --- |
| Host | Windows Server 2022 Standard Evaluation x64, build 20348 |
| PowerShell | 5.1.20348.558 |
| Clean package source | `4db3aa4a596fdba2ea7cdcba286f5ce36068486a` |
| ZIP SHA256 | `04f82f76813c5291d18cbd1f3d542e4d21de7172c9ff4c5e2e97b6e008b78a6f` |
| Runtime-test SHA256 | `2f3218d6621b07a78a09aacbf4badffe923154e0233a4e218499028560acb09f` |
| Demo SHA256 | `48cb4713fc9116ceca3dd37453c665448ace7b915ca7429318cda0f99148583a` |
| Development suite | 206 cases/subtests, zero skipped; vet and clean-source build passed |
| VM packaged native baseline | Both arguments omitted; 14 cases/subtests, zero failed/skipped; 12 top-level tests |
| VM service checks | 11 passed; sanitized [machine-readable results](server2022-alpha17-scm-results.json) |

The ZIP checksum was checked before/after transfer. Uploaded service installer/module bytes exactly match those in the release ZIP. The Administrator context performs host setup only; SCM runs the daemon as a new non-administrator local account, which launches a separate non-administrator game account. All node tokens/passwords are disposable and generated in VM memory.

The [developer fixture](../../tools/service-provisioning-test/README.md) retains the check bodies and a loopback mock panel for review and repeat testing. It is separate from supported operator tooling.

## Service checks

1. Inject service-registration failure after real rights/ACL setup. Verify the new account, game batch right and registration are absent and every preexisting path's DACL equals its original snapshot.
2. Inject startup failure after real SCM registration with `-Start`. Verify service deletion and the same account/rights/full-DACL rollback.
3. Install successfully with `-CreateLocalAccount -ConfigurePermissions`; verify daemon service/process-launch rights and game batch-logon right.
4. Seed the exact game credential as the daemon via a temporary password-backed task and SID-restricted in-memory pipe. No plaintext credential file/command-line argument is used. This deliberately initializes a profile; it does not establish cold-profile behavior.
5. Start the actual SCM service and poll its authenticated loopback API using a local mock panel.
6. Query the service PID's actual owner SID and compare it with the dedicated daemon account.
7. Start the demo through the real node power API as the separate game account.
8. Submit a console command through the real API and verify its echo in the node log response.
9. Query both demo root/child process owner SIDs and compare them with the game identity.
10. Stop the SCM service and verify both game processes terminate.
11. Restart the service, observe offline state, then start/stop the demo again through its API.

## Cleanup and independent audit

The credential was removed in the same daemon identity context. Temporary bootstrap tasks, SCM registration, mock process, daemon/game users and their explicit rights were removed. A subsequent independent read-only audit confirmed zero matching users, processes, tasks, service registrations and explicit rights.

One temporary daemon profile remains marked loaded by Windows even after all owned processes exit. It was deliberately retained; no forced profile deletion, shared-service restart or host reboot was attempted. Disposable files and reports remain on the test VM. SSH access remains for authorized ongoing testing.

## Scope and remaining gates

This establishes an actual non-administrator SCM demo workflow with an initialized password-backed credential profile and the node's normal configuration/rule-generation/API path. It supersedes the SCM gap of the earlier batch-task probe for these specific checks.

It does not validate a real panel or trusted TLS, subuser revocation, real game loading, host reboot/cold credential-profile availability, profile teardown/recovery, service logging, password rotation, sustained load, WFP traffic or hostile filesystem/loader races. Networking is unmanaged. Account-specific deny policy and domain/group policy still require operator review. The public installer does not automate the credential-bootstrap task used by this developer test. M2 and M6 remain open.
