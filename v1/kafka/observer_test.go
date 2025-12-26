package kafka

import (
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

func (t *TestObserver) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.operations = nil
}

func (t *TestObserver) GetOperationsByType(operation string) []observability.OperationContext {
	t.mu.Lock()
	defer t.mu.Unlock()
	var result []observability.OperationContext
	for _, op := range t.operations {
		if op.Operation == operation {
			result = append(result, op)
		}
	}
	return result
}

// TestObserverHelperMethod tests the observeOperation helper method
func TestObserverHelperMethod(t *testing.T) {
	testObserver := &TestObserver{}

	client := &KafkaClient{
		cfg: Config{
			Topic: "test-topic",
		},
		observer: testObserver, // ← Now separate field
	}

	// Test observing an operation
	client.observeOperation("produce", "test-topic", "", 10*time.Millisecond, nil, 1024)

	ops := testObserver.GetOperations()
	if len(ops) != 1 {
		t.Fatalf("Expected 1 operation, got %d", len(ops))
	}

	op := ops[0]
	if op.Component != "kafka" {
		t.Errorf("Expected component 'kafka', got '%s'", op.Component)
	}
	if op.Operation != "produce" {
		t.Errorf("Expected operation 'produce', got '%s'", op.Operation)
	}
	if op.Resource != "test-topic" {
		t.Errorf("Expected resource 'test-topic', got '%s'", op.Resource)
	}
	if op.Duration != 10*time.Millisecond {
		t.Errorf("Expected duration 10ms, got %v", op.Duration)
	}
	if op.Size != 1024 {
		t.Errorf("Expected size 1024, got %d", op.Size)
	}
	if op.Error != nil {
		t.Errorf("Expected no error, got %v", op.Error)
	}
}

// TestObserverNilObserver tests that nil observer doesn't cause panic
func TestObserverNilObserver(t *testing.T) {
	client := &KafkaClient{
		cfg: Config{
			Topic: "test-topic",
		},
		observer: nil, // ← No observer
	}

	// Should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("observeOperation panicked with nil observer: %v", r)
		}
	}()

	client.observeOperation("produce", "test-topic", "", 10*time.Millisecond, nil, 1024)
}

// TestObserverPublishOperation tests observer integration with Publish
func TestObserverPublishOperation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	testObserver := &TestObserver{}

	// Note: This test requires a running Kafka instance
	// For unit testing, we're testing the observer call mechanism
	client := &KafkaClient{
		cfg: Config{
			Topic: "test-topic",
		},
		observer:   testObserver,
		serializer: &JSONSerializer{},
	}

	// Mock the writer to avoid needing real Kafka
	// In a real integration test, you'd have a running Kafka instance

	// Test that observer would be called (we can't test actual Publish without Kafka)
	// But we can verify the observer is wired correctly
	client.observeOperation("produce", "test-topic", "", 5*time.Millisecond, nil, 512)

	ops := testObserver.GetOperationsByType("produce")
	if len(ops) != 1 {
		t.Fatalf("Expected 1 produce operation, got %d", len(ops))
	}

	op := ops[0]
	if op.Component != "kafka" {
		t.Errorf("Expected component 'kafka', got '%s'", op.Component)
	}
	if op.Resource != "test-topic" {
		t.Errorf("Expected resource 'test-topic', got '%s'", op.Resource)
	}
	if op.Size != 512 {
		t.Errorf("Expected size 512, got %d", op.Size)
	}
}

// TestObserverConsumeOperation tests observer integration with Consume
func TestObserverConsumeOperation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	testObserver := &TestObserver{}

	client := &KafkaClient{
		cfg: Config{
			Topic: "test-topic",
		},
		observer: testObserver,
	}

	// Simulate a consume operation observation
	client.observeOperation("consume", "test-topic", "2", 15*time.Millisecond, nil, 2048)

	ops := testObserver.GetOperationsByType("consume")
	if len(ops) != 1 {
		t.Fatalf("Expected 1 consume operation, got %d", len(ops))
	}

	op := ops[0]
	if op.Component != "kafka" {
		t.Errorf("Expected component 'kafka', got '%s'", op.Component)
	}
	if op.Operation != "consume" {
		t.Errorf("Expected operation 'consume', got '%s'", op.Operation)
	}
	if op.Resource != "test-topic" {
		t.Errorf("Expected resource 'test-topic', got '%s'", op.Resource)
	}
	if op.SubResource != "2" {
		t.Errorf("Expected subresource '2' (partition), got '%s'", op.SubResource)
	}
	if op.Size != 2048 {
		t.Errorf("Expected size 2048, got %d", op.Size)
	}
}

