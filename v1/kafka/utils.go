package kafka

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
)

// ConsumerMessage implements the Message interface and wraps a Kafka message.
type ConsumerMessage struct {
	message      kafka.Message
	reader       *kafka.Reader
	deserializer Deserializer // Reference to deserializer for BodyAs
}

// Consume starts consuming messages from the topic specified in the configuration.
// This method provides a channel where consumed messages will be delivered.
//
// Parameters:
//   - ctx: Context for cancellation control
//   - wg: WaitGroup for coordinating shutdown
//
// Returns a channel that delivers Message interfaces for each consumed message.
//
// Example:
//
//	wg := &sync.WaitGroup{}
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	msgChan := kafkaClient.Consume(ctx, wg)
//	for msg := range msgChan {
//	    // Option 1: Use BodyAs for automatic deserialization
//	    var event UserEvent
//	    if err := msg.BodyAs(&event); err != nil {
//	        log.Printf("Failed to deserialize: %v", err)
//	        continue
//	    }
//
//	    // Option 2: Use Body() for raw bytes
//	    rawBytes := msg.Body()
//	    fmt.Println("Received:", string(rawBytes))
//
//	    // Commit successful processing
//	    if err := msg.CommitMsg(); err != nil {
//	        log.Printf("Failed to commit message: %v", err)
//	    }
//	}
func (k *KafkaClient) Consume(ctx context.Context, wg *sync.WaitGroup) <-chan Message {
	return k.ConsumeParallel(ctx, wg, 1)
}

// ConsumeParallel starts consuming messages from the topic with multiple concurrent goroutines.
// This method provides better throughput for high-volume topics by processing messages in parallel.
//
// Parameters:
//   - ctx: Context for cancellation control
//   - wg: WaitGroup for coordinating shutdown
//   - numWorkers: Number of concurrent goroutines to use for consuming (recommended: 1-10)
//
// Returns a channel that delivers Message interfaces for each consumed message.
//
// Example:
//
//	wg := &sync.WaitGroup{}
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	// Use 5 concurrent workers for high throughput
//	msgChan := kafkaClient.ConsumeParallel(ctx, wg, 5)
//	for msg := range msgChan {
//	    // Process the message
//	    fmt.Println("Received:", string(msg.Body()))
//
//	    // Commit successful processing
//	    if err := msg.CommitMsg(); err != nil {
//	        log.Printf("Failed to commit message: %v", err)
//	    }
//	}
func (k *KafkaClient) ConsumeParallel(ctx context.Context, wg *sync.WaitGroup, numWorkers int) <-chan Message {
	if numWorkers < 1 {
		numWorkers = 1
	}

	outChan := make(chan Message, 100*numWorkers)

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(outChan)

		workerWg := &sync.WaitGroup{}

		// Start multiple worker goroutines
		for i := 0; i < numWorkers; i++ {
			workerWg.Add(1)
			go func(workerID int) {
				defer workerWg.Done()
				k.consumeWorker(ctx, outChan, workerID)
			}(i)
		}

		workerWg.Wait()
	}()

	return outChan
}

