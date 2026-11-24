# PHERI-19: Multi-Statement Batch Execution & Tabs

- **Task Code**: `PHERI-19`
- **Feature Name**: Multi-Statement Batch Execution & Tabs
- **Category**: SQL Editor
- **Priority**: `P2 - Medium`
- **Status**: `Completed`
- **Target Files**: `ui/sql_batch.go`, `ui/sqleditor.go`, `ui/browser.go`, `ui/theme.go`, `ui/footer.go`, `ui/help.go`, `ui/sql_batch_test.go`, `ui/sqleditor_test.go`
- **GitHub Issue**: [#34](https://github.com/vickychhetri/pheri/issues/34)

---

## 1. Reason of Adding

Database engineers frequently execute multi-statement migration scripts, ad-hoc updates paired with verification `SELECT`s, or multi-query audit scripts directly inside the SQL Editor. Previously:
1. Only the first SQL token was inspected by `ClassifySQLStatement`, causing mixed `SELECT` + DML/DDL batches to drop downstream result sets or bypass Read-Only checks on trailing statements.
2. There was no tabbed result grid UI to switch between multiple `SELECT` result sets or review per-statement execution outcomes and error line numbers.

### Operational Improvements:
1. **Lexical SQL Batch Splitter (`SplitSQLStatements`)**: Splits multi-statement scripts while preserving `;` inside `'...'` / `"..."` string literals, escaped quotes (`\'`, `''`, `\"`, `""`), `` `...` `` identifiers, `--` / `#` / `/* ... */` comments, and client-side `DELIMITER` blocks.
2. **Pre-Flight Safety & Fail-Fast Sequential Execution**: Checks every statement in the batch against Read-Only mode prior to execution, runs all statements on one acquired transaction (`EnsureActiveTx`) or pinned session connection (`db.Conn`), supports mid-batch `Ctrl+C` / `Esc` abort, and halts cleanly on first error with exact `Stmt #X (Ln Y)` attribution.
3. **Tabbed Result Grids & Summary View (`resultTabBar`)**: Renders clickable and keyboard-navigable (`[` / `]`, `Alt+Left` / `Alt+Right`) tabs directly above `dataTable` for each `SELECT` result set alongside a `Summary` grid covering all executed, failed, canceled, and skipped statements.
4. **Selection & Cursor Execution**: `Ctrl+R` / `F5` / `Ctrl+Enter` execute highlighted text if selected (or the full script), `Alt+Enter` runs the single statement under cursor, and the SQL Editor border shows live `K stmts` telemetry.

---

## 2. Verification & Automated Testing

### Unit Tests Added:
- [ui/sql_batch_test.go](file:///c:/Users/vicky/Desktop/pj/pheri/ui/sql_batch_test.go): `TestSplitSQLStatements_EdgeCases`, `TestSplitSQLStatements_CommentOnlyAndEmpty`, `TestBuildBatchResultTabs`, `TestBatchReadOnlyPreFlightGuard`, `TestBatchTabRenderingAndSwitching`.
- [ui/sqleditor_test.go](file:///c:/Users/vicky/Desktop/pj/pheri/ui/sqleditor_test.go): `TestSQLEditor_ExecutableTextAndStatementAtCursor`.

### Commands Executed:
```powershell
go test -v ./ui -run "TestSplitSQLStatements.*|TestBuildBatchResultTabs.*|TestBatch.*|TestSQLEditor.*"
go test ./...
go build -ldflags="-s -w" -o pheri.exe .
```
