[CmdletBinding()]
param([switch]$Initialize)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$plan = Get-Content -Raw -Encoding UTF8 -LiteralPath (Join-Path $projectRoot 'docs\progress-plan.json') | ConvertFrom-Json
if (-not (Get-Command gh -ErrorAction SilentlyContinue)) { throw 'GitHub CLI is required for updating repository progress.' }
$scratch = Join-Path $projectRoot 'dist'
New-Item -ItemType Directory -Path $scratch -Force | Out-Null
function Invoke-GitHub {
    param([string]$Path, [string]$Method = 'GET', $Payload = $null)
    $arguments = @('api', $Path, '--method', $Method)
    if ($null -ne $Payload) {
        $payloadPath = Join-Path $scratch ('progress-request-' + [guid]::NewGuid().ToString('N') + '.json')
        [IO.File]::WriteAllText($payloadPath, ($Payload | ConvertTo-Json -Depth 10), [Text.UTF8Encoding]::new($false))
        $arguments += @('--input', $payloadPath)
    }
    $raw = & gh @arguments
    if ($LASTEXITCODE -ne 0) { throw "GitHub $Method failed: $Path" }
    return (($raw -join "`n") | ConvertFrom-Json)
}
function Get-AllItems {
    param([string]$Path)
    $items = @()
    for ($page = 1; $page -le 100; $page++) {
        $batch = @(Invoke-GitHub ($Path + '&per_page=100&page=' + $page))
        $items += $batch
        if ($batch.Count -lt 100) { return $items }
    }
    throw 'Progress pagination exceeded its safety bound.'
}
$repository = 'repos/' + $plan.repository
$milestones = @(Get-AllItems ($repository + '/milestones?state=all'))
if ($Initialize) {
    $issues = @(Get-AllItems ($repository + '/issues?state=all'))
    foreach ($stage in $plan.milestones) {
        $matches = @($milestones | Where-Object title -EQ $stage.title)
        if ($matches.Count) { $milestone = $matches[0] } else {
            $milestone = Invoke-GitHub ($repository + '/milestones') 'POST' @{title=$stage.title; description=$stage.description}
            $milestones += $milestone
        }
        foreach ($task in $stage.tasks) {
            $title = $stage.key + ': ' + $task.title
            $existing = @($issues | Where-Object title -EQ $title)
            if ($existing.Count) {
                if ($null -eq $existing[0].milestone -or $existing[0].milestone.number -ne $milestone.number) {
                    Invoke-GitHub ($repository + '/issues/' + $existing[0].number) 'PATCH' @{milestone=$milestone.number} | Out-Null
                }
                continue
            }
            $body = "Deliverable for #$($stage.tracking_issue).`n`n$($task.title).`n`n$($task.notes)`n`nAcceptance: implementation, appropriate regression tests and documented limits. Live gates require actual host/panel/game evidence. See ROADMAP.md, docs/VERIFICATION.md and docs/WINGS-PARITY-AUDIT.md."
            if ($task.verified_in) { $body += "`n`nLocally implemented and regression-tested in $($task.verified_in); see CHANGELOG.md. This is not a live-deployment certification." }
            $created = Invoke-GitHub ($repository + '/issues') 'POST' @{title=$title;body=$body;milestone=$milestone.number}
            if ($task.verified_in) { Invoke-GitHub ($repository + '/issues/' + $created.number) 'PATCH' @{state='closed';state_reason='completed'} | Out-Null }
            $issues += $created
            Write-Host ($title + ' -> #' + $created.number)
        }
    }
    $milestones = @(Get-AllItems ($repository + '/milestones?state=all'))
}
$intro = '[Milestone progress bars](https://github.com/' + $plan.repository + '/milestones) update as their task issues close. The badges below refresh automatically through Shields.io; GitHub/image caches may delay updates.'
$lines = @('<!-- progress:start -->', '', '## Project progress', '', $intro, '', '| Milestone | Live issue progress | Tracking |', '| --- | --- | --- |')
foreach ($stage in $plan.milestones) {
    $matches = @($milestones | Where-Object title -EQ $stage.title)
    if (-not $matches.Count) { throw ('Missing milestone ' + $stage.title + '; run with -Initialize once.') }
    $milestone = $matches[0]
    $url = $milestone.html_url
    $badge = 'https://img.shields.io/github/milestones/progress/' + $plan.repository + '/' + $milestone.number + '?label=tasks&cacheSeconds=300'
    $lines += '| ' + $stage.title + ' | [![' + $stage.key + ' closed tasks](' + $badge + ')](' + $url + ') | [Details #' + $stage.tracking_issue + '](https://github.com/' + $plan.repository + '/issues/' + $stage.tracking_issue + ') |'
    Write-Host ('{0}: {1}/{2} tasks closed; milestone {3}' -f $stage.key,$milestone.closed_issues,($milestone.open_issues+$milestone.closed_issues),$milestone.state)
}
$lines += @('', '**Completion means all six milestones are closed after their acceptance checks, including live panel/game/service validation.** Task counts are not estimates of engineering effort or universal Linux egg compatibility. The current release remains experimental.', '', 'GitHub Issues are the source of truth; ROADMAP.md records implementation scope and verification. Run `scripts/Update-Progress.ps1` to regenerate this section. Use `-Initialize` to add missing planned milestones/tasks; rerunning it never automatically re-closes existing issues.', '<!-- progress:end -->')
$readmePath = Join-Path $projectRoot 'README.md'
$readme = Get-Content -Raw -Encoding UTF8 -LiteralPath $readmePath
$section = $lines -join "`n"
if ($readme -match '(?s)<!-- progress:start -->.*?<!-- progress:end -->') {
    $readme = [regex]::Replace($readme, '(?s)<!-- progress:start -->.*?<!-- progress:end -->', [Text.RegularExpressions.MatchEvaluator]{ param($match) $section })
} else {
    $anchor = '## What works'
    if (-not $readme.Contains($anchor)) { throw 'README progress insertion anchor missing.' }
    $readme = $readme.Replace($anchor, $section + "`n`n" + $anchor)
}
[IO.File]::WriteAllText($readmePath, $readme, [Text.UTF8Encoding]::new($false))
