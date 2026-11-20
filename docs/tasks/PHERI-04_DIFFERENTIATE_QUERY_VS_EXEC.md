# PHERI-04: Differentiate QueryContext vs ExecContext

- **Task Code**: `PHERI-04`
- **Feature Name**: Differentiate QueryContext vs ExecContext
- **Category**: Core Engine & Statement Execution
- **Priority**: `P0 - Critical`
- **Status**: `Completed`
- **Target Files**: `ui/sql_classifier.go`, `ui/browser.go`, `ui/sql_classifier_test.go`
- **GitHub Issue**: [#19](https://github.com/vickychhetri/pheri/issues/19)

---

## 1. Reason of Adding

In standard relational database drivers, statements must be routed to either:
1. `QueryContext`: For queries expecting a tabular data stream (`SELECT`, `SHOW`, `DESCRIBE`, `EXPLAIN`).
2. `ExecContext`: For statements modifying data, schema, or session state (`INSERT`, `UPDATE`, `DELETE`, `CREATE`, `ALTER`, `DROP`, `TRUNCATE`, `SET`, `TCL`, `Admin`).

### Critical Real-World Failures Prevented:
1. **Comment-Prepended Mutations Bypassed Detection**:
   In previous versions of Pheri, mutations were detected via `strings.HasPrefix(cleanQ, "INSERT")`. If a user wrote comments at the top of the query:
   ```sql
   -- weekly active user update
   UPDATE users SET status = 'active' WHERE last_login > NOW() - INTERVAL 7 DAY;
   ```
   or `/* block comment */ DELETE FROM logs;`, the query string began with `--` or `/*`. `isMutation` evaluated to `false`.
   - **Critical Security Hole**: The query bypassed `ActiveReadOnly` protection and executed in what was supposed to be a locked read-only session!
   - **Broken Metrics**: Executing via `QueryContext` failed to report `RowsAffected` and `LastInsertId`, leaving the user with an empty or confusing table.
2. **CTE Mutations (`WITH ... UPDATE/DELETE/INSERT`)**:
   Common Table Expressions with DML mutations begin with `WITH`, causing them to execute via `QueryContext` instead of `ExecContext`.
3. **Missing DDL, TCL, and Admin Statements**:
   `RENAME TABLE`, `OPTIMIZE TABLE`, `ANALYZE TABLE`, `SET`, `LOCK TABLES`, `START TRANSACTION`, and `COMMIT` were not recognized as mutations.
4. **Missing `LastInsertId` Reporting**:
   When an `INSERT` statement generated an auto-increment primary key, Pheri never displayed `LastInsertId()`. Users had to run a separate query (`SELECT LAST_INSERT_ID()`) to find the new ID.

---

## 2. Technical Architecture & Implementation

### 1. Comment-Aware SQL Classifier (`ui/sql_classifier.go`)
- **`StripLeadingSQLComments(query string) string`**:
  Robustly peels away:
  - `-- line comments\n`
  - `# mysql comments\n`
  - `/* multi-line block comments */`
  - Leading whitespace
- **`ClassifySQLStatement(query string) (isMutation bool, stmtType string)`**:
  - Classifies statement types accurately regardless of comments.
  - Detects CTE mutations: parses `WITH cte AS (...)` and inspects the terminal statement (`SELECT`, `UPDATE`, `DELETE`, `INSERT`).
  - Covers all DML (`INSERT`, `UPDATE`, `DELETE`, `REPLACE`), DDL (`CREATE`, `ALTER`, `DROP`, `TRUNCATE`, `RENAME`), TCL (`START`, `BEGIN`, `COMMIT`, `ROLLBACK`), and Admin statements (`SET`, `GRANT`, `REVOKE`, `OPTIMIZE`, `ANALYZE`, `KILL`).

### 2. Dual-Route Execution Engine (`ui/browser.go`)
- **Read-Only Enforcement**: If `isMutation && ActiveReadOnly`, execution is strictly blocked with `Action Blocked: <stmtType> statement not allowed in READ-ONLY Mode.`
- **ExecContext Route**:
  - Executes `conn.ExecContext(ctx, query)`.
  - Captures `RowsAffected()`.
  - Captures `LastInsertId()` if `> 0`.
- **QueryContext Route**:
  - Executes `conn.QueryContext(ctx, query)` for all result set queries.

### 3. TUI Result Presentation (`ui/browser.go`)
- Mutation display:
  `Query OK, %d row(s) affected (Execution time: %s) │ Last Insert ID: %d`
- Status title bar:
  `[ <StatementType> OK: %d rows affected (%s) ]`
- Footer status bar:
  `Query OK, %d row(s) affected (%s), Last Insert ID: %d`

---

## 3. Verification Matrix

| Test Suite / Method | Verification Target | Result |
| :--- | :--- | :--- |
| `TestStripLeadingSQLComments` | Validates stripping of `--`, `#`, and `/* */` comments including stacked and CRLF variations. | PASS |
| `TestClassifySQLStatement_DML` | Validates `INSERT`, `UPDATE`, `DELETE`, `REPLACE` classification. | PASS |
| `TestClassifySQLStatement_WithLeadingComments` | Validates that commented mutations are accurately detected as mutations and cannot bypass read-only mode. | PASS |
| `TestClassifySQLStatement_CTE` | Validates CTE classification: `WITH ... SELECT` vs `WITH ... UPDATE/DELETE/INSERT`. | PASS |
| `TestClassifySQLStatement_DDL_TCL_Admin` | Validates `CREATE`, `ALTER`, `DROP`, `TRUNCATE`, `RENAME`, `SET`, `COMMIT`, `ROLLBACK`, `OPTIMIZE`, `ANALYZE`, `GRANT`. | PASS |
| `TestClassifySQLStatement_Queries` | Validates `SELECT`, `SHOW`, `DESCRIBE`, `EXPLAIN`, `CHECK TABLE`, `CALL`. | PASS |
| `TestQueryResult_LastInsertIDReporting` | Validates `LastInsertID` capture and formatting. | PASS |
