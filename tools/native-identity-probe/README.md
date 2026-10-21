# Developer-only distinct-account probe

This is the source of the ad hoc harness used for the Server 2022 alpha 16 identity evidence. It calls the existing `identity` guards and `native.Start` directly and launches a harmless second copy of itself. It does not test the panel API, the node's full configuration/rule derivation, an actual game, SCM or real network policy. It is not included in the alpha 16 ZIP.

Build on Windows from the repository root:

```powershell
go build -trimpath -o dist/live-identity-probe.exe ./tools/native-identity-probe
```

Provision the daemon/game accounts, required logon/launch rights, protected executable directory, private fixture file, peer fixture file, writable working/`.tmp` directories and credential using the [identity guide](../../docs/IDENTITIES.md). The harness creates no accounts, grants no rights and repairs no ACLs. Use harmless fixture contents, not real secrets.

Run as the intended daemon identity in a credential-capable Windows logon context, passing one private JSON plan:

```json
{
  "SID": "S-1-5-21-REPLACE-WITH-ACTUAL-GAME-SID",
  "ServerID": "11111111-1111-4111-8111-111111111111",
  "Root": "C:\\WingsProbe\\data",
  "Peer": "C:\\WingsProbe\\peer\\other-server.txt",
  "Secret": "C:\\WingsProbe\\private\\fake-secret.txt"
}
```

The plan contains no password. The credential target is exactly `WingsWindows/<ServerID>` in the invoking daemon account's credential set. The peer/private files must exist and be unreadable by the game identity; the game must read/execute the probe but must not modify it or its immediate parent. The daemon must inspect all those paths. Root must contain `.tmp`.

```powershell
.\live-identity-probe.exe C:\WingsProbe\private\plan.json
```

Successful output requires all eight stages: real batch logon/restricted token, daemon process/token DACL guard, protected-path ACL guard, required launch grants, native separate-account creation, actual child SID/file/memory checks, stdio echo and graceful job exit. Exit 1 reports the first failure as JSON. The child writes two harmless test files in Root/`.tmp`. This is a narrow fixture check; the process security and loader/race policy remain incomplete.

The VM provisioning/handoff used for the recorded run was an ad hoc password-backed non-administrator scheduled task, not a supported account installer. Cleanup removed the credential, task, both users and their explicit rights; a loaded temporary Windows profile was retained. See [the evidence and limits](../../docs/evidence/server2022-alpha16-ssh-identity.md).
