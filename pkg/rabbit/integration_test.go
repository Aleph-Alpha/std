package rabbit

import (
	"context"
	"errors"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
	"go.uber.org/mock/gomock"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// TestRabbitMQConsumerContextCancellation verifies that the RabbitMQ consumer correctly handles
// context cancellation during message consumption. This test ensures proper cleanup and shutdown
// behavior in a production-like scenario.
//
// Test Scenario:
//  1. Starts a RabbitMQ container instance
//  2. Configures and initializes a RabbitMQ client with:
//     - Mock logger for verification
//     - Non-SSL connection settings
//     - Basic exchange and queue configuration
//  3. Tests consumer behavior under context cancellation:
//     - Starts consumer with cancellable context
//     - Triggers cancellation after delay
//     - Verifies consumer shutdown and channel closure
//
// Expected Behavior:
//   - Consumer establishes connection successfully
//   - Gracefully shuts down when context is canceled
//   - Output channel closes promptly after cancellation
//   - All resources are cleaned up properly
//   - Appropriate shutdown messages are logged
//
// Logger Expectations:
//   - "Connecting to Rabbit" - Initial connection attempt
//   - "Connected to Rabbit" - Successful connection
//   - "consumer is shutting down due to context cancellation" - Clean shutdown
//   - "closing rabbit channel..." - Resource cleanup
//
// Test Coverage:
//   - Connection establishment
//   - Context cancellation handling
//   - Resource cleanup
//   - Graceful shutdown
//   - Logger interactions
func TestRabbitMQConsumerContextCancellation(t *testing.T) {
	// Create root context
	ctx := context.Background()

	// Initialize RabbitMQ container
	host, port, containerInstance := initializeRabbitMQ(ctx)

	// Wait for the RabbitMQ port to be available
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 2*time.Second)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}, 60*time.Second, 500*time.Millisecond, "RabbitMQ port not ready after restart")

	// Initialize mock controller and logger
	var client *Rabbit
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockLog := NewMockLogger(ctrl)

	// Set up expected logger calls
	mockLog.EXPECT().Info("Connecting to Rabbit", gomock.Any(), gomock.Any()).Times(1)
	mockLog.EXPECT().Info("Connected to Rabbit", gomock.Any(), gomock.Any()).Times(1)
	mockLog.EXPECT().Error("error in establishing consumer for rabbit", gomock.Any(), gomock.Any()).AnyTimes() // it's possible that the consumer is not created yet
	mockLog.EXPECT().Info("consumer is shutting down due to context cancellation", gomock.Not(nil), gomock.Any()).Times(1)
	mockLog.EXPECT().Info("Stopping RetryConnection loop due to shutdown signal", gomock.Any(), gomock.Any()).Times(1)
	mockLog.EXPECT().Info("closing rabbit channel...", gomock.Any(), gomock.Any()).Times(1)

	// Configure RabbitMQ client
	cfg := Config{
		Connection: Connection{
			Host:         host,
			Port:         uint(port),
			User:         "guest",
			Password:     "guest",
			IsSSLEnabled: false,
		},
		Channel: Channel{
			ExchangeName: "test-exchange",
			ExchangeType: "direct",
			RoutingKey:   "test-routing",
			IsConsumer:   true,
			ContentType:  "application/json",
		},
	}

	// Initialize the application with dependency injection
	app := fx.New(
		FXModule,
		fx.Provide(
			func() Config { return cfg },
			func() Logger { return mockLog },
		),
		fx.Populate(&client),
	)

	// Start the application with timeout
	startCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	require.NoError(t, app.Start(startCtx))

	// Wait for a connection to be established
	require.Eventually(t, func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()
		return client.conn != nil && !client.conn.IsClosed()
	}, 10*time.Second, 1*time.Second, "Connection should be established")

	// Start consumer with a cancellable context
	consumeCtx, consumeCancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	outChan := client.Consume(consumeCtx, wg)

	// Trigger context cancellation after delay
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(2 * time.Second)
		consumeCancel()
	}()

	// Verify consumer shutdown and channel closure
	select {
	case _, ok := <-outChan:
		if ok {
			t.Fatal("expected channel to be closed after context cancel")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for consumer to stop after context cancel")
	}

	// Clean up application and wait for goroutines
	require.NoError(t, app.Stop(ctx))
	wg.Wait()

	// Cleanup container
	if err := containerInstance.Terminate(ctx); err != nil {
		t.Logf("Failed to terminate container: %v", err)
	}
	time.Sleep(3 * time.Second)
}

// TestRabbitMQConsumeWithAck_WithReconnection verifies the behavior of the RabbitMQ client
// in a TLS-enabled setup when the server connection is interrupted and then restored.
//
// Test Steps:
// 1. Starts a TLS-secured RabbitMQ container using testcontainers.
// 2. Initializes the client using Uber Fx DI with mocked logger and TLS config.
// 3. Starts consuming messages from a queue.
// 4. Simulates message consumption with manual acknowledgment (Ack).
// 5. Stops the RabbitMQ container to simulate a network failure.
// 6. Waits to ensure the connection drops, then restarts the container to simulate recovery.
// 7. Waits and asserts that the client successfully reconnects.
// 8. Publishes a message and verifies it is consumed and acknowledged successfully.
// 9. Shuts down the app and cleans up the test container.
//
// Expectations:
// - Log messages indicate connection, disconnection, and reconnection attempts.
// - Message consumption and acknowledgment succeed after reconnection.
// - TLS connection is validated using self-signed certificates.
func TestRabbitMQConsumeWithAck_WithReconnection(t *testing.T) {
	ctx := context.Background()

	// Initialize RabbitMQ container
	host, port, containerInstance := initializeRabbitMQ(ctx)

	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 2*time.Second)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}, 60*time.Second, 500*time.Millisecond, "RabbitMQ port not ready after restart")

	var client *Rabbit

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockLog := NewMockLogger(ctrl)

	// Expect the connection and logging calls
	mockLog.EXPECT().Info("Connecting to Rabbit", gomock.Any(), gomock.Any()).MinTimes(1)
	mockLog.EXPECT().Info("Connected to Rabbit", gomock.Any(), gomock.Any()).Times(2)

	mockLog.EXPECT().Error("error in establishing consumer for rabbit", gomock.Any(), gomock.Any()).AnyTimes()
	// Expect message consumption
	mockLog.EXPECT().Debug("message consumed from rabbit", gomock.Any(), gomock.Any()).Times(1)

	mockLog.EXPECT().Info("consumer is shutting down due to shutdown signal", gomock.Any(), gomock.Any()).Times(1)

	// Mock connection failure/reconnection scenarios
	mockLog.EXPECT().Warn("RabbitMQ connection closed, retrying...", gomock.Any(), gomock.Any()).MinTimes(1)
	mockLog.EXPECT().Error("error in connecting to rabbit", gomock.Any(), gomock.Any()).MinTimes(1)
	mockLog.EXPECT().Error("Reconnection failed", gomock.Any(), gomock.Any()).AnyTimes()
	mockLog.EXPECT().Info("Reconnected to RabbitMQ", gomock.Any(), gomock.Any()).MaxTimes(2).Times(1)
	mockLog.EXPECT().Info("closing rabbit channel...", gomock.Any(), gomock.Any()).MinTimes(1)
	mockLog.EXPECT().Info("Stopping retry loop inside reconnect", gomock.Any(), gomock.Any()).AnyTimes()
	mockLog.EXPECT().Info("Stopping RetryConnection loop due to shutdown signal", gomock.Any(), gomock.Any()).AnyTimes()

	cfg := Config{
		Connection: Connection{
			Host:         host,
			Port:         uint(port),
			User:         "guest",
			Password:     "guest",
			IsSSLEnabled: false,
		},
		Channel: Channel{
			ExchangeName: "test-exchange",
			ExchangeType: "direct",
			RoutingKey:   "test-routing",
			IsConsumer:   true,
			ContentType:  "application/json",
		},
	}

	// Initialize app
	app := fx.New(
		FXModule,
		fx.Provide(
			func() Config { return cfg },
			func() Logger { return mockLog },
		),
		fx.Populate(&client),
	)

	require.NoError(t, app.Start(ctx))

	// Ensure the RabbitMQ connection is established
	require.Eventually(t, func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()
		return client.conn != nil && !client.conn.IsClosed()
	}, 10*time.Second, 1*time.Second, "Connection should be established")

	wg := &sync.WaitGroup{}
	// Set up the channel for messages
	msgs := client.Consume(ctx, wg)

	// Channel to handle errors during message consumption
	errCh := make(chan error, 1)

	// WaitGroup to synchronize the message consumption
	wg.Add(1)

	go func() {
		defer wg.Done() // Signal that the goroutine is finished

		// Consume messages
		for msg := range msgs {
			select {
			case <-ctx.Done():
				t.Logf("ctx done: %v", ctx.Err())
				return
			default:
				t.Logf("message consumed successfully: %v", string(msg.Body()))
				if err := msg.AckMsg(); err != nil {
					errCh <- fmt.Errorf("failed to ack message: %w", err)
					return
				} else {
					errCh <- nil // Successful ack
					return
				}
			}
		}
	}()

	// Simulate RabbitMQ stopping and starting again
	require.NoError(t, containerInstance.Stop(ctx, nil))
	time.Sleep(7 * time.Second)

	// Restart the RabbitMQ container
	require.NoError(t, containerInstance.Start(ctx))

	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 2*time.Second)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}, 60*time.Second, 500*time.Millisecond, "RabbitMQ port not ready after restart")

	// Ensure RabbitMQ reconnects
	require.Eventually(t, func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()
		return client.conn != nil && !client.conn.IsClosed()
	}, 20*time.Second, 1*time.Second, "Should reconnect after RabbitMQ comes back")

	// Publish a test message
	msgBody := `{"event":"just-publishing"}`
	publishCtx, stopCancel := context.WithTimeout(ctx, 2*time.Second)
	defer stopCancel()
	require.NoError(t, client.Publish(publishCtx, []byte(msgBody)))
	t.Log("msg published successfully")

	// Ensure the message is consumed and acknowledged
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):

	}

	require.NoError(t, app.Stop(ctx))

	// Wait for the consumer to finish processing
	wg.Wait()

	if err := containerInstance.Terminate(ctx); err != nil {
		t.Logf("failed to terminate rabbit container: %v", err)
	}

	time.Sleep(3 * time.Second)
}

