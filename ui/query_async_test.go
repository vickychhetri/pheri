package ui

import (
	"context"
	"errors"
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
	activeQueryMutex.Unlock()

	if !IsQueryRunning() {
		t.Errorf("expected IsQueryRunning() to be true")
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
