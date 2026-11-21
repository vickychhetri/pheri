# PHERI-08: Auto-Commit Toggle (F2 / :autocommit)

- **Task Code**: `PHERI-08`
- **Feature Name**: Auto-Commit Toggle (F2 / :autocommit)
- **Category**: Transaction Safety & Mutation Guard
- **Priority**: `P1 - High`
- **Status**: `Completed`
- **Target Files**: `ui/transaction.go`, `ui/transaction_test.go`, `ui/browser.go`, `ui/footer.go`, `ui/help.go`, `ui/help_test.go`
- **GitHub Issue**: [#23](https://github.com/vickychhetri/pheri/issues/23)

---

## 1. Reason of Adding

In standard relational database workflows (especially across staging and production environments), accidental executions of destructive DML queries (`DELETE FROM users;` without a `WHERE` clause, unconstrained `UPDATE` statements, or inadvertent grid cell edits) take effect immediately when `autocommit=1`.

### Critical Real-World Hazards Addressed:
1. **Accidental Instant Mutations**:
   Developers running queries or running ad-hoc fixes can mistakenly modify or drop thousands of rows before realizing the syntax mistake.
2. **Missing Rollback Safety Net**:
   Without an interactive transaction boundary, mistakes require restoring backups or replaying binlogs, causing downtime and potential data loss.
3. **Database Connection Pool Transaction Isolation in Go**:
   Calling `SET autocommit = 0;` on Go's `*sql.DB` connection pool is unreliable because subsequent queries may check out different underlying connections from the pool. Proper transaction management requires a persistent `*sql.Tx` handle tied to the active session.

---

## 2. Technical Architecture & Implementation Design

### 1. Active Transaction Management (`ui/transaction.go`)
- Thread-safe transaction controller synchronized via `sync.Mutex` (`txMutex`).
- States tracked:
  - `isAutoCommitMode` (boolean, defaults to `true`).
  - `activeTx` (`*sql.Tx`, holds active manual transaction when Auto-Commit is OFF).
  - `uncommittedTxCount` (integer, counts executed mutations inside the active transaction).
  - `activeTxDB` (`*sql.DB`, remembers active database handle).
- Key operations:
  - `IsAutoCommitMode()`: Returns whether auto-commit is enabled.
  - `EnsureActiveTx(ctx, db)`: Guarantees a valid `*sql.Tx` is open on `db` when Auto-Commit is disabled.
  - `RecordMutationInTx()`: Increments the dirty mutation counter and updates footer status.
  - `CommitActiveTx(app)`: Commits pending mutations, resets the uncommitted count, and seamlessly starts a fresh transaction if manual mode remains active.
  - `RollbackActiveTx(app)`: Rolls back pending mutations, resets the dirty counter, and starts a fresh transaction.
  - `ToggleAutoCommit(app, db)`: Toggles mode between Auto-Commit ON and manual transaction mode. If uncommitted mutations exist when switching back to Auto-Commit ON, prompts the user to Commit, Rollback, or Cancel.
  - `PromptUncommittedChangesModal(app, actionName, onCommit, onRollback, onCancel)`: Modal dialog displaying pending count and preventing accidental loss.

### 2. Execution Routing (`ui/browser.go`)
- **Query Execution (`ExecuteQueryAsync`)**:
  - When `IsAutoCommitMode()` is true: checks out connection via `db.Conn(ctx)` and executes directly.
  - When `!IsAutoCommitMode()`:
    - Queries `EnsureActiveTx(ctx, db)` for the active transaction handle.
    - Resolves server thread ID via `tx.QueryRowContext(ctx, "SELECT CONNECTION_ID()")`.
    - Mutations execute via `tx.ExecContext(ctx, query)`. On success, calls `RecordMutationInTx()`.
    - Reads execute via `tx.QueryContext(ctx, query)`, providing Read-Your-Own-Writes consistency for pending uncommitted rows.
- **In-Place Cell Editing (`EnableCellEditing`)**:
  - When `!IsAutoCommitMode()`, cell updates execute on `tx.Exec(updateQuery, queryArgs...)` and increment the pending mutation counter via `RecordMutationInTx()`.

### 3. Context Guard & Dirty Transaction Protection
- **Database Switching Guard (`UseDatabase`)**:
  - If `GetUncommittedTxCount() > 0`, intercepts the database switch and displays `PromptUncommittedChangesModal` with "Commit & Proceed", "Rollback & Discard", or "Cancel".
- **Application Exit Guard (`exitButton` & `:quit` / `:exit` / `:q`)**:
  - If uncommitted mutations exist, prevents application termination until the user explicitly commits or rolls back.

### 4. Status Badges & Keybindings (`ui/footer.go`, `ui/help.go`)
- **Live Footer Badges**:
  - Auto-Commit Enabled: `AUTOCOMMIT: ON` (Green).
  - Manual Mode (Clean): `TX: READY` (Yellow).
  - Manual Mode (Dirty): `TX: N PENDING (:commit / :rollback)` (Orange).
- **Shortcut & Command Re-mapping**:
  - `F2`: Toggles Auto-Commit mode (`:autocommit` / `:tx`).
  - `Ctrl+H`: Searchable Query History modal (freeing `F2` from query history while keeping `:history`).
  - `:commit`, `:c`: Commits active transaction.
  - `:rollback`, `:rb`, `:abort`: Rolls back active transaction.

---

## 3. Test Coverage

Comprehensive unit tests implemented in `ui/transaction_test.go`:
- `TestAutoCommitDefaultState`: Validates default state (`isAutoCommitMode = true`, `activeTx = nil`, `uncommittedTxCount = 0`).
- `TestToggleAutoCommit`: Validates toggling between Auto-Commit ON and manual transaction mode.
- `TestRecordMutationInTx`: Verifies mutation counting and resetting.
- `TestCommitAndRollbackWithoutTx`: Confirms safe no-op when commit or rollback is called with 0 pending changes.
- `TestEnsureActiveTxValidation`: Verifies nil DB validation and error handling in manual mode.
- `TestPromptUncommittedChangesModal`: Validates modal construction, button options, and callback routing.
- `TestFooterBadgesInTransactionModes`: Tests live badge updates across `AUTOCOMMIT: ON`, `TX: READY`, and `TX: N PENDING`.
- `TestGetLaymanHelpGuide`: Confirms help documentation contains `F2`, `Ctrl+H`, `:autocommit`, `:commit`, and `:rollback`.
