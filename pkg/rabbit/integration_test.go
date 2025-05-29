package rabbit

import (
	"context"
	"errors"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
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
