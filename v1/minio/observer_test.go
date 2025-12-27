package minio

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
	m := &MinioClient{
		cfg:      Config{Connection: ConnectionConfig{BucketName: "testbucket"}},
		observer: nil,
	}

	// Should not panic.
	m.observeOperation("put", "testbucket", "test-key", 10*time.Millisecond, nil, 1024, nil)
}

func TestObserveOperationCallsObserver(t *testing.T) {
	obs := &TestObserver{}
	m := &MinioClient{
		cfg:      Config{Connection: ConnectionConfig{BucketName: "testbucket"}},
		observer: obs,
	}

	m.observeOperation("get", "testbucket", "test-key", 10*time.Millisecond, nil, 2048, map[string]interface{}{"foo": "bar"})

	ops := obs.GetOperations()
	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
	if ops[0].Component != "minio" {
		t.Fatalf("expected component minio, got %q", ops[0].Component)
	}
	if ops[0].Operation != "get" {
		t.Fatalf("expected operation get, got %q", ops[0].Operation)
	}
	if ops[0].Resource != "testbucket" {
		t.Fatalf("expected resource testbucket, got %q", ops[0].Resource)
	}
	if ops[0].SubResource != "test-key" {
		t.Fatalf("expected subresource test-key, got %q", ops[0].SubResource)
	}
	if ops[0].Size != 2048 {
		t.Fatalf("expected size 2048, got %d", ops[0].Size)
	}
	if ops[0].Metadata == nil || ops[0].Metadata["foo"] != "bar" {
		t.Fatalf("expected metadata foo=bar, got %#v", ops[0].Metadata)
	}
}

func TestObserveOperationResourceFallbackToBucketName(t *testing.T) {
	obs := &TestObserver{}
	m := &MinioClient{
		cfg:      Config{Connection: ConnectionConfig{BucketName: "testbucket"}},
		observer: obs,
	}

	m.observeOperation("list", "", "prefix/", 1*time.Millisecond, nil, 0, nil)

	ops := obs.GetOperations()
	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
	if ops[0].Resource != "testbucket" {
		t.Fatalf("expected fallback resource testbucket, got %q", ops[0].Resource)
	}
}

func TestWithObserver(t *testing.T) {
	obs := &TestObserver{}
	m := &MinioClient{
		cfg:      Config{Connection: ConnectionConfig{BucketName: "testbucket"}},
		observer: nil,
	}

	if m.observer != nil {
		t.Fatalf("expected no observer initially")
	}

	out := m.WithObserver(obs)
	if out != m {
		t.Fatalf("WithObserver should return same instance for chaining")
	}
	if m.observer != obs {
		t.Fatalf("expected observer to be set")
	}
}
