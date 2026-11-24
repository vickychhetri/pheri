// ui/connect_test.go
package ui

import (
	"errors"
	"strings"
	"testing"

	"mysql-tui/dbs"

	"github.com/rivo/tview"
)

func TestShowConnectionForm_Render(t *testing.T) {
	app := tview.NewApplication()

	// Invoke connection form without CLI args
	ShowConnectionForm(app, "", "", "", "")

	// Check defaults
	if ActiveEnv != "DEV" {
		t.Errorf("expected default ActiveEnv to be DEV, got %s", ActiveEnv)
	}
	if ActiveReadOnly != false {
		t.Errorf("expected default ActiveReadOnly to be false")
	}
}

func TestConnectionErrorPresentationFormatting(t *testing.T) {
	tests := []struct {
		name          string
		inputErr      error
		expectedTitle string
	}{
		{
			name:          "timeout error",
			inputErr:      errors.New("dial tcp 10.255.255.1:3306: i/o timeout"),
			expectedTitle: "Connection Timed Out",
		},
		{
			name:          "connection refused",
			inputErr:      errors.New("dial tcp 127.0.0.1:3306: connect: connection refused"),
			expectedTitle: "Connection Refused",
		},
		{
			name:          "auth failure",
			inputErr:      errors.New("Error 1045: Access denied for user 'tester'@'localhost'"),
			expectedTitle: "Authentication Failed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			info := dbs.CategorizeConnectionError(tc.inputErr)
			if !strings.Contains(info.Title, tc.expectedTitle) {
				t.Errorf("expected title to contain %q, got %q", tc.expectedTitle, info.Title)
			}
			if info.Hint == "" {
				t.Errorf("expected non-empty hint for %s", tc.name)
			}
		})
	}
}

func TestShowConnectionForm_SplitPaneLayout(t *testing.T) {
	app := tview.NewApplication()
	ShowConnectionForm(app, "", "", "", "")

	if ActiveEnv != "DEV" {
		t.Errorf("expected ActiveEnv to be reset to DEV, got %s", ActiveEnv)
	}
	if ActiveReadOnly != false {
		t.Errorf("expected ActiveReadOnly to be reset to false")
	}

	// Verify focus is assigned to profile list on initial launch
	focus := app.GetFocus()
	if focus == nil {
		t.Fatalf("expected non-nil focus on initial split-pane render")
	}
}

func TestAttemptBackToLogin_UncommittedBarrier(t *testing.T) {
	defer ResetTransactionState()

	app := tview.NewApplication()
	ResetTransactionState()

	// Clean state (0 uncommitted mutations) should allow immediate logout
	User = "tester"
	Host = "127.0.0.1"
	if ok := attemptBackToLogin(app, nil); !ok {
		t.Errorf("expected attemptBackToLogin to proceed when uncommitted count is 0")
	}
	if User != "" || Host != "" {
		t.Errorf("expected credentials to be cleared on clean logout, got User=%q Host=%q", User, Host)
	}

	// Dirty state (>0 uncommitted mutations) should block immediate logout and show barrier modal
	SetAutoCommitMode(false)
	RecordMutationInTx("UPDATE orders SET status = 'shipped'", "SQL Editor")
	User = "tester"
	Host = "127.0.0.1"

	if ok := attemptBackToLogin(app, nil); ok {
		t.Errorf("expected attemptBackToLogin to be blocked when uncommitted mutations exist")
	}
	if User != "tester" || Host != "127.0.0.1" {
		t.Errorf("expected credentials NOT to be cleared while barrier modal is active")
	}
}