// consumeWorker is a worker goroutine that fetches and sends messages
func (k *KafkaClient) consumeWorker(ctx context.Context, outChan chan<- Message, workerID int) {
	for {
		select {
		case <-k.shutdownSignal:
			k.logInfo(ctx, "Stopping consumer worker due to shutdown signal", map[string]interface{}{
				"worker_id": workerID,
			})
			return
		case <-ctx.Done():
			k.logInfo(ctx, "Stopping consumer worker due to context cancellation", map[string]interface{}{
				"worker_id": workerID,
				"error":     ctx.Err().Error(),
			})
			return
		default:
			start := time.Now()
			k.mu.RLock()
			reader := k.reader
			k.mu.RUnlock()

			if reader == nil {
				k.logError(ctx, "Kafka reader is not initialized", map[string]interface{}{
					"worker_id": workerID,
				})
				return
			}

			msg, err := reader.FetchMessage(ctx)

			// Observe the consume operation
			msgSize := int64(0)
			if err == nil {
				msgSize = int64(len(msg.Value))
			}
			k.observeOperation("consume", k.cfg.Topic, fmt.Sprintf("%d", msg.Partition), time.Since(start), err, msgSize)

			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					k.logInfo(ctx, "Consumer worker context cancelled", map[string]interface{}{
						"worker_id": workerID,
						"error":     err.Error(),
					})
					return
				}
				k.logError(ctx, "Worker failed to fetch message", map[string]interface{}{
					"worker_id": workerID,
					"error":     err.Error(),
				})
				continue
			}

			select {
			case outChan <- &ConsumerMessage{
				message:      msg,
				reader:       reader,
				deserializer: k.deserializer,
			}:
			case <-ctx.Done():
				return
			case <-k.shutdownSignal:
				return
			}
		}
	}
}

// Publish sends a message to the Kafka topic specified in the configuration.
// This method is thread-safe and respects context cancellation.
//
// Parameters:
//   - ctx: Context for cancellation control
//   - key: Message key for partitioning
//   - msg: Message payload as a byte slice
//   - headers: Optional message headers as a map of key-value pairs; can be used for metadata
//     and distributed tracing propagation
//
// The headers parameter is particularly useful for distributed tracing, allowing trace
// context to be propagated across service boundaries through message queues. When using
// with the tracer package, you can extract trace headers and include them in the message:
//
//	traceHeaders := tracerClient.GetCarrier(ctx)
//	err := kafkaClient.Publish(ctx, "key", message, traceHeaders)
//
// Returns an error if publishing fails or if the context is canceled.
//
// Example:
//
//	ctx := context.Background()
//	message := []byte("Hello, Kafka!")
//
//	// Basic publishing without headers
//	err := kafkaClient.Publish(ctx, "my-key", message, nil)
//	if err != nil {
//	    log.Printf("Failed to publish message: %v", err)
//	}
//
// Example with distributed tracing:
//
//	// Create a span for the publish operation
//	ctx, span := tracer.StartSpan(ctx, "publish-message")
//	defer span.End()
//
//	// Extract trace context to include in the message headers
//	traceHeaders := tracerClient.GetCarrier(ctx)
//
//	// Publish the message with trace headers
//	err := kafkaClient.Publish(ctx, "my-key", message, traceHeaders)
//	if err != nil {
//	    span.RecordError(err)
//	    log.Printf("Failed to publish message: %v", err)
//	}
func (k *KafkaClient) Publish(ctx context.Context, key string, data interface{}, headers ...map[string]interface{}) error {
	start := time.Now()
	var publishErr error
	var msgSize int64

	defer func() {
		// Observe the operation after it completes
		k.observeOperation("produce", k.cfg.Topic, "", time.Since(start), publishErr, msgSize)
	}()

	select {
	case <-ctx.Done():
		publishErr = ctx.Err()
		return publishErr
	default:
		k.mu.RLock()
		writer := k.writer
		k.mu.RUnlock()

		if writer == nil {
			publishErr = ErrWriterNotInitialized
			return publishErr
		}

		// Serialize the data
		var msgBytes []byte
		var err error

		// If data is already []byte, use it directly
		if bytes, ok := data.([]byte); ok {
			msgBytes = bytes
		} else if k.serializer != nil {
			// Use injected serializer
			msgBytes, err = k.serializer.Serialize(data)
			if err != nil {
				publishErr = fmt.Errorf("failed to serialize message: %w", err)
				return publishErr
			}
		} else {
			// No serializer available and data is not []byte
			publishErr = fmt.Errorf("cannot publish non-[]byte data without a serializer, got type %T", data)
			return publishErr
		}

		// Track message size
		msgSize = int64(len(msgBytes))

		// Build Kafka message
		kafkaMsg := kafka.Message{
			Key:   []byte(key),
			Value: msgBytes,
		}

		// Add headers if provided
		if len(headers) > 0 {
			header := headers[0]
			kafkaHeaders := make([]kafka.Header, 0, len(header))
			for k, v := range header {
				// Convert value to string
				var valStr string
				switch val := v.(type) {
				case string:
					valStr = val
				case []byte:
					valStr = string(val)
				default:
					valStr = fmt.Sprintf("%v", val)
				}
				kafkaHeaders = append(kafkaHeaders, kafka.Header{
					Key:   k,
					Value: []byte(valStr),
				})
			}
			kafkaMsg.Headers = kafkaHeaders
		}

		err = writer.WriteMessages(ctx, kafkaMsg)
		if err != nil {
			publishErr = err
			return publishErr
		}

		return nil
	}
}

