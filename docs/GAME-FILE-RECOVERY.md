# Inspect and quarantine unfinished code staging

Alpha 39 adds `scripts/Resolve-ServiceGameStaging.ps1` for leftover `.stage-*` directories from [local file import](GAME-FILES.md). Run it as an administrator in **Windows PowerShell 5.1** with `WingsWindowsNative` stopped and the managed configuration/identities valid. It inspects staged bytes and permissions, then can move one explicitly selected stage into administrator-only quarantine. It does not resume copying, publish executable code, delete files or infer a server UUID from a random stage name.

## Inspect first

```powershell
Stop-Service WingsWindowsNative
$root = 'C:\WingsWindows'
$command = "$root\bin\scripts\Resolve-ServiceGameStaging.ps1"
& $command -InstallationRoot $root
# Or inspect exactly one item:
$stage = '.stage-REPLACE_WITH_32_LOWERCASE_HEX_DIGITS'
$report = & $command -InstallationRoot $root -StageName $stage
$report | Format-List
```

Inspection changes no code, configuration, ACL or quarantine directory. The report includes the exact stage name, file/entry/byte counts, count of daemon-readable entries and a SHA-256 fingerprint of names, file contents/lengths, owners/groups and DACLs. This describes unverified staging, not a valid installer or trusted game package. The original importer did not store a server binding inside the random stage name; use your reviewed source manifest and maintenance records to identify its context.

Only exact `.stage-<32 lowercase hex>` directories under managed `bin\games` are eligible. Unsupported names, links, multi-hardlink files, unexpected owners/identities, unsupported or deny ACEs and writable daemon grants are refused. SYSTEM and Administrators must retain full control; the only other supported staging grant is daemon read/execute, allowing interrupted ACL application to be inspected. A tree referenced by configured game/installer/dependency, node data/log or TLS paths is refused after path normalization. Resolve that binding explicitly before quarantine.

Bounds are 100 discovered stages, 10,000 entries, depth 16, 1,000 files and 8 GiB per selected stage. An unsafe stage makes inspection fail closed; the command does not repair arbitrary directories or ACLs. Keep writers stopped throughout maintenance. File handles block ordinary writes during each inspection, while directory/ACL checks remain bounded snapshots.

## Quarantine the selected stage

Use the fingerprint from the inspection you reviewed:

```powershell
$options = @{
    InstallationRoot = $root
    StageName = $stage
    ExpectedFingerprint = $report.fingerprint
    Quarantine = $true
}
& $command @options -WhatIf
& $command @options
```

WhatIf validates the selected stage and expected fingerprint without changing directories or permissions. Real quarantine shares the credential-provisioning lock, holds configuration against ordinary writes/replacement, rechecks service identity and rescans the stage. Changed contents or ACLs require a fresh inspection/fingerprint.

The command seals the selected tree to SYSTEM/Administrators and moves it to `private\code-quarantine\SAME-STAGE-NAME` on the same local volume. It never merges with or overwrites an existing quarantine destination. The result reports `state: quarantined`, the destination and the approved `inspected_fingerprint`; that fingerprint describes the input ACLs before sealing. Bytes are retained, but the quarantine's permissions intentionally differ. Game accounts and the daemon receive no quarantine access.

After successful quarantine, another new-UUID import and permission-preparation attempt can proceed. Inspect other blockers too; moving one stage does not resolve unrelated unfinished stages. Existing published code, server data/logs, profiles, credentials and logon rights are unchanged.

## Failure and recovery limits

If sealing or moving fails, the command retains staged bytes and any more restrictive ACLs already applied. An empty quarantine parent may remain. Inspect again before retrying; the earlier fingerprint can change after partial sealing. A crash after the directory move may leave a complete quarantine even if the caller saw no result. Replay refuses a missing source or existing destination. There is no durable operation journal, automatic retry, source rollback, automatic deletion, promotion resume or verification of a destination left by a crashed import.

These protections assume a trusted stopped node. They do not confine hostile administrators or eliminate directory, ACL, mapping or loader races. Current VM tests use a **simulated interrupted-copy tree**, the harmless demo and a loopback panel mock. They do not claim an actual crash/reboot test, real game or live panel certification. Retained quarantine removal/archive is a separate operator action.
