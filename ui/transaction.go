// ui/transaction.go
package ui

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var (
	txMutex            sync.Mutex
	isAutoCommitMode   = true
	activeTx           *sql.Tx
	uncommittedTxCount = 0
	activeTxDB         *sql.DB
)

// IsAutoCommitMode returns whether auto-commit is enabled (true) or manual transactions are active (false).
func IsAutoCommitMode() bool {
	txMutex.Lock()
	defer txMutex.Unlock()
	return isAutoCommitMode
}

// SetAutoCommitMode updates auto-commit mode directly (primarily for testing/resets).
func SetAutoCommitMode(enabled bool) {
	txMutex.Lock()
	defer txMutex.Unlock()
	isAutoCommitMode = enabled
	if enabled && activeTx != nil {
		_ = activeTx.Rollback()
		activeTx = nil
		uncommittedTxCount = 0
	}
}

// GetActiveTx returns the current active *sql.Tx if in manual transaction mode, or nil.
func GetActiveTx() *sql.Tx {
	txMutex.Lock()
	defer txMutex.Unlock()
	return activeTx
}

// GetUncommittedTxCount returns the number of uncommitted mutations executed in the active transaction.
func GetUncommittedTxCount() int {
	txMutex.Lock()
	defer txMutex.Unlock()
	return uncommittedTxCount
}

// ResetTransactionState resets all transaction tracking variables (used for tests or clean disconnects).
func ResetTransactionState() {
	txMutex.Lock()
	defer txMutex.Unlock()
	if activeTx != nil {
		_ = activeTx.Rollback()
	}
	activeTx = nil
	uncommittedTxCount = 0
	isAutoCommitMode = true
	activeTxDB = nil
}

