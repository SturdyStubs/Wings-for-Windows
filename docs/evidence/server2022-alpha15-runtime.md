# Server 2022 runtime baseline: alpha 15

Evidence source: the operator pasted the generated `host-report.json` and successful console output from the VM. The report was reviewed here; the VM and full `runtime-tests.log` were not independently inspected. The full local account SID is omitted from this public summary.

| Report field | Observed value |
| --- | --- |
| Version | `0.1.0-alpha.15` |
| Source revision | `39ec532e9349288177458c27468945ff46aa1c10` |
| OS | Microsoft Windows Server 2022 Standard Evaluation, x64 |
| Kernel/build | `10.0.20348`, build `20348` |
| PowerShell | `5.1.20348.558` |
| Invoking context | Administrator token, session 2; account SID ends in RID 500 |
| Started/finished UTC | `2026-10-21T03:29:21.2626532Z` / `2026-10-21T03:29:23.8108836Z` |
| Overall/runtime fixture verified | Both `true` |
| Cases/subtests | 14 passed, 0 failed, 0 skipped |
| Expected top-level tests / exit code | 12 / 0 |
| Panel / distinct-account / service / network filtering tested | All `false` |

The initial parameter-free alpha 15 invocation failed before tests ran because of its PowerShell parameter default. The successful rerun supplied `-PackageDirectory "%CD%"` from the package root in cmd.exe. Alpha 16 fixes that default-resolution bug. This successful report verifies the alpha 15 runtime baseline, not execution of the corrected alpha 16 script on the VM.

The report's executable hashes match the retained clean-source alpha 15 package's HOST-TESTS manifest:

- Runtime tests: `2f3218d6621b07a78a09aacbf4badffe923154e0233a4e218499028560acb09f`
- Demo: `78a2611ced5ed3e24ca0e2915cc67876b90853c50479b2de20ec3b47aadf93aa`

The restricted-token test derives its token from the invoking account. The network cleanup-order test uses a fake lease. Consequently this result does not verify a provisioned game identity, non-administrator service, real panel/TLS workflow, WFP traffic/crash behavior, real game or sustained multi-server load. Those gates remain open. Preserve the original JSON/log with the private VM test records.
