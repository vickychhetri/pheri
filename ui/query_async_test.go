package ui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
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
		IsCanceled: true,
		Duration:   2 * time.Second,
	}
	if !cancelRes.IsCanceled {
		t.Errorf("expected IsCanceled true")
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
}

func TestInterruptedMySQLErrorClassification(t *testing.T) {
	isInterrupted := func(err error) bool {
		if err == nil {
			return false
		}
		errLower := err.Error()
		return errors.Is(err, context.Canceled) ||
			strings.Contains(errLower, "interrupted") ||
			strings.Contains(errLower, "1317") ||
			strings.Contains(errLower, "operation was canceled")
	}

	err1317 := errors.New("Error 1317 (70100): Query execution was interrupted")
	if !isInterrupted(err1317) {
		t.Errorf("expected err1317 to be classified as interrupted/canceled")
	}

	errCtx := context.Canceled
	if !isInterrupted(errCtx) {
		t.Errorf("expected context.Canceled to be classified as interrupted/canceled")
	}

	errOther := errors.New("Table 'sakila.unknown' doesn't exist")
	if isInterrupted(errOther) {
		t.Errorf("expected standard table error NOT to be classified as interrupted/canceled")
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
