param(
    [string]$TagName = "v3.3.0",
    [string]$ReleaseName = "Pheri v3.3.0 - Modern Split-Pane Connection Hub, 10-Theme Palette Engine & High-Contrast Light Accessibility",
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
# Pheri v3.3.0

Pheri v3.3.0 introduces a major visual and architectural redesign featuring a modern split-pane connection hub with live latency ping, a 10-theme customization engine with persistent user preferences, and a complete high-contrast accessibility overhaul for the Light theme.

---

## Highlights in v3.3.0

### Modern Split-Pane Connection & Authentication Hub
- **Split-Screen Ergonomics**: Replaced the legacy single-column modal with a sleek two-pane layout: Saved Connection Profiles on the left and Connection Details & Authentication on the right.
- **Live Latency Ping**: Added interactive `[ Test Ping ]` action that measures connection latency and verifies MySQL server version before connecting.
- **Asynchronous Non-Blocking Connection**: Connection attempts and pings run on background goroutines using `app.QueueUpdateDraw()`, eliminating UI freezes and goroutine deadlocks.
- **Complete Profile Management**: Easily create, test, save (with AES-256-GCM encryption), delete, and clear connection profiles directly from the TUI.
- **Seamless Navigation**: Smooth transition between database lists and connection screen via `[ Back to Login ]` and `Esc` without state leakage.
- **Quick Keyboard Shortcuts**: `Tab` / `Shift+Tab` to switch panels, `n` for new connection, `t` for test ping, `d` for delete profile.

### 10-Theme Customization Engine & Live Global Switcher
- **10 Curated Professional Themes**:
  - **Dark**: Deep navy developer theme.
  - **Light**: Crisp paper-white theme with slate depth.
  - **Developer**: VS Code Dark+ inspired palette.
  - **Hacker**: Matrix-inspired emerald phosphor theme.
  - **Classic**: Retro midnight blue terminal palette.
  - **Gen-Z**: Vibrant neon violet and cyan theme.
  - **Nordic Frost**: Cool arctic ice blue palette.
  - **Tokyo Night**: Soft neon indigo and sky blue palette.
  - **Catppuccin Mocha**: Cozy pastel palette.
  - **Charcoal Slate**: Neutral low-distraction dark palette.
- **F12 Theme Selector**: Open the theme picker modal instantly via `F12` hotkey anywhere in the application or using the `Theme (F12)` button on the login screen.
- **Live Global Redraw**: Theme changes take effect instantly across all open screens and components.
- **Persistent Preferences**: Active theme selection is automatically saved in `~/.pheri_connections.json` and restored across application restarts.

### High-Contrast Light Theme Accessibility (WCAG AAA)
- **Eliminated Terminal Background Leaks**: Replaced `tcell.ColorReset` in favor of explicit `SurfaceBg` (`#ffffff`) and `FieldBg` (`#f1f5f9`), ensuring the Light theme displays properly on terminals that default to black.
- **Dynamic Markup Contrast Mapper (`FormatThemeMarkup`)**: Automatically transforms low-contrast TUI color tags (`[white]`, `[gray]`, `[yellow]`, `[lime]`, `[aqua]`) into WCAG AAA compliant dark tones on the Light theme.
- **Full Component Coverage**: Contrast verified across the split-pane login screen, SQL editor, data table grid (with alternating zebra rows), schema tree, export modal, and help dialogs.

### Enterprise Features from Previous Releases
- **Explicit Transaction Controls (`F2`, `F10`, `Shift+F10`, `:review`)**: Auto-Commit toggle with mutation badges, transaction audit trail, and auto-refresh on rollback.
- **Active Query Abort (`Ctrl+C` / `Esc`)**: Server-side query termination using `KILL QUERY <thread_id>`.
- **Hardware-Bound AES-256-GCM Encryption**: Secure password storage for saved profiles.
- **Interactive Multiline SQL Editor**: Autocomplete (`Ctrl+T`), snippets (`Ctrl+S`), Undo/Redo (`Ctrl+Z`/`Ctrl+Y`), and system clipboard integration.

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

Upload-Asset -filePath "installer/Output/pheri-setup-windows-amd64.exe" -assetName "pheri-setup-windows-amd64.exe" -contentType "application/vnd.microsoft.portable-executable"
Upload-Asset -filePath "build/pheri-windows-amd64.exe" -assetName "pheri-windows-amd64.exe" -contentType "application/vnd.microsoft.portable-executable"
Upload-Asset -filePath "build/pheri-windows-amd64.zip" -assetName "pheri-windows-amd64.zip" -contentType "application/zip"
Upload-Asset -filePath "build/pheri-windows-arm64.exe" -assetName "pheri-windows-arm64.exe" -contentType "application/vnd.microsoft.portable-executable"
Upload-Asset -filePath "build/pheri-windows-arm64.zip" -assetName "pheri-windows-arm64.zip" -contentType "application/zip"
Upload-Asset -filePath "build/pheri-linux-amd64.tar.gz" -assetName "pheri-linux-amd64.tar.gz" -contentType "application/gzip"
Upload-Asset -filePath "build/pheri-linux-arm64.tar.gz" -assetName "pheri-linux-arm64.tar.gz" -contentType "application/gzip"

Write-Host "`nRelease $TagName published successfully with all assets!" -ForegroundColor Green
