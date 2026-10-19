# Game credential lifecycle

These tools manage one exact `WingsWindows/<full-server-UUID>` generic credential in the **current Windows account's** credential set. Run them as the daemon's eventual Windows identity. They do not enumerate other credentials, contact the panel, log on a game account, launch/stop games, delete users or change rights/ACLs.

## Create and inspect

```powershell
$game = Get-Credential -UserName '.\WingsGameRust'
.\scripts\Store-GameCredential.ps1 -ServerUUID 11111111-1111-4111-8111-111111111111 -Credential $game
.\wings-windows.exe --credential-status 11111111-1111-4111-8111-111111111111
```

Status emits JSON containing `target`, `present` and, for a present entry, `username`, `last_written` in UTC and `persistence`. It does not decode or output the password. Windows returns a credential memory block including its blob; the tool clears that returned copy before freeing it. Presence is not proof that the password, pinned SID, account policy, service privileges or launch configuration works.

Uppercase UUID input normalizes to lowercase. Invalid UUIDs, wildcard/prefix targets, mixed CLI operations and positional arguments fail before management operations. These commands need no node configuration file. Missing status returns `present: false` with exit status zero; credential-store API errors return nonzero.

## Rotate deliberately

```powershell
$updated = Get-Credential -UserName '.\WingsGameRust'
.\scripts\Store-GameCredential.ps1 -ServerUUID 11111111-1111-4111-8111-111111111111 -Credential $updated -Replace
```

The store helper refuses to overwrite an existing target unless `-Replace` is supplied. Rotation requires an existing target and the same account name, compared without case sensitivity. It refuses a missing target or a different account name. To replace the identity itself, retire the old target explicitly, update the profile's pinned SID and provision the new account/credential through the host's administrative procedure.

Rotation updates the stored credential only. It does **not** reset the Windows account password. Coordinate the actual account password change and credential update during planned maintenance, then validate a real start. Existing processes retain their current token; the next start reads the new credential. Failed logon does not fall back to the daemon account.

The helper checks before writing; it is not a cross-process compare-and-swap transaction. Use one provisioning writer per target. A check/write race or interrupted password reset can leave a mismatch. Inspect metadata and repeat the intended operation explicitly; no automatic account-password rollback or durable rotation journal exists. SecureString/unmanaged and runtime buffers are cleared on return, but administrator inspection and process dumps remain outside these tools' guarantees.

## Retire a credential

Stop/pause the server's scheduled or manual starts through your operational workflow before retiring its identity. Remove/update its profile and restart the daemon as appropriate. Then, as the account that owns the credential:

```powershell
.\wings-windows.exe --remove-credential 11111111-1111-4111-8111-111111111111
.\wings-windows.exe --credential-status 11111111-1111-4111-8111-111111111111
```

Removal emits JSON with the exact `target` and `removed: true`. Repeating removal succeeds with `removed: false`. No wildcard, prefix or unrelated credential can be selected. Removal does not terminate an already running game, cancel an in-flight launch that already copied the credential, disable/delete the account or revoke tokens. It prevents later lookups from succeeding until the target is recreated.

Account deletion, service logon rights, filesystem ownership/ACL cleanup, network rules and stored profile references need separate administrative cleanup. Do not delete a Windows account while its game processes or owned data still require migration. Automatic account lifecycle/recovery remains part of M2's open work.

## Verification scope

Local tests use unique temporary fake generic credentials and never call account logon. They cover creation, metadata without secret fields, rejected accidental overwrite, rejected identity change, explicit same-account rotation, removal/repeated removal, missing status and target validation. Packaged CLI checks also verify mutually exclusive operations and exact-target behavior.

These checks do not validate a real Windows Server service account, account password change or running-game token revocation. See [identity setup](IDENTITIES.md) and [verification gates](VERIFICATION.md). Native APIs are documented in [CredWrite](https://learn.microsoft.com/en-us/windows/win32/api/wincred/nf-wincred-credwritew) and [CredDelete](https://learn.microsoft.com/en-us/windows/win32/api/wincred/nf-wincred-creddeletew).
