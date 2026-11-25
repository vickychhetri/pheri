package ui

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestSplitSQLStatements_EdgeCases(t *testing.T) {
	script := `
-- Leading comment before query
SELECT 'a;b;c' AS str_with_semi, "d;e" AS dbl_with_semi, 'it\'s;ok' AS escaped_bs, 'it''s;fine' AS escaped_doubled;

# MySQL hash comment with semicolon ; inside
INSERT INTO ` + "`weird;table`" + ` (note) VALUES ('line1
;line2');

/* Multi-line
   block comment ; still comment */
UPDATE accounts SET status = 'done' /* inline ; comment */ WHERE id = 1;

DELIMITER //
CREATE TRIGGER trg_audit AFTER INSERT ON accounts
FOR EACH ROW
BEGIN
    INSERT INTO audit_log (msg) VALUES ('created;ok');
END //
DELIMITER ;

SELECT COUNT(*) FROM audit_log;
-- trailing comment only without SQL
`

	chunks := SplitSQLStatements(script)
	if len(chunks) != 5 {
		t.Fatalf("expected 5 SQL chunks, got %d: %+v", len(chunks), chunks)
	}

	if chunks[0].Index != 1 || chunks[0].IsMutation || chunks[0].StatementType != StmtSelect {
		t.Errorf("unexpected chunk[0] metadata: %+v", chunks[0])
	}
	if !strings.Contains(chunks[0].SQL, "'a;b;c'") || !strings.Contains(chunks[0].SQL, `'it\'s;ok'`) {
		t.Errorf("chunk[0] altered quoted semicolons: %q", chunks[0].SQL)
	}
	if chunks[0].StartLine != 3 {
		t.Errorf("expected chunk[0].StartLine = 3, got %d", chunks[0].StartLine)
	}

	if chunks[1].Index != 2 || !chunks[1].IsMutation || chunks[1].StatementType != StmtInsert {
		t.Errorf("unexpected chunk[1] metadata: %+v", chunks[1])
	}
	if !strings.Contains(chunks[1].SQL, "`weird;table`") {
		t.Errorf("chunk[1] split inside backtick identifier: %q", chunks[1].SQL)
	}

	if chunks[2].Index != 3 || !chunks[2].IsMutation || chunks[2].StatementType != StmtUpdate {
		t.Errorf("unexpected chunk[2] metadata: %+v", chunks[2])
	}

	if chunks[3].Index != 4 || !chunks[3].IsMutation || chunks[3].StatementType != "DDL (CREATE)" {
		t.Errorf("unexpected chunk[3] trigger metadata: %+v", chunks[3])
	}
	if !strings.Contains(chunks[3].SQL, "CREATE TRIGGER trg_audit") || !strings.Contains(chunks[3].SQL, "VALUES ('created;ok');") {
		t.Errorf("chunk[3] lost inner trigger semicolon under DELIMITER //: %q", chunks[3].SQL)
	}

	if chunks[4].Index != 5 || chunks[4].IsMutation || chunks[4].StatementType != StmtSelect {
		t.Errorf("unexpected chunk[4] metadata: %+v", chunks[4])
	}
}

func TestSplitSQLStatements_CommentOnlyAndEmpty(t *testing.T) {
	if chunks := SplitSQLStatements("   \n\t  "); len(chunks) != 0 {
		t.Errorf("expected 0 chunks for whitespace, got %d", len(chunks))
	}
	if chunks := SplitSQLStatements("-- just a comment\n/* block */\n# hash"); len(chunks) != 0 {
		t.Errorf("expected 0 chunks for comments only, got %d", len(chunks))
	}
	chunks := SplitSQLStatements("SELECT 1")
	if len(chunks) != 1 || chunks[0].SQL != "SELECT 1" || chunks[0].StartLine != 1 {
		t.Errorf("unexpected single statement without semicolon: %+v", chunks)
	}
}