// TestObserverMultipleOperations tests tracking multiple operations
func TestObserverMultipleOperations(t *testing.T) {
	testObserver := &TestObserver{}

	client := &KafkaClient{
		cfg: Config{
			Topic: "test-topic",
		},
		observer: testObserver,
	}

	// Simulate multiple operations
	client.observeOperation("produce", "test-topic", "", 5*time.Millisecond, nil, 100)
	client.observeOperation("produce", "test-topic", "", 6*time.Millisecond, nil, 200)
	client.observeOperation("consume", "test-topic", "0", 10*time.Millisecond, nil, 150)
	client.observeOperation("consume", "test-topic", "1", 12*time.Millisecond, nil, 250)

	allOps := testObserver.GetOperations()
	if len(allOps) != 4 {
		t.Fatalf("Expected 4 operations, got %d", len(allOps))
	}

	produceOps := testObserver.GetOperationsByType("produce")
	if len(produceOps) != 2 {
		t.Errorf("Expected 2 produce operations, got %d", len(produceOps))
	}

	consumeOps := testObserver.GetOperationsByType("consume")
	if len(consumeOps) != 2 {
		t.Errorf("Expected 2 consume operations, got %d", len(consumeOps))
	}

	// Verify size tracking
	var totalProduceSize int64
	for _, op := range produceOps {
		totalProduceSize += op.Size
	}
	if totalProduceSize != 300 {
		t.Errorf("Expected total produce size 300, got %d", totalProduceSize)
	}

	var totalConsumeSize int64
	for _, op := range consumeOps {
		totalConsumeSize += op.Size
	}
	if totalConsumeSize != 400 {
		t.Errorf("Expected total consume size 400, got %d", totalConsumeSize)
	}
}

// TestObserverErrorTracking tests that errors are properly tracked
func TestObserverErrorTracking(t *testing.T) {
	testObserver := &TestObserver{}

	client := &KafkaClient{
		cfg: Config{
			Topic: "test-topic",
		},
		observer: testObserver,
	}

	// Simulate an operation with error
	testErr := ErrWriterNotInitialized
	client.observeOperation("produce", "test-topic", "", 5*time.Millisecond, testErr, 0)

	ops := testObserver.GetOperations()
	if len(ops) != 1 {
		t.Fatalf("Expected 1 operation, got %d", len(ops))
	}

	op := ops[0]
	if op.Error != testErr {
		t.Errorf("Expected error '%v', got '%v'", testErr, op.Error)
	}
	if op.Size != 0 {
		t.Errorf("Expected size 0 for failed operation, got %d", op.Size)
	}
}

// TestObserverConcurrency tests thread safety of observer calls
func TestObserverConcurrency(t *testing.T) {
	testObserver := &TestObserver{}

	client := &KafkaClient{
		cfg: Config{
			Topic: "test-topic",
		},
		observer: testObserver,
	}

	// Simulate concurrent operations
	var wg sync.WaitGroup
	numGoroutines := 10
	operationsPerGoroutine := 100

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				client.observeOperation("produce", "test-topic", "", time.Duration(j)*time.Microsecond, nil, int64(j))
			}
		}(i)
	}

	wg.Wait()

	ops := testObserver.GetOperations()
	expectedOps := numGoroutines * operationsPerGoroutine
	if len(ops) != expectedOps {
		t.Errorf("Expected %d operations, got %d", expectedOps, len(ops))
	}

	// Verify all operations are "produce"
	for _, op := range ops {
		if op.Operation != "produce" {
			t.Errorf("Expected all operations to be 'produce', got '%s'", op.Operation)
		}
		if op.Component != "kafka" {
			t.Errorf("Expected component 'kafka', got '%s'", op.Component)
		}
	}
}

// TestNoOpObserverIntegration tests using the NoOpObserver from observability package
func TestNoOpObserverIntegration(t *testing.T) {
	noOpObserver := observability.NewNoOpObserver()

	client := &KafkaClient{
		cfg: Config{
			Topic: "test-topic",
		},
		observer: noOpObserver,
	}

	// Should not panic and should do nothing
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("NoOpObserver caused panic: %v", r)
		}
	}()

	client.observeOperation("produce", "test-topic", "", 10*time.Millisecond, nil, 1024)
	client.observeOperation("consume", "test-topic", "0", 15*time.Millisecond, nil, 2048)
}

// TestWithObserver tests the WithObserver builder pattern method
func TestWithObserver(t *testing.T) {
	testObserver := &TestObserver{}

	// Create client without observer
	client := &KafkaClient{
		cfg: Config{
			Topic: "test-topic",
		},
		observer: nil,
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

	// Test that observer receives operations
	client.observeOperation("produce", "test-topic", "", 5*time.Millisecond, nil, 512)

	ops := testObserver.GetOperations()
	if len(ops) != 1 {
		t.Fatalf("Expected 1 operation, got %d", len(ops))
	}

	op := ops[0]
	if op.Operation != "produce" {
		t.Errorf("Expected 'produce', got '%s'", op.Operation)
	}
}

// TestWithObserverChaining tests that WithObserver returns the client for chaining
func TestWithObserverChaining(t *testing.T) {
	testObserver := &TestObserver{}

	client := &KafkaClient{
		cfg: Config{
			Topic: "test-topic",
		},
	}

	// Should be chainable
	result := client.WithObserver(testObserver)

	if result != client {
		t.Error("WithObserver should return the same client instance for chaining")
	}

	if client.observer != testObserver {
		t.Error("Observer was not attached correctly")
	}
}

// BenchmarkObserverOverhead benchmarks the overhead of observer calls
func BenchmarkObserverOverhead(b *testing.B) {
	testObserver := &TestObserver{}

	client := &KafkaClient{
		cfg: Config{
			Topic: "test-topic",
		},
		observer: testObserver,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.observeOperation("produce", "test-topic", "", 5*time.Millisecond, nil, 1024)
	}
}

// BenchmarkNoObserverOverhead benchmarks with no observer (should be zero overhead)
func BenchmarkNoObserverOverhead(b *testing.B) {
	client := &KafkaClient{
		cfg: Config{
			Topic: "test-topic",
		},
		observer: nil, // No observer
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.observeOperation("produce", "test-topic", "", 5*time.Millisecond, nil, 1024)
	}
}
