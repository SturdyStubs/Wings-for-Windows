# Server 2022 alpha 43 operator-managed configuration policy

Recorded October 23, 2026 (America/Phoenix). This release implements explicit acknowledgement of operator-managed configuration; it does not remotely apply panel settings or rotate credentials.

| Field | Result |
| --- | --- |
| Clean package source | `99b17e4465fdbd0da4ec9b048c793bf10c6cc162` |
| ZIP SHA256 | `903449cbd2f6f6eb7218246b0c6bf4414a4c66f209209d5b2be3ba4623785a45` |
| Deployed daemon SHA256 | `bc5f448e66339771adde2bbd371032398c192d3f0f4ae02ca757ef8807a8e895` |
| Local suite | 945 cases/subtests; zero skipped; vet and clean release build pass |
| Server 2022 packaged native suite | 14 cases/subtests; zero failed/skipped |
| Server 2022 portable configuration API suite | 14 cases/subtests; zero failed/skipped |
| Host | Microsoft Windows Server 2022 Standard Evaluation, build 20348; PowerShell 5.1.20348.558 |
| Native baseline UTC | 2026-10-23T16:07:24.3303542Z through 2026-10-23T16:07:26.9007743Z |
| Live deployment UTC | 2026-10-23T16:08:19.7150607Z |

[Sanitized results](server2022-alpha43-config-policy-results.json) bind the source/package, runtime/demo/API executable hashes and contract fixture. The clean package was transferred through fingerprint-pinned SSH, checked before extraction and exercised through its default Test-Host invocation. The portable developer API executable and sidecar are separate from the runtime ZIP.

## Contract and regression checks

The independently sourced fixture follows upstream `POST /api/update` and its explicit ignore-panel-configuration branch. Tests verify an exact `applied: false` response, unchanged local configuration and disk sentinel, unchanged outgoing credentials, original incoming token validity, refusal of the requested replacement token, no credential echo, absent/false mode refusal, missing/null/array/scalar/malformed/trailing/oversized bodies, bearer rejection and unsupported method handling. All 945 local tests rerun the existing file/runtime regressions; the focused portable suite runs 14 policy cases/subtests on Server 2022.

## Live upgrade and request

The existing demo was confirmed running and stopped gracefully before stopping the service. The old executable, build metadata and secret configuration were backed up in the operator-only directory. The new clean-package daemon executable was copied in place, its SHA256 verified and its exact executable SDDL compared unchanged. Build metadata was updated. This was a **runtime-only upgrade**, not replacement of every deployed package file. The existing dedicated service/game accounts, approved profile, data, protected transport and provisioning scripts were retained. The new local policy flag was enabled by the operator-context deployment script; the service does not need config write privileges.

After restart, the demo returned to running. An authenticated request over public HTTPS containing replacement credentials, a public API port and a Linux data directory returned **200 with applied false**. The native config hash was identical before and after that request. A subsequent authenticated system request reports alpha 43 and the operator-managed capability using the original token. A direct server sync returned **204** against the real panel with the original outgoing credentials. Service and demo remain running; no request credentials or local secret hashes are published.

## Limits

The panel Save Changes button has **not** been retried by the operator yet; no disappearance of its warning is claimed. Source review shows that the reviewed panel client accepts a successful HTTP response, but exact UI behavior needs confirmation. Acknowledgement leaves every panel field unapplied, including master-key changes: manual coordinated native credential changes remain required. This is upstream's opt-in operator-owned policy, not runtime configuration mutation or token-rotation parity.

The existing 35-check mock SCM suite, earlier 98-case portable file suite, browser file controls, subuser revocation, real games and reboot/long-outage recovery were not rerun in this batch. The managed service is an actual retained deployment, not a cleaned-up fixture. Earlier [live tunnel/files/permission evidence](server2022-alpha42-live-operations.md) remains version-specific. HTTP inventory is **20/38 with partial behavior**; milestones remain open. See [operator scope](../CONFIGURATION-UPDATES.md).
