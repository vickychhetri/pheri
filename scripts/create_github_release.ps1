param(
    [string]$TagName = "v3.1.0",
    [string]$ReleaseName = "Pheri v3.1.0 - Asynchronous Query Engine & Interactive SQL Editor",
    [string]$Repo = "vickychhetri/pheri"
)

# 1. Obtain GitHub token from Git Credential Manager
$cred = "protocol=https`nhost=github.com" | git credential fill
$token = ($cred | Where-Object { $_ -match '^password=(.+)$' } | ForEach-Object { $Matches[1] })

if (-not $token) {
    Write-Error "Failed to retrieve GitHub authentication token."
    exit 1
}

$headers = @{
    "Authorization" = "Bearer $token"
    "Accept"        = "application/vnd.github+json"
    "User-Agent"    = "PowerShell-Pheri-Release-Publisher"
}

# 2. Prepare Release Body with Detail List & Features
$releaseBody = @"
# Pheri v3.1.0

Pheri v3.1.0 introduces a major architectural milestone featuring non-blocking asynchronous query execution with instant cancellation, a completely redesigned multiline interactive SQL code editor, and vital developer ergonomics improvements.

---

## What's New in v3.1.0

### Asynchronous Query Execution with Instant Cancellation (PHERI-01)
- Non-blocking execution: Slow and unindexed queries run in isolated background goroutines via ``ExecuteQueryAsync``, keeping the terminal user interface responsive.
- Real-time query cancellation: Press ``Ctrl+C`` or ``Esc`` while a query is running to abort immediately and safely without terminating Pheri.
- Live duration progress indicator: A 100ms ticker continuously displays elapsed execution time in the status bar.
- Dedicated DML mutation handling: ``INSERT``, ``UPDATE``, ``DELETE``, ``ALTER``, and ``CREATE`` statements execute via ``db.ExecContext``, displaying affected row counts and execution timing.
- Thread-safe data grid sync: Off-thread row buffering into memory structures before atomic UI rendering via ``app.QueueUpdateDraw``.

### Professional Interactive SQL Code Editor (PHERI-16)
- Native multiline text buffer: Re-architected on top of ``tview.TextArea`` with native hardware cursor, line wrapping, and smooth scrolling.
- Native keyboard navigation: Home, End, PageUp, PageDown, and word jumps (``Ctrl+Left`` / ``Ctrl+Right``).
- Multi-level Undo & Redo: Revert and re-apply edits using standard ``Ctrl+Z`` (Undo) and ``Ctrl+Y`` (Redo).
- Two-way OS clipboard integration: Full system clipboard synchronization (``Ctrl+C`` to copy text or active selection, ``Ctrl+V`` / ``Ctrl+P`` to paste).
- Table name auto-completion (``Ctrl+T``): Context-aware table suggestion modal that replaces the current word at the cursor with the selected table name.
- SQL templates and snippets modal (``Ctrl+S`` / ``Ctrl+Space``): Instant insertion of boilerplate queries for ``SELECT``, ``INSERT``, ``UPDATE``, ``DELETE``, ``JOIN``, and ``CREATE TABLE``.
- Universal editor focus hotkey: Press ``e`` from anywhere in the data grid or sidebar to jump directly into the SQL Editor.
- Direct execution hotkeys: Execute queries instantly with ``Ctrl+R``, ``Ctrl+Enter``, or ``F5``.
- Clear query: Clear editor buffer instantly with ``Ctrl+K``.
- Fullscreen editor mode: Expand editor to 100% terminal height with ``F11``.

### TUI Safeguards & Key Collision Fixes
- Resolved Backspace conflict: Re-mapped Query Execution History to ``F2`` (and ``:history`` / ``:hist``), eliminating the terminal collision where Backspace was previously intercepted as ``KeyCtrlH``.
- In-app About dialog: Added ``:about`` and ``:info`` commands in the global footer command launcher.
- Windows Smart App Control compliance: Configured compiler flags (``-ldflags="-s -w"``) and provided ``build.ps1`` to satisfy Windows 11 Smart App Control policies without unsigned binary errors.

---

## Complete Available Features Directory

### Core Database Operations
- SQL Query Execution: Fast execution of arbitrary SQL statements with syntax highlighted results.
- Interactive Data Grid: Navigate tabular results with row selection, column headers, and pagination (``Ctrl+N`` / ``Ctrl+P``).
- Cell Inspector: Press ``v`` on any highlighted cell to view complete unformatted contents in a popup viewer.
- Inline Cell Editing: Edit table values directly in the grid.
- Foreign Key Inspector: Inspect referenced table relationships and schema constraints.

### DBA & DevOps Diagnostics
- Live Process Manager (``F4`` / ``:process`` / ``:top``): Real-time monitor of active MySQL threads with instant query termination (``KILL query_id``).
- EXPLAIN Query Analyzer (``F5`` / ``:explain``): Performance analysis for queries in the editor, identifying scanned rows, join types, and missing index bottlenecks.
- Real-Time Server Health Dashboard (``F6`` / ``:health``): Live monitoring of Queries Per Second (QPS), active client threads, InnoDB buffer pool hit ratios, and slow query counts.
- Schema Diff & Migration Generator (``F7`` / ``:diff``): Compare active database tables and generate verification SQL scripts.
- Schema Inspector (``F3``): Detailed structural breakdown of columns, data types, primary keys, and indexes.

### Backup & Restore Engines
- High-Speed Parallel Compressed Export (``F8`` / ``Ctrl+W``): Multi-threaded database dumper creating compressed ``.sql.gz`` archives (up to 95% reduction in disk size).
- Selective Database Export Wizard (``Ctrl+E`` / ``:export``): Export specific tables into ``.sql.gz``, plain ``.sql``, CSV, or JSON formats.
- Database Import Wizard (``F9`` / ``:import``): Interactive step-by-step importer supporting ``.sql`` and ``.sql.gz`` files with destructive operation warnings and Read-Only enforcement.

### Security & Operational Safeguards
- Environment Tagging: Tag database connections as ``PROD``, ``STAGING``, or ``DEV`` with visual warning indicators.
- Read-Only Mode: Pass ``-readonly`` flag on connection to block all mutating queries (``INSERT``, ``UPDATE``, ``DELETE``, ``DROP``, ``ALTER``, ``TRUNCATE``) and disable imports.
- Saved Connection Profiles: Encrypted local storage for database connection profiles (``~/.pheri_connections.json``).

---

## Quick Start & Verification

Run Pheri in your terminal:
```bash
./pheri
```

Or connect directly with command-line flags:
```bash
./pheri -u root -p password -host 127.0.0.1 -port 3306 -db my_database -env DEV
```

To test non-blocking execution and cancellation:
```sql
SELECT SLEEP(10);
```
Press ``Ctrl+C`` or ``Esc`` to cancel the query instantly.

---

## Available Assets

- ``pheri-windows-amd64.exe``: Windows 64-bit standalone executable (Smart App Control compliant).
- ``pheri-windows-amd64.zip``: Windows 64-bit package with binary, documentation, and build scripts.
"@

# 3. Create Git Tag if it doesn't exist locally/remotely
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
    Write-Host "Uploading asset: $assetName ($sizeMB MB)" -ForegroundColor Cyan

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
Upload-Asset -filePath "build/pheri_1.0.0_amd64.deb" -assetName "pheri-linux-amd64.deb" -contentType "application/vnd.debian.binary-package"

Write-Host "`nRelease $TagName published successfully with all assets!" -ForegroundColor Green