func TestBuildBatchResultTabs(t *testing.T) {
	// 1. Mixed batch (INSERT + 2 SELECTs) all succeeding -> defaults to first SELECT tab
	records := []BatchStatementRecord{
		{
			Chunk:        SQLStatementChunk{Index: 1, StartLine: 1, SQL: "INSERT INTO t VALUES (1)", IsMutation: true, StatementType: StmtInsert},
			Status:       "OK",
			Duration:     2 * time.Millisecond,
			RowsAffected: 1,
			LastInsertID: 99,
			Result:       &QueryResult{IsMutation: true, StatementType: StmtInsert, RowsAffected: 1, LastInsertID: 99},
		},
		{
			Chunk:       SQLStatementChunk{Index: 2, StartLine: 2, SQL: "SELECT * FROM t", StatementType: StmtSelect},
			Status:      "OK",
			Duration:    3 * time.Millisecond,
			RowsFetched: 2,
			Result:      &QueryResult{Columns: []string{"id"}, Rows: [][]string{{"1"}, {"2"}}, RowCount: 2},
		},
		{
			Chunk:       SQLStatementChunk{Index: 3, StartLine: 3, SQL: "SHOW TABLES", StatementType: StmtShow},
			Status:      "OK",
			Duration:    1 * time.Millisecond,
			RowsFetched: 1,
			Result:      &QueryResult{Columns: []string{"Tables"}, Rows: [][]string{{"t"}}, RowCount: 1},
		},
	}

	tabs, defaultIdx := BuildBatchResultTabs(records, 6*time.Millisecond)
	if len(tabs) != 3 {
		t.Fatalf("expected 3 tabs (2 queries + Summary), got %d", len(tabs))
	}
	if defaultIdx != 0 {
		t.Errorf("expected default tab index 0 (first SELECT), got %d", defaultIdx)
	}
	if tabs[0].Title != "2: SELECT (2)" || tabs[1].Title != "3: SHOW (1)" {
		t.Errorf("unexpected query tab titles: %q, %q", tabs[0].Title, tabs[1].Title)
	}
	if !tabs[2].IsSummary || tabs[2].Title != "Summary (3/3 OK)" {
		t.Errorf("unexpected summary tab: %+v", tabs[2])
	}
	if tabs[2].Result.RowCount != 3 || len(tabs[2].Result.RawRows) != 3 {
		t.Errorf("expected summary grid to have 3 rows with RawRows, got %d/%d", tabs[2].Result.RowCount, len(tabs[2].Result.RawRows))
	}

	// 2. Batch with error -> defaults to Summary tab and marks trailing statement SKIPPED
	errRecords := []BatchStatementRecord{
		records[1],
		{
			Chunk:        SQLStatementChunk{Index: 2, StartLine: 5, SQL: "SELECT * FROM missing_tbl", StatementType: StmtSelect},
			Status:       "ERROR",
			Duration:     1 * time.Millisecond,
			ErrorMessage: "Stmt #2 (Ln 5): Table doesn't exist",
			Result:       &QueryResult{ErrorMessage: "Stmt #2 (Ln 5): Table doesn't exist"},
		},
		{
			Chunk:  SQLStatementChunk{Index: 3, StartLine: 6, SQL: "SELECT 1", StatementType: StmtSelect},
			Status: "SKIPPED",
		},
	}

	errTabs, errDefault := BuildBatchResultTabs(errRecords, 4*time.Millisecond)
	if len(errTabs) != 2 {
		t.Fatalf("expected 2 tabs (1 completed SELECT + Summary), got %d", len(errTabs))
	}
	if errDefault != 1 || !errTabs[errDefault].IsSummary {
		t.Errorf("expected default tab to focus Summary on batch error, got idx=%d (%+v)", errDefault, errTabs[errDefault])
	}
	if errTabs[errDefault].Title != "Summary (1 ERR)" {
		t.Errorf("expected 'Summary (1 ERR)', got %q", errTabs[errDefault].Title)
	}
}

