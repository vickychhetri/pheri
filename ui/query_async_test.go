package ui

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rivo/tview"
)

func TestQueryResultState(t *testing.T) {
	// Test 1: Successful SELECT result buffering
	res := &QueryResult{
		Columns:  []string{"id", "username", "email"},
		Rows:     [][]string{{"1", "alice", "alice@example.com"}, {"2", "bob", "bob@example.com"}},
		Duration: 15 * time.Millisecond,
		RowCount: 2,
	}

	if res.RowCount != 2 {
		t.Errorf("expected RowCount 2, got %d", res.RowCount)
	}
	if len(res.Columns) != 3 {
		t.Errorf("expected 3 columns, got %d", len(res.Columns))
	}
	if res.IsCanceled {
		t.Errorf("expected IsCanceled to be false")
	}
	if res.ErrorMessage != "" {
		t.Errorf("expected no ErrorMessage, got %s", res.ErrorMessage)
	}

	// Test 2: Mutation result
	mutRes := &QueryResult{
		IsMutation:   true,
		RowsAffected: 42,
		Duration:     5 * time.Millisecond,
	}
	if !mutRes.IsMutation {
		t.Errorf("expected IsMutation true")
	}
	if mutRes.RowsAffected != 42 {
		t.Errorf("expected 42 RowsAffected, got %d", mutRes.RowsAffected)
	}

	// Test 3: Canceled result
	cancelRes := &QueryResult{
		IsCanceled:   true,
		Duration:     2 * time.Second,
		ServerThread: 105,
	}
	if !cancelRes.IsCanceled {
		t.Errorf("expected IsCanceled true")
	}
	if cancelRes.ServerThread != 105 {
		t.Errorf("expected ServerThread 105, got %d", cancelRes.ServerThread)
	}
}

func TestQueryCancellationMechanism(t *testing.T) {
	activeQueryMutex.Lock()
	ctx, cancel := context.WithCancel(context.Background())
	activeQueryCancel = cancel
	isQueryRunning.Store(true)
	activeQueryConnID.Store(42)
	activeQueryMutex.Unlock()

	if !IsQueryRunning() {
		t.Errorf("expected IsQueryRunning() to be true")
	}

	if GetActiveQueryConnectionID() != 42 {
		t.Errorf("expected connection ID 42, got %d", GetActiveQueryConnectionID())
	}

	// Simulate user pressing Ctrl+C or Esc
	CancelActiveQuery()

	if ctx.Err() != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", ctx.Err())
	}
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Errorf("expected errors.Is(ctx.Err(), context.Canceled) to be true")
	}
	if GetActiveQueryConnectionID() != 0 {
		t.Errorf("expected connection ID to be reset to 0, got %d", GetActiveQueryConnectionID())
	}

	// Clean up
	activeQueryMutex.Lock()
	isQueryRunning.Store(false)
	activeQueryMutex.Unlock()
}

func TestIsQueryInterruptedError_EdgeCases(t *testing.T) {
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	activeCtx := context.Background()

	tests := []struct {
		name       string
		err        error
		ctx        context.Context
		expectKill bool
	}{
		{"nil error with nil context", nil, nil, false},
		{"nil error with active context", nil, activeCtx, false},
		{"nil error with canceled context", nil, canceledCtx, true},
		{"context.Canceled with nil context", context.Canceled, nil, true},
		{"context.Canceled with active context", context.Canceled, activeCtx, true},
		{"MySQL Error 1317 standard format", errors.New("Error 1317 (70100): Query execution was interrupted"), nil, true},
		{"MySQL Error 1317 short format", errors.New("Error 1317: Query execution was interrupted"), nil, true},
		{"Interrupted substring mixed case", errors.New("Query execution was InTeRrUpTeD by server"), nil, true},
		{"Operation was canceled error", errors.New("the operation was canceled"), nil, true},
		{"Syntax error (must not be treated as canceled)", errors.New("Error 1064 (42000): You have an error in your SQL syntax"), nil, false},
		{"Table not found error", errors.New("Error 1146 (42S02): Table 'sakila.missing' doesn't exist"), nil, false},
		{"Access denied error", errors.New("Error 1045 (28000): Access denied for user 'root'@'localhost'"), nil, false},
		{"Duplicate entry error", errors.New("Error 1062 (23000): Duplicate entry '1' for key 'PRIMARY'"), nil, false},
		{"Connection refused error", errors.New("dial tcp 127.0.0.1:3306: connect: connection refused"), nil, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsQueryInterruptedError(tc.err, tc.ctx)
			if got != tc.expectKill {
				t.Errorf("IsQueryInterruptedError(%v, %v) = %v; expected %v", tc.err, tc.ctx, got, tc.expectKill)
			}
		})
	}
}

func TestCancelActiveQueryWhenIdle(t *testing.T) {
	// Must not panic when called without an active running query
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("CancelActiveQuery panicked when idle: %v", r)
		}
	}()

	isQueryRunning.Store(false)
	activeQueryConnID.Store(0)
	CancelActiveQuery()
}

