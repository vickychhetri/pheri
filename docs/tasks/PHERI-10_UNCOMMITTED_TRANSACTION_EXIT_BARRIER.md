# PHERI-10: Uncommitted Transaction Exit Barrier

- **Task Code**: `PHERI-10`
- **Feature Name**: Uncommitted Transaction Exit Barrier
- **Category**: Transactions
- **Priority**: `P1 - High`
- **Status**: `Completed`
- **Target Files**: `ui/browser.go`, `ui/connect.go`, `ui/transaction.go`, `ui/footer.go`, `ui/transaction_test.go`, `ui/connect_test.go`
- **GitHub Issue**: [#25](https://github.com/vickychhetri/pheri/issues/25)

---

## 1. Reason of Adding

When running in manual transaction mode (`Auto-Commit: OFF`), uncommitted DML statements (`INSERT`, `UPDATE`, `DELETE`) hold open transactions (`*sql.Tx`) and database row/table locks in MySQL. Previously, certain exit and navigation paths (such as returning to the login screen from the database list or pressing `Esc` in the sidebar search input) could switch context or close `*sql.DB` immediately without warning, silently abandoning uncommitted changes and risking data loss or lingering locks.

### Critical Operational Problems Addressed:
1. **Back-to-Login Silent Connection Drop (`ui/connect.go`)**:
   Calling `backToLogin()` from `ShowDatabaseList` closed the active `*sql.DB` connection unconditionally without inspecting `GetUncommittedTxCount()`.
2. **Sidebar Escape Navigation Bypass (`ui/browser.go`)**:
   Pressing `Esc` while focused on the sidebar `searchInput` navigated straight to `ShowDatabaseList` even with pending uncommitted changes.
3. **Modal Context Restoration & Label Clarity (`ui/transaction.go`)**:
   `PromptUncommittedChangesModal` always restored `mainFlex` upon cancellation regardless of whether it was triggered from `"browser"` or `"dblist"`, and did not differentiate `[Commit & Exit]` / `[Rollback & Exit]` button labels for exit flows.
4. **Command Bar Database Switching (`ui/footer.go`)**:
   The `:use <db>` command advertised in the footer command bar placeholder was not wired to `UseDatabase`.

---

## 2. Technical Architecture & Implementation Details

### 1. Screen-Aware Exit Barrier Modal (`ui/transaction.go`)
- Updated `PromptUncommittedChangesModal`:
  - Dynamically selects `[Commit & Exit]` / `[Rollback & Exit]` / `[Cancel]` when invoked for exit, quit, disconnect, or login navigation, and `[Commit & Proceed]` / `[Rollback & Discard]` / `[Cancel]` for database or auto-commit mode switches.
  - Formats prompt text using `FormatThemeMarkup()` for WCAG AAA readability in Light and Dark themes.
  - Updates `closeModal()` to inspect `ActiveScreen` (`"dblist"`, `"login"`, `"browser"`) so cancelling safely returns the operator to their exact previous screen and focus.

### 2. Protected Back-to-Login Flow (`ui/connect.go`)
- Extracted `attemptBackToLogin(app *tview.Application, db *sql.DB) bool`:
  - Checks `GetUncommittedTxCount() > 0` before closing `*sql.DB` or clearing active credentials.
  - Prompts `[Commit & Exit]`, `[Rollback & Exit]`, or `[Cancel]`.
  - Resets transaction tracking cleanly via `ResetTransactionState()` once committed or rolled back.

### 3. Browser & Command Bar Guards (`ui/browser.go`, `ui/footer.go`)
- Guarded `searchInput`'s `KeyEscape` handler in `ui/browser.go` with `GetUncommittedTxCount() > 0` before invoking `ShowDatabaseList`.
- Added error checks (`err == nil && activeTx == nil`) inside `UseDatabase`'s commit/rollback callbacks to avoid re-prompt loops if a commit fails.
- Implemented `:use <database>` in `handleGlobalCommand` (`ui/footer.go`), routing directly through `UseDatabase`.

---

## 3. Verification & Automated Testing

### Unit Tests Added:
- `TestPromptUncommittedChangesModal_DynamicLabelsAndScreens` in [ui/transaction_test.go](file:///c:/Users/vicky/Desktop/pj/pheri/ui/transaction_test.go): Validates dynamic exit vs context-switch button labels and `ActiveScreen == "dblist"` cancel handling.
- `TestAttemptBackToLogin_UncommittedBarrier` in [ui/connect_test.go](file:///c:/Users/vicky/Desktop/pj/pheri/ui/connect_test.go): Verifies immediate logout when clean and interception when pending mutations exist.

### Commands Executed:
```powershell
go test -v ./ui -run "TestPromptUncommittedChangesModal.*|TestAttemptBackToLogin.*"
go test -v ./...
```
