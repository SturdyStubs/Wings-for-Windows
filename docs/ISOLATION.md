# Windows isolation design

Status: architecture decision for M2; implementation and security validation remain open. Alpha 6 adds an opt-in identity/token launch path; alpha 7 adds handle-based ancestor/recursive ACL auditing and observed link rejection; legacy profiles still run as the daemon account. Alpha 8 adds daemon process/primary-token and declared code/dependency DACL guards. Alpha 11 adds an optional SID-scoped WFP connection-blocking path whose live installation/traffic behavior is unverified. Full policy enforcement and separate-account deployment validation remain open.

## Decision

Continue the native EXE backend for trusted Windows game workloads. The next hardening stage will use a distinct non-administrator identity per server, explicit filesystem ACLs and a constrained primary token. Job Objects remain responsible for process trees and resource controls. This is a planned reduction in privileges and accidental cross-server access; it does not establish a hostile multi-tenant sandbox.

For mutually untrusted hosting, require a separate virtualization-backed execution backend and its own deployment verification. Hyper-V isolated Windows containers or per-tenant VMs are candidates; neither is implemented or selected for release yet. Microsoft describes Hyper-V isolation as a separate optimized VM/kernel, whereas process isolation shares the host kernel. See [isolation modes](https://learn.microsoft.com/en-us/virtualization/windowscontainers/manage-containers/hyperv-container) and [Windows security boundaries](https://www.microsoft.com/en-us/msrc/windows-security-servicing-criteria).

Native identities preserve direct Windows executable compatibility and avoid imposing container image provisioning on the current backend. A restricted token can remove privileges and restrict access to securable objects; it is only one part of that design. See [restricted tokens](https://learn.microsoft.com/en-us/windows/win32/secauthz/restricted-tokens). The experimental `run_as` profile field selects a pinned local account and credential reference. See [identity setup and limits](IDENTITIES.md). It does not establish the complete isolation model.

## Required native implementation

- Keep node credentials, TLS keys, daemon binaries and installer recipes outside all game identities' readable/writable ACLs. Protect the daemon account and its process handles from game accounts.
- Assign one non-administrator identity to each server. Separate credential storage and rotation from panel-controlled variables, logs and child environments. Do not grant interactive or remote logon as part of provisioning.
- Define executable/dependency read access, server-root write access and explicit shared/read-only mounts. Reject ambiguous link, hardlink and reparse-point access before claiming confinement. File editor checks alone do not constrain executable code.
- Launch suspended with the selected primary token, restricted inherited handles and Job Object assignment before resume; fail closed if identity/token/job setup fails. Define ownership and cleanup of user profiles and temporary files.
- Define allocation/firewall and outbound-network rules independently of file ACLs. Binding addresses in arguments is not network isolation. Define game port conflicts and shared-host policy.
- Provide audit records and idempotent account/ACL cleanup across failed installation, delete, daemon restart and upgrades.

## Acceptance gates

M2 remains open until separate credentials/policies and adversarial tests exist. Test attempts to read node secrets, open daemon process handles, access another server, traverse links, exploit case/device aliases, modify shared executables and race filesystem operations. Test console/file permissions and allocation enforcement under actual game identities.

Verify supported host/service configurations on disposable Windows Server installations and document the threat model and residual access. Administrative host compromise, kernel vulnerabilities and game-specific networking are not solved by a token setting. Strong multi-tenant claims require the virtualization backend, isolation tests and deployment review as separate acceptance gates.

## Alternatives

| Model | Decision |
| --- | --- |
| Shared daemon/game identity | Default for profiles without `run_as`; trusted code and users only. |
| Per-server identity, restricted token, ACLs | Initial opt-in launch and ACL rejection guard in alpha 6. Successful separate-account deployment and complete policy/testing gates remain open. |
| Process-isolated Windows containers | Not selected as the hostile multi-tenant boundary. Adds an image/runtime/host compatibility matrix. |
| Hyper-V isolated containers or VMs | Required direction for stronger separation; backend selection and implementation remain open. |
| Linux VM with upstream Wings | Separate option for unchanged Linux eggs; native profiles do not translate Linux binaries/images. |
