# PHERI-15: Quick Copy Row as JSON / TSV / SQL

- **Task Code**: `PHERI-15`
- **Feature Name**: Quick Copy Row as JSON / TSV / SQL
- **Category**: Data Grid
- **Priority**: `P2 - Medium`
- **Status**: `Completed`
- **Target Files**: `ui/row_copy.go`, `ui/row_copy_test.go`, `ui/browser.go`, `ui/help.go`
- **GitHub Issue**: [#30](https://github.com/vickychhetri/pheri/issues/30)

---

## 1. Reason of Adding

Previously, operators viewing query results could only inspect/copy a single cell (`v` -> `c`) or run a full export (`Ctrl+E`). When writing unit test fixtures, JSON API payloads, bug reports, or seed SQL statements, extracting one formatted row required awkward manual reformatting.

### Operational Improvements:
1. **Quick Copy Row Menu (`Ctrl+Shift+C` / `c` / `y`)**: Opens a centered format selector with a live clipboard preview directly from any highlighted Data Grid row.
2. **Four Clipboard Output Formats**:
   - **`1` - JSON Object**: Ordered keys matching the grid column order, unpadded integers/floats as JSON numbers, leading-zero codes preserved as strings, properly escaped text, and `null` for SQL `NULL`.
   - **`2` - TSV Row (Values Only)**: Clean tab-separated line with embedded newlines/tabs flattened for spreadsheet pasting.
   - **`3` - TSV with Header Row**: Column header row + values row for quick Slack/GitHub/spreadsheet snippets.
   - **`4` - SQL `INSERT INTO` Statement**: Backtick-quoted target table & columns with escaped SQL literals and `NULL` keywords.
3. **Terminal-Safe Shortcut Routing**: Updated `setupGlobalInputCapture` so `Ctrl+Shift+C` passes through to `dataTable` (while preserving plain `Ctrl+C` query abort / exit hint), and provided fallback single-key triggers **`c`** and **`y`** for terminal emulators that intercept `Ctrl+Shift+C` at the OS level.

---

## 2. Technical Architecture & Implementation Details

### 1. Row Snapshot & Formatters (`ui/row_copy.go`)
- `ExtractRowCopyData(...)`: Captures raw `GridCell` metadata and stripped column names (`getHeaderColumnName`) so active sort (`▲`/`▼`) or FK (`↗`) markers never leak into clipboard output.
- `FormatRowAsJSON`, `FormatRowAsTSV`, `FormatRowAsSQLInsert`: Deterministic formatters with SQL injection-safe escaping via `util.FormatSQLValueLiteral` and `util.QuoteIdentifier`.
- `ShowCopyRowMenuModal` & `buildCopyRowModalUI`: Interactive `tview.List` + live `Live Clipboard Preview` `tview.TextView`, supporting instant `1`–`4` keys, `Enter`, and `Esc` cancellation.

### 2. Grid & Global Key Integration (`ui/browser.go`, `ui/help.go`)
- Allowed `KeyCtrlC` with `ModShift` to pass through `setupGlobalInputCapture`.
- Bound `Ctrl+Shift+C`, `c`/`C`, and `y`/`Y` inside `setupTableInputCapture`.
- Documented `Ctrl+Shift+C / c / y` in [ui/help.go](file:///c:/Users/vicky/Desktop/pj/pheri/ui/help.go).

---

## 3. Verification & Automated Testing

### Unit Tests Added ([ui/row_copy_test.go](file:///c:/Users/vicky/Desktop/pj/pheri/ui/row_copy_test.go)):
- `TestExtractRowCopyData_StripsIndicators`
- `TestFormatRowAsJSON`
- `TestFormatRowAsTSV`
- `TestFormatRowAsSQLInsert`
- `TestCopyRowModal_AndShortcuts`

### Commands Executed:
```powershell
go test -v ./ui -run "TestExtractRowCopyData.*|TestFormatRowAs.*|TestCopyRowModal.*"
go test ./...
go build -ldflags="-s -w" -o pheri.exe .
```
