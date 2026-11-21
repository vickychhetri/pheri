// ui/transaction.go
package ui

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// TxMutationRecord represents an audit record of a mutation executed within the active transaction.
type TxMutationRecord struct {
	Index     int
	Timestamp time.Time
	Statement string
	Source    string
}

var (
	txMutex              sync.Mutex
	isAutoCommitMode     = true
	activeTx             *sql.Tx
	uncommittedTxCount   = 0
	activeTxDB           *sql.DB
	txMutations          []TxMutationRecord
	txStartTime          time.Time
	onTxStateChangedHook func()
)

// SetOnTxStateChangedHook registers a callback that is invoked whenever transaction state or counts change.
func SetOnTxStateChangedHook(hook func()) {
	txMutex.Lock()
	defer txMutex.Unlock()
	onTxStateChangedHook = hook
}

func notifyTxStateChanged() {
	txMutex.Lock()
	hook := onTxStateChangedHook
	txMutex.Unlock()
	if hook != nil {
		hook()
	}
}

// IsAutoCommitMode returns whether auto-commit is enabled (true) or manual transactions are active (false).
func IsAutoCommitMode() bool {
	txMutex.Lock()
	defer txMutex.Unlock()
	return isAutoCommitMode
}

// SetAutoCommitMode updates auto-commit mode directly (primarily for testing/resets).
func SetAutoCommitMode(enabled bool) {
	txMutex.Lock()
	isAutoCommitMode = enabled
	if enabled && activeTx != nil {
		_ = activeTx.Rollback()
		activeTx = nil
		uncommittedTxCount = 0
		txMutations = nil
		txStartTime = time.Time{}
	}
	txMutex.Unlock()
	notifyTxStateChanged()
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

// GetActiveTxMutations returns a copy of all mutations recorded in the active transaction audit trail.
func GetActiveTxMutations() []TxMutationRecord {
	txMutex.Lock()
	defer txMutex.Unlock()
	copied := make([]TxMutationRecord, len(txMutations))
	copy(copied, txMutations)
	return copied
}

// GetTxDuration returns the elapsed time since the current active transaction began its first mutation.
func GetTxDuration() time.Duration {
	txMutex.Lock()
	defer txMutex.Unlock()
	if txStartTime.IsZero() {
		return 0
	}
	return time.Since(txStartTime)
}

// ResetTransactionState resets all transaction tracking variables (used for tests or clean disconnects).
func ResetTransactionState() {
	txMutex.Lock()
	if activeTx != nil {
		_ = activeTx.Rollback()
	}
	activeTx = nil
	uncommittedTxCount = 0
	isAutoCommitMode = true
	activeTxDB = nil
	txMutations = nil
	txStartTime = time.Time{}
	txMutex.Unlock()
	notifyTxStateChanged()
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

// RecordMutationInTx increments the uncommitted mutation count and logs the statement in the active transaction audit trail.
func RecordMutationInTx(details ...string) {
	txMutex.Lock()
	uncommittedTxCount++
	stmt := "DML / Cell Mutation"
	source := "SQL Editor"
	if len(details) > 0 && details[0] != "" {
		stmt = details[0]
	}
	if len(details) > 1 && details[1] != "" {
		source = details[1]
	}
	if txStartTime.IsZero() {
		txStartTime = time.Now()
	}
	txMutations = append(txMutations, TxMutationRecord{
		Index:     uncommittedTxCount,
		Timestamp: time.Now(),
		Statement: stmt,
		Source:    source,
	})
	count := uncommittedTxCount
	txMutex.Unlock()

	notifyTxStateChanged()
	updateFooterText(fmt.Sprintf("Recorded mutation #%d in active transaction", count))
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
	txMutations = nil
	txStartTime = time.Time{}
	txMutex.Unlock()

	notifyTxStateChanged()
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
	txMutations = nil
	txStartTime = time.Time{}
	txMutex.Unlock()

	notifyTxStateChanged()
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
		activeTxDB = db
		txMutex.Unlock()

		notifyTxStateChanged()
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
		txMutations = nil
		txStartTime = time.Time{}
		txMutex.Unlock()

		notifyTxStateChanged()
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

	closeModal := func() {
		if mainFlex != nil && app != nil {
			orig := CreateLayoutWithFooter(app, mainFlex)
			app.SetRoot(orig, true)
			if dataTable != nil {
				app.SetFocus(dataTable)
			}
		}
	}

	modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		closeModal()
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
		default: // e.g. Esc key pressed
			if onCancel != nil {
				onCancel()
			}
		}
	})

	modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			closeModal()
			if onCancel != nil {
				onCancel()
			}
			return nil
		}
		return event
	})

	if app != nil {
		app.SetRoot(modal, true)
	}
	return modal
}

