// ui/fk_nav_test.go
package ui

import (
	"mysql-tui/util"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestFKNavBreadcrumbStack(t *testing.T) {
	ClearNavHistory()
	defer ClearNavHistory()

	if NavHistoryDepth() != 0 {
		t.Fatalf("expected empty stack, got depth %d", NavHistoryDepth())
	}

	PushNavBreadcrumb(NavBreadcrumb{DBName: "sakila", TableName: "rental", CursorRow: 3, CursorCol: 2})
	PushNavBreadcrumb(NavBreadcrumb{DBName: "sakila", TableName: "customer", CursorRow: 1, CursorCol: 5})

	if NavHistoryDepth() != 2 {
		t.Fatalf("expected stack depth 2, got %d", NavHistoryDepth())
	}

	top, ok := PopNavBreadcrumb()
	if !ok || top.TableName != "customer" || top.CursorCol != 5 {
		t.Errorf("expected customer breadcrumb, got %+v (ok=%v)", top, ok)
	}

	first, ok := PopNavBreadcrumb()
	if !ok || first.TableName != "rental" || first.CursorRow != 3 {
		t.Errorf("expected rental breadcrumb, got %+v (ok=%v)", first, ok)
	}

	if _, ok := PopNavBreadcrumb(); ok {
		t.Errorf("expected empty pop to return ok=false")
	}
}

func TestRenderQueryResult_FKVisualIndicatorsAndRawValueIntegrity(t *testing.T) {
	ClearNavHistory()
	defer func() {
		ClearNavHistory()
		activeGridForeignKeys = nil
	}()

	app := tview.NewApplication()
	table := tview.NewTable().SetFixed(1, 0)

	fkMap := map[string]util.ForeignKeyRef{
		"customer_id": {
			ConstraintName:   "fk_rental_customer",
			ColumnName:       "customer_id",
			ReferencedSchema: "sakila",
			ReferencedTable:  "customer",
			ReferencedColumn: "customer_id",
		},
	}

	res := &QueryResult{
		Columns:     []string{"rental_id", "customer_id"},
		ForeignKeys: fkMap,
		RowCount:    2,
		Duration:    1 * time.Millisecond,
		Rows: [][]string{
			{"1", "42"},
			{"2", "[gray::i]NULL[-::-]"},
		},
		RawRows: [][]*string{
			{strPtr("1"), strPtr("42")},
			{strPtr("2"), nil},
		},
	}

	renderQueryResult(app, res, table)

	// Header should carry ↗ indicator yet resolve back to raw 'customer_id'
	if !strings.Contains(table.GetCell(0, 1).Text, "↗") {
		t.Errorf("expected FK column header to contain ↗, got %q", table.GetCell(0, 1).Text)
	}
	if got := getHeaderColumnName(table.GetCell(0, 1)); got != "customer_id" {
		t.Errorf("expected raw column name 'customer_id', got %q", got)
	}

	// Non-NULL FK cell displays ↗ while keeping GridCell raw value clean ("42")
	fkCell := table.GetCell(1, 1)
	if !strings.Contains(fkCell.Text, "↗") {
		t.Errorf("expected non-NULL FK cell text to contain ↗, got %q", fkCell.Text)
	}
	gc := GetGridCellFromTableCell(fkCell, "customer_id")
	if gc.IsNull() || gc.Value() != "42" {
		t.Errorf("expected underlying GridCell value to remain pure '42', got IsNull=%v Value=%q", gc.IsNull(), gc.Value())
	}

	// NULL FK cell must NOT display ↗ and remains marked NULL
	nullCell := table.GetCell(2, 1)
	if strings.Contains(nullCell.Text, "↗") {
		t.Errorf("expected NULL FK cell not to contain ↗, got %q", nullCell.Text)
	}
	gcNull := GetGridCellFromTableCell(nullCell, "customer_id")
	if !gcNull.IsNull() {
		t.Errorf("expected NULL FK cell GridCell.IsNull() == true")
	}
}

func TestTriggerForeignKeyJump_Guards(t *testing.T) {
	ClearNavHistory()
	defer func() {
		ClearNavHistory()
		activeGridForeignKeys = nil
	}()

	app := tview.NewApplication()
	CreateLayoutWithFooter(app, tview.NewBox())
	table := tview.NewTable().SetFixed(1, 0)

	fkMap := map[string]util.ForeignKeyRef{
		"customer_id": {
			ConstraintName:   "fk_rental_customer",
			ColumnName:       "customer_id",
			ReferencedSchema: "sakila",
			ReferencedTable:  "customer",
			ReferencedColumn: "customer_id",
		},
	}

	res := &QueryResult{
		Columns:     []string{"rental_id", "customer_id"},
		ForeignKeys: fkMap,
		RowCount:    2,
		Duration:    1 * time.Millisecond,
		Rows: [][]string{
			{"1", "42"},
			{"2", "[gray::i]NULL[-::-]"},
		},
		RawRows: [][]*string{
			{strPtr("1"), strPtr("42")},
			{strPtr("2"), nil},
		},
	}

	renderQueryResult(app, res, table)

	// 1. Selecting non-FK column (col 0) returns false without crashing
	table.Select(1, 0)
	if jumped := triggerForeignKeyJump(app, table); jumped {
		t.Errorf("expected non-FK column jump to return false")
	}

	// 2. Selecting NULL FK cell returns true (handled as FK column) with no stack push
	table.Select(2, 1)
	if jumped := triggerForeignKeyJump(app, table); !jumped {
		t.Errorf("expected NULL FK cell jump attempt to be handled (return true)")
	}
	if NavHistoryDepth() != 0 {
		t.Errorf("expected NULL FK cell not to push breadcrumb onto stack, got depth %d", NavHistoryDepth())
	}

	// 3. Pressing Backspace when stack is empty returns false cleanly
	handler := table.GetInputCapture()
	if handler == nil {
		t.Fatalf("expected table input capture to be set")
	}
	ret := handler(tcell.NewEventKey(tcell.KeyBackspace, 0, 0))
	if ret == nil {
		// Unhandled Backspace on empty history should pass event through
		t.Errorf("expected Backspace with empty nav history to pass event through")
	}
}
