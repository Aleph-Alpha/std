package redis

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
	r := &RedisClient{
		observer: nil,
	}

	// Should not panic.
	r.observeOperation("get", "test-key", "", 10*time.Millisecond, nil, 0, nil)
}

func TestObserveOperationCallsObserver(t *testing.T) {
	obs := &TestObserver{}
	r := &RedisClient{
		observer: obs,
	}

	r.observeOperation("set", "my-key", "", 10*time.Millisecond, nil, 100, map[string]interface{}{"ttl": "60s"})

	ops := obs.GetOperations()
	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
	if ops[0].Component != "redis" {
		t.Fatalf("expected component redis, got %q", ops[0].Component)
	}
	if ops[0].Operation != "set" {
		t.Fatalf("expected operation set, got %q", ops[0].Operation)
	}
	if ops[0].Resource != "my-key" {
		t.Fatalf("expected resource my-key, got %q", ops[0].Resource)
	}
	if ops[0].Size != 100 {
		t.Fatalf("expected size 100, got %d", ops[0].Size)
	}
	if ops[0].Metadata == nil || ops[0].Metadata["ttl"] != "60s" {
		t.Fatalf("expected metadata ttl=60s, got %#v", ops[0].Metadata)
	}
}

func TestWithObserver(t *testing.T) {
	obs := &TestObserver{}
	r := &RedisClient{
		observer: nil,
	}

	if r.observer != nil {
		t.Fatalf("expected no observer initially")
	}

	out := r.WithObserver(obs)
	if out != r {
		t.Fatalf("WithObserver should return same instance for chaining")
	}
	if r.observer != obs {
		t.Fatalf("expected observer to be set")
	}
}