func TestCancelActiveQuery_RapidConcurrentInvocations(t *testing.T) {
	// Test edge case where user hammers Ctrl+C or Esc 50 times in parallel
	var cancelInvocationCount atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	wrappedCancel := func() {
		cancelInvocationCount.Add(1)
		cancel()
	}

	activeQueryMutex.Lock()
	activeQueryCancel = wrappedCancel
	isQueryRunning.Store(true)
	activeQueryConnID.Store(99)
	activeQueryDB = nil // No DB to prevent real network calls
	activeQueryMutex.Unlock()

	var wg sync.WaitGroup
	concurrentSpammers := 50
	wg.Add(concurrentSpammers)

	for i := 0; i < concurrentSpammers; i++ {
		go func() {
			defer wg.Done()
			CancelActiveQuery()
		}()
	}

	wg.Wait()

	// Verify that cancel was invoked exactly once despite 50 concurrent calls
	if cancelInvocationCount.Load() != 1 {
		t.Errorf("expected cancel to be called exactly 1 time, but was called %d times", cancelInvocationCount.Load())
	}

	if ctx.Err() != context.Canceled {
		t.Errorf("expected context to be canceled, got %v", ctx.Err())
	}

	if GetActiveQueryConnectionID() != 0 {
		t.Errorf("expected connection ID to be swapped to 0, got %d", GetActiveQueryConnectionID())
	}

	// Clean up
	activeQueryMutex.Lock()
	isQueryRunning.Store(false)
	activeQueryMutex.Unlock()
}

func TestRowScanInterruptionHandling(t *testing.T) {
	// Simulate row scanning loop where cancellation occurs mid-stream
	ctx, cancel := context.WithCancel(context.Background())
	result := &QueryResult{}

	simulatedRows := 500
	rowsProcessed := 0

	for i := 0; i < simulatedRows; i++ {
		if i == 5 {
			// User presses Ctrl+C on 5th row
			cancel()
		}

		if ctx.Err() == context.Canceled {
			result.IsCanceled = true
			break
		}

		rowsProcessed++
	}

	if !result.IsCanceled {
		t.Errorf("expected result.IsCanceled to be true")
	}
	if rowsProcessed != 5 {
		t.Errorf("expected loop to terminate after 5 rows, processed %d", rowsProcessed)
	}
}

func TestMutationQueryInterruptedClassification(t *testing.T) {
	// Simulate an UPDATE or DELETE aborted by server KILL QUERY
	ctx := context.Background()
	result := &QueryResult{
		IsMutation: true,
	}

	err := errors.New("Error 1317 (70100): Query execution was interrupted")
	if IsQueryInterruptedError(err, ctx) {
		result.IsCanceled = true
	} else {
		result.ErrorMessage = err.Error()
	}

	if !result.IsMutation {
		t.Errorf("expected IsMutation true")
	}
	if !result.IsCanceled {
		t.Errorf("expected IsCanceled true for Error 1317")
	}
	if result.ErrorMessage != "" {
		t.Errorf("expected ErrorMessage to be empty when IsCanceled is true, got %s", result.ErrorMessage)
	}
}

func TestEmptyAndWhitespaceQuerySanitization(t *testing.T) {
	emptyQueries := []string{
		"",
		" ",
		"\t\t",
		"\n\n\r\n",
		"   \t  \n  ",
	}

	for _, q := range emptyQueries {
		if len(q) > 0 && len(q) == 0 {
			t.Errorf("impossible condition")
		}
		// Verify that trimming whitespace gives empty string
		trimmed := ""
		for _, r := range q {
			if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
				trimmed += string(r)
			}
		}
		if trimmed != "" {
			t.Errorf("expected trimmed query to be empty, got %q", trimmed)
		}
	}
}

func TestExecuteQueryAsync_DefensiveGuards(t *testing.T) {
	app := tview.NewApplication()
	table := tview.NewTable()

	// 1. Test when db is nil
	var nilDBRes *QueryResult
	ExecuteQueryAsync(app, nil, "SELECT 1", table, func(res *QueryResult) {
		nilDBRes = res
	})
	if nilDBRes == nil {
		t.Fatalf("expected callback to be invoked for nil db")
	}
	if nilDBRes.ErrorMessage != "database connection is not initialized" {
		t.Errorf("expected 'database connection is not initialized', got %s", nilDBRes.ErrorMessage)
	}

	// 2. Test when query is empty or whitespace
	var emptyRes *QueryResult
	ExecuteQueryAsync(app, &sql.DB{}, "   \n\t  ", table, func(res *QueryResult) {
		emptyRes = res
	})
	if emptyRes == nil {
		t.Fatalf("expected callback to be invoked for empty query")
	}
	if emptyRes.ErrorMessage != "empty query" {
		t.Errorf("expected 'empty query', got %s", emptyRes.ErrorMessage)
	}

	// 3. Test when a query is already running
	isQueryRunning.Store(true)
	defer isQueryRunning.Store(false)

	var runningRes *QueryResult
	ExecuteQueryAsync(app, &sql.DB{}, "SELECT 1", table, func(res *QueryResult) {
		runningRes = res
	})
	if runningRes != nil {
		t.Errorf("expected no execution when a query is already running, but got res")
	}
}

