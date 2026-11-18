package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestSQLEditorBasicEditing(t *testing.T) {
	app := tview.NewApplication()
	editor := NewSQLEditor(app)

	// Verify editor initialization
	if editor.TextArea == nil {
		t.Fatal("Expected TextArea to be initialized, got nil")
	}
	if editor.Editor != editor.TextArea {
		t.Fatal("Expected Editor alias to point directly to TextArea")
	}

	// Test SetText and GetText
	sampleQuery := "SELECT id, name, email FROM users WHERE active = 1 ORDER BY id DESC LIMIT 50;"
	editor.SetText(sampleQuery)

	got := editor.GetText()
	if got != sampleQuery {
		t.Fatalf("Expected '%s', got '%s'", sampleQuery, got)
	}

	// Test Clear
	editor.Clear()
	if editor.GetText() != "" {
		t.Fatalf("Expected empty editor after Clear(), got '%s'", editor.GetText())
	}
}

func TestSQLEditorMultilineText(t *testing.T) {
	app := tview.NewApplication()
	editor := NewSQLEditor(app)

	multilineQuery := `SELECT
    c.id,
    c.first_name,
    c.last_name,
    COUNT(r.rental_id) as total_rentals
FROM customer c
INNER JOIN rental r ON c.customer_id = r.customer_id
GROUP BY c.customer_id
ORDER BY total_rentals DESC
LIMIT 10;`

	editor.SetText(multilineQuery)

	lines := strings.Split(editor.GetText(), "\n")
	if len(lines) != 10 {
		t.Fatalf("Expected 10 lines in multiline query, got %d", len(lines))
	}

	if !strings.Contains(editor.GetText(), "INNER JOIN") {
		t.Fatal("Expected query to contain 'INNER JOIN'")
	}
}

func TestSQLEditorExecutionCallback(t *testing.T) {
	app := tview.NewApplication()
	editor := NewSQLEditor(app)

	executedQuery := ""
	editor.OnExecute = func(q string) {
		executedQuery = q
	}

	target := "SELECT * FROM film WHERE rating = 'PG';"
	editor.SetText(target)

	if editor.OnExecute != nil {
		editor.OnExecute(editor.GetText())
	}

	if executedQuery != target {
		t.Fatalf("Expected executed query '%s', got '%s'", target, executedQuery)
	}
}

func TestSQLEditorBackspaceHandling(t *testing.T) {
	app := tview.NewApplication()
	editor := NewSQLEditor(app)

	// Set initial text "SELECT 1" with cursor at end
	editor.SetText("SELECT 1")
	handler := editor.TextArea.InputHandler()

	// Press Backspace: "SELECT 1" should become "SELECT "
	handler(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone), nil)
	if editor.GetText() != "SELECT " {
		t.Fatalf("Expected 'SELECT ' after Backspace, got '%s'", editor.GetText())
	}

	// Press Backspace2 (0x7F / ASCII DEL): "SELECT " should become "SELECT"
	handler(tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone), nil)
	if editor.GetText() != "SELECT" {
		t.Fatalf("Expected 'SELECT' after Backspace2, got '%s'", editor.GetText())
	}
}
