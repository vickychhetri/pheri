# PHERI-14: Interactive Row Deletion ('d' / Del)

- **Task Code**: `PHERI-14`
- **Feature Name**: Interactive Row Deletion ('d' / Del)
- **Category**: Data Grid
- **Priority**: `P1 - High`
- **Status**: `Completed`
- **Target Files**: `ui/delete_row.go`, `ui/delete_row_test.go`, `ui/browser.go`, `ui/help.go`
- **GitHub Issue**: [#29](https://github.com/vickychhetri/pheri/issues/29)

---

## 1. Reason of Adding

Previously, deleting a single record from a table required manually inspecting Primary Keys and writing `DELETE FROM ... WHERE ... LIMIT 1` queries in the SQL Editor, increasing both friction and the risk of unintended multi-row deletions.

### Operational Improvements:
1. **Single-Key Trigger (`d` / `D` / `Delete`)**: Prompts deletion for the highlighted row inside any base `TABLE` in the Data Grid.
2. **Targeted `WHERE ... LIMIT 1` Builder (`BuildRowWhereClause`)**: Uses single or composite Primary Keys when all PK columns are present in the grid, and safely falls back to matching all visible columns (`= ?` / `IS NULL`) for keyless tables or partial projections. Also cleanly resolves raw column names via `getHeaderColumnName` even when sort (`▲`/`▼`) or FK (`↗`) indicators are shown.
3. **Safety Confirmation Modal**: Fetches PK metadata asynchronously off the UI thread, renders the exact preview SQL statement, and defaults focus to **`Cancel`** (with `Esc` cancel support) to guard against reflexive double-key presses.
4. **Transaction & Read-Only Integration**: Blocked cleanly in Read-Only mode; integrates with Manual Transaction mode (`F2`) via `EnsureActiveTx` and `RecordMutationInTx(..., "Row Delete Modal")`, logs history, surfaces database constraint errors (e.g. FK RESTRICT errors), and refreshes the Data Grid upon completion.

---

## 2. Technical Architecture & Implementation Details

### 1. Row Deletion & Shared `WHERE` Builder (`ui/delete_row.go`)
- `BuildRowWhereClause(table, row, pkCols)`: Shared parameterized predicate builder reused by both `BuildParameterizedDeleteQuery` and `EnableCellEditing` (`UPDATE`).
- `BuildParameterizedDeleteQuery(...)`: Generates `DELETE FROM `db`.`table` WHERE <clause> LIMIT 1` with `phhistory.ReplacePlaceholders` preview.
- `TriggerRowDeleteFlow(...)` & `buildDeleteRowModalUI(...)`: Async PK lookup, confirmation modal defaulting to Cancel, non-blocking execution (`5s` timeout), and grid refresh.

### 2. Data Grid Integration (`ui/browser.go`, `ui/help.go`)
- Bound `tcell.KeyDelete`, `'d'`, and `'D'` in `setupTableInputCapture` with `ActiveReadOnly` and `activeGridObjectType == "TABLE"` guards.
- Refactored `EnableCellEditing` inline `UPDATE` matching to call `BuildRowWhereClause`.
- Documented `d / Delete` in [ui/help.go](file:///c:/Users/vicky/Desktop/pj/pheri/ui/help.go).

---

## 3. Verification & Automated Testing

### Unit Tests Added ([ui/delete_row_test.go](file:///c:/Users/vicky/Desktop/pj/pheri/ui/delete_row_test.go)):
- `TestBuildRowWhereClause_PrimaryKeyAndIndicators`
- `TestBuildRowWhereClause_KeylessFallbackAndNulls`
- `TestBuildParameterizedDeleteQuery`
- `TestDeleteRowModal_AndGuards`

### Commands Executed:
```powershell
go test -v ./ui -run "TestBuildRowWhereClause.*|TestBuildParameterizedDeleteQuery|TestDeleteRowModal.*"
go test ./...
go build -ldflags="-s -w" -o pheri.exe .
```
