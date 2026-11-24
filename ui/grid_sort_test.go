// ui/grid_sort_test.go
package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func strPtr(s string) *string {
	return &s
}

func TestSortDirectionCycle(t *testing.T) {
	if SortNone.Next() != SortAsc {
		t.Errorf("expected SortNone -> SortAsc")
	}
	if SortAsc.Next() != SortDesc {
		t.Errorf("expected SortAsc -> SortDesc")
	}
	if SortDesc.Next() != SortNone {
		t.Errorf("expected SortDesc -> SortNone")
	}

	if SortAsc.Indicator() != "▲" {
		t.Errorf("expected SortAsc indicator ▲, got %q", SortAsc.Indicator())
	}
	if SortDesc.Indicator() != "▼" {
		t.Errorf("expected SortDesc indicator ▼, got %q", SortDesc.Indicator())
	}
	if SortNone.Indicator() != "" {
		t.Errorf("expected SortNone empty indicator, got %q", SortNone.Indicator())
	}

	state := &GridSortState{PendingRestoreRow: -1, PendingRestoreCol: -1}
	col, dir := state.Toggle("id")
	if col != "id" || dir != SortAsc {
		t.Fatalf("expected (id, SortAsc), got (%q, %v)", col, dir)
	}
	col, dir = state.Toggle("id")
	if col != "id" || dir != SortDesc {
		t.Fatalf("expected (id, SortDesc), got (%q, %v)", col, dir)
	}
	col, dir = state.Toggle("id")
	if col != "" || dir != SortNone {
		t.Fatalf("expected ('', SortNone), got (%q, %v)", col, dir)
	}

	// Switching to a different column immediately starts at ASC
	state.Toggle("id")
	col, dir = state.Toggle("created_at")
	if col != "created_at" || dir != SortAsc {
		t.Fatalf("expected switching column to yield (created_at, SortAsc), got (%q, %v)", col, dir)
	}
}

func TestBuildBrowseQuery(t *testing.T) {
	q1 := BuildBrowseQuery("sakila", "actor", "", SortNone, 100, 0)
	want1 := "SELECT * FROM `sakila`.`actor` LIMIT 100"
	if q1 != want1 {
		t.Errorf("got %q, want %q", q1, want1)
	}

	q2 := BuildBrowseQuery("sakila", "actor", "first_name", SortAsc, 100, 200)
	want2 := "SELECT * FROM `sakila`.`actor` ORDER BY `first_name` ASC LIMIT 100 OFFSET 200"
	if q2 != want2 {
		t.Errorf("got %q, want %q", q2, want2)
	}

	q3 := BuildBrowseQuery("sakila", "weird`col", "weird`col", SortDesc, 50, 0)
	want3 := "SELECT * FROM `sakila`.`weird``col` ORDER BY `weird``col` DESC LIMIT 50"
	if q3 != want3 {
		t.Errorf("got %q, want %q", q3, want3)
	}
}

func TestApplySortToSQLQuery(t *testing.T) {
	tests := []struct {
		name    string
		base    string
		col     string
		dir     SortDirection
		want    string
		wantOK  bool
	}{
		{
			name:   "simple select",
			base:   "SELECT id, name FROM users;",
			col:    "name",
			dir:    SortAsc,
			want:   "SELECT id, name FROM users ORDER BY `name` ASC",
			wantOK: true,
		},
		{
			name:   "select with existing order by and limit",
			base:   "SELECT id, age FROM users WHERE active = 1 ORDER BY id DESC LIMIT 25 OFFSET 10;",
			col:    "age",
			dir:    SortDesc,
			want:   "SELECT id, age FROM users WHERE active = 1 ORDER BY `age` DESC LIMIT 25 OFFSET 10",
			wantOK: true,
		},
		{
			name:   "select with limit no order by",
			base:   "SELECT * FROM orders LIMIT 50",
			col:    "total",
			dir:    SortAsc,
			want:   "SELECT * FROM orders ORDER BY `total` ASC LIMIT 50",
			wantOK: true,
		},
		{
			name:   "subquery order by inside parens ignored",
			base:   "SELECT * FROM (SELECT id FROM t ORDER BY id DESC) sub LIMIT 10",
			col:    "id",
			dir:    SortAsc,
			want:   "SELECT * FROM (SELECT id FROM t ORDER BY id DESC) sub ORDER BY `id` ASC LIMIT 10",
			wantOK: true,
		},
		{
			name:   "union query wrapped cleanly",
			base:   "SELECT id FROM a UNION SELECT id FROM b;",
			col:    "id",
			dir:    SortDesc,
			want:   "SELECT * FROM (SELECT id FROM a UNION SELECT id FROM b) AS _pheri_sort ORDER BY `id` DESC",
			wantOK: true,
		},
		{
			name:   "reset sort returns clean base query",
			base:   "SELECT id FROM users;",
			col:    "",
			dir:    SortNone,
			want:   "SELECT id FROM users",
			wantOK: true,
		},
		{
			name:   "show tables uses fallback",
			base:   "SHOW TABLES;",
			col:    "Tables_in_db",
			dir:    SortAsc,
			want:   "",
			wantOK: false,
		},
		{
			name:   "describe rejected for sql rewrite",
			base:   "DESCRIBE users",
			col:    "Field",
			dir:    SortDesc,
			want:   "",
			wantOK: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ApplySortToSQLQuery(tc.base, tc.col, tc.dir)
			if ok != tc.wantOK {
				t.Fatalf("ApplySortToSQLQuery(%q) ok = %v, want %v (got=%q)", tc.base, ok, tc.wantOK, got)
			}
			if got != tc.want {
				t.Errorf("ApplySortToSQLQuery(%q) = %q, want %q", tc.base, got, tc.want)
			}
		})
	}
}

