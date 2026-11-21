param(
    [string]$Title = "feat(transaction): implement explicit transaction controls, toolbar buttons, and review modal (PHERI-09)",
    [string]$Head = "24-pheri-09-explicit-transaction-controls-commit-rollback",
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
Closes #24

## Summary of Changes

This Pull Request implements **[PHERI-09] Explicit Transaction Controls (:commit / :rollback)**.

Building upon session transaction isolation, this PR adds explicit visual controls in the workspace toolbar, fast keyboard shortcuts, and a full transaction audit review modal before committing.

---

## Key Improvements

### 1. Interactive Workspace Toolbar Controls (`ui/browser.go`)
- Added `[ Commit ]` and `[ Rollback ]` buttons to the query action bar:
  `[ Run ]  [ Commit ]  [ Rollback ]  [ Save ]  [ Load ]  [ Exit ]`
- Dynamic Visual Styling:
  - Auto-Commit ON: Rendered dim gray.
  - Manual Mode with 0 pending: `Commit (0)` and `Rollback (0)` in cyan.
  - Manual Mode with N pending: `Commit (N)` in bold green and `Rollback (N)` in bold red.
- Tab Chain:
  `runButton -> commitButton -> rollbackButton -> button1 (Save) -> button2 (Load) -> exitButton -> dataTable`.

### 2. Transaction Review & Audit Modal (`ShowTransactionReviewModal`)
- Accessible via `:review`, `:tx`, `:audit`, or `:status`.
- Shows:
  - Header: Active mode, elapsed transaction duration (`Xm Ys`), and pending mutation count.
  - Scrollable Table: Lists `#`, `Timestamp` (HH:MM:SS), `Source` (`SQL Editor` / `Grid Cell Edit`), and `Statement`.
  - Action buttons: `[ Commit All (F10) ]`, `[ Rollback All (Shift+F10) ]`, and `[ Close (Esc) ]`.

### 3. Fast Hotkeys & Automated Data Grid Refresh
- `F10`: Commit active transaction (`CommitActiveTx`).
- `Shift+F10` / `Alt+Backspace`: Rollback active transaction (`RollbackActiveTx`).
- `Ctrl+Enter`: Contextual commit outside active text editing.
- Automatic Data Grid Refresh: When rolling back, automatically re-executes the active grid query so operators visually see reverted rows.

### 4. Mutation Audit Trail & State Hooks (`ui/transaction.go`)
- Added `TxMutationRecord` struct logging index, timestamp, statement, and source.
- `RecordMutationInTx(details ...string)`: Logs statement and source while preserving zero-argument backward compatibility.
- `GetActiveTxMutations()` and `GetTxDuration()`: Exposed for UI modals and telemetry.
- `SetOnTxStateChangedHook`: Reactive event hook ensuring toolbar buttons immediately reflect database transaction state.

### 5. Knowledge Transfer (KT) in Help Guide (`ui/help.go`)
- Added comprehensive step-by-step Knowledge Transfer section in the user guide explaining how transactions operate and how to use the review modal and hotkeys.

---

## Verification & Test Coverage

- Added unit tests in `ui/transaction_test.go`:
  - `TestTxMutationRecordAuditTrail`
  - `TestTxDurationTracking`
  - `TestShowTransactionReviewModal`
  - `TestSetOnTxStateChangedHook`
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