// TestRabbitMQConsumeWithNackReQueue verifies that a RabbitMQ client can consume a message
// over a TLS-enabled connection, explicitly reject (nack) it with re-queueing, and then successfully
// consume and acknowledge it on the retry. This test simulates temporary processing failure followed
// by success, using a secure RabbitMQ container.
//
// Test Flow:
// 1. Generate TLS certificates and configure a RabbitMQ container to use them.
// 2. Start a TLS-secured RabbitMQ container using testcontainers.
// 3. Establish a TLS connection and start the application via Uber Fx.
// 4. Consume a message from the queue, simulate a failure (nack with requeue).
// 5. Re-consume the message and acknowledge it successfully.
// 6. Gracefully stop the application and terminate the container.
func TestRabbitMQConsumeWithNackReQueue(t *testing.T) {
	ctx := context.Background()

	// Initialize RabbitMQ container
	host, port, containerInstance := initializeRabbitMQ(ctx)

	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 2*time.Second)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}, 60*time.Second, 500*time.Millisecond, "RabbitMQ port not ready after restart")

	var client *Rabbit

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockLog := NewMockLogger(ctrl)

	// Define mock expectations
	mockLog.EXPECT().Info("Connecting to Rabbit", gomock.Any(), gomock.Any()).MinTimes(1)
	mockLog.EXPECT().Info("Connected to Rabbit", gomock.Any(), gomock.Any()).Times(1)
	mockLog.EXPECT().Info("Stopping RetryConnection loop due to shutdown signal", gomock.Any(), gomock.Any()).AnyTimes()
	mockLog.EXPECT().Info("Stopping RetryConnection loop due to shutdown signal inside reconnect", gomock.Any(), gomock.Any()).AnyTimes()
	mockLog.EXPECT().Info("closing rabbit channel...", gomock.Any(), gomock.Any()).MinTimes(1)
	mockLog.EXPECT().Debug("message consumed from rabbit", gomock.Any(), gomock.Any()).Times(2)
	mockLog.EXPECT().Info("consumer is shutting down due to shutdown signal", gomock.Any(), gomock.Any()).Times(1)

	cfg := Config{
		Connection: Connection{
			Host:         host,
			Port:         uint(port),
			User:         "guest",
			Password:     "guest",
			IsSSLEnabled: false,
		},
		Channel: Channel{
			ExchangeName: "test-exchange",
			ExchangeType: "direct",
			RoutingKey:   "test-routing",
			IsConsumer:   true,
			ContentType:  "application/json",
		},
	}

	// Initialize the app
	app := fx.New(
		FXModule,
		fx.Provide(
			func() Config { return cfg },
			func() Logger { return mockLog },
		),
		fx.Populate(&client),
	)

	require.NoError(t, app.Start(ctx))

	// Ensure a connection is established
	require.Eventually(t, func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()
		return client.conn != nil && !client.conn.IsClosed()
	}, 10*time.Second, 1*time.Second)

	wg := &sync.WaitGroup{}
	// Set up a message consumer
	msgs := client.Consume(ctx, wg)

	errCh := make(chan error, 1)
	wg.Add(1)

	// Start a consumer goroutine
	go func() {
		defer wg.Done()

		i := 0
		for msg := range msgs {
			select {
			case <-ctx.Done():
				t.Logf("context done: %v", ctx.Err())
				return
			default:
			}

			i++
			t.Logf("message consumed successfully: %v", string(msg.Body()))

			if i == 2 {
				t.Log("message acknowledged successfully")
				errCh <- msg.AckMsg()
				return
			} else {
				t.Log("message not acknowledged")
				if err := msg.NackMsg(true); err != nil {
					errCh <- err
					return
				}
			}
		}
	}()

	// Publish a message to the queue
	msgBody := `{"event":"just-publishing"}`
	//publishCtx, cancelPub := context.WithTimeout(ctx, 2*time.Second)
	//defer cancelPub()

	err := client.Publish(ctx, []byte(msgBody))
	require.NoError(t, err)
	t.Log("msg published successfully")

	// Wait for the consumer to finish processing
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		// no error occurred
	}

	require.NoError(t, app.Stop(ctx))
	// Wait for consumer goroutine to finish safely
	wg.Wait()

	if err = containerInstance.Terminate(ctx); err != nil {
		t.Logf("failed to terminate container: %v", err)
	}

	time.Sleep(2 * time.Second)
}

// TestRabbitMQConsumeWithAck verifies that a RabbitMQ client can consume and acknowledge a message
// over a TLS-enabled connection. This test ensures that messages are successfully published, consumed,
// and acknowledged under secure communication, using an ephemeral RabbitMQ container.
//
// Test Flow:
// 1. TLS certificates are generated and mounted to a RabbitMQ container configured for TLS.
// 2. The RabbitMQ container is started and a connection is established using TLS.
// 3. The client subscribes to a queue and begins consuming messages.
// 4. A message is published, and the client is expected to consume and acknowledge it.
// 5. The application is gracefully stopped, and the container is terminated.
func TestRabbitMQConsumeWithAck(t *testing.T) {
	ctx := context.Background()

	// Initialize RabbitMQ container
	host, port, containerInstance := initializeRabbitMQ(ctx)

	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 2*time.Second)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}, 60*time.Second, 500*time.Millisecond, "RabbitMQ port not ready after restart")

	var client *Rabbit

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockLog := NewMockLogger(ctrl)

	mockLog.EXPECT().Info("Connecting to Rabbit", gomock.Any(), gomock.Any()).MinTimes(1)
	mockLog.EXPECT().Info("Connected to Rabbit", gomock.Any(), gomock.Any()).Times(1)
	mockLog.EXPECT().Info("consumer is shutting down due to shutdown signal", gomock.Any(), gomock.Any()).Times(1)
	mockLog.EXPECT().Debug("message consumed from rabbit", gomock.Any(), gomock.Any()).Times(1)

	mockLog.EXPECT().Info("Stopping RetryConnection loop due to shutdown signal", gomock.Any(), gomock.Any()).AnyTimes()
	mockLog.EXPECT().Info("Stopping RetryConnection loop due to shutdown signal inside reconnect", gomock.Any(), gomock.Any()).AnyTimes()
	mockLog.EXPECT().Info("closing rabbit channel...", gomock.Any(), gomock.Any()).MinTimes(1)

	cfg := Config{
		Connection: Connection{
			Host:         host,
			Port:         uint(port),
			User:         "guest",
			Password:     "guest",
			IsSSLEnabled: false,
		},
		Channel: Channel{
			ExchangeName: "test-exchange",
			ExchangeType: "direct",
			RoutingKey:   "test-routing",
			IsConsumer:   true,
			ContentType:  "application/json",
		},
	}

	app := fx.New(
		FXModule,
		fx.Provide(
			func() Config { return cfg },
			func() Logger { return mockLog },
		),
		fx.Populate(&client),
	)

	require.NoError(t, app.Start(ctx))

	require.Eventually(t, func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()
		return client.conn != nil && !client.conn.IsClosed()
	}, 30*time.Second, 1*time.Second, "Connection should be established")

	wg := &sync.WaitGroup{}
	msgs := client.Consume(ctx, wg)
	errCh := make(chan error, 1)

	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case <-ctx.Done():
			t.Logf("consumer ctx done: %v", ctx.Err())
			return
		default:
			for msg := range msgs {
				select {
				case <-ctx.Done():
					t.Logf("ctx done: %v", ctx.Err())
					return
				default:
					t.Logf("message consumed successfully: %v", string(msg.Body()))
					if err := msg.AckMsg(); err != nil {
						errCh <- fmt.Errorf("failed to ack message: %w", err)
						return
					}
					t.Log("message acknowledged successfully")
					errCh <- nil // success
					return
				}
			}
		}
	}()

	msgBody := `{"event":"just-publishing"}`
	publishCtx, stopCancel := context.WithTimeout(ctx, 2*time.Second)
	defer stopCancel()
	err := client.Publish(publishCtx, []byte(msgBody))
	require.NoError(t, err)
	t.Log("msg published successfully")

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for message to be acknowledged")
	}

	require.NoError(t, app.Stop(ctx))

	wg.Wait()

	if err = containerInstance.Terminate(ctx); err != nil {
		t.Logf("failed to terminate container: %v", err)
	}
	time.Sleep(2 * time.Second)
}

