package postgres

import (
	"sync"
	"testing"
	"time"

	"github.com/Aleph-Alpha/std/v1/observability"
	"gorm.io/gorm"
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
	pg := &Postgres{
		cfg:      Config{Connection: Connection{DbName: "testdb"}},
		observer: nil,
	}

	// Should not panic.
	pg.observeOperation("find", "users", "", 10*time.Millisecond, nil, 1, nil)
}

func TestObserveOperationCallsObserver(t *testing.T) {
	obs := &TestObserver{}
	pg := &Postgres{
		cfg:      Config{Connection: Connection{DbName: "testdb"}},
		observer: obs,
	}

	pg.observeOperation("find", "users", "", 10*time.Millisecond, nil, 3, map[string]interface{}{"foo": "bar"})

	ops := obs.GetOperations()
	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
	if ops[0].Component != "postgres" {
		t.Fatalf("expected component postgres, got %q", ops[0].Component)
	}
	if ops[0].Operation != "find" {
		t.Fatalf("expected operation find, got %q", ops[0].Operation)
	}
	if ops[0].Resource != "users" {
		t.Fatalf("expected resource users, got %q", ops[0].Resource)
	}
	if ops[0].Size != 3 {
		t.Fatalf("expected size 3, got %d", ops[0].Size)
	}
	if ops[0].Metadata == nil || ops[0].Metadata["foo"] != "bar" {
		t.Fatalf("expected metadata foo=bar, got %#v", ops[0].Metadata)
	}
}

func TestObserveOperationResourceFallbackToDbName(t *testing.T) {
	obs := &TestObserver{}
	pg := &Postgres{
		cfg:      Config{Connection: Connection{DbName: "testdb"}},
		observer: obs,
	}

	pg.observeOperation("exec", "", "", 1*time.Millisecond, nil, 0, nil)

	ops := obs.GetOperations()
	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
	if ops[0].Resource != "testdb" {
		t.Fatalf("expected fallback resource testdb, got %q", ops[0].Resource)
	}
}

func TestWithObserver(t *testing.T) {
	obs := &TestObserver{}
	pg := &Postgres{
		cfg:      Config{Connection: Connection{DbName: "testdb"}},
		observer: nil,
	}

	if pg.observer != nil {
		t.Fatalf("expected no observer initially")
	}

	out := pg.WithObserver(obs)
	if out != pg {
		t.Fatalf("WithObserver should return same instance for chaining")
	}
	if pg.observer != obs {
		t.Fatalf("expected observer to be set")
	}
}

func TestCloneWithTxCopiesObserverAndLogger(t *testing.T) {
	obs := &TestObserver{}
	pg := &Postgres{
		cfg:      Config{Connection: Connection{DbName: "testdb"}},
		observer: obs,
		logger:   nil,
	}

	tx := &gorm.DB{}
	cloned := pg.cloneWithTx(tx)

	if cloned.observer != obs {
		t.Fatalf("expected clone to copy observer")
	}
	if cloned.logger != pg.logger {
		t.Fatalf("expected clone to copy logger")
	}
}
