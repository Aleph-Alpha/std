package observability

// NoOpObserver is a no-op implementation of Observer.
// It does nothing when ObserveOperation is called.
// This can be useful for testing or as a default value.
type NoOpObserver struct{}

// ObserveOperation does nothing (no-op).
func (n *NoOpObserver) ObserveOperation(ctx OperationContext) {
	// No-op
}

// NewNoOpObserver creates a new NoOpObserver.
func NewNoOpObserver() Observer {
	return &NoOpObserver{}
}
