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
    & $GoExecutable test -count=1 -timeout 90s ./...
    if ($LASTEXITCODE -ne 0) { throw 'Tests failed.' }
    & $GoExecutable vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
    $version = '0.1.0-alpha.1'
    $stage = Join-Path $projectRoot ('dist\stage-' + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $stage -Force | Out-Null
    & $GoExecutable build -trimpath -o (Join-Path $stage 'wings-windows.exe') ./cmd/wings-windows
    if ($LASTEXITCODE -ne 0) { throw 'Daemon build failed.' }
    & $GoExecutable build -trimpath -o (Join-Path $stage 'wings-windows-demo.exe') ./cmd/testserver
    if ($LASTEXITCODE -ne 0) { throw 'Demo build failed.' }
    Copy-Item -LiteralPath 'README.md', 'LICENSE', 'THIRD-PARTY-NOTICES.md' -Destination $stage
    Copy-Item -LiteralPath 'configs', 'docs', 'licenses' -Destination $stage -Recurse
    New-Item -ItemType Directory -Path (Join-Path $stage 'scripts') -Force | Out-Null
    Copy-Item -LiteralPath 'scripts\Install-Service.ps1' -Destination (Join-Path $stage 'scripts')
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
