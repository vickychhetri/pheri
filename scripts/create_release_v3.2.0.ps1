param(
    [string]$TagName = "v3.2.0",
    [string]$ReleaseName = "Pheri v3.2.0 - Explicit Transaction Controls, Resilient Connection Architecture & Security Engine",
    [string]$Repo = "vickychhetri/pheri"
)

# 1. Obtain GitHub token from Git Credential Manager
$cred = "protocol=https`nhost=github.com" | git credential fill
$token = ($cred | Where-Object { $_ -match '^password=(.+)$' } | ForEach-Object { $Matches[1] })

if (-not $token) {
    Write-Error "Failed to retrieve GitHub authentication token from Git Credential Manager."
    exit 1
}

$headers = @{
    "Authorization" = "Bearer $token"
    "Accept"        = "application/vnd.github+json"
    "User-Agent"    = "PowerShell-Pheri-Release-Publisher"
}

# 2. Release Notes Body
$releaseBody = @'
# Pheri v3.2.0

Pheri v3.2.0 delivers an enterprise-grade upgrade across database transaction controls, connection pool resiliency, encrypted credential storage, and developer ergonomics.

---

## Highlights in v3.2.0

### Explicit Transaction Controls & Review Audit Trail (PHERI-08 & PHERI-09)
- **Auto-Commit Mode Toggle (`F2` / `:autocommit`)**: Switch smoothly between Auto-Commit ON and manual transaction isolation mode.
- **Dedicated Toolbar Action Buttons**: Interactive `[ Run ]`, `[ Commit ]`, and `[ Rollback ]` toolbar buttons with dynamic mutation counter badges (`Commit (N)` / `Rollback (N)`).
- **Fast Transaction Shortcuts**: Press `F10` to commit or `Shift+F10` / `Alt+Backspace` to roll back pending mutations.
- **Transaction Review Modal (`:review` / `:tx`)**: Comprehensive audit trail displaying every pending statement, invocation timestamp, and source (`SQL Editor` / `Grid Cell Edit`).
- **Data Protection Safeguards**: Prompts the user with `Commit & Proceed`, `Rollback & Discard`, or `Cancel (Esc)` modals when exiting or toggling modes with uncommitted mutations.
- **Auto-Refresh on Rollback**: Automatically re-queries and refreshes the data grid upon rollback to display restored database rows.
- **Zero-Deadlock Event Loop Architecture**: Resolved `QueueUpdateDraw` self-deadlock on `F2` key capture.

### Robust Connection Architecture & Lifecycle Management (PHERI-05 & PHERI-06)
- **Network Safeguards & Timeouts**: Configured aggressive connection timeouts (10s connect, 5s ping) to prevent hangs during network drops or frozen database servers.
- **Configurable Connection Pool Sizing**: Fine-tuned pool lifecycle parameters (`SetMaxOpenConns`, `SetMaxIdleConns`, `SetConnMaxLifetime`, `SetConnMaxIdleTime`) to prevent connection starvation and MySQL `Too many connections` errors.

### Encrypted Connection Profile Passwords (PHERI-07)
- **AES-256-GCM Hardware-Bound Encryption**: Database passwords stored in local connection profiles (`~/.pheri_connections.json`) are automatically encrypted using authenticated AES-256-GCM.
- **Machine-Specific Key Derivation**: Encryption keys are derived using PBKDF2 with SHA-256 seeded by system hardware identifiers and user environment salt.

### Execution Engine & SQL Classifier (PHERI-04)
- **Intelligent Statement Classification**: Robust regex classifier differentiates `QueryContext` (record-fetching queries) from `ExecContext` (DML/DDL mutations).
- **Comment-Aware Tokenizer**: Accurately classifies queries even when preceded by SQL line comments (`--`, `#`) or multiline block comments (`/* ... */`).

### Grid NULL & Empty String Type Safety (PHERI-03)
- **Visual NULL Distinction**: NULL cells are explicitly styled and distinguished from empty strings (`""`).
- **Dedicated NULL Toggle**: In-place cell editor provides a dedicated shortcut to explicitly set cell values to SQL `NULL`.

### Server-Side Active Query Abort (PHERI-02)
- **Instant Abort on `Ctrl+C` / `Esc`**: Safely sends MySQL server-side `KILL QUERY <thread_id>` using a dedicated management connection, immediately terminating runaway queries without hanging the application.

