# Direct Server 2022 alpha 16 and distinct-account evidence

Validation date: October 20, 2026 (America/Phoenix). The agent connected over SSH after comparing the server's host-key fingerprint with console output supplied by the operator. Full reports/logs were retrieved privately; account SIDs, SSH access details and credentials are omitted here.

## Packaged runtime baseline

| Field | Result |
| --- | --- |
| OS | Windows Server 2022 Standard Evaluation x64, build 20348 |
| PowerShell | 5.1.20348.558 |
| Package | Alpha 16, clean source `146036e69d9b20d6e054dd8b986358e8f38c4197` |
| ZIP SHA256 | `8e43c85ecd4f420db99567b38613e50ce2c9d6be6608d11dfd9b3ef51908a4af` |
| Runtime-test executable SHA256 | `2f3218d6621b07a78a09aacbf4badffe923154e0233a4e218499028560acb09f` |
| Demo SHA256 | `df9c142beeaa02e39ce201c483ab4dc15da3ad1ece8064e1460adb109fb48bb3` |
| Execution context | Administrator SSH session, Windows session 0 |
| Started/finished UTC | 2026-10-21T03:48:47.0439544Z / 2026-10-21T03:48:49.6321448Z |
| Default-path command | Passed, both script arguments omitted |
| Coverage | 12 top-level tests, 14 cases/subtests; zero failures/skips; exit 0 |

The uploaded archive checksum, source revision and executable hashes were verified. Both generated JSON and the full verbose log were retrieved and reviewed. The corrected alpha 16 default script invocation therefore has direct Server 2022 evidence, superseding the earlier operator-only alpha 15 baseline for this case. Panel, distinct-account, service and real filtering flags in the packaged baseline remain false.

## Real distinct-account native probes

A separate developer harness called the unchanged alpha 16 `identity` and `native` packages, with immutable probe code, a private fake-secret file, a peer fixture, and a writable game root/`.tmp`. [Harness source and usage](../../tools/native-identity-probe/README.md) are retained for review. The tested ad hoc binary SHA256 was `bbd80c6fdc2831d85c90f260e5fc227b70a1e3bd04b98ba9471714aa7b8da29b`; it was built from this harness source before copying it into tools. It is not a new production daemon or a panel integration test.

Two contexts passed the same eight native stages:

1. A temporary local SYSTEM scheduled task launching a newly provisioned non-administrator game identity.
2. A password-backed scheduled task running as a different newly provisioned **non-administrator daemon identity**, launching its own separate non-administrator game identity. The invoking daemon token explicitly reported `administrator=false`.

| Stage | SYSTEM daemon task | Non-administrator daemon task |
| --- | --- | --- |
| Credential lookup, batch logon, pinned SID and restricted token | Passed | Passed |
| Daemon process/primary-token DACL guard | Passed | Passed |
| Protected-file/ancestor/peer-tree DACL guard | Passed | Passed |
| Required executable and data/temp permissions | Passed | Passed |
| Production native suspended launch/Job Object assignment | Passed | Passed |
| Actual child identity and access operations | Passed | Passed |
| Stdio command/echo | Passed | Passed |
| Graceful exit, exit code zero | Passed | Passed |

The child SID matched the pinned real game account. Its writes to its own root and `.tmp` succeeded. Reads of the existing peer/private fixture files and opening daemon memory were denied by Windows. There was no real network filter or packet probe.

The temporary game accounts received batch logon. The non-administrator daemon received batch logon, `SeAssignPrimaryTokenPrivilege` and `SeIncreaseQuotaPrivilege`; no existing Administrator rights were changed. Its password was provided to Windows Task Scheduler's credential-backed registration, and the game password was delivered in memory through a named pipe restricted to the temporary daemon SID, SYSTEM and Administrators. No plaintext password was written into plan files or command arguments.

## Credential-context finding and cleanup

The first attempt to use the store helper directly in the key-authenticated Administrator SSH session failed with Win32 error 1312: no usable logon session. It created no stored credential; its temporary game account and batch right were removed. Both the SYSTEM context and password-backed non-administrator task could store, use and remove their per-account game credential. Do not treat successful SSH authentication as proof of Credential Manager readiness for the future service.

The SYSTEM probe removed its credential, task, account and batch right. For the non-administrator probe, the task reported credential removal, and subsequent direct inspection confirmed both accounts and the task absent. LSA enumeration returned error 2 (no rights entry) for both temporary SIDs after rights cleanup. No processes owned by the temporary daemon SID remained at inspection time.

**Profile cleanup is incomplete:** Windows still marked the temporary daemon profile loaded. The harness cleanup wrapper deliberately refused to delete it and exited nonzero after account/right/task cleanup. The native eight-stage report passed; that does not make overall profile teardown a success. The profile was retained rather than forcing an unload or restarting shared Windows services. Profile lifecycle/recovery remains part of the outstanding provisioning work.

## Gates still open

These were password-backed **batch tasks**, not SCM installations. They do not certify the intended service logon/profile lifecycle, reboot/recovery, the node's complete configuration/rule generation, actual panel/TLS/subuser/file workflows, real game loading, hostile link/loader races, network allocation policy, WFP traffic/crash behavior or sustained multi-server load. M2 and M6 remain open; this evidence advances specific native account/host checks only.