func TestSortQueryResultInMemory(t *testing.T) {
	orig := &QueryResult{
		Columns:  []string{"id", "name"},
		RowCount: 4,
		Rows: [][]string{
			{"10", "Charlie"},
			{"2", "alice"},
			{"NULL", "NullRow"},
			{"1", "Bob"},
		},
		RawRows: [][]*string{
			{strPtr("10"), strPtr("Charlie")},
			{strPtr("2"), strPtr("alice")},
			{nil, strPtr("NullRow")},
			{strPtr("1"), strPtr("Bob")},
		},
	}

	// Numeric ASC: NULL first (MySQL behavior), then 1, 2, 10
	ascNum := SortQueryResultInMemory(orig, 0, SortAsc)
	wantAscIDs := []string{"NULL", "1", "2", "10"}
	for i, w := range wantAscIDs {
		if ascNum.Rows[i][0] != w {
			t.Errorf("ASC numeric row %d: got %q, want %q", i, ascNum.Rows[i][0], w)
		}
	}

	// Numeric DESC: 10, 2, 1, NULL last
	descNum := SortQueryResultInMemory(orig, 0, SortDesc)
	wantDescIDs := []string{"10", "2", "1", "NULL"}
	for i, w := range wantDescIDs {
		if descNum.Rows[i][0] != w {
			t.Errorf("DESC numeric row %d: got %q, want %q", i, descNum.Rows[i][0], w)
		}
	}

	// String ASC (case-insensitive): alice, Bob, Charlie, NullRow
	ascStr := SortQueryResultInMemory(orig, 1, SortAsc)
	wantAscNames := []string{"alice", "Bob", "Charlie", "NullRow"}
	for i, w := range wantAscNames {
		if ascStr.Rows[i][1] != w {
			t.Errorf("ASC string row %d: got %q, want %q", i, ascStr.Rows[i][1], w)
		}
	}
}

func TestRenderQueryResult_SortIndicatorsAndKeyboardCycling(t *testing.T) {
	activeGridSort.Reset("SHOW TEST ROWS", false)
	activeGridDB = nil
	isEditingEnabled = false
	defer activeGridSort.Reset("", false)

	app := tview.NewApplication()
	table := tview.NewTable().SetFixed(1, 0)

	res := &QueryResult{
		Columns:  []string{"id", "name"},
		RowCount: 3,
		Duration: 2 * time.Millisecond,
		Rows: [][]string{
			{"2", "Beta"},
			{"1", "Alpha"},
			{"3", "Gamma"},
		},
		RawRows: [][]*string{
			{strPtr("2"), strPtr("Beta")},
			{strPtr("1"), strPtr("Alpha")},
			{strPtr("3"), strPtr("Gamma")},
		},
	}

	renderQueryResult(app, res, table)

	// Header row must be selectable for keyboard navigation to row 0
	if !table.GetCell(0, 0).NotSelectable {
		// good: NotSelectable is false (i.e. Selectable is true)
	} else {
		t.Errorf("expected header cell (0,0) to be selectable")
	}

	if got := getHeaderColumnName(table.GetCell(0, 0)); got != "id" {
		t.Errorf("expected raw header column name 'id', got %q", got)
	}

	// Select column 0 on row 0 and press 's' -> should sort ASC
	table.Select(0, 0)
	handler := table.GetInputCapture()
	if handler == nil {
		t.Fatalf("expected table input capture to be configured")
	}

	handler(tcell.NewEventKey(tcell.KeyRune, 's', 0))

	if !strings.Contains(table.GetCell(0, 0).Text, "▲") {
		t.Errorf("expected ascending indicator ▲ in header, got %q", table.GetCell(0, 0).Text)
	}
	if strings.TrimSpace(table.GetCell(1, 0).Text) != "1" {
		t.Errorf("expected first row after ASC sort to have id=1, got %q", table.GetCell(1, 0).Text)
	}
	if got := getHeaderColumnName(table.GetCell(0, 0)); got != "id" {
		t.Errorf("expected getHeaderColumnName to return 'id' even with indicator, got %q", got)
	}

	// Press 's' again -> should sort DESC
	handler = table.GetInputCapture()
	handler(tcell.NewEventKey(tcell.KeyRune, 's', 0))
	if !strings.Contains(table.GetCell(0, 0).Text, "▼") {
		t.Errorf("expected descending indicator ▼ in header, got %q", table.GetCell(0, 0).Text)
	}
	if strings.TrimSpace(table.GetCell(1, 0).Text) != "3" {
		t.Errorf("expected first row after DESC sort to have id=3, got %q", table.GetCell(1, 0).Text)
	}

	// Press 's' third time -> returns to Unsorted original order (id=2 first)
	handler = table.GetInputCapture()
	handler(tcell.NewEventKey(tcell.KeyRune, 's', 0))
	if strings.Contains(table.GetCell(0, 0).Text, "▲") || strings.Contains(table.GetCell(0, 0).Text, "▼") {
		t.Errorf("expected unsorted header with no arrows, got %q", table.GetCell(0, 0).Text)
	}
	if strings.TrimSpace(table.GetCell(1, 0).Text) != "2" {
		t.Errorf("expected first row after returning to Unsorted to have id=2, got %q", table.GetCell(1, 0).Text)
	}
}
