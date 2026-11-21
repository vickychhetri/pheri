# scripts/build_all_release_assets.ps1
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

$go = "go"
if (Test-Path "$PSScriptRoot\..\.tools\go\bin\go.exe") {
    $go = (Resolve-Path "$PSScriptRoot\..\.tools\go\bin\go.exe").Path
}

$buildDir = Resolve-Path "$PSScriptRoot\..\build"
$rootDir = Resolve-Path "$PSScriptRoot\.."

Write-Host "Compiling all cross-platform targets using $go..." -ForegroundColor Cyan

# 1. Windows AMD64
Write-Host "Building Windows AMD64..." -ForegroundColor Yellow
$env:GOOS = "windows"
$env:GOARCH = "amd64"
& $go build -ldflags="-s -w" -o (Join-Path $rootDir "pheri.exe") (Join-Path $rootDir "main.go")
& $go build -ldflags="-s -w" -o (Join-Path $buildDir "pheri-windows-amd64.exe") (Join-Path $rootDir "main.go")

# 2. Windows ARM64
Write-Host "Building Windows ARM64..." -ForegroundColor Yellow
$env:GOOS = "windows"
$env:GOARCH = "arm64"
& $go build -ldflags="-s -w" -o (Join-Path $buildDir "pheri-windows-arm64.exe") (Join-Path $rootDir "main.go")

# 3. Linux AMD64
Write-Host "Building Linux AMD64..." -ForegroundColor Yellow
$env:GOOS = "linux"
$env:GOARCH = "amd64"
& $go build -ldflags="-s -w" -o (Join-Path $buildDir "pheri-linux-amd64") (Join-Path $rootDir "main.go")

# 4. Linux ARM64
Write-Host "Building Linux ARM64..." -ForegroundColor Yellow
$env:GOOS = "linux"
$env:GOARCH = "arm64"
& $go build -ldflags="-s -w" -o (Join-Path $buildDir "pheri-linux-arm64") (Join-Path $rootDir "main.go")

# Reset environment variables
$env:GOOS = ""
$env:GOARCH = ""

# 5. Packaging archives
Write-Host "Packaging zip and tar.gz archives..." -ForegroundColor Cyan
Set-Location $rootDir

# Windows AMD64 Zip
$winAmd64Zip = Join-Path $buildDir "pheri-windows-amd64.zip"
if (Test-Path $winAmd64Zip) { Remove-Item $winAmd64Zip -Force }
Compress-Archive -Path (Join-Path $buildDir "pheri-windows-amd64.exe"), (Join-Path $rootDir "README.md") -DestinationPath $winAmd64Zip -Force

# Windows ARM64 Zip
$winArm64Zip = Join-Path $buildDir "pheri-windows-arm64.zip"
if (Test-Path $winArm64Zip) { Remove-Item $winArm64Zip -Force }
Compress-Archive -Path (Join-Path $buildDir "pheri-windows-arm64.exe"), (Join-Path $rootDir "README.md") -DestinationPath $winArm64Zip -Force

# Linux AMD64 Tar.gz
$linuxAmd64Tar = Join-Path $buildDir "pheri-linux-amd64.tar.gz"
if (Test-Path $linuxAmd64Tar) { Remove-Item $linuxAmd64Tar -Force }
tar.exe -czf $linuxAmd64Tar -C $buildDir pheri-linux-amd64

# Linux ARM64 Tar.gz
$linuxArm64Tar = Join-Path $buildDir "pheri-linux-arm64.tar.gz"
if (Test-Path $linuxArm64Tar) { Remove-Item $linuxArm64Tar -Force }
tar.exe -czf $linuxArm64Tar -C $buildDir pheri-linux-arm64

Write-Host "All release assets built and packaged successfully!" -ForegroundColor Green
Get-ChildItem -Path $buildDir | Select-Object Name, Length, LastWriteTime | Format-Table -AutoSize
