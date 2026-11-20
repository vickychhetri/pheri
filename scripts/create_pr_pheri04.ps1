param(
    [string]$Title = "feat(engine): differentiate QueryContext vs ExecContext and add comment-aware classification (PHERI-04)",
    [string]$Head = "feature/pheri-04-query-vs-exec-context",
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
Closes #19

## Summary of Changes

This Pull Request implements **PHERI-04: Differentiate QueryContext vs ExecContext**.

Previously, SQL queries were classified as mutations using a naive `strings.HasPrefix` check. If a query began with leading comments (`-- comment`, `# hash`, or `/* block */`) or used a Common Table Expression (`WITH ... UPDATE/DELETE/INSERT`), it was misclassified as a `SELECT` query. This caused:
1. **Critical Security Vulnerability**: Comment-prepended mutations bypassed `ActiveReadOnly` protection, executing writes in a locked read-only session.
2. **Missing Execution Metrics**: Queries executed via `QueryContext` failed to capture `RowsAffected` and completely discarded `LastInsertId`.
3. **Missing DDL / TCL / Admin Support**: Statements such as `RENAME TABLE`, `OPTIMIZE TABLE`, `ANALYZE TABLE`, `SET`, `COMMIT`, and `ROLLBACK` were not categorized as mutations.

---

## Key Improvements

### 1. Comment-Aware SQL Statement Classifier (`ui/sql_classifier.go`)
- **`StripLeadingSQLComments`**: Peels away single-line comments (`--`, `#`), C-style multi-line comments (`/* ... */`), and whitespace.
- **`ClassifySQLStatement`**:
  - Classifies DML mutations: `INSERT`, `UPDATE`, `DELETE`, `REPLACE`.
  - Classifies DDL mutations: `CREATE`, `ALTER`, `DROP`, `TRUNCATE`, `RENAME`.
  - Classifies TCL mutations: `START TRANSACTION`, `BEGIN`, `COMMIT`, `ROLLBACK`, `SAVEPOINT`.
  - Classifies Admin mutations: `SET`, `GRANT`, `REVOKE`, `OPTIMIZE`, `ANALYZE`, `REPAIR`, `FLUSH`, `RESET`, `PURGE`, `KILL`.
  - Classifies Result Set queries: `SELECT`, `SHOW`, `DESCRIBE`, `EXPLAIN`, `CHECK TABLE`, `CALL`.
  - **CTE Mutation Detection**: Traverses `WITH cte AS (...)` definitions to inspect the terminal statement and accurately distinguish `WITH ... SELECT` from `WITH ... UPDATE/DELETE/INSERT`.

### 2. Dual-Route Execution Engine (`ui/browser.go`)
- **Strict Read-Only Enforcement**: Blocks any mutation query (`isMutation && ActiveReadOnly`) regardless of leading comments or CTE formatting:
  `Action Blocked: <StatementType> statement not allowed in READ-ONLY Mode.`
- **ExecContext Route**:
  - Routes all mutations via `conn.ExecContext(ctx, query)`.
  - Captures `RowsAffected()`.
  - Captures `LastInsertId()` whenever an auto-increment primary key is generated.
- **QueryContext Route**:
  - Routes all result set queries via `conn.QueryContext(ctx, query)`.

### 3. TUI Result Presentation (`ui/browser.go`)
- Mutation display now clearly reports the specific statement type, execution time, rows affected, and generated auto-increment ID:
  ```text
  Query OK, 1 row(s) affected (1.2ms) │ Last Insert ID: 1042
  ```
- Result status title bar and application footer status bar are updated dynamically:
  `[ INSERT OK: 1 rows affected (1.2ms) ]`
  `Query OK, 1 row(s) affected (1.2ms), Last Insert ID: 1042`

---

## Verification & Testing

- [x] **Unit Tests (`ui/sql_classifier_test.go`)**:
  - `TestStripLeadingSQLComments`: Validates `--`, `#`, and `/* */` comments including stacked and CRLF variations.
  - `TestClassifySQLStatement_DML`: Validates `INSERT`, `UPDATE`, `DELETE`, `REPLACE` classification.
  - `TestClassifySQLStatement_WithLeadingComments`: Validates that commented mutations cannot bypass read-only mode.
  - `TestClassifySQLStatement_CTE`: Validates CTE classification (`WITH ... SELECT` vs `WITH ... UPDATE/DELETE/INSERT`).
  - `TestClassifySQLStatement_DDL_TCL_Admin`: Validates `CREATE`, `ALTER`, `DROP`, `TRUNCATE`, `RENAME`, `SET`, `COMMIT`, `ROLLBACK`, `OPTIMIZE`, `ANALYZE`, `GRANT`.
  - `TestClassifySQLStatement_Queries`: Validates `SELECT`, `SHOW`, `DESCRIBE`, `EXPLAIN`, `CHECK TABLE`, `CALL`.
  - `TestQueryResult_LastInsertIDReporting`: Validates `LastInsertID` capture and formatting.
- [x] **Full Test Suite**: All 35 unit tests in `mysql-tui/ui` passing cleanly (`go test -v ./...`).
- [x] **Compilation**: Built standalone Windows binaries (`pheri.exe` and `build/pheri-windows-amd64.exe`).
- [x] **Documentation**: Created comprehensive documentation at `docs/tasks/PHERI-04_DIFFERENTIATE_QUERY_VS_EXEC.md`.
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
