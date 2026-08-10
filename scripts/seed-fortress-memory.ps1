# Seeds fortress/memory/ from fortress/memory-templates/ on a fresh clone.
#
# COPY-IF-ABSENT ONLY. An existing memory file is the playing AI's own
# working memory and is never overwritten, even if it is empty. Safe to
# re-run at any time.

$ErrorActionPreference = 'Stop'

$repoRoot = Split-Path -Parent $PSScriptRoot
$templateDir = Join-Path $repoRoot 'fortress/memory-templates'
$memoryDir = Join-Path $repoRoot 'fortress/memory'

if (-not (Test-Path $templateDir)) {
    throw "Template directory not found: $templateDir"
}

if (-not (Test-Path $memoryDir)) {
    New-Item -ItemType Directory -Path $memoryDir | Out-Null
    Write-Host "Created $memoryDir"
}

$seeded = 0
$kept = 0

foreach ($template in Get-ChildItem -Path $templateDir -Filter '*.md') {
    $target = Join-Path $memoryDir $template.Name
    if (Test-Path $target) {
        Write-Host "kept    $($template.Name) (already exists - not touched)"
        $kept++
    } else {
        Copy-Item -Path $template.FullName -Destination $target
        Write-Host "seeded  $($template.Name)"
        $seeded++
    }
}

Write-Host ""
Write-Host "Done. $seeded seeded, $kept left alone."
