package rabbit

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Aleph-Alpha/std/v1/observability"
)

// TestObserver is a mock observer for testing
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
	return append([]observability.OperationContext{}, t.operations...)
}

// TestObserverHelperMethod tests the observeOperation helper method
func TestObserverHelperMethod(t *testing.T) {
	testObserver := &TestObserver{}

	client := &RabbitClient{
		cfg: Config{
			Channel: Channel{
				ExchangeName: "test-exchange",
				RoutingKey:   "test-key",
			},
		},
		observer: testObserver,
	}

	// Test observing an operation
	client.observeOperation("produce", "test-exchange", "test-key", 100*time.Millisecond, nil, 1024)

	ops := testObserver.GetOperations()
	if len(ops) != 1 {
		t.Fatalf("Expected 1 operation, got %d", len(ops))
	}

	op := ops[0]
	if op.Component != "rabbit" {
		t.Errorf("Expected component 'rabbit', got %s", op.Component)
	}
	if op.Operation != "produce" {
		t.Errorf("Expected operation 'produce', got %s", op.Operation)
	}
	if op.Resource != "test-exchange" {
		t.Errorf("Expected resource 'test-exchange', got %s", op.Resource)
	}
	if op.SubResource != "test-key" {
		t.Errorf("Expected subResource 'test-key', got %s", op.SubResource)
	}
	if op.Size != 1024 {
		t.Errorf("Expected size 1024, got %d", op.Size)
	}
}

// TestObserverNilObserver tests that operations work without an observer
func TestObserverNilObserver(t *testing.T) {
	client := &RabbitClient{
		cfg: Config{
			Channel: Channel{
				ExchangeName: "test-exchange",
			},
		},
		observer: nil, // No observer
	}

	// Should not panic
	client.observeOperation("produce", "test-exchange", "test-key", 100*time.Millisecond, nil, 512)
}

// TestWithObserver tests the WithObserver builder pattern method
func TestWithObserver(t *testing.T) {
	testObserver := &TestObserver{}

	client := &RabbitClient{
		cfg: Config{
			Channel: Channel{
				ExchangeName: "test-exchange",
			},
		},
	}

	// Verify no observer initially
	if client.observer != nil {
		t.Error("Expected no observer initially")
	}

	// Attach observer using WithObserver
	client = client.WithObserver(testObserver)

	// Verify observer is attached
	if client.observer == nil {
		t.Fatal("Expected observer to be attached")
	}

	// Test that observer is used
	client.observeOperation("consume", "test-queue", "", 50*time.Millisecond, nil, 256)

	ops := testObserver.GetOperations()
	if len(ops) != 1 {
		t.Errorf("Expected observer to record 1 operation, got %d", len(ops))
	}
}

// TestWithObserverChaining tests method chaining with WithObserver
func TestWithObserverChaining(t *testing.T) {
	testObserver := &TestObserver{}

	client := &RabbitClient{
		cfg: Config{
			Channel: Channel{
				ExchangeName: "test-exchange",
			},
		},
	}

	// Chain WithObserver
	result := client.WithObserver(testObserver)

	// Verify it returns the same client instance for chaining
	if result != client {
		t.Error("WithObserver should return the same client instance for chaining")
	}

	// Verify observer is set
	if client.observer != testObserver {
		t.Error("Observer was not set correctly")
	}
}

// MockLogger for testing
type MockLogger struct {
	InfoCalled  bool
	WarnCalled  bool
	ErrorCalled bool
}

func (m *MockLogger) InfoWithContext(ctx context.Context, msg string, err error, fields ...map[string]interface{}) {
	m.InfoCalled = true
}

func (m *MockLogger) WarnWithContext(ctx context.Context, msg string, err error, fields ...map[string]interface{}) {
	m.WarnCalled = true
}

func (m *MockLogger) ErrorWithContext(ctx context.Context, msg string, err error, fields ...map[string]interface{}) {
	m.ErrorCalled = true
}

// TestWithLogger tests the WithLogger builder pattern method
func TestWithLogger(t *testing.T) {
	mockLogger := &MockLogger{}

	client := &RabbitClient{
		cfg: Config{
			Channel: Channel{
				QueueName: "test-queue",
			},
		},
	}

	// Verify no logger initially
	if client.logger != nil {
		t.Error("Expected no logger initially")
	}

	// Attach logger using WithLogger
	client = client.WithLogger(mockLogger)

	// Verify logger is attached
	if client.logger == nil {
		t.Fatal("Expected logger to be attached")
	}

	// Test that logger is used
	client.logInfo(context.Background(), "test message", map[string]interface{}{"key": "value"})

	if !mockLogger.InfoCalled {
		t.Error("Expected logger.Info to be called")
	}
}

// TestBuilderChaining tests chaining multiple builder methods
func TestBuilderChaining(t *testing.T) {
	testObserver := &TestObserver{}
	mockLogger := &MockLogger{}

	client := &RabbitClient{
		cfg: Config{
			Channel: Channel{
				ExchangeName: "test-exchange",
			},
		},
	}

	// Chain both WithObserver and WithLogger
	client = client.
		WithObserver(testObserver).
		WithLogger(mockLogger)

	// Verify all components are attached
	if client.observer != testObserver {
		t.Error("Observer was not attached")
	}
	if client.logger != mockLogger {
		t.Error("Logger was not attached")
	}
}

// BenchmarkObserverOverhead benchmarks the overhead of observer calls
func BenchmarkObserverOverhead(b *testing.B) {
	testObserver := &TestObserver{}

	client := &RabbitClient{
		cfg: Config{
			Channel: Channel{
				ExchangeName: "test-exchange",
			},
		},
		observer: testObserver,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.observeOperation("produce", "test-exchange", "test-key", 1*time.Millisecond, nil, 100)
	}
}

// BenchmarkNoObserver benchmarks operations without an observer
func BenchmarkNoObserver(b *testing.B) {
	client := &RabbitClient{
		cfg: Config{
			Channel: Channel{
				ExchangeName: "test-exchange",
			},
		},
		observer: nil,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.observeOperation("produce", "test-exchange", "test-key", 1*time.Millisecond, nil, 100)
	}
}
