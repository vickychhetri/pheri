# PHERI-09: Explicit Transaction Controls (:commit / :rollback)

- **Task Code**: `PHERI-09`
- **Feature Name**: Explicit Transaction Controls (:commit / :rollback)
- **Category**: Transactions & Mutation Ergonomics
- **Priority**: `P1 - High`
- **Status**: `Completed`
- **Target Files**: `ui/transaction.go`, `ui/transaction_test.go`, `ui/browser.go`, `ui/footer.go`, `ui/sqleditor.go`, `ui/help.go`, `ui/help_test.go`
- **GitHub Issue**: [#24](https://github.com/vickychhetri/pheri/issues/24)

---

## 1. Reason of Adding

While PHERI-08 introduced underlying session transaction isolation (`*sql.Tx`) and Auto-Commit toggling, operators had no interactive buttons in the query toolbar to commit or rollback without typing terminal commands (`:commit` / `:rollback`). Furthermore, operators had no mechanism to inspect the list of pending mutations executed inside an active transaction before committing them to production.

### Critical Operational Problems Addressed:
1. **Missing Visual Workspace Controls**:
   Operators had to manually type footer commands instead of clicking or tabbing through intuitive `[ Commit ]` and `[ Rollback ]` action buttons in the query toolbar.
2. **Lack of Mutation Audit Visibility**:
   Developers testing complex migration scripts or multi-step DML had no way to inspect an audit trail of executed queries and elapsed transaction duration before making permanent changes.
3. **No Automatic Visual Feedback upon Rollback**:
   When rolling back, the grid table retained stale rows unless manually queried again.

---

## 2. Technical Architecture & Implementation Details

### 1. Mutation Audit Trail & Duration Tracking (`ui/transaction.go`)
- Added `TxMutationRecord` struct:
  ```go
  type TxMutationRecord struct {
      Index     int
      Timestamp time.Time
      Statement string
      Source    string // "SQL Editor" or "Grid Cell Edit"
  }
  ```
- Tracked state:
  - `txMutations []TxMutationRecord`: Ordered audit log of all mutations inside the active transaction.
  - `txStartTime time.Time`: Marks the start of the transaction on the first mutation.
  - `onTxStateChangedHook func()`: Synchronous event hook invoked whenever transaction state, count, or mutations change.
- Functions:
  - `RecordMutationInTx(details ...string)`: Appends an audit record with timestamp, statement, and source. Fully backward-compatible with zero-argument invocations.
  - `GetActiveTxMutations() []TxMutationRecord`: Returns a copy of the audit records.
  - `GetTxDuration() time.Duration`: Computes elapsed duration.
  - `SetOnTxStateChangedHook(hook func())`: Registers reactive listeners.

### 2. Transaction Review & Audit Modal (`ShowTransactionReviewModal`)
- Invoked via `:review`, `:tx`, `:audit`, or `:status`.
- Features:
  - Header: Active mode, elapsed transaction duration (`Xm Ys`), and pending mutation count.
  - Scrollable Table: Lists `#`, `Timestamp` (HH:MM:SS), `Source` (`SQL Editor` / `Grid Cell Edit`), and `Statement`.
  - Actions:
    - `[ Commit All (F10) ]`: Closes modal and executes `CommitActiveTx`.
    - `[ Rollback All (Shift+F10) ]`: Closes modal, executes `RollbackActiveTx`, and refreshes the data grid.
    - `[ Close (Esc) ]`: Closes modal without modifying transaction state.

### 3. Interactive Toolbar Controls (`ui/browser.go`)
- Added `commitButton` and `rollbackButton` into the query panel toolbar:
  `[ Run ]  [ Commit ]  [ Rollback ]  [ Save ]  [ Load ]  [ Exit ]`
- Dynamic Visual Styling (`updateTransactionToolbarButtons`):
  - When Auto-Commit is ON: Buttons are rendered in dim gray (`[ Commit ]`, `[ Rollback ]`).
  - When Auto-Commit is OFF with 0 pending: Styled cyan (`[ Commit (0) ]`, `[ Rollback (0) ]`).
  - When Auto-Commit is OFF with N pending: Styled bold green (`[ Commit (N) ]`) and bold red (`[ Rollback (N) ]`).
- Tab Navigation Chain:
  `runButton -> commitButton -> rollbackButton -> button1 (Save) -> button2 (Load) -> exitButton -> dataTable`.
  Pressing `Esc` on any toolbar button returns focus to the SQL Editor.

### 4. Fast Hotkeys
- `F10`: Commit active transaction (`CommitActiveTx`).
- `Shift+F10` / `Alt+Backspace`: Rollback active transaction (`RollbackActiveTx`) and reload data grid.
- `Ctrl+Enter`: In toolbar, data table, or on empty editor, commits active transaction if mutations are pending; in SQL Editor with typed query, executes query.
- `:review` / `:tx`: Opens Transaction Review Modal.

### 5. Knowledge Transfer (KT) in Help Guide (`ui/help.go`)
Added explicit step-by-step Knowledge Transfer section in the comprehensive help reference:
1. Turn Auto-Commit OFF with `F2`.
2. Run mutations or edit grid cells; toolbar dynamically shows `Commit (N)` and `Rollback (N)`.
3. Type `:review` to inspect the exact statements and timestamps before committing.
4. Press `F10` to Commit permanently or `Shift+F10` to Rollback safely.

---

## 3. Test Coverage

Comprehensive unit tests implemented in `ui/transaction_test.go`:
- `TestTxMutationRecordAuditTrail`: Validates statement, source, and timestamp recording, duration tracking, and reset clearing.
- `TestShowTransactionReviewModal`: Verifies modal construction with audit table and action buttons.
- `TestSetOnTxStateChangedHook`: Verifies callback dispatch on toggle, mutation recording, and reset.
- `TestAutoCommitDefaultState`
- `TestToggleAutoCommit`
- `TestRecordMutationInTx`
- `TestCommitAndRollbackWithoutTx`
- `TestEnsureActiveTxValidation`
- `TestPromptUncommittedChangesModal`
- `TestFooterBadgesInTransactionModes`
- `TestGetLaymanHelpGuide` in `ui/help_test.go` verifying `"F10"` and `":review"`.