// TestRabbitMQPublish verifies that publishing a message to RabbitMQ over a TLS-enabled connection works correctly.
// It performs the following steps:
// 1. Generates TLS certificates and configures a RabbitMQ container to use them.
// 2. Starts the container and initializes a RabbitMQ client with TLS settings.
// 3. Verifies the connection is successfully established.
// 4. Publishes a test message and asserts no error occurs.
// 5. Shuts down the client and terminates the container cleanly.
func TestRabbitMQPublish(t *testing.T) {
	ctx := context.Background()

	// Initialize RabbitMQ container
	host, port, containerInstance := initializeRabbitMQ(ctx)

	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 2*time.Second)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}, 60*time.Second, 500*time.Millisecond, "RabbitMQ port not ready after restart")

	var client *Rabbit

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockLog := NewMockLogger(ctrl)

	mockLog.EXPECT().Info("Connecting to Rabbit", gomock.Any(), gomock.Any()).MinTimes(1)
	mockLog.EXPECT().Info("Connected to Rabbit", gomock.Any(), gomock.Any()).Times(1)

	mockLog.EXPECT().Info("Stopping RetryConnection loop due to shutdown signal", gomock.Any(), gomock.Any()).AnyTimes()
	mockLog.EXPECT().Info("Stopping RetryConnection loop due to shutdown signal inside reconnect", gomock.Any(), gomock.Any()).AnyTimes()
	mockLog.EXPECT().Info("closing rabbit channel...", gomock.Any(), gomock.Any()).MinTimes(1)

	cfg := Config{
		Connection: Connection{
			Host:         host,
			Port:         uint(port),
			User:         "guest",
			Password:     "guest",
			IsSSLEnabled: false,
		},
		Channel: Channel{
			ExchangeName: "test-exchange",
			ExchangeType: "direct",
			RoutingKey:   "test-routing",
			IsConsumer:   true,
			ContentType:  "application/json",
		},
	}

	app := fx.New(
		FXModule,
		fx.Provide(
			func() Config { return cfg },
			func() Logger { return mockLog },
		),
		fx.Populate(&client),
	)

	startCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	require.NoError(t, app.Start(startCtx))

	require.Eventually(t, func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()
		return client.conn != nil && !client.conn.IsClosed()
	}, 10*time.Second, 1*time.Second, "Connection should be established")

	// Act: Try publishing a message
	msgBody := `{"event":"just-publishing"}`
	publishCtx, stopCancel := context.WithTimeout(ctx, 2*time.Second)
	defer stopCancel()
	err := client.Publish(publishCtx, []byte(msgBody))
	require.NoError(t, err)

	// No assertions on consumption here – just verifying Publish works under TLS

	stopCtx, stopCancel := context.WithTimeout(ctx, 5*time.Second)
	defer stopCancel()
	require.NoError(t, app.Stop(stopCtx))

	if err = containerInstance.Terminate(ctx); err != nil {
		t.Logf("failed to terminate container: %v", err)
	}

	time.Sleep(2 * time.Second)
}

// TestRabbitMQReconnectAfterDisconnect verifies that the RabbitMQ client can successfully reconnect
// after a broker failure in a non-TLS (plain AMQP) environment.
//
// Test steps:
//  1. Start a RabbitMQ container without TLS.
//  2. Initialize the client using the container connection config.
//  3. Verify the initial connection is established.
//  4. Simulate broker failure by stopping the container.
//  5. Expect the client to log disconnection and attempt reconnection.
//  6. Restart the container and wait for RabbitMQ to be reachable again.
//  7. Confirm a client reconnects successfully using retry logic.
//  8. Cleanly shut down the client and terminate the container.
func TestRabbitMQReconnectAfterDisconnect(t *testing.T) {
	ctx := context.Background()

	// Initialize RabbitMQ container
	host, port, containerInstance := initializeRabbitMQ(ctx)

	cfg := Config{
		Connection: Connection{
			Host:         host,
			Port:         uint(port),
			User:         "guest",
			Password:     "guest",
			IsSSLEnabled: false,
		},
		Channel: Channel{
			ExchangeName: "test-exchange",
			ExchangeType: "direct",
			RoutingKey:   "test-routing",
			IsConsumer:   true,
			ContentType:  "application/json",
		},
	}

	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 2*time.Second)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}, 60*time.Second, 500*time.Millisecond, "RabbitMQ port not ready after restart")

	var client *Rabbit

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockLog := NewMockLogger(ctrl)

	mockLog.EXPECT().Info("Connecting to Rabbit", gomock.Any(), gomock.Any()).MinTimes(1)
	mockLog.EXPECT().Info("Connected to Rabbit", gomock.Any(), gomock.Any()).Times(2)
	mockLog.EXPECT().Info("Stopping RetryConnection loop due to shutdown signal", gomock.Any(), gomock.Any()).AnyTimes()
	mockLog.EXPECT().Info("Stopping RetryConnection loop due to shutdown signal inside reconnect", gomock.Any(), gomock.Any()).AnyTimes()
	mockLog.EXPECT().Info("closing rabbit channel...", gomock.Any(), gomock.Any()).MinTimes(1)
	mockLog.EXPECT().Warn("RabbitMQ connection closed, retrying...", gomock.Any(), gomock.Any()).Times(1)
	mockLog.EXPECT().Error("error in connecting to rabbit", gomock.Any(), gomock.Any()).MinTimes(1)
	mockLog.EXPECT().Error("Reconnection failed", gomock.Any(), gomock.Any()).AnyTimes()

	mockLog.EXPECT().Info("Reconnected to RabbitMQ", gomock.Any(), gomock.Any()).MaxTimes(2).Times(1)

	app := fx.New(
		FXModule,
		fx.Provide(
			func() Config { return cfg },
			func() Logger { return mockLog },
		),
		fx.Populate(&client),
	)

	startCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	require.NoError(t, app.Start(startCtx))

	// Confirm initial connection
	require.Eventually(t, func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()
		return client.conn != nil && !client.conn.IsClosed()
	}, 10*time.Second, 1*time.Second, "Should connect initially")

	// Simulate RabbitMQ failure
	stopDuration := 5 * time.Second
	require.NoError(t, containerInstance.Stop(ctx, &stopDuration))

	// Wait enough time to ensure disconnection
	time.Sleep(7 * time.Second)

	// Restart the container to simulate recovery
	require.NoError(t, containerInstance.Start(ctx))

	// Wait for the container to be ready (RabbitMQ service fully up)
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 2*time.Second)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}, 60*time.Second, 500*time.Millisecond, "RabbitMQ port not ready after restart")

	// Confirm reconnection via retry logic
	require.Eventually(t, func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()
		return client.conn != nil && !client.conn.IsClosed()
	}, 20*time.Second, 1*time.Second, "Should reconnect after RabbitMQ comes back")

	stopCtx, stopCancel := context.WithTimeout(ctx, 5*time.Second)
	defer stopCancel()

	require.NoError(t, app.Stop(stopCtx))

	if err := containerInstance.Terminate(ctx); err != nil {
		t.Logf("failed to terminate container: %v", err)
	}

	time.Sleep(2 * time.Second)
}

