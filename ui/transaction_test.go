package ui

import (
	"context"
	"strings"
	"testing"

	"github.com/rivo/tview"
)

func TestAutoCommitDefaultState(t *testing.T) {
	ResetTransactionState()

	if !IsAutoCommitMode() {
		t.Fatalf("expected default auto-commit mode to be true, got false")
	}
	if GetActiveTx() != nil {
		t.Fatalf("expected initial activeTx to be nil")
	}
	if count := GetUncommittedTxCount(); count != 0 {
		t.Fatalf("expected uncommitted count to be 0, got %d", count)
	}
}

func TestToggleAutoCommit(t *testing.T) {
	ResetTransactionState()
	app := tview.NewApplication()

	// Toggle OFF (manual transaction mode)
	ToggleAutoCommit(app, nil)
	if IsAutoCommitMode() {
		t.Fatalf("expected auto-commit mode to be false after toggle")
	}

	// Toggle ON (auto-commit mode) when clean (0 uncommitted)
	ToggleAutoCommit(app, nil)
	if !IsAutoCommitMode() {
		t.Fatalf("expected auto-commit mode to be true after second toggle")
	}
}

func TestRecordMutationInTx(t *testing.T) {
	ResetTransactionState()
	SetAutoCommitMode(false)

	if count := GetUncommittedTxCount(); count != 0 {
		t.Fatalf("expected 0 uncommitted mutations, got %d", count)
	}

	RecordMutationInTx()
	RecordMutationInTx()
	RecordMutationInTx()

	if count := GetUncommittedTxCount(); count != 3 {
		t.Fatalf("expected 3 uncommitted mutations, got %d", count)
	}

	ResetTransactionState()
	if count := GetUncommittedTxCount(); count != 0 {
		t.Fatalf("expected 0 uncommitted mutations after reset, got %d", count)
	}
}

func TestCommitAndRollbackWithoutTx(t *testing.T) {
	ResetTransactionState()
	app := tview.NewApplication()

	// When no active tx or 0 pending mutations
	count, err := CommitActiveTx(app)
	if err != nil {
		t.Fatalf("expected no error on commit without tx, got: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 committed count, got %d", count)
	}

	count, err = RollbackActiveTx(app)
	if err != nil {
		t.Fatalf("expected no error on rollback without tx, got: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 rolled back count, got %d", count)
	}
}

func TestEnsureActiveTxValidation(t *testing.T) {
	ResetTransactionState()
	ctx := context.Background()

	// In auto-commit mode, EnsureActiveTx returns (nil, nil)
	tx, err := EnsureActiveTx(ctx, nil)
	if err != nil || tx != nil {
		t.Fatalf("expected (nil, nil) in autocommit mode, got tx: %v, err: %v", tx, err)
	}

	// In manual mode with nil DB, should return error
	SetAutoCommitMode(false)
	tx, err = EnsureActiveTx(ctx, nil)
	if err == nil {
		t.Fatalf("expected error when DB is nil in manual mode, got nil")
	}
	if tx != nil {
		t.Fatalf("expected nil tx on error, got %v", tx)
	}
}

func TestPromptUncommittedChangesModal(t *testing.T) {
	ResetTransactionState()
	SetAutoCommitMode(false)
	RecordMutationInTx()
	RecordMutationInTx()

	app := tview.NewApplication()
	var commitCalled, rollbackCalled, cancelCalled bool

	modal := PromptUncommittedChangesModal(app, "exit Pheri",
		func() { commitCalled = true },
		func() { rollbackCalled = true },
		func() { cancelCalled = true },
	)

	if modal == nil {
		t.Fatalf("expected modal to be non-nil")
	}

	_ = commitCalled
	_ = rollbackCalled
	_ = cancelCalled
}

func TestFooterBadgesInTransactionModes(t *testing.T) {
	ResetTransactionState()
	app := tview.NewApplication()
	CreateLayoutWithFooter(app, tview.NewBox())

	// Auto-Commit ON
	updateFooterText("Ready")
	footerContent := footerBar.GetText(true)
	if !strings.Contains(footerContent, "AUTOCOMMIT: ON") {
		t.Errorf("footer missing AUTOCOMMIT: ON badge: %s", footerContent)
	}
	if !strings.Contains(footerContent, "F2 AutoCommit") {
		t.Errorf("footer missing F2 AutoCommit shortcut legend: %s", footerContent)
	}
	if !strings.Contains(footerContent, "Ctrl+H History") {
		t.Errorf("footer missing Ctrl+H History shortcut legend: %s", footerContent)
	}

	// Switch to manual mode with 0 pending
	SetAutoCommitMode(false)
	updateFooterText("Manual mode")
	footerContent = footerBar.GetText(true)
	if !strings.Contains(footerContent, "TX: READY") {
		t.Errorf("footer missing TX: READY badge: %s", footerContent)
	}

	// Record mutations
	RecordMutationInTx()
	RecordMutationInTx()
	updateFooterText("Pending")
	footerContent = footerBar.GetText(true)
	if !strings.Contains(footerContent, "TX: 2 PENDING") {
		t.Errorf("footer missing TX: 2 PENDING badge: %s", footerContent)
	}
}
