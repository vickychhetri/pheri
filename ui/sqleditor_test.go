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

func TestFormatSchemaObjectBadge(t *testing.T) {
	testCases := []struct {
		objType  string
		expected string
	}{
		{"TABLE", "[aqua::b]TABLE[-::-]"},
		{"VIEW", "[purple::b]VIEW [-::-]"},
		{"PROCEDURE", "[yellow::b]PROC [-::-]"},
		{"PROC", "[yellow::b]PROC [-::-]"},
		{"FUNCTION", "[lime::b]FUNC [-::-]"},
		{"FUNC", "[lime::b]FUNC [-::-]"},
		{"TRIGGER", "[red::b]TRIG [-::-]"},
		{"TRIG", "[red::b]TRIG [-::-]"},
		{"EVENT", "[blue::b]EVENT[-::-]"},
	}

	for _, tc := range testCases {
		badge := FormatSchemaObjectBadge(tc.objType)
		if badge != tc.expected {
			t.Errorf("FormatSchemaObjectBadge(%q) = %q; want %q", tc.objType, badge, tc.expected)
		}
	}
}

func TestSQLEditor_LineCountAndGutterWidth(t *testing.T) {
	if got := CountEditorLines(""); got != 1 {
		t.Errorf("CountEditorLines('') = %d, want 1", got)
	}
	if got := CountEditorLines("SELECT 1;"); got != 1 {
		t.Errorf("CountEditorLines('SELECT 1;') = %d, want 1", got)
	}
	if got := CountEditorLines("SELECT *\nFROM users\nWHERE id = 1;"); got != 3 {
		t.Errorf("CountEditorLines(3 lines) = %d, want 3", got)
	}

	if w := CalculateGutterWidth(5); w != 6 {
		t.Errorf("CalculateGutterWidth(5) = %d, want 6", w)
	}
	if w := CalculateGutterWidth(99); w != 6 {
		t.Errorf("CalculateGutterWidth(99) = %d, want 6", w)
	}
	if w := CalculateGutterWidth(100); w != 7 {
		t.Errorf("CalculateGutterWidth(100) = %d, want 7", w)
	}
	if w := CalculateGutterWidth(1000); w != 8 {
		t.Errorf("CalculateGutterWidth(1000) = %d, want 8", w)
	}

	if lbl := FormatGutterLine(3, 12, 2); lbl != "  3 │ " {
		t.Errorf("FormatGutterLine(3, 12, 2) = %q, want '  3 │ '", lbl)
	}
	if lbl := FormatGutterLine(15, 12, 2); lbl != "    │ " {
		t.Errorf("FormatGutterLine out-of-range = %q, want '    │ '", lbl)
	}
}

func TestSQLEditor_LineNumberGutterRenderAndScrollSync(t *testing.T) {
	app := tview.NewApplication()
	editor := NewSQLEditor(app)
	if editor.Gutter == nil {
		t.Fatalf("expected non-nil Gutter on SQLEditor")
	}

	// Build a 12-line SQL script
	var lines []string
	for i := 1; i <= 12; i++ {
		lines = append(lines, "SELECT * FROM t")
	}
	editor.SetText(strings.Join(lines, "\n"))

	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatalf("failed to init simulation screen: %v", err)
	}
	sim.SetSize(60, 7) // Inner viewport height = 5 lines (border takes top & bottom = 2 rows)

	editor.Container.SetRect(0, 0, 60, 7)

	// 1. Scroll at offset 0 -> gutter rows should show 1..5
	editor.TextArea.SetOffset(0, 0)
	editor.Container.Draw(sim)

	extractColSlice := func(width int, y int) string {
		var sb strings.Builder
		for x := 0; x < width; x++ {
			r, _, _, _ := sim.GetContent(x, y)
			if r != 0 {
				sb.WriteRune(r)
			}
		}
		return sb.String()
	}

	titleRow := extractColSlice(60, 0)
	if !strings.Contains(titleRow, "12 lines") {
		t.Errorf("expected Container title to show '12 lines', got %q", titleRow)
	}
	row1 := extractColSlice(10, 1)
	if !strings.Contains(row1, " 1 │") {
		t.Errorf("expected first viewport row gutter ' 1 │', got %q", row1)
	}
	row5 := extractColSlice(10, 5)
	if !strings.Contains(row5, " 5 │") {
		t.Errorf("expected fifth viewport row gutter ' 5 │', got %q", row5)
	}

	// 2. Scroll down to rowOffset = 4 -> gutter rows should sync to 5..9
	sim.Clear()
	editor.TextArea.SetOffset(4, 0)
	editor.Container.Draw(sim)

	scrolledFirst := extractColSlice(10, 1)
	if !strings.Contains(scrolledFirst, " 5 │") {
		t.Errorf("expected scrolled first row gutter ' 5 │', got %q", scrolledFirst)
	}
	scrolledLast := extractColSlice(10, 5)
	if !strings.Contains(scrolledLast, " 9 │") {
		t.Errorf("expected scrolled fifth row gutter ' 9 │', got %q", scrolledLast)
	}
}

func TestSQLEditor_ApplyThemeUpdatesGutter(t *testing.T) {
	orig := CurrentTheme
	defer func() {
		CurrentTheme = orig
	}()

	app := tview.NewApplication()
	editor := NewSQLEditor(app)
	for _, th := range AvailableThemes {
		if th.Name == "Light" {
			CurrentTheme = th
			break
		}
	}
	editor.ApplyTheme()
	if editor.Gutter.GetBackgroundColor() != CurrentTheme.SurfaceBg {
		t.Errorf("expected Gutter background color to sync with Light theme SurfaceBg")
	}
	if editor.Container.GetBackgroundColor() != CurrentTheme.SurfaceBg {
		t.Errorf("expected Container background color to sync with Light theme SurfaceBg")
	}
}

func TestSQLEditor_ExecutableTextAndStatementAtCursor(t *testing.T) {
	app := tview.NewApplication()
	editor := NewSQLEditor(app)

	script := "SELECT 1;\nUPDATE users SET active = 1 WHERE id = 5;\nSELECT COUNT(*) FROM users;"
	editor.TextArea.SetText(script, false)

	// 1. Border telemetry displays statement count for multi-statement scripts
	if !strings.Contains(editor.Container.GetTitle(), "3 stmts") {
		t.Errorf("expected editor border title to display '3 stmts', got %q", editor.Container.GetTitle())
	}

	// 2. GetExecutableText returns full script when no selection is active
	if got := editor.GetExecutableText(); got != script {
		t.Errorf("expected GetExecutableText() to return full script when unselected, got %q", got)
	}

	// 3. Move cursor down onto line 2 and test Alt+Enter (statement at cursor)
	handler := editor.TextArea.InputHandler()
	handler(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone), func(p tview.Primitive) {})

	chunk, ok := editor.GetStatementAtCursor()
	if !ok {
		t.Fatalf("expected GetStatementAtCursor() to find statement on line 2")
	}
	if chunk.Index != 2 || !strings.Contains(chunk.SQL, "UPDATE users") {
		t.Errorf("expected statement #2 (UPDATE users), got %+v", chunk)
	}

	var executedSQL string
	editor.OnExecute = func(q string) {
		executedSQL = q
	}
	res := editor.handleInput(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModAlt))
	if res != nil {
		t.Errorf("expected Alt+Enter to be handled (nil return)")
	}
	if !strings.Contains(executedSQL, "UPDATE users") {
		t.Errorf("expected Alt+Enter to execute cursor statement, got %q", executedSQL)
	}
}

