// ui/delete_row_test.go
package ui

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func buildTestDeleteTable(app *tview.Application) *tview.Table {
	table := tview.NewTable().SetFixed(1, 0)
	res := &QueryResult{
		Columns:  []string{"id", "tenant_id", "email", "notes"},
		RowCount: 2,
		Duration: 1 * time.Millisecond,
		Rows: [][]string{
			{"10", "5", "alice@example.com", "[gray::i]NULL[-::-]"},
			{"20", "5", "bob@example.com", "active"},
		},
		RawRows: [][]*string{
			{strPtr("10"), strPtr("5"), strPtr("alice@example.com"), nil},
			{strPtr("20"), strPtr("5"), strPtr("bob@example.com"), strPtr("active")},
		},
	}
	renderQueryResult(app, res, table)
	return table
}

func TestBuildRowWhereClause_PrimaryKeyAndIndicators(t *testing.T) {
	activeGridSort.Reset("", false)
	defer activeGridSort.Reset("", false)

	app := tview.NewApplication()
	table := buildTestDeleteTable(app)

	// Simulate sort indicator in header cell text while keeping reference intact
	table.GetCell(0, 0).SetText(" id ▲ ")
	table.GetCell(0, 1).SetText(" tenant_id ↗ ")

	// Single PK
	whereSQL, args, err := BuildRowWhereClause(table, 1, []string{"id"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if whereSQL != "`id` = ?" || len(args) != 1 || args[0] != "10" {
		t.Errorf("got (%q, %v), want (`id` = ?, [10])", whereSQL, args)
	}

	// Composite PK
	whereComp, argsComp, errComp := BuildRowWhereClause(table, 2, []string{"id", "tenant_id"})
	if errComp != nil {
		t.Fatalf("unexpected composite PK error: %v", errComp)
	}
	if whereComp != "`id` = ? AND `tenant_id` = ?" || len(argsComp) != 2 || argsComp[0] != "20" || argsComp[1] != "5" {
		t.Errorf("got (%q, %v), want composite PK match", whereComp, argsComp)
	}
}

func TestBuildRowWhereClause_KeylessFallbackAndNulls(t *testing.T) {
	app := tview.NewApplication()
	table := buildTestDeleteTable(app)

	// No PK -> match all columns including IS NULL for notes on row 1
	whereSQL, args, err := BuildRowWhereClause(table, 1, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantWhere := "`id` = ? AND `tenant_id` = ? AND `email` = ? AND `notes` IS NULL"
	if whereSQL != wantWhere {
		t.Errorf("got WHERE %q, want %q", whereSQL, wantWhere)
	}
	if len(args) != 3 || args[0] != "10" || args[1] != "5" || args[2] != "alice@example.com" {
		t.Errorf("unexpected args: %v", args)
	}

	// Partial composite PK missing from projection -> safely falls back to all visible columns
	wherePartial, argsPartial, errPartial := BuildRowWhereClause(table, 2, []string{"id", "missing_pk_col"})
	if errPartial != nil {
		t.Fatalf("unexpected partial PK error: %v", errPartial)
	}
	if wherePartial != "`id` = ? AND `tenant_id` = ? AND `email` = ? AND `notes` = ?" || len(argsPartial) != 4 {
		t.Errorf("expected full fallback when composite PK is incomplete, got (%q, %v)", wherePartial, argsPartial)
	}
}

func TestBuildParameterizedDeleteQuery(t *testing.T) {
	app := tview.NewApplication()
	table := buildTestDeleteTable(app)

	q, args, preview, err := BuildParameterizedDeleteQuery("appdb", "users", table, 1, []string{"id"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantQ := "DELETE FROM `appdb`.`users` WHERE `id` = ? LIMIT 1"
	if q != wantQ {
		t.Errorf("got query %q, want %q", q, wantQ)
	}
	if len(args) != 1 || args[0] != "10" {
		t.Errorf("unexpected args: %v", args)
	}
	wantPreview := "DELETE FROM `appdb`.`users` WHERE `id` = '10' LIMIT 1"
	if preview != wantPreview {
		t.Errorf("got preview %q, want %q", preview, wantPreview)
	}
}

func TestDeleteRowModal_AndGuards(t *testing.T) {
	origRO := ActiveReadOnly
	origObj := activeGridObject
	origType := activeGridObjectType
	origDBName := activeGridDBName
	defer func() {
		ActiveReadOnly = origRO
		activeGridObject = origObj
		activeGridObjectType = origType
		activeGridDBName = origDBName
	}()

	app := tview.NewApplication()
	mainFlex = tview.NewFlex()
	CreateLayoutWithFooter(app, mainFlex)
	table := buildTestDeleteTable(app)

	// 1. Confirmation modal defaults focus to Cancel and restores grid on Escape
	modal := buildDeleteRowModalUI(app, nil, "appdb", "users", table, 1, []string{"id"})
	if modal == nil {
		t.Fatalf("expected non-nil confirmation modal")
	}
	handler := modal.InputHandler()
	handler(tcell.NewEventKey(tcell.KeyEscape, 0, 0), func(p tview.Primitive) {})

	// 2. Read-Only guard on 'd' and KeyDelete
	ActiveReadOnly = true
	activeGridObject = "users"
	activeGridObjectType = "TABLE"
	activeGridDBName = "appdb"
	setupTableInputCapture(app, table)
	tblHandler := table.GetInputCapture()

	if ret := tblHandler(tcell.NewEventKey(tcell.KeyRune, 'd', 0)); ret != nil {
		t.Errorf("expected 'd' key to be consumed in Read-Only mode")
	}
	if ret := tblHandler(tcell.NewEventKey(tcell.KeyDelete, 0, 0)); ret != nil {
		t.Errorf("expected Delete key to be consumed in Read-Only mode")
	}

	// 3. Non-TABLE guard
	ActiveReadOnly = false
	activeGridObjectType = "VIEW"
	if ret := tblHandler(tcell.NewEventKey(tcell.KeyRune, 'd', 0)); ret != nil {
		t.Errorf("expected 'd' key to be consumed cleanly on non-TABLE object")
	}

	// 4. Header row (row == 0) guard
	activeGridObjectType = "TABLE"
	table.Select(0, 0)
	TriggerRowDeleteFlow(app, nil, "appdb", "users", table, 0)
}
