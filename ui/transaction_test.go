package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
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
	dummyFlex := tview.NewFlex()
	mainFlex = dummyFlex
	var commitCalled, rollbackCalled, cancelCalled bool

	modal := PromptUncommittedChangesModal(app, "exit Pheri",
		func() { commitCalled = true },
		func() { rollbackCalled = true },
		func() { cancelCalled = true },
	)

	if modal == nil {
		t.Fatalf("expected modal to be non-nil")
	}

	handler := modal.InputHandler()
	if handler == nil {
		t.Fatalf("expected modal to have an input handler")
	}

	// Pressing Enter on default button (button 0: Commit & Proceed)
	handler(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(p tview.Primitive) {})
	if !commitCalled {
		t.Errorf("expected commitCalled to be true after Enter")
	}

	// Pressing Escape (Cancel)
	handler(tcell.NewEventKey(tcell.KeyEscape, 0, 0), func(p tview.Primitive) {})
	if !cancelCalled {
		t.Errorf("expected cancelCalled to be true on Escape")
	}

	_ = rollbackCalled
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

func TestTxMutationRecordAuditTrail(t *testing.T) {
	ResetTransactionState()
	SetAutoCommitMode(false)

	RecordMutationInTx("UPDATE users SET status = 'active' WHERE id = 1;", "SQL Editor")
	RecordMutationInTx("UPDATE employees SET salary = salary * 1.1;", "Grid Cell Edit")

	mutations := GetActiveTxMutations()
	if len(mutations) != 2 {
		t.Fatalf("expected 2 mutations recorded, got %d", len(mutations))
	}

	if mutations[0].Index != 1 || !strings.Contains(mutations[0].Statement, "users") || mutations[0].Source != "SQL Editor" {
		t.Errorf("unexpected mutation[0]: %+v", mutations[0])
	}
	if mutations[1].Index != 2 || !strings.Contains(mutations[1].Statement, "employees") || mutations[1].Source != "Grid Cell Edit" {
		t.Errorf("unexpected mutation[1]: %+v", mutations[1])
	}

	// Verify duration tracking
	time.Sleep(5 * time.Millisecond)
	duration := GetTxDuration()
	if duration <= 0 {
		t.Errorf("expected positive transaction duration, got %v", duration)
	}

	// Reset should clear audit trail
	ResetTransactionState()
	if len(GetActiveTxMutations()) != 0 {
		t.Errorf("expected 0 mutations after reset")
	}
	if GetTxDuration() != 0 {
		t.Errorf("expected 0 duration after reset")
	}
}

func TestShowTransactionReviewModal(t *testing.T) {
	ResetTransactionState()
	SetAutoCommitMode(false)
	RecordMutationInTx("DELETE FROM old_logs WHERE created_at < '2024-01-01';", "SQL Editor")

	app := tview.NewApplication()
	var commitCalled, rollbackCalled bool

	modal := ShowTransactionReviewModal(app, nil,
		func() { commitCalled = true },
		func() { rollbackCalled = true },
	)

	if modal == nil {
		t.Fatalf("expected ShowTransactionReviewModal to return non-nil flex")
	}

	_ = commitCalled
	_ = rollbackCalled
}

func TestSetOnTxStateChangedHook(t *testing.T) {
	ResetTransactionState()
	var hookCallCount int

	SetOnTxStateChangedHook(func() {
		hookCallCount++
	})

	SetAutoCommitMode(false)
	if hookCallCount == 0 {
		t.Errorf("expected hook to be called on SetAutoCommitMode")
	}

	prev := hookCallCount
	RecordMutationInTx("INSERT INTO test VALUES (1);", "Test")
	if hookCallCount <= prev {
		t.Errorf("expected hook to be called on RecordMutationInTx")
	}

	prev = hookCallCount
	ResetTransactionState()
	if hookCallCount <= prev {
		t.Errorf("expected hook to be called on ResetTransactionState")
	}

	// Clean up hook
	SetOnTxStateChangedHook(nil)
}

func TestHandleFooterClick_TogglesAutoCommit(t *testing.T) {
	ResetTransactionState()
	app := tview.NewApplication()
	dummyFlex := tview.NewFlex()
	mainFlex = dummyFlex
	_ = CreateLayoutWithFooter(app, mainFlex)

	updateFooterText("Ready")
	plainText := footerBar.GetText(true)
	idxF2 := strings.Index(plainText, "F2 AutoCommit")
	if idxF2 == -1 {
		t.Fatalf("expected 'F2 AutoCommit' to be in footer plain text: %s", plainText)
	}

	// Click on F2 AutoCommit
	if !IsAutoCommitMode() {
		t.Fatalf("expected AutoCommit to be ON initially")
	}

	handleFooterClick(app, idxF2+2, plainText)

	if IsAutoCommitMode() {
		t.Errorf("expected AutoCommit to be toggled OFF after clicking F2 AutoCommit")
	}

	// Reset
	ResetTransactionState()
}

func TestFooterBar_MouseCaptureAndFocusDeflection(t *testing.T) {
	app := tview.NewApplication()
	tbl := tview.NewTable()
	dataTable = tbl

	dummyFlex := tview.NewFlex()
	mainFlex = dummyFlex
	_ = CreateLayoutWithFooter(app, mainFlex)

	mouseCapture := footerBar.GetMouseCapture()
	if mouseCapture == nil {
		t.Fatalf("expected footerBar to have mouse capture set")
	}

	// Simulate left click on footer
	mouseEvt := tcell.NewEventMouse(10, 10, tcell.Button1, 0)
	action, retEvt := mouseCapture(tview.MouseLeftClick, mouseEvt)

	if action != tview.MouseConsumed {
		t.Errorf("expected MouseConsumed action, got %v", action)
	}
	if retEvt != nil {
		t.Errorf("expected nil returned mouse event to prevent footer from stealing focus, got %v", retEvt)
	}

	// Test focus deflection
	footerBar.Focus(func(p tview.Primitive) {
		app.SetFocus(p)
	})
	if app.GetFocus() != tbl {
		t.Errorf("expected focus to deflect to dataTable")
	}
}
