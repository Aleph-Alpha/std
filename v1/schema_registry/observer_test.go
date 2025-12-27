package schema_registry

import (
	"sync"
	"testing"
	"time"

	"github.com/Aleph-Alpha/std/v1/observability"
)

// TestObserver is a mock observer for testing.
type TestObserver struct {
	mu         sync.Mutex
	operations []observability.OperationContext
}

func (t *TestObserver) ObserveOperation(ctx observability.OperationContext) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.operations = append(t.operations, ctx)
}

func (t *TestObserver) GetOperations() []observability.OperationContext {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]observability.OperationContext, len(t.operations))
	copy(out, t.operations)
	return out
}

func TestObserveOperationNilObserverNoPanic(t *testing.T) {
	c := &Client{
		observer: nil,
	}

	// Should not panic.
	c.observeOperation("get_schema", "registry", "123", 10*time.Millisecond, nil, nil)
}

func TestObserveOperationCallsObserver(t *testing.T) {
	obs := &TestObserver{}
	c := &Client{
		observer: obs,
	}

	c.observeOperation("register_schema", "my-subject", "v1", 10*time.Millisecond, nil, map[string]interface{}{"schema_type": "AVRO"})

	ops := obs.GetOperations()
	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
	if ops[0].Component != "schema_registry" {
		t.Fatalf("expected component schema_registry, got %q", ops[0].Component)
	}
	if ops[0].Operation != "register_schema" {
		t.Fatalf("expected operation register_schema, got %q", ops[0].Operation)
	}
	if ops[0].Resource != "my-subject" {
		t.Fatalf("expected resource my-subject, got %q", ops[0].Resource)
	}
	if ops[0].SubResource != "v1" {
		t.Fatalf("expected subresource v1, got %q", ops[0].SubResource)
	}
	if ops[0].Metadata == nil || ops[0].Metadata["schema_type"] != "AVRO" {
		t.Fatalf("expected metadata schema_type=AVRO, got %#v", ops[0].Metadata)
	}
}

func TestWithObserver(t *testing.T) {
	obs := &TestObserver{}
	c := &Client{
		observer: nil,
	}

	if c.observer != nil {
		t.Fatalf("expected no observer initially")
	}

	out := c.WithObserver(obs)
	if out != c {
		t.Fatalf("WithObserver should return same instance for chaining")
	}
	if c.observer != obs {
		t.Fatalf("expected observer to be set")
	}
}
