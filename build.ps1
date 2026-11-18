param(
    [string]$Output = "pheri.exe"
)

# Detect Go 1.24 compiler
$go = "go"
if (Test-Path "$PSScriptRoot\.tools\go\bin\go.exe") {
    $go = "$PSScriptRoot\.tools\go\bin\go.exe"
} elseif (Test-Path "$env:USERPROFILE\go1.24\bin\go.exe") {
    $go = "$env:USERPROFILE\go1.24\bin\go.exe"
}

$tmpDir = Join-Path $PSScriptRoot ".tools\tmp"
if (-not (Test-Path $tmpDir)) {
    New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null
}
$env:GOTMPDIR = $tmpDir

Write-Host "Building Pheri with Smart App Control compliant flags (-ldflags='-s -w')..." -ForegroundColor Cyan
& $go build -ldflags="-s -w" -o $Output main.go

if ($LASTEXITCODE -eq 0) {
    Copy-Item $Output (Join-Path $PSScriptRoot "build\pheri-windows-amd64.exe") -Force
    Write-Host "[SUCCESS] Successfully compiled $Output and updated build/pheri-windows-amd64.exe" -ForegroundColor Green
    Write-Host "          Run with: .\$Output" -ForegroundColor Yellow
} else {
    Write-Host "[ERROR] Build failed with exit code $LASTEXITCODE" -ForegroundColor Red
}