// ShowTransactionReviewModal displays a comprehensive review dialog showing all pending mutations in the active transaction.
func ShowTransactionReviewModal(app *tview.Application, db *sql.DB, onCommit func(), onRollback func()) *tview.Flex {
	mutations := GetActiveTxMutations()
	duration := GetTxDuration()
	count := len(mutations)

	headerView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	headerView.SetBackgroundColor(CurrentTheme.HeaderBg)

	modeStr := "[lime::b]AUTOCOMMIT: ON[-::-]"
	if !IsAutoCommitMode() {
		modeStr = "[yellow::b]MANUAL TRANSACTION MODE[-::-]"
	}
	durStr := "None"
	if duration > 0 {
		durStr = fmt.Sprintf("%02dm %02ds", int(duration.Minutes()), int(duration.Seconds())%60)
	}
	headerView.SetText(fmt.Sprintf(" [white::b]Mode:[-] %s  [gray]│[-] [white::b]Duration:[-] [yellow]%s[-]  [gray]│[-] [white::b]Pending Mutations:[-] [orange::b]%d[-::-]", modeStr, durStr, count))

	table := tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false)
	table.SetBackgroundColor(CurrentTheme.SurfaceBg)
	StyleModalBox(table.Box, "Pending Uncommitted Mutations Audit Trail")

	// Table headers
	headers := []string{" # ", " Timestamp ", " Source ", " SQL Statement "}
	for col, h := range headers {
		cell := tview.NewTableCell(h).
			SetTextColor(CurrentTheme.HeaderFg).
			SetBackgroundColor(CurrentTheme.HeaderBg).
			SetSelectable(false)
		table.SetCell(0, col, cell)
	}

	if count == 0 {
		cell := tview.NewTableCell("  [gray::i]No pending uncommitted mutations in this transaction.[-::-]").
			SetSelectable(false)
		table.SetCell(1, 0, cell)
	} else {
		for i, m := range mutations {
			row := i + 1
			table.SetCell(row, 0, tview.NewTableCell(fmt.Sprintf(" #%d ", m.Index)).SetTextColor(tcell.ColorYellow))
			table.SetCell(row, 1, tview.NewTableCell(" "+m.Timestamp.Format("15:04:05")+" ").SetTextColor(tcell.ColorDarkCyan))
			table.SetCell(row, 2, tview.NewTableCell(" "+m.Source+" ").SetTextColor(tcell.ColorLime))
			table.SetCell(row, 3, tview.NewTableCell(" "+m.Statement+" ").SetTextColor(tcell.ColorWhite))
		}
	}

	closeModal := func() {
		if mainFlex != nil && app != nil {
			orig := CreateLayoutWithFooter(app, mainFlex)
			app.SetRoot(orig, true)
			if dataTable != nil {
				app.SetFocus(dataTable)
			}
		}
	}

	commitBtn := tview.NewButton(fmt.Sprintf(" Commit All (%d) ", count)).SetSelectedFunc(func() {
		closeModal()
		if onCommit != nil {
			onCommit()
		} else {
			_, _ = CommitActiveTx(app)
		}
	})
	commitBtn.SetBackgroundColor(tcell.ColorDarkGreen)

	rollbackBtn := tview.NewButton(fmt.Sprintf(" Rollback All (%d) ", count)).SetSelectedFunc(func() {
		closeModal()
		if onRollback != nil {
			onRollback()
		} else {
			_, _ = RollbackActiveTx(app)
		}
	})
	rollbackBtn.SetBackgroundColor(tcell.ColorDarkRed)

	closeBtn := tview.NewButton(" Close (Esc) ").SetSelectedFunc(func() {
		closeModal()
	})

	buttonBar := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(nil, 2, 0, false).
		AddItem(commitBtn, 22, 0, false).
		AddItem(nil, 2, 0, false).
		AddItem(rollbackBtn, 24, 0, false).
		AddItem(nil, 2, 0, false).
		AddItem(closeBtn, 16, 0, false).
		AddItem(nil, 2, 0, false)
	buttonBar.SetBackgroundColor(CurrentTheme.HeaderBg)

	contentFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(headerView, 1, 0, false).
		AddItem(table, 0, 1, true).
		AddItem(buttonBar, 1, 0, false)

	outerFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 2, 0, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(nil, 3, 0, false).
			AddItem(contentFlex, 0, 1, true).
			AddItem(nil, 3, 0, false),
			0, 1, true).
		AddItem(nil, 2, 0, false)

	contentFlex.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		if e.Key() == tcell.KeyEscape {
			closeModal()
			return nil
		}
		if e.Key() == tcell.KeyF10 {
			closeModal()
			if onCommit != nil {
				onCommit()
			} else {
				_, _ = CommitActiveTx(app)
			}
			return nil
		}
		return e
	})

	if app != nil {
		app.SetRoot(outerFlex, true).SetFocus(table)
	}
	return outerFlex
}
