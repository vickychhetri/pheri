param(
    [string]$Title = "feat(transaction): implement auto-commit toggle and transaction safety mode (PHERI-08)",
    [string]$Head = "23-pheri-08-auto-commit-toggle-f2-autocommit",
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
Closes #23

## Summary of Changes

This Pull Request implements **[PHERI-08] Auto-Commit Toggle (F2 / :autocommit)**.

In standard database workflows, accidental executions of destructive queries (`DELETE` without `WHERE`, unconstrained `UPDATE`, or inadvertent grid cell edits) take effect immediately when `autocommit=1`.
Calling `SET autocommit = 0;` on Go's `*sql.DB` connection pool is unreliable because subsequent queries may check out different underlying connections from the pool.

This PR introduces an interactive, thread-safe transaction controller backed by Go's `*sql.Tx`, providing seamless switching between Auto-Commit and manual transaction safety modes with zero risk of uncommitted mutation loss.

---

## Key Improvements

### 1. Active Transaction Management (`ui/transaction.go`)
- Thread-safe state controller using `sync.Mutex` (`txMutex`) managing `isAutoCommitMode`, `activeTx`, `uncommittedTxCount`, and `activeTxDB`.
- `EnsureActiveTx(ctx, db)`: Guarantees a dedicated `*sql.Tx` connection is active when Auto-Commit is off.
- `RecordMutationInTx()`: Tracks dirty mutation counts for display across the interface.
- `CommitActiveTx(app)`: Commits pending mutations, clears dirty count, and restarts a fresh transaction if manual mode remains active.
- `RollbackActiveTx(app)`: Rolls back pending mutations and starts a fresh transaction.
- `ToggleAutoCommit(app, db)`: Switches between Auto-Commit ON and manual transaction mode with protection prompts if uncommitted mutations exist.

### 2. Execution Routing (`ui/browser.go`)
- **SQL Editor (`ExecuteQueryAsync`)**:
  - Auto-Commit ON: Uses pooled connections (`conn := db.Conn(ctx)`).
  - Auto-Commit OFF: Routes mutations (`UPDATE`, `DELETE`, `INSERT`, etc.) through `activeTx.ExecContext` and tracks mutation counts.
  - Queries (`SELECT`, `SHOW`, etc.) route through `activeTx.QueryContext`, providing Read-Your-Own-Writes consistency for pending changes.
  - Dynamically extracts server connection ID for query cancellation via `KILL QUERY`.
- **Data Grid Inline Cell Editing (`EnableCellEditing`)**:
  - Routes in-place cell updates through `activeTx.Exec` when Auto-Commit is disabled and increments the pending mutation counter.

### 3. Exit and Database Switch Guards
- `UseDatabase`: Intercepts database switches when uncommitted mutations exist and prompts `PromptUncommittedChangesModal` ("Commit & Proceed", "Rollback & Discard", "Cancel").
- `exitButton` & `:quit` / `:exit`: Prevents quitting Pheri with uncommitted mutations without user confirmation.

### 4. UI Status Badges & Keybinding Re-Mapping (`ui/footer.go`, `ui/help.go`)
- **Live Footer Badges**:
  - `AUTOCOMMIT: ON` (Green)
  - `TX: READY` (Yellow)
  - `TX: N PENDING (:commit / :rollback)` (Orange)
- **Shortcuts**:
  - `F2`: Toggles Auto-Commit mode (`:autocommit` / `:tx`).
  - `Ctrl+H`: Searchable Query History modal (freeing `F2` for transaction toggle).
  - `:commit`, `:c`: Commits active transaction.
  - `:rollback`, `:rb`, `:abort`: Rolls back active transaction.

---

## Verification & Test Coverage

- Added unit tests in `ui/transaction_test.go`:
  - `TestAutoCommitDefaultState`
  - `TestToggleAutoCommit`
  - `TestRecordMutationInTx`
  - `TestCommitAndRollbackWithoutTx`
  - `TestEnsureActiveTxValidation`
  - `TestPromptUncommittedChangesModal`
  - `TestFooterBadgesInTransactionModes`
- Updated help guide tests in `ui/help_test.go`.
- All tests pass across the entire repository (`go test ./...`).
- Cross-platform binaries compiled successfully via `build.ps1`.
'@

$bodyObj = @{
    title = $Title
    head  = $Head
    base  = $Base
    body  = $prBody
}

$jsonBody = $bodyObj | ConvertTo-Json -Depth 5

Write-Host "Creating Pull Request on $Repo ($Head -> $Base)..."

try {
    $response = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/pulls" -Method Post -Headers $headers -Body $jsonBody -ContentType "application/json; charset=utf-8"
    Write-Host "Successfully created Pull Request #$($response.number): $($response.html_url)"
    $response.html_url
} catch {
    Write-Error "Failed to create Pull Request: $_"
    if ($_.Exception.Response) {
        $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
        $respText = $reader.ReadToEnd()
        Write-Error "Response details: $respText"
    }
    exit 1
}
