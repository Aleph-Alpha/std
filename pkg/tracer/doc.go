// Package tracer provides a simple, consistent API for distributed tracing in Go applications
// using OpenTelemetry standards. It handles span creation, propagation, attribute management,
// and trace exporting with minimal configuration.
//
// The package includes:
//   - Simple span creation and management
//   - Error recording and status tracking
//   - Automatic trace context propagation across service boundaries
//   - Flexible attribute recording with type support
//   - Integration with Uber FX for dependency injection
//
// # Basic Usage
//
// Initialize the tracer:
//
//	cfg := tracer.Config{
//	    ServiceName: "user-service",
//	    AppEnv: "production",
//	    EnableExport: true,
//	}
//	tracerClient := tracer.NewClient(cfg, logger)
//
// Create spans for operations:
//
//	ctx, span := tracerClient.StartSpan(ctx, "fetch-user-data")
//	defer span.End()
//
// Record errors when they occur:
//
//	if err != nil {
//	    tracerClient.RecordErrorOnSpan(span, err)
//	    return nil, err
//	}
//
// Add attributes to provide context:
//
//	tracerClient.SetAttributes(span, map[string]interface{}{
//	    "user.id": userID,
//	    "request.size": size,
//	    "cache.hit": true,
//	})
//
// # Distributed Tracing
//
// For outgoing requests, extract trace context:
//
//	headers := tracerClient.GetCarrier(ctx)
//	// Add headers to outgoing HTTP request or message
//
// For incoming requests, import trace context:
//
//	// Extract headers from incoming HTTP request or message
//	ctx = tracerClient.SetCarrierOnContext(ctx, headers)
//
// # Uber FX Integration
//
// To use with Uber FX, import the module in your application:
//
//	app := fx.New(
//	    tracer.FXModule,
//	    // other modules...
//	)
//
// This will automatically set up the tracer and manage its lifecycle.
//
// # Best Practices
//
// 1. Always end spans with defer span.End()
// 2. Create spans for significant operations, not every function call
// 3. Add attributes that provide business context to aid troubleshooting
// 4. Propagate context through your call stack to maintain the trace
// 5. Record errors on spans to correlate exceptions with traces
package tracer
