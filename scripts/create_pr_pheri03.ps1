param(
    [string]$Title = "fix(grid): grid NULL & empty value type safety and dedicated NULL toggle (PHERI-03)",
    [string]$Head = "feature/pheri-03-grid-null-type-safety",
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

This Pull Request addresses and resolves **PHERI-03: Fix Grid NULL & Empty Value Type Safety**.

Previously, SQL `NULL` and empty strings `""` were stored and rendered as literal visual formatting markup strings (`"[gray]NULL"` and `"[gray]EMPTY"`). When a user selected a cell to edit, the visual markup string was loaded as literal text into the editor. Upon saving, Pheri sent `"[gray]NULL"` directly to MySQL, silently corrupting `VARCHAR`/`TEXT` columns with formatting markup and causing fatal syntax or type errors on `INT`, `DECIMAL`, and `DATE` columns. Additionally, users had no way to set a value to SQL `NULL`, and `WHERE` clauses matching against `"[gray]EMPTY"` failed silently.

---

## Key Improvements

### 1. Typed `GridCell` Data Model (`ui/grid_cell.go`)
- Introduced `GridCell` struct strictly separating underlying database values from TUI visual presentation:
  ```go
  type GridCell struct {
      RawValue *string // nil = SQL NULL; non-nil = literal string (including "" for empty)
      ColName  string  // Database column name
      IsHeader bool
  }
  ```
- Methods:
  - `IsNull()`: Returns `RawValue == nil`.
  - `IsEmpty()`: Returns `RawValue != nil && *RawValue == ""`.
  - `Value()`: Returns exact unadorned string or `""` if NULL.
  - `DisplayText()`: Safely renders `[gray::i]NULL[-::-]`, `[gray::i]EMPTY[-::-]`, or user text escaped with `tview.Escape` to prevent unescaped brackets in data from breaking TUI formatting.
  - `ExportValue(placeholder)`: Returns unescaped raw string or empty string for CSV export.
  - `GetGridCellFromTableCell(cell, colName)`: Extracts attached cell reference with graceful fallback parser.

### 2. Dual-Layer `QueryResult` (`ui/browser.go`)
- Extended `QueryResult` to retain both `Rows [][]string` (for display) and `RawRows [][]*string` (for exact database value preservation).
- When populating `tview.Table`, each `TableCell` binds its typed `GridCell` pointer via `cell.SetReference(gridCell)`.

### 3. Interactive Cell Edit Modal with NULL Toggle (`ui/browser.go`)
- Upgraded inline cell editing to a 3-state editor:
  - **`Ctrl+N`**: Toggles between `[X] SQL NULL` and Literal Value mode. In NULL mode, the text area displays a placeholder and binds `nil` on save.
  - **`Ctrl+E`**: Clears text to an exact empty string `""`.
  - **Typing characters**: Automatically switches mode from NULL to Literal Value if the user begins typing.
  - **`Enter`**: Executes `UPDATE table SET col = ?` with `nil` (for SQL NULL) or the exact string value.
  - **`Esc`**: Safely dismisses modal and restores data grid focus and footer bar.

### 4. Sanitized `WHERE` Clause Builders
- Primary key and fallback full-row `WHERE` clauses now evaluate `GetGridCellFromTableCell`:
  - If NULL: appends `col IS NULL`.
  - If Value: appends `col = ?` and binds `gc.Value()` (exact string without color markup or padding spaces).

### 5. Clean CSV & JSON Export Serialization
- In `showExportResultsModal`:
  - CSV exports serialize NULL cells as empty strings `""` instead of `"[gray]NULL"`.
  - JSON exports serialize NULL cells as genuine JSON `null` (`"col": null`) instead of string `"NULL"`.

---

## Verification & Testing

- [x] **New Unit Tests (`ui/grid_cell_test.go`)**:
  - `TestGridCell_NullAndEmptyHandling`: PASS
  - `TestGridCell_DisplayTextFormatting`: PASS
  - `TestGridCell_ExportValue`: PASS
  - `TestGetGridCellFromTableCell_WithReference`: PASS
  - `TestGetGridCellFromTableCell_FallbackParsing`: PASS
  - `TestGridCell_SQLParameterBinding`: PASS
  - `TestGridCell_WhereClauseGeneration`: PASS
  - `TestGridCell_ExportSerializationFormatting`: PASS
- [x] **Full Test Suite**: All 28 unit tests passing cleanly (`go test -v ./...`).
- [x] **Compilation**: Built standalone Windows binaries (`pheri.exe` and `build/pheri-windows-amd64.exe`).
- [x] **Documentation**: Created comprehensive documentation at `docs/tasks/PHERI-03_GRID_NULL_TYPE_SAFETY.md`.
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
