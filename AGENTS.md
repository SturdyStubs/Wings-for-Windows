# Working on Wings for Windows

This directory is a standalone Git repository for SturdyStubs/Wings-for-Windows. Keep commits scoped to it; the parent workspace contains unrelated panel packages.

Read ROADMAP.md, docs/PROTOCOL.md and docs/WINGS-PARITY-AUDIT.md before choosing the next batch. The audit records the alpha 1 baseline; ROADMAP.md records current status. Update the roadmap, corresponding GitHub tracking issue, changelog and verification evidence with implementation changes.

Build and test with Windows amd64 Go 1.26 or newer. Use `go test -count=1 -timeout 90s ./...`, `go vet ./...` and `scripts/Build.ps1` for a release. Native tests compile a harmless fixture executable, so `go` must be available on PATH. Keep generated binaries, node credentials, local configuration and test output out of source control.

Use upstream/panel contract fixtures for protocol changes. Mock-panel success does not establish live compatibility. Keep unsupported routes explicit and retain the private/trusted-node limitation until isolation is implemented and verified. Windows-native execution does not imply support for unchanged Linux eggs.
