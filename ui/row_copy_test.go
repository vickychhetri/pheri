// ui/row_copy_test.go
package ui

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func buildTestCopyTable(app *tview.Application) *tview.Table {
	table := tview.NewTable().SetFixed(1, 0)
	res := &QueryResult{
		Columns:  []string{"id", "zip", "name", "bio", "score"},
		RowCount: 2,
		Duration: 1 * time.Millisecond,
		Rows: [][]string{
			{"42", "00123", "O'Reilly", "[gray::i]NULL[-::-]", "98.5"},
			{"43", "94107", "Line1\tLine2\nNext", "Engineer", "100"},
		},
		RawRows: [][]*string{
			{strPtr("42"), strPtr("00123"), strPtr("O'Reilly"), nil, strPtr("98.5")},
			{strPtr("43"), strPtr("94107"), strPtr("Line1\tLine2\nNext"), strPtr("Engineer"), strPtr("100")},
		},
	}
	renderQueryResult(app, res, table)
	return table
}

func TestExtractRowCopyData_StripsIndicators(t *testing.T) {
	app := tview.NewApplication()
	table := buildTestCopyTable(app)

	// Simulate sort & FK glyphs on headers and FK arrow on cell text
	table.GetCell(0, 0).SetText(" id ▲ ")
	table.GetCell(0, 1).SetText(" zip ↗ ")
	table.GetCell(1, 1).SetText(" 00123 ↗ ")

	data, err := ExtractRowCopyData(table, 1, "appdb", "users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.Columns[0] != "id" || data.Columns[1] != "zip" {
		t.Errorf("expected stripped column names [id zip], got %v", data.Columns[:2])
	}
	if data.Cells[1].Value() != "00123" {
		t.Errorf("expected raw value '00123' without indicator, got %q", data.Cells[1].Value())
	}
	if !data.Cells[3].IsNull() {
		t.Errorf("expected bio column on row 1 to be NULL")
	}

	if _, errHeader := ExtractRowCopyData(table, 0, "appdb", "users"); errHeader == nil {
		t.Errorf("expected error extracting header row 0")
	}
}

func TestFormatRowAsJSON(t *testing.T) {
	app := tview.NewApplication()
	table := buildTestCopyTable(app)

	data, err := ExtractRowCopyData(table, 1, "appdb", "users")
	if err != nil {
		t.Fatalf("unexpected extract error: %v", err)
	}

	jsonOut, err := FormatRowAsJSON(data)
	if err != nil {
		t.Fatalf("unexpected FormatRowAsJSON error: %v", err)
	}

	// Verify valid JSON and expected types
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(jsonOut), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, jsonOut)
	}
	if parsed["id"] != float64(42) {
		t.Errorf("expected numeric id 42, got %#v", parsed["id"])
	}
	if parsed["zip"] != "00123" {
		t.Errorf("expected padded zip '00123' preserved as string, got %#v", parsed["zip"])
	}
	if parsed["name"] != "O'Reilly" {
		t.Errorf("expected name O'Reilly, got %#v", parsed["name"])
	}
	if parsed["bio"] != nil {
		t.Errorf("expected bio nil, got %#v", parsed["bio"])
	}
	if parsed["score"] != 98.5 {
		t.Errorf("expected float 98.5, got %#v", parsed["score"])
	}

	// Verify ordered keys match column order
	idxID := strings.Index(jsonOut, `"id"`)
	idxZip := strings.Index(jsonOut, `"zip"`)
	idxScore := strings.Index(jsonOut, `"score"`)
	if !(idxID < idxZip && idxZip < idxScore) {
		t.Errorf("expected JSON keys in column order, got:\n%s", jsonOut)
	}
}

func TestFormatRowAsTSV(t *testing.T) {
	app := tview.NewApplication()
	table := buildTestCopyTable(app)

	// Row 1: NULL cell
	data1, _ := ExtractRowCopyData(table, 1, "appdb", "users")
	tsv1 := FormatRowAsTSV(data1, false)
	want1 := "42\t00123\tO'Reilly\tNULL\t98.5"
	if tsv1 != want1 {
		t.Errorf("got TSV %q, want %q", tsv1, want1)
	}

	// Row 2 with header: embedded tabs/newlines flattened
	data2, _ := ExtractRowCopyData(table, 2, "appdb", "users")
	tsv2 := FormatRowAsTSV(data2, true)
	want2 := "id\tzip\tname\tbio\tscore\n43\t94107\tLine1 Line2 Next\tEngineer\t100"
	if tsv2 != want2 {
		t.Errorf("got TSV with headers %q, want %q", tsv2, want2)
	}
}

func TestFormatRowAsSQLInsert(t *testing.T) {
	app := tview.NewApplication()
	table := buildTestCopyTable(app)

	data, _ := ExtractRowCopyData(table, 1, "appdb", "users")
	sqlOut := FormatRowAsSQLInsert(data)
	want := "INSERT INTO `appdb`.`users` (`id`, `zip`, `name`, `bio`, `score`) VALUES (42, '00123', 'O''Reilly', NULL, '98.5');"
	if sqlOut != want {
		t.Errorf("got SQL:\n%s\nwant:\n%s", sqlOut, want)
	}

	// Empty tableName fallback for custom query results
	dataNoTbl, _ := ExtractRowCopyData(table, 1, "appdb", "")
	sqlFallback := FormatRowAsSQLInsert(dataNoTbl)
	if !strings.HasPrefix(sqlFallback, "INSERT INTO `target_table` ") {
		t.Errorf("expected fallback table name target_table, got %q", sqlFallback)
	}
}

func TestCopyRowModal_AndShortcuts(t *testing.T) {
	origObj := activeGridObject
	origDBName := activeGridDBName
	defer func() {
		activeGridObject = origObj
		activeGridDBName = origDBName
	}()

	app := tview.NewApplication()
	mainFlex = tview.NewFlex()
	CreateLayoutWithFooter(app, mainFlex)
	table := buildTestCopyTable(app)

	activeGridDBName = "appdb"
	activeGridObject = "users"

	data, _ := ExtractRowCopyData(table, 1, "appdb", "users")
	modal := buildCopyRowModalUI(app, table, data, 1)
	if modal == nil {
		t.Fatalf("expected non-nil copy row modal")
	}

	// Verify table shortcut handling ('c', 'y', Ctrl+Shift+C) and header row guard
	setupTableInputCapture(app, table)
	handler := table.GetInputCapture()
	if handler == nil {
		t.Fatalf("expected table input capture to be configured")
	}

	// Row 0 guard
	table.Select(0, 0)
	if ret := handler(tcell.NewEventKey(tcell.KeyRune, 'c', 0)); ret != nil {
		t.Errorf("expected 'c' on header row 0 to be consumed cleanly")
	}

	// Data row 1 triggers copy modal via Ctrl+Shift+C and 'y'
	table.Select(1, 0)
	if ret := handler(tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModShift)); ret != nil {
		t.Errorf("expected Ctrl+Shift+C to be handled on data row")
	}
	if ret := handler(tcell.NewEventKey(tcell.KeyRune, 'y', 0)); ret != nil {
		t.Errorf("expected 'y' to be handled on data row")
	}
}
