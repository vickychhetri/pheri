# PHERI-11: Dynamic Column Header Sorting (ASC / DESC)

- **Task Code**: `PHERI-11`
- **Feature Name**: Dynamic Column Header Sorting (ASC / DESC)
- **Category**: Data Grid
- **Priority**: `P1 - High`
- **Status**: `Completed`
- **Target Files**: `ui/grid_sort.go`, `ui/grid_sort_test.go`, `ui/browser.go`, `ui/help.go`
- **GitHub Issue**: [#26](https://github.com/vickychhetri/pheri/issues/26)

---

## 1. Reason of Adding

Previously, column headers (`row == 0`) in the Pheri Data Grid were marked non-selectable and offered no interactive sorting mechanism. Operators had to manually edit queries in the SQL Editor with `ORDER BY` clauses whenever inspecting tables or result sets.

### Operational Improvements:
1. **3-State Interactive Sort Cycling**: Pressing **`s` / `S`** on any cell in a column, pressing **`Enter`** on a header cell (`row == 0`), or left-clicking a header cycles `Unsorted -> ASC (▲) -> DESC (▼) -> Unsorted`.
2. **Server-Side Query Re-Execution**:
   - Table and View browsing rebuilds `SELECT * FROM db.obj ORDER BY col ASC|DESC LIMIT ... OFFSET ...`, resets to Page 1 on sort toggle, and preserves active sorting during pagination (`Ctrl+N` / `Ctrl+P`).
   - Custom `SELECT` queries inject or replace top-level `ORDER BY` clauses before `LIMIT`/`OFFSET` (and safely wrap top-level `UNION` queries) before re-executing asynchronously.
3. **Graceful In-Memory Fallback**: Non-rewritable diagnostic queries (`SHOW`, `DESCRIBE`, `EXPLAIN`) fall back to stable numeric-aware and MySQL `NULL`-ordered in-memory sorting while preserving original row order when reset to `Unsorted`.
4. **Header Navigation & Visual Indicators**: Column headers are keyboard-selectable (`row == 0`), display directional arrows (`▲` / `▼`) with active accent highlighting, update grid titles/footer status, and preserve column focus across re-renders.

---

## 2. Technical Architecture & Implementation Details

### 1. Sort Engine & SQL Rewriter (`ui/grid_sort.go`)
- `SortDirection`: `SortNone`, `SortAsc` (`▲`), `SortDesc` (`▼`).
- `GridSortState`: Thread-safe coordinator holding `Column`, `Direction`, `BaseQuery`, `IsTableBrowse`, `OriginalResult`, and selection restoration coordinates.
- `BuildBrowseQuery(...)`: Cleanly constructs quoted table/view SQL with optional `ORDER BY` and `LIMIT/OFFSET`.
- `ApplySortToSQLQuery(...)`: Quote-aware, comment-aware, parenthesis-depth SQL scanner that locates outer `ORDER BY` / `LIMIT` clauses or wraps `UNION` queries without mutating inner subqueries.
- `SortQueryResultInMemory(...)`: Stable sort comparing numeric values numerically, strings case-insensitively, and `NULL`s using MySQL semantics (`NULL` first in `ASC`, last in `DESC`).

### 2. Data Grid Integration (`ui/browser.go`, `ui/help.go`)
- Extracted shared `openTableOrView(...)` helper removing duplicated browse logic in `filterTableList` and `UseDatabase`.
- Added `getHeaderColumnName(...)` backed by `SetReference(col)` on header cells so cell editing, cell inspection (`v`), and sorting always resolve raw column names without directional glyphs.
- Updated `renderQueryResult(...)` to render header indicators, keep headers selectable (`row == 0`), enable full cell navigation (`SetSelectable(true, true)`), and restore sorted column selection after async re-execution.
- Documented `s / Header Enter` sorting in [ui/help.go](file:///c:/Users/vicky/Desktop/pj/pheri/ui/help.go).

---

## 3. Verification & Automated Testing

### Unit Tests Added ([ui/grid_sort_test.go](file:///c:/Users/vicky/Desktop/pj/pheri/ui/grid_sort_test.go)):
- `TestSortDirectionCycle`
- `TestBuildBrowseQuery`
- `TestApplySortToSQLQuery`
- `TestSortQueryResultInMemory`
- `TestRenderQueryResult_SortIndicatorsAndKeyboardCycling`

### Commands Executed:
```powershell
go test -v ./ui -run "TestSort.*|TestBuildBrowseQuery|TestApplySortToSQLQuery|TestRenderQueryResult_Sort.*"
go test ./...
go build -ldflags="-s -w" -o pheri.exe .
```