// CommitMsg commits the message, informing Kafka that the message
// has been successfully processed.
//
// Returns an error if the commit fails.
func (cm *ConsumerMessage) CommitMsg() error {
	return cm.reader.CommitMessages(context.Background(), cm.message)
}

// Body returns the message payload as a byte slice.
func (cm *ConsumerMessage) Body() []byte {
	return cm.message.Value
}

// BodyAs deserializes the message body into the target structure.
// It uses the configured Deserializer, or falls back to JSONDeserializer if none is configured.
//
// This is a convenience method that makes consuming messages more intuitive:
//
//	msgChan := consumer.Consume(ctx, wg)
//	for msg := range msgChan {
//	    var event UserEvent
//	    if err := msg.BodyAs(&event); err != nil {
//	        log.Printf("Failed to deserialize: %v", err)
//	        continue
//	    }
//	    fmt.Printf("Event: %s, UserID: %d\n", event.Event, event.UserID)
//	    msg.CommitMsg()
//	}
func (cm *ConsumerMessage) BodyAs(target interface{}) error {
	deserializer := cm.deserializer

	// If no deserializer configured, use JSONDeserializer as fallback
	if deserializer == nil {
		deserializer = &JSONDeserializer{}
	}

	return deserializer.Deserialize(cm.message.Value, target)
}

// Key returns the message key as a string.
func (cm *ConsumerMessage) Key() string {
	return string(cm.message.Key)
}

// Header returns the headers associated with the message.
func (cm *ConsumerMessage) Header() map[string]interface{} {
	headers := make(map[string]interface{})
	for _, h := range cm.message.Headers {
		headers[h.Key] = string(h.Value)
	}
	return headers
}

// Partition returns the partition this message came from.
func (cm *ConsumerMessage) Partition() int {
	return cm.message.Partition
}

// Offset returns the offset of this message.
func (cm *ConsumerMessage) Offset() int64 {
	return cm.message.Offset
}

// Deserialize is a helper method to deserialize a message body.
// It automatically uses the injected Deserializer or falls back to JSONDeserializer.
//
// Parameters:
//   - msg: The message to deserialize
//   - target: Pointer to the target structure to deserialize into
//
// Returns an error if deserialization fails.
//
// Example with FX-injected deserializer:
//
//	// In your FX app:
//	fx.Provide(func() kafka.Deserializer {
//	    return &kafka.JSONDeserializer{}
//	})
//
//	// Usage:
//	msgChan := kafkaClient.Consume(ctx, wg)
//	for msg := range msgChan {
//	    var event UserEvent
//	    if err := kafkaClient.Deserialize(msg, &event); err != nil {
//	        log.Printf("Failed to deserialize: %v", err)
//	        continue
//	    }
//	    fmt.Printf("Event: %s, UserID: %d\n", event.Event, event.UserID)
//	    msg.CommitMsg()
//	}
func (k *KafkaClient) Deserialize(msg Message, target interface{}) error {
	k.mu.RLock()
	deserializer := k.deserializer
	k.mu.RUnlock()

	// If no deserializer injected, try JSONDeserializer as fallback
	if deserializer == nil {
		deserializer = &JSONDeserializer{}
	}

	return deserializer.Deserialize(msg.Body(), target)
}