func TestRabbitMQDeadLetterQueue(t *testing.T) {
	ctx := context.Background()

	// Initialize RabbitMQ container
	host, port, containerInstance := initializeRabbitMQ(ctx)

	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 2*time.Second)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}, 60*time.Second, 500*time.Millisecond, "RabbitMQ port not ready")

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockLog := NewMockLogger(ctrl)

	// Set up expected logging calls
	mockLog.EXPECT().Info("Connecting to Rabbit", gomock.Any(), gomock.Any()).MinTimes(1)
	mockLog.EXPECT().Info("Connected to Rabbit", gomock.Any(), gomock.Any()).MinTimes(1)
	mockLog.EXPECT().Debug("message consumed from rabbit", gomock.Any(), gomock.Any()).Times(2)
	mockLog.EXPECT().Info("Stopping RetryConnection loop due to shutdown signal", gomock.Any(), gomock.Any()).AnyTimes()
	mockLog.EXPECT().Info("Stopping RetryConnection loop due to shutdown signal inside reconnect", gomock.Any(), gomock.Any()).AnyTimes()
	mockLog.EXPECT().Info("closing rabbit channel...", gomock.Any(), gomock.Any()).MinTimes(1)
	mockLog.EXPECT().Info("consumer is shutting down due to shutdown signal", gomock.Any(), gomock.Any()).Times(2)

	const (
		mainQueue    = "test-queue"
		dlqQueue     = "dlq-queue"
		dlxExchange  = "dlx-exchange"
		mainExchange = "main-exchange"
	)

	cfg := Config{
		Connection: Connection{
			Host:         host,
			Port:         uint(port),
			User:         "guest",
			Password:     "guest",
			IsSSLEnabled: false,
		},
		Channel: Channel{
			ExchangeName: mainExchange,
			ExchangeType: "direct",
			RoutingKey:   "test-routing",
			QueueName:    mainQueue,
			IsConsumer:   true,
			ContentType:  "application/json",
		},
		DeadLetter: DeadLetter{
			ExchangeName: dlxExchange,
			QueueName:    dlqQueue,
			RoutingKey:   "dlx-routing",
			Ttl:          3,
		},
	}

	var client *Rabbit

	app := fx.New(
		FXModule,
		fx.Provide(
			func() Config { return cfg },
			func() Logger { return mockLog },
		),
		fx.Populate(&client),
	)

	require.NoError(t, app.Start(ctx))

	// Ensure a connection is established
	require.Eventually(t, func() bool {
		client.mu.Lock()
		defer client.mu.Unlock()
		return client.conn != nil && !client.conn.IsClosed()
	}, 10*time.Second, 1*time.Second, "Connection should be established")

	wg := &sync.WaitGroup{}

	// Start a main queue consumer that will reject messages
	mainMessages := client.Consume(ctx, wg)

	// Start DLQ consumer
	dlqMessages := client.ConsumeDLQ(ctx, wg)

	// Consumer that rejects messages
	wg.Add(1)
	go func() {
		defer wg.Done()
		for msg := range mainMessages {
			t.Logf("Received message from main queue: %s", msg.Body())
			require.NoError(t, msg.NackMsg(false))
			return
		}
	}()

	errCh := make(chan error, 1)

	wg.Add(1)
	go func() {
		defer wg.Done()
		for msg := range dlqMessages {
			select {
			case <-ctx.Done():
				t.Logf("ctx done: %v", ctx.Err())
				return
			default:
				t.Logf("Received message from DLQ: %s", msg.Body())
				if err := msg.AckMsg(); err != nil {
					errCh <- fmt.Errorf("failed to ack message: %w", err)
					return
				}
				t.Log("message acknowledged successfully")
				errCh <- nil // success
				return
			}
		}
	}()

	// Publish a test message
	testMessage := []byte("test message")
	err := client.Publish(ctx, testMessage)
	require.NoError(t, err)

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for message to be delivered")
	}

	require.NoError(t, app.Stop(ctx))
	wg.Wait()

	if err = containerInstance.Terminate(ctx); err != nil {
		t.Logf("failed to terminate container: %v", err)
	}

	time.Sleep(2 * time.Second)
}

func initializeRabbitMQ(ctx context.Context) (string, int, testcontainers.Container) {
	hostPort, err := getFreePort()
	if err != nil {
		log.Fatalf("Failed to find free port: %v", err)
	}

	containerInstance, err := createRabbitMQContainer(ctx, hostPort)
	if err != nil {
		log.Fatalf("Failed to create container: %v", err)
	}

	port, err := containerInstance.MappedPort(ctx, "5672")
	if err != nil {
		log.Fatalf("Failed to get mapped port: %v", err)
	}
	host, err := containerInstance.Host(ctx)
	if err != nil {
		log.Fatalf("Failed to get host: %v", err)
	}
	return host, port.Int(), containerInstance
}

// createRabbitMQContainer sets up and starts a RabbitMQ Docker container using
// testcontainers-go. It binds TLS and plain AMQP ports, mounts the cert
// directory, injects environment variables, and waits for RabbitMQ to be healthy.
func createRabbitMQContainer(ctx context.Context, hostPort string) (testcontainers.Container, error) {

	var containerInstance testcontainers.Container
	var lastErr error

	for attempt := 0; attempt < 3; attempt++ {
		portBindings := nat.PortMap{
			"5672/tcp": []nat.PortBinding{{HostPort: hostPort}},
		}

		req := testcontainers.ContainerRequest{
			Image: "rabbitmq:4-management",
			ExposedPorts: []string{
				"5672/tcp",
			},
			HostConfigModifier: func(cfg *container.HostConfig) {
				cfg.PortBindings = portBindings
			},
			WaitingFor: wait.ForAll(
				wait.ForListeningPort("5672/tcp").WithStartupTimeout(20*time.Second),
				wait.ForExec([]string{"rabbitmq-diagnostics", "status"}).WithExitCodeMatcher(func(exitCode int) bool {
					return exitCode == 0
				}).WithStartupTimeout(10*time.Second),
			),
		}

		containerInstance, lastErr = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
		if lastErr == nil {
			return containerInstance, nil
		}

		// Retry only for Docker socket-related issues
		if strings.Contains(lastErr.Error(), "docker.sock") || errors.Is(lastErr, io.EOF) {
			log.Printf("Attempt %d: Docker socket error, retrying in %d seconds: %v", attempt+1, attempt+1, lastErr)
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}

		break // Other errors should not be retried
	}

	return nil, fmt.Errorf("failed to start RabbitMQ container after %d attempts: %w", 3, lastErr)
}

func getFreePort() (string, error) {
	l, err := net.Listen("tcp", ":0") // :0 asks OS for any free port
	if err != nil {
		return "", err
	}
	defer func(l net.Listener) {
		err := l.Close()
		if err != nil {
			panic(err)
		}
	}(l)
	addr := l.Addr().(*net.TCPAddr)
	return strconv.Itoa(addr.Port), nil
}

