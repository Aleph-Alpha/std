package observability_test

import (
	"testing"
	"time"

	"github.com/Aleph-Alpha/std/v1/observability"
)

func TestOperationContext(t *testing.T) {
	ctx := observability.OperationContext{
		Component:   "postgres",
		Operation:   "insert",
		Resource:    "users",
		SubResource: "",
		Duration:    23 * time.Millisecond,
		Error:       nil,
		Size:        1,
		Metadata: map[string]interface{}{
			"query_type": "simple",
		},
	}

	if ctx.Component != "postgres" {
		t.Errorf("expected component 'postgres', got '%s'", ctx.Component)
	}

	if ctx.Operation != "insert" {
		t.Errorf("expected operation 'insert', got '%s'", ctx.Operation)
	}

	if ctx.Duration != 23*time.Millisecond {
		t.Errorf("expected duration 23ms, got %v", ctx.Duration)
	}
}

func TestNoOpObserver(t *testing.T) {
	observer := observability.NewNoOpObserver()

	// Should not panic
	observer.ObserveOperation(observability.OperationContext{
		Component: "test",
		Operation: "test",
	})
}

// Mock observer for testing
type mockObserver struct {
	called bool
	ctx    observability.OperationContext
}

func (m *mockObserver) ObserveOperation(ctx observability.OperationContext) {
	m.called = true
	m.ctx = ctx
}

func TestMockObserver(t *testing.T) {
	mock := &mockObserver{}

	ctx := observability.OperationContext{
		Component: "postgres",
		Operation: "insert",
		Resource:  "users",
		Duration:  10 * time.Millisecond,
	}

	mock.ObserveOperation(ctx)

	if !mock.called {
		t.Error("expected observer to be called")
	}

	if mock.ctx.Component != "postgres" {
		t.Errorf("expected component 'postgres', got '%s'", mock.ctx.Component)
	}
}