// EnsureActiveTx verifies or begins a transaction on the provided database handle when autocommit is off.
func EnsureActiveTx(ctx context.Context, db *sql.DB) (*sql.Tx, error) {
	txMutex.Lock()
	defer txMutex.Unlock()

	if isAutoCommitMode {
		return nil, nil
	}
	if activeTx != nil {
		return activeTx, nil
	}
	if db == nil {
		return nil, fmt.Errorf("cannot begin transaction: database connection is nil")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	activeTx = tx
	activeTxDB = db
	return activeTx, nil
}

// RecordMutationInTx increments the uncommitted mutation count and updates the status footer.
func RecordMutationInTx() {
	txMutex.Lock()
	uncommittedTxCount++
	txMutex.Unlock()

	updateFooterText("Pending changes in active transaction")
}

// CommitActiveTx commits all pending mutations in the current transaction.
// If manual transaction mode is still active, it opens a fresh transaction for subsequent queries.
func CommitActiveTx(app *tview.Application) (int, error) {
	txMutex.Lock()
	if activeTx == nil || uncommittedTxCount == 0 {
		txMutex.Unlock()
		updateFooterText("No active uncommitted changes to commit.")
		return 0, nil
	}

	err := activeTx.Commit()
	if err != nil {
		txMutex.Unlock()
		errMsg := fmt.Sprintf("Commit failed: %v", err)
		updateFooterText(errMsg)
		return 0, err
	}

	committedCount := uncommittedTxCount
	uncommittedTxCount = 0
	activeTx = nil

	// If still in manual mode, immediately begin a fresh transaction
	if !isAutoCommitMode && activeTxDB != nil {
		newTx, beginErr := activeTxDB.BeginTx(context.Background(), nil)
		if beginErr == nil {
			activeTx = newTx
		}
	}
	txMutex.Unlock()

	msg := fmt.Sprintf("Committed %d mutation(s) successfully.", committedCount)
	updateFooterText(msg)
	return committedCount, nil
}

// RollbackActiveTx rolls back all pending mutations in the current transaction.
// If manual transaction mode is still active, it opens a fresh transaction.
func RollbackActiveTx(app *tview.Application) (int, error) {
	txMutex.Lock()
	if activeTx == nil || uncommittedTxCount == 0 {
		txMutex.Unlock()
		updateFooterText("No active uncommitted changes to rollback.")
		return 0, nil
	}

	err := activeTx.Rollback()
	if err != nil {
		txMutex.Unlock()
		errMsg := fmt.Sprintf("Rollback failed: %v", err)
		updateFooterText(errMsg)
		return 0, err
	}

	rolledBackCount := uncommittedTxCount
	uncommittedTxCount = 0
	activeTx = nil

	// If still in manual mode, immediately begin a fresh transaction
	if !isAutoCommitMode && activeTxDB != nil {
		newTx, beginErr := activeTxDB.BeginTx(context.Background(), nil)
		if beginErr == nil {
			activeTx = newTx
		}
	}
	txMutex.Unlock()

	msg := fmt.Sprintf("Rolled back %d mutation(s).", rolledBackCount)
	updateFooterText(msg)
	return rolledBackCount, nil
}

// ToggleAutoCommit toggles between auto-commit enabled and manual transaction mode.
func ToggleAutoCommit(app *tview.Application, db *sql.DB) {
	txMutex.Lock()
	currentMode := isAutoCommitMode
	dirtyCount := uncommittedTxCount
	txMutex.Unlock()

	if currentMode {
		// Switch to Manual Transaction Mode (Autocommit OFF)
		txMutex.Lock()
		isAutoCommitMode = false
		uncommittedTxCount = 0
		if db != nil {
			activeTxDB = db
			newTx, err := db.BeginTx(context.Background(), nil)
			if err == nil {
				activeTx = newTx
			}
		}
		txMutex.Unlock()

		updateFooterText("Auto-Commit disabled. Manual transaction mode ACTIVE (F2 / :commit / :rollback).")
	} else {
		// Switch to Auto-Commit Mode (Autocommit ON)
		if dirtyCount > 0 {
			// Prompt user before switching to avoid accidental loss or commit
			PromptUncommittedChangesModal(app, "enable Auto-Commit",
				func() {
					_, _ = CommitActiveTx(app)
					SetAutoCommitMode(true)
					updateFooterText("Auto-Commit enabled.")
				},
				func() {
					_, _ = RollbackActiveTx(app)
					SetAutoCommitMode(true)
					updateFooterText("Auto-Commit enabled.")
				},
				func() {
					updateFooterText("Auto-Commit toggle cancelled.")
				},
			)
			return
		}

		// Clean switch
		txMutex.Lock()
		if activeTx != nil {
			_ = activeTx.Rollback()
			activeTx = nil
		}
		isAutoCommitMode = true
		uncommittedTxCount = 0
		txMutex.Unlock()

		updateFooterText("Auto-Commit enabled.")
	}
}

// PromptUncommittedChangesModal presents a confirmation dialog when exiting or switching context with uncommitted mutations.
func PromptUncommittedChangesModal(app *tview.Application, actionName string, onCommit func(), onRollback func(), onCancel func()) *tview.Modal {
	count := GetUncommittedTxCount()

	modalText := fmt.Sprintf("[red::b]Pending Uncommitted Changes[::-]\n\nYou have [yellow::b]%d pending mutation(s)[::-] in your active transaction.\nBefore you %s, what would you like to do?", count, actionName)

	modal := tview.NewModal().
		SetText(modalText).
		AddButtons([]string{"Commit & Proceed", "Rollback & Discard", "Cancel"}).
		SetBackgroundColor(tcell.ColorDarkBlue).
		SetButtonBackgroundColor(tcell.ColorDarkCyan)

	modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		switch buttonIndex {
		case 0: // Commit & Proceed
			if onCommit != nil {
				onCommit()
			}
		case 1: // Rollback & Discard
			if onRollback != nil {
				onRollback()
			}
		case 2: // Cancel
			if onCancel != nil {
				onCancel()
			}
		}
	})

	if app != nil {
		app.SetRoot(modal, true)
	}
	return modal
}