### UI Ergonomics & Layman Knowledge Transfer (KT)
- **Full Mouse Support**: Mouse click routing enabled across sidebar objects, query editor, action toolbar buttons, results table cells, and bottom footer buttons.
- **Two-Way Focus Cycling**: Seamless keyboard navigation using `Tab` (forward) and `Shift+Tab` (backward) across all workspaces.
- **Layman Help Reference Guide (`F1` / `:help`)**: Plain-English user guide explaining workspaces, keyboard navigation, DevOps diagnostics, and complete transaction mechanics.
- **Zero-Emoji Professional Styling**: Standardized visual design conforming to clean, distraction-free terminal standards.

---

## Assets Included

- `pheri-windows-amd64.exe`: Windows 64-bit standalone binary (Smart App Control compliant).
- `pheri-windows-amd64.zip`: Windows 64-bit portable archive with executable and documentation.
- `pheri-windows-arm64.exe`: Windows ARM64 standalone binary.
- `pheri-windows-arm64.zip`: Windows ARM64 portable archive.
- `pheri-linux-amd64.tar.gz`: Linux 64-bit standalone archive.
- `pheri-linux-arm64.tar.gz`: Linux ARM64 standalone archive.
'@

# 3. Create Git Tag if needed
Write-Host "Creating and pushing git tag $TagName..." -ForegroundColor Cyan
git tag -a $TagName -m "$ReleaseName"
git push origin $TagName

# 4. Create Release on GitHub
$payload = @{
    tag_name         = $TagName
    target_commitish = "main"
    name             = $ReleaseName
    body             = $releaseBody
    draft            = $false
    prerelease       = $false
} | ConvertTo-Json -Depth 5

Write-Host "Publishing release $TagName to GitHub..." -ForegroundColor Cyan
$release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases" -Method Post -Headers $headers -Body $payload
Write-Host "Successfully published release: $($release.name)" -ForegroundColor Green
Write-Host "Release URL: $($release.html_url)" -ForegroundColor Yellow

$releaseId = $release.id

# 5. Upload Release Assets
function Upload-Asset($filePath, $assetName, $contentType) {
    if (-not (Test-Path $filePath)) {
        Write-Warning "File not found: $filePath"
        return
    }

    $uploadUri = "https://uploads.github.com/repos/$Repo/releases/$releaseId/assets?name=$assetName"
    $sizeMB = [math]::Round((Get-Item $filePath).Length / 1MB, 2)
    Write-Host "Uploading asset: $assetName ($sizeMB MB)..." -ForegroundColor Cyan

    $uploadHeaders = @{
        "Authorization" = "Bearer $token"
        "Content-Type"  = $contentType
        "Accept"        = "application/vnd.github+json"
        "User-Agent"    = "PowerShell-Pheri-Release-Publisher"
    }

    $fileBytes = [System.IO.File]::ReadAllBytes((Resolve-Path $filePath).Path)
    $uploadResp = Invoke-RestMethod -Uri $uploadUri -Method Post -Headers $uploadHeaders -Body $fileBytes
    Write-Host "  -> Uploaded successfully: $($uploadResp.browser_download_url)" -ForegroundColor Green
}

Upload-Asset -filePath "build/pheri-windows-amd64.exe" -assetName "pheri-windows-amd64.exe" -contentType "application/vnd.microsoft.portable-executable"
Upload-Asset -filePath "build/pheri-windows-amd64.zip" -assetName "pheri-windows-amd64.zip" -contentType "application/zip"
Upload-Asset -filePath "build/pheri-windows-arm64.exe" -assetName "pheri-windows-arm64.exe" -contentType "application/vnd.microsoft.portable-executable"
Upload-Asset -filePath "build/pheri-windows-arm64.zip" -assetName "pheri-windows-arm64.zip" -contentType "application/zip"
Upload-Asset -filePath "build/pheri-linux-amd64.tar.gz" -assetName "pheri-linux-amd64.tar.gz" -contentType "application/gzip"
Upload-Asset -filePath "build/pheri-linux-arm64.tar.gz" -assetName "pheri-linux-arm64.tar.gz" -contentType "application/gzip"

Write-Host "`nRelease $TagName published successfully with all assets!" -ForegroundColor Green