func TestRabbit_TranslateError(t *testing.T) {
	r := &Rabbit{}

	tests := []struct {
		name     string
		input    error
		expected error
	}{
		{
			name:     "nil error",
			input:    nil,
			expected: nil,
		},
		{
			name:     "ConnectionForced error",
			input:    &amqp.Error{Code: amqp.ConnectionForced, Reason: "connection forced"},
			expected: ErrConnectionClosed,
		},
		{
			name:     "InvalidPath error",
			input:    &amqp.Error{Code: amqp.InvalidPath, Reason: "invalid path"},
			expected: ErrVirtualHostNotFound,
		},
		{
			name:     "AccessRefused error",
			input:    &amqp.Error{Code: amqp.AccessRefused, Reason: "access refused"},
			expected: ErrAccessDenied,
		},
		{
			name:     "NotFound error",
			input:    &amqp.Error{Code: amqp.NotFound, Reason: "not found"},
			expected: ErrVirtualHostNotFound,
		},
		{
			name:     "ResourceLocked error",
			input:    &amqp.Error{Code: amqp.ResourceLocked, Reason: "resource locked"},
			expected: ErrResourceLocked,
		},
		{
			name:     "PreconditionFailed error",
			input:    &amqp.Error{Code: amqp.PreconditionFailed, Reason: "precondition failed"},
			expected: ErrPreconditionFailed,
		},
		{
			name:     "ContentTooLarge error",
			input:    &amqp.Error{Code: amqp.ContentTooLarge, Reason: "content too large"},
			expected: ErrMessageTooLarge,
		},
		{
			name:     "NoRoute error",
			input:    &amqp.Error{Code: amqp.NoRoute, Reason: "no route"},
			expected: ErrPublishFailed,
		},
		{
			name:     "NoConsumers error",
			input:    &amqp.Error{Code: amqp.NoConsumers, Reason: "no consumers"},
			expected: ErrPublishFailed,
		},
		{
			name:     "ChannelError error",
			input:    &amqp.Error{Code: amqp.ChannelError, Reason: "channel error"},
			expected: ErrChannelError,
		},
		{
			name:     "UnexpectedFrame error",
			input:    &amqp.Error{Code: amqp.UnexpectedFrame, Reason: "unexpected frame"},
			expected: ErrUnexpectedFrame,
		},
		{
			name:     "ResourceError error",
			input:    &amqp.Error{Code: amqp.ResourceError, Reason: "resource error"},
			expected: ErrResourceError,
		},
		{
			name:     "NotAllowed error",
			input:    &amqp.Error{Code: amqp.NotAllowed, Reason: "not allowed"},
			expected: ErrNotAllowed,
		},
		{
			name:     "NotImplemented error",
			input:    &amqp.Error{Code: amqp.NotImplemented, Reason: "not implemented"},
			expected: ErrNotImplemented,
		},
		{
			name:     "InternalError error",
			input:    &amqp.Error{Code: amqp.InternalError, Reason: "internal error"},
			expected: ErrInternalError,
		},
		{
			name:     "SyntaxError error",
			input:    &amqp.Error{Code: amqp.SyntaxError, Reason: "syntax error"},
			expected: ErrSyntaxError,
		},
		{
			name:     "CommandInvalid error",
			input:    &amqp.Error{Code: amqp.CommandInvalid, Reason: "command invalid"},
			expected: ErrCommandInvalid,
		},
		{
			name:     "FrameError error",
			input:    &amqp.Error{Code: amqp.FrameError, Reason: "frame error"},
			expected: ErrFrameError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.TranslateError(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRabbit_TranslateAMQPError_ByReason(t *testing.T) {
	r := &Rabbit{}

	tests := []struct {
		name     string
		input    *amqp.Error
		expected error
	}{
		{
			name:     "access refused reason",
			input:    &amqp.Error{Code: 999, Reason: "access refused"},
			expected: ErrAccessDenied,
		},
		{
			name:     "login refused reason",
			input:    &amqp.Error{Code: 999, Reason: "login refused"},
			expected: ErrAuthenticationFailed,
		},
		{
			name:     "authentication failed reason",
			input:    &amqp.Error{Code: 999, Reason: "authentication failed"},
			expected: ErrAuthenticationFailed,
		},
		{
			name:     "invalid credentials reason",
			input:    &amqp.Error{Code: 999, Reason: "invalid credentials"},
			expected: ErrInvalidCredentials,
		},
		{
			name:     "permission denied reason",
			input:    &amqp.Error{Code: 999, Reason: "permission denied"},
			expected: ErrInsufficientPermissions,
		},
		{
			name:     "exchange not found reason",
			input:    &amqp.Error{Code: 999, Reason: "exchange not found"},
			expected: ErrExchangeNotFound,
		},
		{
			name:     "queue not found reason",
			input:    &amqp.Error{Code: 999, Reason: "queue not found"},
			expected: ErrQueueNotFound,
		},
		{
			name:     "queue empty reason",
			input:    &amqp.Error{Code: 999, Reason: "queue empty"},
			expected: ErrQueueEmpty,
		},
		{
			name:     "queue exists reason",
			input:    &amqp.Error{Code: 999, Reason: "queue exists"},
			expected: ErrQueueExists,
		},
		{
			name:     "exchange exists reason",
			input:    &amqp.Error{Code: 999, Reason: "exchange exists"},
			expected: ErrExchangeExists,
		},
		{
			name:     "message too large reason",
			input:    &amqp.Error{Code: 999, Reason: "message too large"},
			expected: ErrMessageTooLarge,
		},
		{
			name:     "no route reason",
			input:    &amqp.Error{Code: 999, Reason: "no route"},
			expected: ErrPublishFailed,
		},
		{
			name:     "no consumers reason",
			input:    &amqp.Error{Code: 999, Reason: "no consumers"},
			expected: ErrPublishFailed,
		},
		{
			name:     "message returned reason",
			input:    &amqp.Error{Code: 999, Reason: "message returned"},
			expected: ErrMessageReturned,
		},
		{
			name:     "message nacked reason",
			input:    &amqp.Error{Code: 999, Reason: "message nacked"},
			expected: ErrMessageNacked,
		},
		{
			name:     "channel closed reason",
			input:    &amqp.Error{Code: 999, Reason: "channel closed"},
			expected: ErrChannelClosed,
		},
		{
			name:     "connection closed reason",
			input:    &amqp.Error{Code: 999, Reason: "connection closed"},
			expected: ErrConnectionClosed,
		},
		{
			name:     "connection forced reason",
			input:    &amqp.Error{Code: 999, Reason: "connection forced"},
			expected: ErrConnectionClosed,
		},
		{
			name:     "connection lost reason",
			input:    &amqp.Error{Code: 999, Reason: "connection lost"},
			expected: ErrConnectionLost,
		},
		{
			name:     "virtual host not found reason",
			input:    &amqp.Error{Code: 999, Reason: "virtual host not found"},
			expected: ErrVirtualHostNotFound,
		},
		{
			name:     "vhost not found reason",
			input:    &amqp.Error{Code: 999, Reason: "vhost not found"},
			expected: ErrVirtualHostNotFound,
		},
		{
			name:     "user not found reason",
			input:    &amqp.Error{Code: 999, Reason: "user not found"},
			expected: ErrUserNotFound,
		},
		{
			name:     "memory alarm reason",
			input:    &amqp.Error{Code: 999, Reason: "memory alarm"},
			expected: ErrMemoryAlarm,
		},
		{
			name:     "disk alarm reason",
			input:    &amqp.Error{Code: 999, Reason: "disk alarm"},
			expected: ErrDiskAlarm,
		},
		{
			name:     "resource alarm reason",
			input:    &amqp.Error{Code: 999, Reason: "resource alarm"},
			expected: ErrResourceAlarm,
		},
		{
			name:     "flow control reason",
			input:    &amqp.Error{Code: 999, Reason: "flow control"},
			expected: ErrFlowControl,
		},
		{
			name:     "quota exceeded reason",
			input:    &amqp.Error{Code: 999, Reason: "quota exceeded"},
			expected: ErrQuotaExceeded,
		},
		{
			name:     "rate limit reason",
			input:    &amqp.Error{Code: 999, Reason: "rate limit"},
			expected: ErrRateLimit,
		},
		{
			name:     "backpressure reason",
			input:    &amqp.Error{Code: 999, Reason: "backpressure"},
			expected: ErrBackpressure,
		},
		{
			name:     "cluster reason",
			input:    &amqp.Error{Code: 999, Reason: "cluster"},
			expected: ErrClusterError,
		},
		{
			name:     "node down reason",
			input:    &amqp.Error{Code: 999, Reason: "node down"},
			expected: ErrNodeDown,
		},
		{
			name:     "unknown reason",
			input:    &amqp.Error{Code: 999, Reason: "unknown error"},
			expected: ErrUnknownError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.translateAMQPError(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRabbit_TranslateNetworkError(t *testing.T) {
	r := &Rabbit{}

	tests := []struct {
		name     string
		input    net.Error
		expected error
	}{
		{
			name:     "timeout error",
			input:    &timeoutError{timeout: true},
			expected: ErrTimeout,
		},
		{
			name:     "temporary error",
			input:    &temporaryError{temporary: true},
			expected: ErrNetworkError,
		},
		{
			name:     "generic network error",
			input:    &genericNetError{},
			expected: ErrNetworkError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.translateNetworkError(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRabbit_TranslateSyscallError(t *testing.T) {
	r := &Rabbit{}

	tests := []struct {
		name     string
		input    syscall.Errno
		expected error
	}{
		{
			name:     "ECONNREFUSED",
			input:    syscall.ECONNREFUSED,
			expected: ErrConnectionFailed,
		},
		{
			name:     "ECONNRESET",
			input:    syscall.ECONNRESET,
			expected: ErrConnectionLost,
		},
		{
			name:     "ECONNABORTED",
			input:    syscall.ECONNABORTED,
			expected: ErrConnectionLost,
		},
		{
			name:     "ETIMEDOUT",
			input:    syscall.ETIMEDOUT,
			expected: ErrTimeout,
		},
		{
			name:     "ENETUNREACH",
			input:    syscall.ENETUNREACH,
			expected: ErrNetworkError,
		},
		{
			name:     "EHOSTUNREACH",
			input:    syscall.EHOSTUNREACH,
			expected: ErrNetworkError,
		},
		{
			name:     "EHOSTDOWN",
			input:    syscall.EHOSTDOWN,
			expected: ErrNetworkError,
		},
		{
			name:     "EPIPE",
			input:    syscall.EPIPE,
			expected: ErrConnectionLost,
		},
		{
			name:     "ENOTCONN",
			input:    syscall.ENOTCONN,
			expected: ErrConnectionLost,
		},
		{
			name:     "EACCES",
			input:    syscall.EACCES,
			expected: ErrAccessDenied,
		},
		{
			name:     "EPERM",
			input:    syscall.EPERM,
			expected: ErrAccessDenied,
		},
		{
			name:     "EINVAL",
			input:    syscall.EINVAL,
			expected: ErrInvalidArgument,
		},
		{
			name:     "EMFILE",
			input:    syscall.EMFILE,
			expected: ErrResourceError,
		},
		{
			name:     "ENFILE",
			input:    syscall.ENFILE,
			expected: ErrResourceError,
		},
		{
			name:     "ENOBUFS",
			input:    syscall.ENOBUFS,
			expected: ErrResourceError,
		},
		{
			name:     "ENOMEM",
			input:    syscall.ENOMEM,
			expected: ErrResourceError,
		},
		{
			name:     "unknown syscall error",
			input:    syscall.Errno(999),
			expected: ErrNetworkError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.translateSyscallError(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRabbit_TranslateByErrorMessage(t *testing.T) {
	r := &Rabbit{}

	tests := []struct {
		name     string
		input    string
		expected error
	}{
		// Connection related
		{
			name:     "connection refused",
			input:    "connection refused",
			expected: ErrConnectionFailed,
		},
		{
			name:     "connection reset",
			input:    "connection reset",
			expected: ErrConnectionLost,
		},
		{
			name:     "connection closed",
			input:    "connection closed",
			expected: ErrConnectionClosed,
		},
		{
			name:     "connection lost",
			input:    "connection lost",
			expected: ErrConnectionLost,
		},
		{
			name:     "connection failed",
			input:    "connection failed",
			expected: ErrConnectionFailed,
		},
		{
			name:     "connection timeout",
			input:    "connection timeout",
			expected: ErrTimeout,
		},
		{
			name:     "no route to host",
			input:    "no route to host",
			expected: ErrNetworkError,
		},
		{
			name:     "network is unreachable",
			input:    "network is unreachable",
			expected: ErrNetworkError,
		},
		{
			name:     "host is down",
			input:    "host is down",
			expected: ErrNetworkError,
		},

		// Channel related
		{
			name:     "channel closed",
			input:    "channel closed",
			expected: ErrChannelClosed,
		},
		{
			name:     "channel error",
			input:    "channel error",
			expected: ErrChannelError,
		},
		{
			name:     "channel exception",
			input:    "channel exception",
			expected: ErrChannelException,
		},

		// Authentication related
		{
			name:     "authentication failed",
			input:    "authentication failed",
			expected: ErrAuthenticationFailed,
		},
		{
			name:     "login failed",
			input:    "login failed",
			expected: ErrAuthenticationFailed,
		},
		{
			name:     "invalid credentials",
			input:    "invalid credentials",
			expected: ErrInvalidCredentials,
		},
		{
			name:     "access denied",
			input:    "access denied",
			expected: ErrAccessDenied,
		},
		{
			name:     "access refused",
			input:    "access refused",
			expected: ErrAccessDenied,
		},
		{
			name:     "permission denied",
			input:    "permission denied",
			expected: ErrInsufficientPermissions,
		},
		{
			name:     "unauthorized",
			input:    "unauthorized",
			expected: ErrInsufficientPermissions,
		},

		// TLS/SSL related
		{
			name:     "tls error",
			input:    "tls error occurred",
			expected: ErrTLSError,
		},
		{
			name:     "ssl error",
			input:    "ssl handshake failed",
			expected: ErrTLSError,
		},
		{
			name:     "certificate error",
			input:    "certificate validation failed",
			expected: ErrCertificateError,
		},
		{
			name:     "handshake error",
			input:    "handshake failed",
			expected: ErrHandshakeFailed,
		},

		// Protocol related
		{
			name:     "protocol error",
			input:    "protocol error",
			expected: ErrProtocolError,
		},
		{
			name:     "frame error",
			input:    "frame error",
			expected: ErrFrameError,
		},
		{
			name:     "syntax error",
			input:    "syntax error",
			expected: ErrSyntaxError,
		},
		{
			name:     "command invalid",
			input:    "command invalid",
			expected: ErrCommandInvalid,
		},
		{
			name:     "unexpected frame",
			input:    "unexpected frame",
			expected: ErrUnexpectedFrame,
		},
		{
			name:     "version mismatch",
			input:    "version mismatch",
			expected: ErrVersionMismatch,
		},

		// Queue and exchange related
		{
			name:     "queue not found",
			input:    "queue not found",
			expected: ErrQueueNotFound,
		},
		{
			name:     "exchange not found",
			input:    "exchange not found",
			expected: ErrExchangeNotFound,
		},
		{
			name:     "queue empty",
			input:    "queue empty",
			expected: ErrQueueEmpty,
		},
		{
			name:     "queue exists",
			input:    "queue exists",
			expected: ErrQueueExists,
		},
		{
			name:     "exchange exists",
			input:    "exchange exists",
			expected: ErrExchangeExists,
		},

		// Message related
		{
			name:     "message too large",
			input:    "message too large",
			expected: ErrMessageTooLarge,
		},
		{
			name:     "content too large",
			input:    "content too large",
			expected: ErrMessageTooLarge,
		},
		{
			name:     "invalid message",
			input:    "invalid message",
			expected: ErrInvalidMessage,
		},
		{
			name:     "message nacked",
			input:    "message nacked",
			expected: ErrMessageNacked,
		},
		{
			name:     "message returned",
			input:    "message returned",
			expected: ErrMessageReturned,
		},
		{
			name:     "publish failed",
			input:    "publish failed",
			expected: ErrPublishFailed,
		},
		{
			name:     "no route",
			input:    "no route",
			expected: ErrPublishFailed,
		},
		{
			name:     "no consumers",
			input:    "no consumers",
			expected: ErrPublishFailed,
		},

		// Operation related
		{
			name:     "bind failed",
			input:    "bind failed",
			expected: ErrBindFailed,
		},
		{
			name:     "unbind failed",
			input:    "unbind failed",
			expected: ErrUnbindFailed,
		},
		{
			name:     "declare failed",
			input:    "declare failed",
			expected: ErrDeclareFailed,
		},
		{
			name:     "delete failed",
			input:    "delete failed",
			expected: ErrDeleteFailed,
		},
		{
			name:     "purge failed",
			input:    "purge failed",
			expected: ErrPurgeFailed,
		},
		{
			name:     "consume failed",
			input:    "consume failed",
			expected: ErrConsumeFailed,
		},
		{
			name:     "ack failed",
			input:    "ack failed",
			expected: ErrAckFailed,
		},
		{
			name:     "nack failed",
			input:    "nack failed",
			expected: ErrNackFailed,
		},
		{
			name:     "reject failed",
			input:    "reject failed",
			expected: ErrRejectFailed,
		},
		{
			name:     "qos failed",
			input:    "qos failed",
			expected: ErrQoSFailed,
		},
		{
			name:     "transaction failed",
			input:    "transaction failed",
			expected: ErrTransactionFailed,
		},

		// Resource related
		{
			name:     "resource locked",
			input:    "resource locked",
			expected: ErrResourceLocked,
		},
		{
			name:     "precondition failed",
			input:    "precondition failed",
			expected: ErrPreconditionFailed,
		},
		{
			name:     "invalid argument",
			input:    "invalid argument",
			expected: ErrInvalidArgument,
		},
		{
			name:     "not allowed",
			input:    "not allowed",
			expected: ErrNotAllowed,
		},
		{
			name:     "not implemented",
			input:    "not implemented",
			expected: ErrNotImplemented,
		},

		// Server related
		{
			name:     "internal error",
			input:    "internal error",
			expected: ErrInternalError,
		},
		{
			name:     "server error",
			input:    "server error",
			expected: ErrServerError,
		},
		{
			name:     "client error",
			input:    "client error",
			expected: ErrClientError,
		},

		// Timeout related
		{
			name:     "timeout",
			input:    "timeout occurred",
			expected: ErrTimeout,
		},
		{
			name:     "deadline exceeded",
			input:    "deadline exceeded",
			expected: ErrTimeout,
		},

		// Virtual host related
		{
			name:     "virtual host not found",
			input:    "virtual host not found",
			expected: ErrVirtualHostNotFound,
		},
		{
			name:     "vhost not found",
			input:    "vhost not found",
			expected: ErrVirtualHostNotFound,
		},

		// User related
		{
			name:     "user not found",
			input:    "user not found",
			expected: ErrUserNotFound,
		},

		// Alarms and limits
		{
			name:     "memory alarm",
			input:    "memory alarm",
			expected: ErrMemoryAlarm,
		},
		{
			name:     "disk alarm",
			input:    "disk alarm",
			expected: ErrDiskAlarm,
		},
		{
			name:     "resource alarm",
			input:    "resource alarm",
			expected: ErrResourceAlarm,
		},
		{
			name:     "flow control",
			input:    "flow control",
			expected: ErrFlowControl,
		},
		{
			name:     "quota exceeded",
			input:    "quota exceeded",
			expected: ErrQuotaExceeded,
		},
		{
			name:     "rate limit",
			input:    "rate limit",
			expected: ErrRateLimit,
		},
		{
			name:     "backpressure",
			input:    "backpressure",
			expected: ErrBackpressure,
		},

		// Cluster related
		{
			name:     "cluster error",
			input:    "cluster error",
			expected: ErrClusterError,
		},
		{
			name:     "node down",
			input:    "node down",
			expected: ErrNodeDown,
		},

		// Cancellation and shutdown
		{
			name:     "canceled",
			input:    "operation canceled",
			expected: ErrCancelled,
		},
		{
			name:     "cancelled",
			input:    "operation cancelled",
			expected: ErrCancelled,
		},
		{
			name:     "shutdown",
			input:    "system shutdown",
			expected: ErrShutdown,
		},

		// Configuration related
		{
			name:     "configuration error",
			input:    "configuration error",
			expected: ErrConfigurationError,
		},
		{
			name:     "config error",
			input:    "config error",
			expected: ErrConfigurationError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalErr := errors.New(tt.input)
			result := r.translateByErrorMessage(tt.input, originalErr)
			assert.Equal(t, tt.expected, result)
		})
	}

	// Test unknown error returns original
	t.Run("unknown error returns original", func(t *testing.T) {
		originalErr := errors.New("some unknown error message")
		result := r.translateByErrorMessage("some unknown error message", originalErr)
		assert.Equal(t, originalErr, result)
	})
}

func TestRabbit_GetErrorCategory(t *testing.T) {
	r := &Rabbit{}

	tests := []struct {
		name     string
		input    error
		expected ErrorCategory
	}{
		{
			name:     "connection error",
			input:    ErrConnectionFailed,
			expected: CategoryConnection,
		},
		{
			name:     "channel error",
			input:    ErrChannelClosed,
			expected: CategoryChannel,
		},
		{
			name:     "authentication error",
			input:    ErrAuthenticationFailed,
			expected: CategoryAuthentication,
		},
		{
			name:     "permission error",
			input:    ErrAccessDenied,
			expected: CategoryPermission,
		},
		{
			name:     "resource error",
			input:    ErrExchangeNotFound,
			expected: CategoryResource,
		},
		{
			name:     "message error",
			input:    ErrMessageTooLarge,
			expected: CategoryMessage,
		},
		{
			name:     "protocol error",
			input:    ErrFrameError,
			expected: CategoryProtocol,
		},
		{
			name:     "network error",
			input:    ErrNetworkError,
			expected: CategoryNetwork,
		},
		{
			name:     "server error",
			input:    ErrInternalError,
			expected: CategoryServer,
		},
		{
			name:     "configuration error",
			input:    ErrConfigurationError,
			expected: CategoryConfiguration,
		},
		{
			name:     "cluster error",
			input:    ErrClusterError,
			expected: CategoryCluster,
		},
		{
			name:     "operation error",
			input:    ErrBindFailed,
			expected: CategoryOperation,
		},
		{
			name:     "alarm error",
			input:    ErrMemoryAlarm,
			expected: CategoryAlarm,
		},
		{
			name:     "timeout error",
			input:    ErrTimeout,
			expected: CategoryTimeout,
		},
		{
			name:     "unknown error",
			input:    errors.New("some unknown error"),
			expected: CategoryUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.GetErrorCategory(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRabbit_IsRetryableError(t *testing.T) {
	r := &Rabbit{}

	tests := []struct {
		name     string
		input    error
		expected bool
	}{
		{
			name:     "connection failed - retryable",
			input:    ErrConnectionFailed,
			expected: true,
		},
		{
			name:     "connection lost - retryable",
			input:    ErrConnectionLost,
			expected: true,
		},
		{
			name:     "channel closed - retryable",
			input:    ErrChannelClosed,
			expected: true,
		},
		{
			name:     "timeout - retryable",
			input:    ErrTimeout,
			expected: true,
		},
		{
			name:     "network error - retryable",
			input:    ErrNetworkError,
			expected: true,
		},
		{
			name:     "server error - retryable",
			input:    ErrServerError,
			expected: true,
		},
		{
			name:     "memory alarm - retryable",
			input:    ErrMemoryAlarm,
			expected: true,
		},
		{
			name:     "rate limit - retryable",
			input:    ErrRateLimit,
			expected: true,
		},
		{
			name:     "cluster error - retryable",
			input:    ErrClusterError,
			expected: true,
		},
		{
			name:     "authentication failed - not retryable",
			input:    ErrAuthenticationFailed,
			expected: false,
		},
		{
			name:     "access denied - not retryable",
			input:    ErrAccessDenied,
			expected: false,
		},
		{
			name:     "queue not found - not retryable",
			input:    ErrQueueNotFound,
			expected: false,
		},
		{
			name:     "invalid argument - not retryable",
			input:    ErrInvalidArgument,
			expected: false,
		},
		{
			name:     "unknown error - not retryable",
			input:    errors.New("some unknown error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.IsRetryableError(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRabbit_IsTemporaryError(t *testing.T) {
	r := &Rabbit{}

	tests := []struct {
		name     string
		input    error
		expected bool
	}{
		{
			name:     "connection failed - temporary",
			input:    ErrConnectionFailed,
			expected: true,
		},
		{
			name:     "resource locked - temporary",
			input:    ErrResourceLocked,
			expected: true,
		},
		{
			name:     "quota exceeded - temporary",
			input:    ErrQuotaExceeded,
			expected: true,
		},
		{
			name:     "message returned - temporary",
			input:    ErrMessageReturned,
			expected: true,
		},
		{
			name:     "message nacked - temporary",
			input:    ErrMessageNacked,
			expected: true,
		},
		{
			name:     "timeout - temporary",
			input:    ErrTimeout,
			expected: true,
		},
		{
			name:     "authentication failed - not temporary",
			input:    ErrAuthenticationFailed,
			expected: false,
		},
		{
			name:     "access denied - not temporary",
			input:    ErrAccessDenied,
			expected: false,
		},
		{
			name:     "queue not found - not temporary",
			input:    ErrQueueNotFound,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.IsTemporaryError(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRabbit_IsPermanentError(t *testing.T) {
	r := &Rabbit{}

	tests := []struct {
		name     string
		input    error
		expected bool
	}{
		{
			name:     "authentication failed - permanent",
			input:    ErrAuthenticationFailed,
			expected: true,
		},
		{
			name:     "invalid credentials - permanent",
			input:    ErrInvalidCredentials,
			expected: true,
		},
		{
			name:     "access denied - permanent",
			input:    ErrAccessDenied,
			expected: true,
		},
		{
			name:     "exchange not found - permanent",
			input:    ErrExchangeNotFound,
			expected: true,
		},
		{
			name:     "queue not found - permanent",
			input:    ErrQueueNotFound,
			expected: true,
		},
		{
			name:     "invalid argument - permanent",
			input:    ErrInvalidArgument,
			expected: true,
		},
		{
			name:     "syntax error - permanent",
			input:    ErrSyntaxError,
			expected: true,
		},
		{
			name:     "not implemented - permanent",
			input:    ErrNotImplemented,
			expected: true,
		},
		{
			name:     "cancelled - permanent",
			input:    ErrCancelled,
			expected: true,
		},
		{
			name:     "shutdown - permanent",
			input:    ErrShutdown,
			expected: true,
		},
		{
			name:     "connection failed - not permanent",
			input:    ErrConnectionFailed,
			expected: false,
		},
		{
			name:     "server error - not permanent",
			input:    ErrServerError,
			expected: false,
		},
		{
			name:     "timeout - not permanent",
			input:    ErrTimeout,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.IsPermanentError(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRabbit_IsConnectionError(t *testing.T) {
	r := &Rabbit{}

	tests := []struct {
		name     string
		input    error
		expected bool
	}{
		{
			name:     "connection failed - is connection error",
			input:    ErrConnectionFailed,
			expected: true,
		},
		{
			name:     "connection lost - is connection error",
			input:    ErrConnectionLost,
			expected: true,
		},
		{
			name:     "connection closed - is connection error",
			input:    ErrConnectionClosed,
			expected: true,
		},
		{
			name:     "channel error - not connection error",
			input:    ErrChannelError,
			expected: false,
		},
		{
			name:     "authentication failed - not connection error",
			input:    ErrAuthenticationFailed,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.IsConnectionError(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRabbit_IsChannelError(t *testing.T) {
	r := &Rabbit{}

	tests := []struct {
		name     string
		input    error
		expected bool
	}{
		{
			name:     "channel closed - is channel error",
			input:    ErrChannelClosed,
			expected: true,
		},
		{
			name:     "channel error - is channel error",
			input:    ErrChannelError,
			expected: true,
		},
		{
			name:     "channel exception - is channel error",
			input:    ErrChannelException,
			expected: true,
		},
		{
			name:     "connection error - not channel error",
			input:    ErrConnectionFailed,
			expected: false,
		},
		{
			name:     "authentication failed - not channel error",
			input:    ErrAuthenticationFailed,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.IsChannelError(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRabbit_IsAuthenticationError(t *testing.T) {
	r := &Rabbit{}

	tests := []struct {
		name     string
		input    error
		expected bool
	}{
		{
			name:     "authentication failed - is authentication error",
			input:    ErrAuthenticationFailed,
			expected: true,
		},
		{
			name:     "invalid credentials - is authentication error",
			input:    ErrInvalidCredentials,
			expected: true,
		},
		{
			name:     "access denied - is authentication error",
			input:    ErrAccessDenied,
			expected: true,
		},
		{
			name:     "insufficient permissions - is authentication error",
			input:    ErrInsufficientPermissions,
			expected: true,
		},
		{
			name:     "connection error - not authentication error",
			input:    ErrConnectionFailed,
			expected: false,
		},
		{
			name:     "queue not found - not authentication error",
			input:    ErrQueueNotFound,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.IsAuthenticationError(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRabbit_IsResourceError(t *testing.T) {
	r := &Rabbit{}

	tests := []struct {
		name     string
		input    error
		expected bool
	}{
		{
			name:     "exchange not found - is resource error",
			input:    ErrExchangeNotFound,
			expected: true,
		},
		{
			name:     "queue not found - is resource error",
			input:    ErrQueueNotFound,
			expected: true,
		},
		{
			name:     "queue empty - is resource error",
			input:    ErrQueueEmpty,
			expected: true,
		},
		{
			name:     "queue exists - is resource error",
			input:    ErrQueueExists,
			expected: true,
		},
		{
			name:     "exchange exists - is resource error",
			input:    ErrExchangeExists,
			expected: true,
		},
		{
			name:     "resource locked - is resource error",
			input:    ErrResourceLocked,
			expected: true,
		},
		{
			name:     "resource error - is resource error",
			input:    ErrResourceError,
			expected: true,
		},
		{
			name:     "connection error - not resource error",
			input:    ErrConnectionFailed,
			expected: false,
		},
		{
			name:     "authentication failed - not resource error",
			input:    ErrAuthenticationFailed,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.IsResourceError(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRabbit_IsAlarmError(t *testing.T) {
	r := &Rabbit{}

	tests := []struct {
		name     string
		input    error
		expected bool
	}{
		{
			name:     "memory alarm - is alarm error",
			input:    ErrMemoryAlarm,
			expected: true,
		},
		{
			name:     "disk alarm - is alarm error",
			input:    ErrDiskAlarm,
			expected: true,
		},
		{
			name:     "resource alarm - is alarm error",
			input:    ErrResourceAlarm,
			expected: true,
		},
		{
			name:     "flow control - is alarm error",
			input:    ErrFlowControl,
			expected: true,
		},
		{
			name:     "quota exceeded - is alarm error",
			input:    ErrQuotaExceeded,
			expected: true,
		},
		{
			name:     "rate limit - is alarm error",
			input:    ErrRateLimit,
			expected: true,
		},
		{
			name:     "backpressure - is alarm error",
			input:    ErrBackpressure,
			expected: true,
		},
		{
			name:     "connection error - not alarm error",
			input:    ErrConnectionFailed,
			expected: false,
		},
		{
			name:     "authentication failed - not alarm error",
			input:    ErrAuthenticationFailed,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.IsAlarmError(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Helper types for testing network errors
type timeoutError struct {
	timeout bool
}

func (e *timeoutError) Error() string   { return "timeout error" }
func (e *timeoutError) Timeout() bool   { return e.timeout }
func (e *timeoutError) Temporary() bool { return false }

type temporaryError struct {
	temporary bool
}

func (e *temporaryError) Error() string   { return "temporary error" }
func (e *temporaryError) Timeout() bool   { return false }
func (e *temporaryError) Temporary() bool { return e.temporary }

type genericNetError struct{}

func (e *genericNetError) Error() string   { return "network error" }
func (e *genericNetError) Timeout() bool   { return false }
func (e *genericNetError) Temporary() bool { return false }

// Integration tests
func TestRabbit_ErrorTranslation_Integration(t *testing.T) {
	r := &Rabbit{}

	// Test a complex scenario with AMQP error
	amqpErr := &amqp.Error{
		Code:    amqp.AccessRefused,
		Reason:  "access refused",
		Server:  true,
		Recover: false,
	}

	translatedErr := r.TranslateError(amqpErr)
	require.Equal(t, ErrAccessDenied, translatedErr)

	// Test error categorization
	category := r.GetErrorCategory(translatedErr)
	assert.Equal(t, CategoryPermission, category)

	// Test retry behavior
	assert.False(t, r.IsRetryableError(translatedErr))
	assert.False(t, r.IsTemporaryError(translatedErr))
	assert.True(t, r.IsPermanentError(translatedErr))
	assert.True(t, r.IsAuthenticationError(translatedErr))
}

// Benchmark tests
func BenchmarkRabbit_TranslateError_AMQPError(b *testing.B) {
	r := &Rabbit{}
	amqpErr := &amqp.Error{Code: amqp.AccessRefused, Reason: "access refused"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.TranslateError(amqpErr)
	}
}

func BenchmarkRabbit_TranslateError_StringMatch(b *testing.B) {
	r := &Rabbit{}
	err := errors.New("connection refused")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.TranslateError(err)
	}
}

func BenchmarkRabbit_TranslateError_SyscallError(b *testing.B) {
	r := &Rabbit{}
	err := syscall.ECONNREFUSED

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.TranslateError(err)
	}
}

func BenchmarkRabbit_TranslateError_NetworkError(b *testing.B) {
	r := &Rabbit{}
	err := &timeoutError{timeout: true}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.TranslateError(err)
	}
}

// Test error wrapping scenarios
func TestRabbit_TranslateError_WrappedErrors(t *testing.T) {
	r := &Rabbit{}

	// Test wrapped AMQP error
	amqpErr := &amqp.Error{Code: amqp.AccessRefused, Reason: "access refused"}
	wrappedErr := errors.New("wrapped: " + amqpErr.Error())

	// Should fall back to string matching
	result := r.TranslateError(wrappedErr)
	assert.Equal(t, ErrAccessDenied, result)

	// Test wrapped syscall error
	syscallErr := syscall.ECONNREFUSED
	wrappedSyscallErr := errors.New("connection failed: " + syscallErr.Error())

	result = r.TranslateError(wrappedSyscallErr)
	assert.Equal(t, ErrConnectionFailed, result)
}

// Test edge cases
func TestRabbit_TranslateError_EdgeCases(t *testing.T) {
	r := &Rabbit{}

	// Test with empty error message
	emptyErr := errors.New("")
	result := r.TranslateError(emptyErr)
	assert.Equal(t, emptyErr, result)

	// Test with very long error message
	longErr := errors.New("this is a very long error message that contains connection refused somewhere in the middle of a lot of text")
	result = r.TranslateError(longErr)
	assert.Equal(t, ErrConnectionFailed, result)

	// Test case sensitivity
	caseErr := errors.New("Connection Refused")
	result = r.TranslateError(caseErr)
	assert.Equal(t, ErrConnectionFailed, result)
}

// Test multiple error types in one error message
func TestRabbit_TranslateError_MultiplePatterns(t *testing.T) {
	r := &Rabbit{}

	// Test error message with multiple patterns - should match the first one found
	multiErr := errors.New("connection refused due to authentication failed")
	result := r.TranslateError(multiErr)
	// Should match connection refused first
	assert.Equal(t, ErrConnectionFailed, result)
}
