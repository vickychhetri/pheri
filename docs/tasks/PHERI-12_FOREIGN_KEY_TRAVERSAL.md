# PHERI-12: Foreign Key Traversal (Follow Relationship)

- **Task Code**: `PHERI-12`
- **Feature Name**: Foreign Key Traversal (Follow Relationship)
- **Category**: Data Grid
- **Priority**: `P1 - High`
- **Status**: `Completed`
- **Target Files**: `util/util.go`, `util/util_test.go`, `ui/fk_nav.go`, `ui/fk_nav_test.go`, `ui/browser.go`, `ui/grid_sort.go`, `ui/help.go`
- **GitHub Issue**: [#27](https://github.com/vickychhetri/pheri/issues/27)

---

## 1. Reason of Adding

In relational MySQL databases, foreign key columns appear as bare numeric IDs or string codes (`customer_id`, `film_id`, `country_id`). Inspecting referenced parent records previously required manually inspecting schema DDL and composing `SELECT ... WHERE` queries in the SQL Editor, losing cursor position and context.

### Capabilities Delivered:
1. **Non-Blocking Foreign Key Discovery**: Queries `INFORMATION_SCHEMA.KEY_COLUMN_USAGE` asynchronously in `ExecuteQueryAsync`'s background worker with a 2-second timeout and thread-safe caching (`GetCachedForeignKeys`).
2. **Clear Visual Indicators (`↗`)**: Highlights non-NULL FK cells and FK column headers with `↗` and accent cyan styling while keeping raw `GridCell` values clean for editing and inspection.
3. **One-Key Jump (`f` / `F` / `Ctrl+J`)**: Jumps directly to the referenced schema, table, and matching parent row (`SELECT * FROM `ref_db`.`ref_table` WHERE `ref_col` = ... LIMIT 100`).
4. **Multi-Hop Breadcrumb Return (`Backspace`)**: Pushes source database, table, query, page, sort state, and `(row, col)` cursor position onto `fkNavHistory` so pressing `Backspace` returns cleanly step-by-step.

---

## 2. Technical Architecture & Implementation Details

### 1. Metadata Lookup & Query Builder (`util/util.go`)
- `ForeignKeyRef`: Holds `ConstraintName`, `ColumnName`, `ReferencedSchema`, `ReferencedTable`, `ReferencedColumn`.
- `GetTableForeignKeys(ctx, db, dbName, tableName)`: Queries `INFORMATION_SCHEMA.KEY_COLUMN_USAGE` for foreign keys referencing external or self-referencing tables.
- `FormatSQLValueLiteral(val)` & `BuildForeignKeyTargetQuery(ref, val)`: Safely formats integers (preserving leading-zero string codes) and single-quote-escaped string literals into backtick-quoted SQL queries.

### 2. Navigation Stack & FK Caching (`ui/fk_nav.go`)
- `fkCache`: Thread-safe `RWMutex` map keyed by `dbName.tableName`.
- `NavBreadcrumb` & `fkNavHistory`: Thread-safe LIFO stack preserving prior table, active SQL query, pagination offset, active sort column/direction, editing flag, and exact cell cursor.

### 3. Data Grid Rendering & Shortcuts (`ui/browser.go`, `ui/help.go`)
- `ExecuteQueryAsync`: Populates `QueryResult.ForeignKeys` off the UI thread.
- `renderQueryResult`: Renders `↗` on FK headers and non-NULL FK cells, updates title/footer with `Backspace: Return (N)` when history is present, and restores prior cursor position upon return.
- `setupTableInputCapture`: Routes `f` / `F` and `Ctrl+J` (on FK cells) to `triggerForeignKeyJump`, and `Backspace` / `Backspace2` to `navigateBackFromFK`.
- Documented `f / Ctrl+J` and `Backspace` in [ui/help.go](file:///c:/Users/vicky/Desktop/pj/pheri/ui/help.go).

---

## 3. Verification & Automated Testing

### Unit Tests Added:
- [util/util_test.go](file:///c:/Users/vicky/Desktop/pj/pheri/util/util_test.go): `TestFormatSQLValueLiteral`, `TestBuildForeignKeyTargetQuery`.
- [ui/fk_nav_test.go](file:///c:/Users/vicky/Desktop/pj/pheri/ui/fk_nav_test.go): `TestFKNavBreadcrumbStack`, `TestRenderQueryResult_FKVisualIndicatorsAndRawValueIntegrity`, `TestTriggerForeignKeyJump_Guards`.

### Commands Executed:
```powershell
go test -v ./util/...
go test -v ./ui -run "TestFK.*|TestRenderQueryResult_FK.*|TestTriggerForeignKeyJump.*"
go test ./...
go build -ldflags="-s -w" -o pheri.exe .
```
