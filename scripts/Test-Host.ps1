[CmdletBinding()]
param(
    [string]$PackageDirectory = (Split-Path -Parent $PSScriptRoot),
    [string]$OutputDirectory = (Join-Path (Get-Location).Path ('host-validation-' + [guid]::NewGuid().ToString('N')))
)
$ErrorActionPreference = 'Stop'

# Executes only the packaged native test suite and harmless demo. No config,
# panel credentials, account logon, service setup or WFP operations are used.
$packagePath = (Resolve-Path -LiteralPath $PackageDirectory).Path
$manifestPath = Join-Path $packagePath 'HOST-TESTS.json'
$manifest = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
$buildInfo = Get-Content -LiteralPath (Join-Path $packagePath 'BUILD-INFO.json') -Raw | ConvertFrom-Json
if ($manifest.schema_version -ne 1 -or @($manifest.tests).Count -eq 0) {
    throw 'Invalid host-test manifest.'
}
if ($buildInfo.source_dirty -ne $false -or $manifest.source_revision -ne $buildInfo.source_revision -or $manifest.version -ne $buildInfo.version -or $manifest.source_revision -notmatch '^(?:[0-9a-f]{40}|[0-9a-f]{64})$') {
    throw 'Host validation requires a clean-source package with matching build and test-manifest revisions.'
}
$testPath = Join-Path $packagePath 'wings-windows-runtime-tests.exe'
$fixturePath = Join-Path $packagePath 'wings-windows-demo.exe'
foreach ($entry in @(
    @{Path = $testPath; Hash = $manifest.runtime_tests_sha256},
    @{Path = $fixturePath; Hash = $manifest.demo_sha256}
)) {
    $actual = (Get-FileHash -LiteralPath $entry.Path -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $entry.Hash -or $entry.Hash -notmatch '^[0-9a-f]{64}$') {
        throw ('Package hash mismatch: ' + (Split-Path -Leaf $entry.Path))
    }
}
if (Test-Path -LiteralPath $OutputDirectory) {
    throw 'OutputDirectory already exists. Choose a new directory for each validation run.'
}
$outputPath = [IO.Path]::GetFullPath($OutputDirectory)
[IO.Directory]::CreateDirectory($outputPath) | Out-Null
$started = [DateTime]::UtcNow.ToString('o')
$report = [ordered]@{
    schema_version = 1
    version = [string]$manifest.version
    source_revision = [string]$manifest.source_revision
    started_utc = $started
    finished_utc = $null
    passed = $false
    runtime_fixture_verified = $false
    panel_tested = $false
    distinct_account_tested = $false
    service_tested = $false
    network_filtering_tested = $false
    tests_passed = 0
    tests_failed = 0
    tests_skipped = 0
    expected_top_level_tests = @($manifest.tests).Count
    test_exit_code = $null
    host = $null
    identity = $null
    runtime_tests_sha256 = [string]$manifest.runtime_tests_sha256
    demo_sha256 = [string]$manifest.demo_sha256
    error = $null
}
$logPath = Join-Path $outputPath 'runtime-tests.log'
$reportPath = Join-Path $outputPath 'host-report.json'
try {
    $osInfo = Get-CimInstance -ClassName Win32_OperatingSystem
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    try {
        $principal = [Security.Principal.WindowsPrincipal]::new($identity)
        $report.identity = [ordered]@{
            sid = $identity.User.Value
            administrator_token = $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
            session_id = [Diagnostics.Process]::GetCurrentProcess().SessionId
        }
    } finally { $identity.Dispose() }
    $report.host = [ordered]@{
        caption = [string]$osInfo.Caption
        version = [string]$osInfo.Version
        build = [string]$osInfo.BuildNumber
        architecture = [string]$osInfo.OSArchitecture
        windows_server = ($osInfo.ProductType -ne 1)
        powershell = $PSVersionTable.PSVersion.ToString()
    }
    # A packaged Go test binary needs neither Go nor repository sources.
    # Merge stderr so panics/failure evidence is retained in the same log.
    $oldPreference = $ErrorActionPreference
    try {
        $ErrorActionPreference = 'Continue'
        $testOutput = @(& $testPath '-wings-fixture' $fixturePath '-test.v' '-test.count=1' '-test.timeout=90s' 2>&1 | ForEach-Object { $_.ToString() })
        $testExitCode = $LASTEXITCODE
    } finally { $ErrorActionPreference = $oldPreference }
    [IO.File]::WriteAllLines($logPath, [string[]]$testOutput, [Text.UTF8Encoding]::new($false))
    $report.test_exit_code = $testExitCode
    $topPassed = @()
    foreach ($line in $testOutput) {
        if ($line -match '^\s*--- (PASS|FAIL|SKIP): (\S+)') {
            $testName = $Matches[2]
            switch ($Matches[1]) {
                'PASS' {
                    $report.tests_passed++
                    if ($testName -notmatch '/') { $topPassed += $testName }
                }
                'FAIL' { $report.tests_failed++ }
                'SKIP' { $report.tests_skipped++ }
            }
        }
    }
    $missing = @($manifest.tests | Where-Object { $_ -notin $topPassed })
    $unexpected = @($topPassed | Where-Object { $_ -notin $manifest.tests })
    if ($testExitCode -ne 0 -or $report.tests_failed -ne 0 -or $report.tests_skipped -ne 0 -or $missing.Count -ne 0 -or $unexpected.Count -ne 0 -or $topPassed.Count -ne @($manifest.tests).Count) {
        throw 'Runtime tests failed, skipped tests, or did not cover the exact packaged test manifest. See runtime-tests.log.'
    }
    $report.runtime_fixture_verified = $true
    $report.passed = $true
} catch {
    $report.error = $_.Exception.Message
} finally {
    $report.finished_utc = [DateTime]::UtcNow.ToString('o')
    [IO.File]::WriteAllText($reportPath, ($report | ConvertTo-Json -Depth 8), [Text.UTF8Encoding]::new($false))
}
Write-Output ('Report: ' + $reportPath)
if (-not $report.passed) { throw $report.error }
Write-Output ('Runtime fixture validation passed: {0} cases/subtests, zero skipped. Panel, distinct-account, service and WFP gates remain unverified.' -f $report.tests_passed)
