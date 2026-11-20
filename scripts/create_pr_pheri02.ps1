param(
    [string]$Title = "feat(engine): active query abort on Ctrl+C / Esc with MySQL server-side KILL QUERY (PHERI-02)",
    [string]$Head = "feature/pheri-02-active-query-abort",
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
## Summary of Changes

This Pull Request implements **PHERI-02: Active Query Abort on `Ctrl+C` / `Esc` with MySQL Server-Side `KILL QUERY <id>`**.

Previously, query execution lacked a server-side termination mechanism. Cancelling a query on the client left MySQL continuing to execute heavy queries (e.g. table scans, recursive CTEs, unindexed joins) in the background, consuming server CPU, disk I/O, and buffer pool resources. Additionally, pressing `Ctrl+C` when no query was executing could inadvertently terminate the terminal session.

---

## Key Improvements

### 1. Dedicated Connection Reservation & Thread Tracking (`ui/browser.go`)
- **Dedicated Pool Connection**: Switched execution from pool queries to `conn, err := db.Conn(ctx)`. This guarantees that all operations occur on a single physical MySQL connection.
- **Connection Thread Identification**: Prior to query execution, Pheri queries `SELECT CONNECTION_ID()` on the dedicated connection and records the thread ID atomically (`activeQueryConnID`).
- **Connection Cleanup**: Ensures `defer conn.Close()` always returns the connection to the driver pool safely, even after server-side cancellation.

### 2. Dual-Layer Asynchronous Query Cancellation
- **Context Cancellation**: Pressing `Ctrl+C` or `Esc` cancels the Go `context.Context` immediately.
- **Server-Side Thread Termination**: If a valid `activeQueryConnID` is tracked, Pheri asynchronously launches an auxiliary background connection to issue:
  ```sql
  KILL QUERY <thread_id>
  ```
  with an aggressive 3-second timeout.
- This immediately signals the MySQL server engine to abort statement execution without killing the user session.

### 3. Error Classification & Graceful Interruption Handling
- **MySQL Error 1317 Detection**: When MySQL aborts a query via `KILL QUERY`, it returns error code `1317 (70100): Query execution was interrupted`.
- **Classification**: Pheri detects `1317` and `"interrupted"` strings, cleanly categorizing the outcome as `IsCanceled = true` rather than a catastrophic database error.
- **Query Results Status**: Renders `[!] Query canceled by user (MySQL Thread <id> killed on server)` with total elapsed milliseconds.

### 4. Accidental Exit Protection (Idle `Ctrl+C` Safeguard)
- **Problem**: Pressing `Ctrl+C` while browsing or editing queries previously risked abruptly terminating the TUI application.
- **Solution**:
  - In `ui/browser.go`, `app.SetInputCapture` intercepts `Ctrl+C`:
    - If a query is currently running: immediately triggers `CancelActiveQuery()`.
    - If no query is running: notifies the user in the footer (`"Press Ctrl+Q or type :quit to exit Pheri"`) without quitting.
  - Standard clipboard copying inside the SQL Editor textarea is preserved without conflict.
  - OS-level signal handling in `main.go` gracefully checks for active running queries on `os.Interrupt` / `SIGTERM`.

### 5. Live Footer & Thread Visibility (`ui/footer.go`)
- The footer running status now explicitly displays the active MySQL thread ID and cancellation guidance:
  ```
  Executing Query... (2.3s) (Thread 42) | Ctrl+C / Esc: Cancel (Server Kill)
  ```

---

## Verification & Testing

- [x] **Unit Tests (`ui/query_async_test.go`)**:
  - `TestQueryResultState`: Validates state transitions.
  - `TestQueryCancellationMechanism`: Validates cancellation flag, elapsed time recording, and connection ID cleanup.
  - `TestInterruptedMySQLErrorClassification`: Validates error code 1317 and "interrupted" string mapping to `IsCanceled = true`.
  - `TestCancelActiveQueryWhenIdle`: Validates safe no-op behavior when cancel is invoked while idle.
- [x] **Full Test Suite**: All 17 unit tests passing cleanly (`go test -v ./...`).
- [x] **Compilation**: Built standalone Windows binaries (`pheri.exe` and `build/pheri-windows-amd64.exe`).
- [x] **Documentation**: Created comprehensive architectural and verification document `docs/tasks/PHERI-02_ACTIVE_QUERY_ABORT.md`.
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
