# PHERI-17: Line Numbers Margin & Cursor Sync

- **Task Code**: `PHERI-17`
- **Feature Name**: Line Numbers Margin & Cursor Sync
- **Category**: SQL Editor
- **Priority**: `P2 - Medium`
- **Status**: `Completed`
- **Target Files**: `ui/sqleditor.go`, `ui/sqleditor_test.go`, `ui/help.go`
- **GitHub Issue**: [#32](https://github.com/vickychhetri/pheri/issues/32)

---

## 1. Reason of Adding

Multi-line SQL queries often produce syntax or runtime errors referencing specific line numbers (e.g., `at line 8`). Without a line-number margin beside the text buffer, operators had to manually count lines or estimate cursor location from the border title.

### Operational Improvements:
1. **Synchronized Line Number Gutter (`1 │`, `2 │`, ...)**: Displays right-aligned 1-based line numbers separated cleanly by `│` along the left edge of `SQLEditor`.
2. **Real-Time Scroll & Cursor Sync**: Reads `TextArea.GetOffset()` and `TextArea.GetCursor()` directly on each render pass so the gutter scrolls in lockstep with keyboard navigation and mouse scrolling.
3. **Active Line Highlighting**: Highlights the currently focused line (and multi-line selection span) with `CurrentTheme.AccentPrimary` and bold formatting, synced with live `Ln X, Col Y | N lines | M chars` telemetry on the unified container title.
4. **1-to-1 Error Line Parity**: Configures `textArea.SetWrap(false)` with horizontal scrolling so displayed line numbers match MySQL `\n` line counts accurately across queries of any length, expanding gutter width smoothly at 100 / 1,000+ lines.

---

## 2. Technical Architecture & Implementation Details

### 1. `lineNumberGutter` & Dynamic Layout (`ui/sqleditor.go`)
- `lineNumberGutter`: Custom primitive embedding `*tview.Box` that renders right-aligned line numbers against `CurrentTheme` colors and transfers focus to `TextArea` on mouse click.
- `CountEditorLines(text)` & `CalculateGutterWidth(totalLines)`: Computes line counts and dynamically resizes the gutter (`digits + 4`, minimum 6 cols) when queries grow across digit thresholds.
- Unified Outer Frame: Moved the border and telemetry title (`Ln X, Col Y | N lines | M chars`) onto `SQLEditor.Container` (`*tview.FlexColumn` enclosing `Gutter` + borderless `TextArea`).
- `ApplyTheme()`: Synchronizes `TextArea`, `Gutter`, and `Container` colors with active dark/light themes.

### 2. Help Guide (`ui/help.go`)
- Added Line Numbers Margin documentation under Section 4 (`WRITING & RUNNING SQL QUERIES (SQL EDITOR)`).

---

## 3. Verification & Automated Testing

### Unit Tests Added ([ui/sqleditor_test.go](file:///c:/Users/vicky/Desktop/pj/pheri/ui/sqleditor_test.go)):
- `TestSQLEditor_LineCountAndGutterWidth`
- `TestSQLEditor_LineNumberGutterRenderAndScrollSync`
- `TestSQLEditor_ApplyThemeUpdatesGutter`

### Commands Executed:
```powershell
go test -v ./ui -run "TestSQLEditor.*"
go test ./...
go build -ldflags="-s -w" -o pheri.exe .
```
