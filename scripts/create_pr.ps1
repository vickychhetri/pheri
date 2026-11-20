param(
    [string]$Title = "feat(ui): professional UI overhaul, layman help guide, and export picker search",
    [string]$Head = "feature/ui-ux-standardization-and-help",
    [string]$Base = "main",
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
    "User-Agent"    = "PowerShell-Pheri-PR-Creator"
}

# 2. Prepare Detailed Pull Request Body
$prBody = @'
## Summary of Changes

This Pull Request delivers a comprehensive visual, functional, and developer-ergonomics overhaul for Pheri. It replaces the prototype aesthetic with a professional standard dark design system, fixes schema object identification, introduces an exhaustive layman-friendly Help Screen (`F1`), and resolves object selection visibility and search in the database export picker.

---

## Key Improvements

### 1. Professional Dark UI Overhaul & Zero Emoji Clutter
- **Design System (`ui/theme.go`)**: Implemented a centralized color palette engine supporting 4 standard dark themes: *Nordic Frost*, *Tokyo Night*, *Catppuccin Mocha*, and *Charcoal Slate*.
- **Consistent Elevation & Borders**: Replaced ad-hoc borders with standardized single-line frames (`StyleBox`, `StyleList`, `StyleTable`, `StyleModalBox`).
- **Clean Icon-Free Typography**: Eliminated all visual emojis from borders, buttons, and status lines for an uncluttered, corporate-ready appearance.
- **Optimized Layout Geometry**: Restructured the 3-pane layout with a 32-column sidebar, a 9-row query editor, and maximized screen real-estate for the query results data grid.

### 2. Schema Object Badging Fix
- **Problem**: Stored procedures, functions, triggers, and events were erroneously displaying with a `TABLE` badge in the database object tree.
- **Fix**: Implemented `FormatSchemaObjectBadge` to accurately identify and render distinct, color-coded badges for all database routine types:
  - `[aqua]TABLE[-]`
  - `[purple]VIEW [-]`
  - `[yellow]PROC [-]`
  - `[lime]FUNC [-]`
  - `[red]TRIG [-]`
  - `[blue]EVENT[-]`

### 3. Comprehensive Layman Help Screen (`F1` / `:help`)
- **Plain-English Explanations (`ui/help.go`)**: Designed a complete reference guide explaining every screen pane, hotkey, and command in accessible layman language without requiring deep database jargon.
- **DevOps Tools in Plain English**:
  - `F4 / :process / :top` — Live Process Manager & Query Killer (*"Task Manager for your database"*).
  - `F5 / :explain` — Execution Plan Analyzer (*"See how MySQL executes your query to fix bottlenecks before running on large tables"*).
  - `F6 / :health` — Real-Time Server Health Dashboard (live QPS, connection limits, and cache hit ratios).
  - `F7 / :diff` — Schema Difference & Migration Generator (*"Compare dev vs prod databases and generate sync SQL"*).
  - `Ctrl+Y` — Direct Full Database Backup to `.sql.gz`.
  - `Ctrl+W / F8` — Parallel Multi-Threaded Compressed Backup.
  - `F9 / :import` — Database Restore Wizard with safety checks.
- **Universal Access**: Mapped globally to `F1`, `?` (in data grid), and `:help` / `:h` in the footer command launcher.

### 4. Database Object Export Picker Overhaul (`ui/export.go`)
- **Checkbox Visibility Bug Resolved**: Fixed an issue where `tview` parsed `[x]` as a color tag, stripping the checkbox from selected items. Used `tview.Escape` so that `[X]` (bright lime bold) and `[ ]` (dim gray) render with 100% clarity.
- **Real-Time Search & Prefix Filtering**: Added a `Search:` input bar directly above the object list allowing instant filtering as you type, supporting plain text and prefixes (`table:`, `view:`, `proc:`, `func:`, `trig:`, `event:`).
- **Bulk Selection Shortcuts**:
  - `Space` — Toggle highlighted item.
  - `a` — Select all currently visible / filtered items.
  - `n` — Deselect all currently visible / filtered items.
  - `i` — Invert selection of currently visible / filtered items.
- **Single-Line Compact Items**: Removed the redundant secondary line repeated under every item, doubling visible list capacity.
- **Modal Lifecycle**: Fixed dialogs to preserve the application footer bar on exit.

---

## Commits in this PR

1. `33c4c3b` — `feat(ui): overhaul UI with professional design system, modern color palette, and zero emoji clutter`
2. `b52dbb4` — `fix(schema): correctly badge PROCEDURES, FUNCTIONS, TRIGGERS, and EVENTS in schema list`
3. `45b7228` — `feat(ui): add comprehensive layman help screen and unified command reference`
4. `e691a57` — `docs(help): document Ctrl+Y direct full database backup in layman guide`
5. `816528b` — `feat(export): fix checkbox visibility and add real-time object search filter in export picker`

---

## Verification & Testing

- [x] All 15 unit tests pass cleanly (`go test -v ./ui`).
- [x] Full project tests pass (`go test ./...`).
- [x] Verified checkbox visibility and search filtering with MySQL sample database (`sakila`).
- [x] Verified `F1` Help screen scrolling and clean `Esc` / `Enter` dismissal.
- [x] Compiled standalone Windows binaries (`pheri.exe` and `build/pheri-windows-amd64.exe`).
'@

# 3. Create Pull Request via GitHub REST API
$payload = @{
    title = $Title
    head  = $Head
    base  = $Base
    body  = $prBody
} | ConvertTo-Json -Depth 5

Write-Host "Creating Pull Request from '$Head' into '$Base' on $Repo..." -ForegroundColor Cyan

try {
    $response = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/pulls" `
        -Method Post `
        -Headers $headers `
        -Body $payload

    Write-Host "`n[SUCCESS] Pull Request created successfully!" -ForegroundColor Green
    Write-Host "PR Number : #$($response.number)" -ForegroundColor Yellow
    Write-Host "Title     : $($response.title)" -ForegroundColor White
    Write-Host "URL       : $($response.html_url)" -ForegroundColor Cyan
    Write-Host "State     : $($response.state)" -ForegroundColor Green

    # Output URL as well
    $response.html_url
} catch {
    Write-Error "Failed to create Pull Request: $_"
    if ($_.Exception.Response) {
        $stream = $_.Exception.Response.GetResponseStream()
        $reader = New-Object System.IO.StreamReader($stream)
        $respBody = $reader.ReadToEnd()
        Write-Host "API Response: $respBody" -ForegroundColor Red
    }
    exit 1
}
