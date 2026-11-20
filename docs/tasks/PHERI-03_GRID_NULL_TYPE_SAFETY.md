# PHERI-03: Fix Grid NULL & Empty Value Type Safety

- **Task Code**: `PHERI-03`
- **Feature Name**: Fix Grid NULL & Empty Value Type Safety
- **Category**: Data Grid & Storage Integrity
- **Priority**: `P0 - Critical`
- **Status**: `Completed`
- **Target Files**: `ui/grid_cell.go`, `ui/browser.go`, `ui/grid_cell_test.go`

---

## 1. Reason of Adding

In previous versions of Pheri, database `NULL` values and empty string values `""` were rendered into the TUI table as visual color-formatted strings:
- SQL `NULL` -> `"[gray]NULL"` or `"[gray::i]NULL[-::-]"`
- Empty String `""` -> `"[gray]EMPTY"` or `"[gray::i]EMPTY[-::-]"`

When an end user selected a cell to edit its value, the inline editor extracted the raw display text `cell.Text`. As a result, the literal string `"[gray]NULL"` or `"[gray]EMPTY"` was loaded into the editing buffer. Upon saving, Pheri issued:
```sql
UPDATE `table` SET `col` = '[gray]NULL' WHERE ...
```

### Critical Real-World Failures Prevented:
1. **Silent Data Corruption**: On `VARCHAR`, `CHAR`, `TEXT`, `BLOB`, and `JSON` columns, MySQL successfully saved the literal string `"[gray]NULL"`, corrupting records.
2. **Type Mismatch Errors**: On `INT`, `BIGINT`, `DECIMAL`, `FLOAT`, `DATE`, `DATETIME`, and `TIMESTAMP` columns, MySQL rejected the update with fatal parsing errors (e.g. `Incorrect integer value: '[gray]NULL' for column 'amount'`).
3. **Inability to Assign SQL NULL**: Users had no mechanism to set a column to SQL `NULL`. Clearing the text area sent an empty string `""` instead of `NULL`.
4. **Broken `WHERE` Clauses**: For updates on tables lacking primary keys or with composite keys, `WHERE` clauses matched against `"[gray]EMPTY"`, causing queries to find 0 matching rows and fail silently.
5. **Markup Leaks in Exports**: CSV and JSON exports dumped literal `"[gray]NULL"` strings instead of empty commas `,,` or JSON `null`.

---

## 2. Technical Architecture & Implementation

### 1. The `GridCell` Type (`ui/grid_cell.go`)
Pheri now decouples visual TUI formatting from underlying database values using `GridCell`:
```go
type GridCell struct {
    RawValue *string // nil = SQL NULL; non-nil = literal string (including "" for empty)
    ColName  string  // Database column name
    IsHeader bool    // Header indicator
}
```
- `IsNull()`: Returns `RawValue == nil`.
- `IsEmpty()`: Returns `RawValue != nil && *RawValue == ""`.
- `Value()`: Returns exact unadorned string value, or `""` if NULL.
- `DisplayText()`: Renders `[gray::i]NULL[-::-]`, `[gray::i]EMPTY[-::-]`, or user text escaped with `tview.Escape` to prevent unescaped brackets from breaking TUI styling.
- `ExportValue(nullPlaceholder)`: Returns `""` for CSV export of NULLs, or raw text.

### 2. Dual-Layer Storage in `QueryResult` (`ui/browser.go`)
`QueryResult` preserves both `Rows [][]string` (for visual rendering) and `RawRows [][]*string` (for exact SQL value retention). When populating `tview.Table`, each `TableCell` has its `GridCell` pointer bound via `cell.SetReference(gridCell)`.

### 3. Interactive 3-State Cell Edit Modal (`ui/browser.go`)
The inline cell editor now features dynamic mode switching:
- **`Ctrl+N`**: Toggles between `[X] SQL NULL` and Literal Value mode. In NULL mode, the text area shows a placeholder and will bind `nil` on save.
- **`Ctrl+E`**: Clears text to an exact empty string `""`.
- **Typing characters**: If user begins typing while in NULL mode, Pheri automatically switches back to Literal Value mode.
- **`Enter`**: Commits the update:
  - If NULL: binds `nil` parameter (or `SET col = NULL`).
  - If text: binds `*valToSave`.
- **`Esc`**: Cancels and returns to table.

### 4. Sanitized `WHERE` Clause Builders
Primary key and fallback full-row `WHERE` clauses extract `GetGridCellFromTableCell`:
- If `pkGC.IsNull()`: generates `col IS NULL`.
- If `!pkGC.IsNull()`: generates `col = ?` and binds `pkGC.Value()` (exact string without padding spaces or color tags).

### 5. Clean CSV & JSON Export Serialization
- **CSV Export**: NULL cells export as empty fields `""`.
- **JSON Export**: NULL cells serialize as genuine JSON `null` (e.g. `"address2": null`), not string `"NULL"`.

---

## 3. Verification Matrix

| Test Name | Verification Focus | Result |
| :--- | :--- | :--- |
| `TestGridCell_NullAndEmptyHandling` | Validates `IsNull()`, `IsEmpty()`, and `Value()` across NULL, empty, and literal values. | PASS |
| `TestGridCell_DisplayTextFormatting` | Validates TUI markup generation and `tview.Escape` bracket escaping on user data. | PASS |
| `TestGridCell_ExportValue` | Validates clean string exports for CSV and plain text. | PASS |
| `TestGetGridCellFromTableCell_WithReference` | Validates reference extraction from `tview.TableCell`. | PASS |
| `TestGetGridCellFromTableCell_FallbackParsing` | Validates backwards-compatible text fallback parser for unreferenced cells. | PASS |
| `TestGridCell_SQLParameterBinding` | Validates that NULL binds `nil`, empty string binds `""`, and text binds string. | PASS |
| `TestGridCell_WhereClauseGeneration` | Validates `IS NULL` vs `= ?` query generation. | PASS |
| `TestGridCell_ExportSerializationFormatting` | Validates JSON `null` and empty string serialization. | PASS |
