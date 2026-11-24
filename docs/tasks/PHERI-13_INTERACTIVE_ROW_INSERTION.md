# PHERI-13: Interactive Row Insertion Modal ('+' / 'a')

- **Task Code**: `PHERI-13`
- **Feature Name**: Interactive Row Insertion Modal ('+' / 'a')
- **Category**: Data Grid
- **Priority**: `P1 - High`
- **Status**: `Completed`
- **Target Files**: `ui/insert_modal.go`, `ui/insert_modal_test.go`, `ui/browser.go`, `ui/help.go`
- **GitHub Issue**: [#28](https://github.com/vickychhetri/pheri/issues/28)

---

## 1. Reason of Adding

Previously, inserting a new row into a table forced operators to inspect column constraints separately and manually compose `INSERT INTO ... VALUES (...)` queries in the SQL Editor.

### Operational Improvements:
1. **Instant Grid Shortcut (`+` / `a` / `A`)**: Opens a dynamic insertion modal directly from any base `TABLE` inside the Data Grid.
2. **Schema-Aware Input Form**: Inspects `INFORMATION_SCHEMA.COLUMNS` asynchronously (`3s` timeout) to label every column with its data type, Primary/Foreign key marker (`[PK]`/`[FK]`), required `*` indicator, and smart placeholders (`AUTO_INCREMENT`, `DEFAULT`, `NULLABLE`).
3. **Explicit `NULL` / Empty `""` / `DEFAULT` Controls**: Mirrors Pheri's inline cell-editing conventions (`Ctrl+N` toggles SQL `NULL`, `Ctrl+E` toggles explicit empty string `""`, leaving `AUTO_INCREMENT`/`DEFAULT` columns blank omits them so MySQL fills defaults).
4. **Parameterized SQL & Transaction Integration**: Validates required/numeric types before submitting, runs parameterized `INSERT INTO` off the UI thread, participates cleanly in manual transactions (`EnsureActiveTx` + `RecordMutationInTx` with `"Row Insert Modal"` audit source), preserves form inputs if MySQL rejects a constraint, and refreshes the grid automatically upon commit/insert.

---

## 2. Technical Architecture & Implementation Details

### 1. Metadata, Validation & SQL Builder (`ui/insert_modal.go`)
- `TableColumnMeta`: Captures column name, data type, column type, nullability, default presence/value, `EXTRA` (`auto_increment`), and `COLUMN_KEY`.
- `FetchTableColumnsMeta(ctx, db, dbName, tableName)`: Ordered lookup from `INFORMATION_SCHEMA.COLUMNS`.
- `ValidateInsertValue(col, val)`: Enforces required `NOT NULL` fields and parses integer/decimal types prior to submission.
- `BuildParameterizedInsertQuery(...)`: Constructs safe backtick-quoted `INSERT INTO ... VALUES (?, ...)` (or `() VALUES ()` when all columns use defaults/auto-increment) and generates preview SQL via `phhistory.ReplacePlaceholders`.
- `ShowInsertRowModal` & `buildInsertRowModalUI`: Theme-consistent form modal with inline validation/error bar, `atomic.Bool` double-submit guard, and async execution.

### 2. Data Grid Integration (`ui/browser.go`, `ui/help.go`)
- Bound `+`, `a`, and `A` in `setupTableInputCapture` with Read-Only (`ActiveReadOnly`) and base-table (`activeGridObjectType == "TABLE"`) guards.
- Documented `+ / a` in [ui/help.go](file:///c:/Users/vicky/Desktop/pj/pheri/ui/help.go).

---

## 3. Verification & Automated Testing

### Unit Tests Added ([ui/insert_modal_test.go](file:///c:/Users/vicky/Desktop/pj/pheri/ui/insert_modal_test.go)):
- `TestTableColumnMeta_Helpers`
- `TestValidateInsertValue`
- `TestBuildParameterizedInsertQuery`
- `TestBuildInsertFormUI_HotkeysAndGuards`

### Commands Executed:
```powershell
go test -v ./ui -run "TestTableColumnMeta.*|TestValidateInsertValue|TestBuildParameterizedInsertQuery|TestBuildInsertFormUI.*"
go test ./...
go build -ldflags="-s -w" -o pheri.exe .
```
