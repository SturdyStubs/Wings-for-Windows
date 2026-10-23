[CmdletBinding()]
param([string]$GoExecutable = 'go')
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$oldCGO = $env:CGO_ENABLED
$oldPath = $env:PATH
Push-Location -LiteralPath $projectRoot
try {
    $env:CGO_ENABLED = '0'
    $resolvedGo = (Get-Command $GoExecutable -ErrorAction Stop).Source
    $env:PATH = (Split-Path -Parent $resolvedGo) + ';' + $env:PATH
    $toolchain = & $GoExecutable version
    if ($LASTEXITCODE -ne 0) { throw 'Go toolchain not available.' }
    if ($toolchain -notmatch 'windows/amd64') { throw 'Build and test this release with Windows amd64 Go.' }
    New-Item -ItemType Directory -Path 'dist' -Force | Out-Null
    $testOutput = @(& $GoExecutable test -json -count=1 -timeout 90s ./...)
    $testExit = $LASTEXITCODE
    [IO.File]::WriteAllLines((Join-Path $projectRoot 'dist\test-results.jsonl'), [string[]]$testOutput, [Text.UTF8Encoding]::new($false))
    $testEvents = @($testOutput | ForEach-Object { $_ | ConvertFrom-Json })
    if ($testExit -ne 0) {
        $testEvents | Where-Object Action -EQ 'output' | ForEach-Object { Write-Output $_.Output }
        throw 'Tests failed. See dist/test-results.jsonl.'
    }
    $passedTests = @($testEvents | Where-Object { $_.Action -eq 'pass' -and $_.Test }).Count
    $skippedTests = @($testEvents | Where-Object { $_.Action -eq 'skip' -and $_.Test }).Count
    Write-Output "Tests passed: $passedTests (including subtests); skipped: $skippedTests"
    & $GoExecutable vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
    $version = '0.1.0-alpha.43'
    $stage = Join-Path $projectRoot ('dist\stage-' + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $stage -Force | Out-Null
    $revision = 'unavailable'
    $dirty = $null
    if ((Test-Path -LiteralPath '.git') -and (Get-Command git -ErrorAction SilentlyContinue)) {
        $revision = [string](& git rev-parse HEAD)
        $dirty = @(& git status --porcelain).Count -gt 0
    }
    $buildInfo = [ordered]@{version=$version; toolchain=[string]$toolchain; source_revision=$revision; source_dirty=$dirty; tests_passed=$passedTests; tests_skipped=$skippedTests; verification='Local Windows fixtures and mock panel; live deployment pending'}
    [IO.File]::WriteAllText((Join-Path $stage 'BUILD-INFO.json'), ($buildInfo | ConvertTo-Json), [Text.UTF8Encoding]::new($false))
    & $GoExecutable build -trimpath -o (Join-Path $stage 'wings-windows.exe') ./cmd/wings-windows
    if ($LASTEXITCODE -ne 0) { throw 'Daemon build failed.' }
    & $GoExecutable build -trimpath -o (Join-Path $stage 'wings-windows-demo.exe') ./cmd/testserver
    if ($LASTEXITCODE -ne 0) { throw 'Demo build failed.' }
    & $GoExecutable test -c -trimpath -o (Join-Path $stage 'wings-windows-runtime-tests.exe') ./internal/native
    if ($LASTEXITCODE -ne 0) { throw 'Portable runtime-test build failed.' }
    $runtimeTestPath = Join-Path $stage 'wings-windows-runtime-tests.exe'
    $testNames = @(& $runtimeTestPath '-wings-fixture' (Join-Path $stage 'wings-windows-demo.exe') '-test.list=^Test' | Where-Object { $_ -match '^Test\w+$' })
    if ($LASTEXITCODE -ne 0 -or $testNames.Count -eq 0) { throw 'Runtime test inventory failed.' }
    $hostManifest = [ordered]@{
        schema_version = 1
        version = $version
        source_revision = $revision
        runtime_tests_sha256 = (Get-FileHash -LiteralPath $runtimeTestPath -Algorithm SHA256).Hash.ToLowerInvariant()
        demo_sha256 = (Get-FileHash -LiteralPath (Join-Path $stage 'wings-windows-demo.exe') -Algorithm SHA256).Hash.ToLowerInvariant()
        tests = $testNames
    }
    [IO.File]::WriteAllText((Join-Path $stage 'HOST-TESTS.json'), ($hostManifest | ConvertTo-Json -Depth 4), [Text.UTF8Encoding]::new($false))
    Copy-Item -LiteralPath 'README.md', 'ROADMAP.md', 'CHANGELOG.md', 'LICENSE', 'THIRD-PARTY-NOTICES.md' -Destination $stage
    Copy-Item -LiteralPath 'configs', 'docs', 'licenses' -Destination $stage -Recurse
    New-Item -ItemType Directory -Path (Join-Path $stage 'scripts') -Force | Out-Null
    Copy-Item -LiteralPath 'scripts\Install-Service.ps1' -Destination (Join-Path $stage 'scripts')
    Copy-Item -LiteralPath 'scripts\Register-EventSource.ps1' -Destination (Join-Path $stage 'scripts')
    Copy-Item -LiteralPath 'scripts\NodeSetup.psm1' -Destination (Join-Path $stage 'scripts')
    Copy-Item -LiteralPath 'scripts\GamePermissions.psm1','scripts\Set-ServiceGamePermissions.ps1' -Destination (Join-Path $stage 'scripts')
    Copy-Item -LiteralPath 'scripts\GameAccounts.psm1','scripts\New-ServiceGameAccount.ps1' -Destination (Join-Path $stage 'scripts')
    Copy-Item -LiteralPath 'scripts\GameFiles.psm1','scripts\Import-ServiceGameFiles.ps1' -Destination (Join-Path $stage 'scripts')
    Copy-Item -LiteralPath 'scripts\GameFileRecovery.psm1','scripts\Resolve-ServiceGameStaging.ps1' -Destination (Join-Path $stage 'scripts')
    Copy-Item -LiteralPath 'scripts\Update-Progress.ps1' -Destination (Join-Path $stage 'scripts')
    Copy-Item -LiteralPath 'scripts\Store-GameCredential.ps1' -Destination (Join-Path $stage 'scripts')
    Copy-Item -LiteralPath 'scripts\Store-RCONCredential.ps1','scripts\Set-ServiceRCONCredential.ps1' -Destination (Join-Path $stage 'scripts')
    Copy-Item -LiteralPath 'scripts\Set-ServiceGameCredential.ps1' -Destination (Join-Path $stage 'scripts')
    Copy-Item -LiteralPath 'scripts\CredentialTask.ps1' -Destination (Join-Path $stage 'scripts')
    Copy-Item -LiteralPath 'scripts\CredentialBridge.psm1' -Destination (Join-Path $stage 'scripts')
    Copy-Item -LiteralPath 'scripts\Test-Host.ps1' -Destination (Join-Path $stage 'scripts')
    Copy-Item -LiteralPath 'scripts\Start-NodeTunnel.ps1' -Destination (Join-Path $stage 'scripts')
    if ($dirty -eq $false) {
        # Exercise both omitted defaults through Windows PowerShell -File,
        # from outside the package, before publishing a clean-source ZIP.
        $smokeDirectory = Join-Path $projectRoot ('dist\host-runner-' + [guid]::NewGuid().ToString('N'))
        New-Item -ItemType Directory -Path $smokeDirectory -Force | Out-Null
        Push-Location -LiteralPath $smokeDirectory
        try {
            $windowsPowerShell = Join-Path $env:SystemRoot 'System32\WindowsPowerShell\v1.0\powershell.exe'
            & $windowsPowerShell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $stage 'scripts\Test-Host.ps1')
            if ($LASTEXITCODE -ne 0) { throw 'Portable host runner default-invocation regression failed.' }
            # Preserve local evidence outside the distributable package.
            $evidenceFolders = @(Get-ChildItem -LiteralPath $stage -Directory -Filter 'host-validation-*')
            if ($evidenceFolders.Count -ne 1) { throw 'Unexpected host validation evidence inventory.' }
            $sourceEvidence = [IO.Path]::GetFullPath($evidenceFolders[0].FullName)
            $targetEvidence = [IO.Path]::GetFullPath((Join-Path $smokeDirectory 'host-evidence'))
            $stagePrefix = [IO.Path]::GetFullPath($stage).TrimEnd('\') + '\'
            $smokePrefix = [IO.Path]::GetFullPath($smokeDirectory).TrimEnd('\') + '\'
            if (-not $sourceEvidence.StartsWith($stagePrefix, [StringComparison]::OrdinalIgnoreCase) -or -not $targetEvidence.StartsWith($smokePrefix, [StringComparison]::OrdinalIgnoreCase)) {
                throw 'Host evidence paths escaped the build staging directories.'
            }
            Move-Item -LiteralPath $sourceEvidence -Destination $targetEvidence
        } finally { Pop-Location }
    }
    $archive = Join-Path $projectRoot "dist\wings-windows-native-$version-windows-amd64.zip"
    Compress-Archive -Path (Join-Path $stage '*') -DestinationPath $archive -Force
    $hash = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant()
    Set-Content -LiteralPath ($archive + '.sha256') -Value ($hash + '  ' + (Split-Path -Leaf $archive)) -Encoding Ascii
    Write-Output $archive
    Write-Output "SHA256 $hash"
} finally {
    $env:CGO_ENABLED = $oldCGO
    $env:PATH = $oldPath
    Pop-Location
}