func TestBatchReadOnlyPreFlightGuard(t *testing.T) {
	// Verify script starting with SELECT followed by DROP TABLE is blocked before executing Stmt #1
	origReadOnly := ActiveReadOnly
	defer func() { ActiveReadOnly = origReadOnly }()
	ActiveReadOnly = true

	app := tview.NewApplication()
	table := tview.NewTable()
	var gotRes *QueryResult

	ExecuteQueryAsync(app, &sql.DB{}, "SELECT 1;\nDROP TABLE users;", table, func(res *QueryResult) {
		gotRes = res
	})

	if gotRes == nil {
		t.Fatalf("expected onComplete callback when Read-Only mode blocks multi-statement batch")
	}
	if gotRes.ErrorMessage != "read-only mode enabled" {
		t.Errorf("expected read-only error message, got %q", gotRes.ErrorMessage)
	}
	if gotRes.StatementType != "DDL (DROP)" {
		t.Errorf("expected blocked statement type 'DDL (DROP)', got %q", gotRes.StatementType)
	}
	if IsQueryRunning() {
		t.Errorf("expected IsQueryRunning() to remain false after pre-flight block")
	}
}

func TestBatchTabRenderingAndSwitching(t *testing.T) {
	origTabBar := resultTabBar
	origContainer := resultsContainer
	origActiveTabs := activeBatchTabs
	origActiveIdx := activeBatchTabIx
	defer func() {
		resultTabBar = origTabBar
		resultsContainer = origContainer
		batchTabMutex.Lock()
		activeBatchTabs = origActiveTabs
		activeBatchTabIx = origActiveIdx
		batchTabMutex.Unlock()
	}()

	app := tview.NewApplication()
	table := tview.NewTable()
	resultTabBar = tview.NewTextView().SetDynamicColors(true).SetWrap(false)
	resultsContainer = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(resultTabBar, 0, 0, false).
		AddItem(table, 0, 1, false)

	tabs, defaultIdx := BuildBatchResultTabs([]BatchStatementRecord{
		{
			Chunk:       SQLStatementChunk{Index: 1, StartLine: 1, SQL: "SELECT 'first' AS val", StatementType: StmtSelect},
			Status:      "OK",
			Duration:    1 * time.Millisecond,
			RowsFetched: 1,
			Result:      &QueryResult{Columns: []string{"val"}, Rows: [][]string{{"first"}}, RowCount: 1},
		},
		{
			Chunk:       SQLStatementChunk{Index: 2, StartLine: 2, SQL: "SELECT 'second' AS val", StatementType: StmtSelect},
			Status:      "OK",
			Duration:    1 * time.Millisecond,
			RowsFetched: 1,
			Result:      &QueryResult{Columns: []string{"val"}, Rows: [][]string{{"second"}}, RowCount: 1},
		},
	}, 2*time.Millisecond)

	batchTabMutex.Lock()
	activeBatchTabs = tabs
	activeBatchTabIx = defaultIdx
	batchTabMutex.Unlock()

	switchBatchResultTab(app, table, 0)
	if !strings.Contains(table.GetCell(1, 0).Text, "first") {
		t.Errorf("expected tab 0 cell text to contain 'first', got %q", table.GetCell(1, 0).Text)
	}
	if len(batchTabBounds) != 3 {
		t.Fatalf("expected 3 clickable tab hitboxes, got %d", len(batchTabBounds))
	}

	// Cycle forward with ']'
	handled := table.GetInputCapture()(tcell.NewEventKey(tcell.KeyRune, ']', tcell.ModNone))
	if handled != nil {
		t.Errorf("expected ']' key to be consumed by tab switcher")
	}
	if !strings.Contains(table.GetCell(1, 0).Text, "second") {
		t.Errorf("expected cycled tab 1 cell text to contain 'second', got %q", table.GetCell(1, 0).Text)
	}

	// Cycle backward wraps cleanly back to tab 0
	if !cycleBatchResultTab(app, table, -1) {
		t.Errorf("expected cycleBatchResultTab(-1) to return true")
	}
	if !strings.Contains(table.GetCell(1, 0).Text, "first") {
		t.Errorf("expected cycled back tab 0 cell text to contain 'first', got %q", table.GetCell(1, 0).Text)
	}

	clearBatchResultTabs()
	if len(activeBatchTabs) != 0 || resultTabBar.GetText(false) != "" {
		t.Errorf("expected clearBatchResultTabs to reset tab state")
	}
}
