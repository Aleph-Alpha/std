package rabbit

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Aleph-Alpha/std/v1/observability"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Rabbit represents a client for interacting with RabbitMQ.
// It manages connections, channels, and provides methods for publishing
// and consuming messages with automatic reconnection capabilities.
type RabbitClient struct {
	// cfg stores the configuration for this RabbitMQ client
	cfg Config

	// observer provides optional observability hooks for tracking operations
	observer observability.Observer

	// logger provides optional logging for lifecycle and background operations
	logger Logger

	// Channel is the main AMQP channel used for publishing and consuming messages.
	// It's exposed publicly to allow direct operations when needed.
	Channel *amqp.Channel

	// conn is the underlying AMQP connection to the RabbitMQ server
	conn *amqp.Connection

	// mu protects concurrent access to connection and channel
	mu sync.RWMutex

	// shutdownSignal is closed when the client is being shut down
	shutdownSignal chan struct{}

	closeShutdownOnce sync.Once
}

// NewClient creates and initializes a new RabbitMQ client with the provided configuration.
// This function establishes the initial connection to RabbitMQ, sets up channels,
// and configures exchanges and queues as specified in the configuration.
//
// Parameters:
//   - cfg: Configuration for connecting to RabbitMQ and setting up channels
//   - logger: Logger implementation for recording events and errors
//
// Returns a new RabbitClient instance that is ready to use.
// If connection fails after all retries or channel setup fails, it will return an error.
//
// Example:
//
//	client, err := rabbit.NewClient(config)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer client.GracefulShutdown()
func NewClient(config Config) (*RabbitClient, error) {
	con, err := newConnection(config)
	if err != nil {
		return nil, err
	}

	ch, err := connectToChannel(con, config)
	if ch == nil || err != nil {
		return nil, err
	}

	return &RabbitClient{
		cfg:            config,
		observer:       nil, // No observer by default
		logger:         nil, // No logger by default
		conn:           con,
		Channel:        ch,
		shutdownSignal: make(chan struct{}),
	}, nil
}

// connectToChannel creates and configures a RabbitMQ channel for consumers and publishers.
// This function sets up the channel with the appropriate exchanges, queues, and bindings
// based on the provided configuration.
//
// Parameters:
//   - rb: The AMQP connection to create the channel from
//   - cfg: Configuration specifying channel, exchange, queue, and binding settings
//   - logger: Logger for recording events and errors
//
// Returns:
//   - *amqp.Channel: The configured channel
//   - error: Any error encountered during set up
//
// The function handles several configuration scenarios:
//   - Basic channel setup for publishers
//   - Full exchange, queue, and binding setup for consumers
//   - Dead letter exchange and queue configuration when enabled
//   - QoS (prefetch) settings for controlling message delivery
func connectToChannel(rb *amqp.Connection, cfg Config) (*amqp.Channel, error) {
	ch, err := rb.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to create channel: %w", err)
	}

	if err = ch.Confirm(false); err != nil {
		return nil, fmt.Errorf("failed to enable publisher confirms: %w", err)
	}

	if !cfg.Channel.IsConsumer {
		return ch, nil
	}

	// Declare the main exchange
	err = ch.ExchangeDeclare(
		cfg.Channel.ExchangeName,
		cfg.Channel.ExchangeType,
		true,  // Durable
		false, // AutoDelete
		false, // Internal
		false, // NoWait
		nil,   // Arguments
	)
	if err != nil {
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Set up dead letter configuration if enabled
	queueArgs := amqp.Table{}
	if cfg.DeadLetter.ExchangeName != "" && cfg.DeadLetter.Ttl > 0 {
		// Declare dead letter exchange
		err = ch.ExchangeDeclare(
			cfg.DeadLetter.ExchangeName,
			"direct",
			true,  // Durable
			false, // AutoDelete
			false, // Internal
			false, // NoWait
			nil,   // Arguments
		)
		if err != nil {
			return nil, fmt.Errorf("failed to declare dead letter exchange: %w", err)
		}

		// Set up dead letter queue with configured name
		_, err = ch.QueueDeclare(
			cfg.DeadLetter.QueueName,
			true,  // Durable
			false, // AutoDelete
			false, // Exclusive
			false, // NoWait
			nil,   // Arguments
		)
		if err != nil {
			return nil, fmt.Errorf("failed to declare dead letter queue: %w", err)
		}

		// Bind a dead letter queue to exchange
		err = ch.QueueBind(
			cfg.DeadLetter.QueueName,
			cfg.DeadLetter.RoutingKey,
			cfg.DeadLetter.ExchangeName,
			false, // NoWait
			nil,   // Arguments
		)
		if err != nil {
			return nil, fmt.Errorf("failed to bind dead letter queue: %w", err)
		}

		// Configure main queue arguments for dead lettering
		queueArgs = amqp.Table{
			"x-dead-letter-exchange":    cfg.DeadLetter.ExchangeName,
			"x-dead-letter-routing-key": cfg.DeadLetter.RoutingKey,
			"x-message-ttl":             cfg.DeadLetter.Ttl * 1000, // Convert to milliseconds
		}
	}

	// Declare main queue
	_, err = ch.QueueDeclare(
		cfg.Channel.QueueName,
		true,      // Durable
		false,     // AutoDelete
		false,     // Exclusive
		false,     // NoWait
		queueArgs, // Arguments including dead letter config
	)
	if err != nil {
		return nil, fmt.Errorf("failed to declare queue: %w", err)
	}

	// Bind main queue to exchange
	err = ch.QueueBind(
		cfg.Channel.QueueName,
		cfg.Channel.RoutingKey,
		cfg.Channel.ExchangeName,
		false, // NoWait
		nil,   // Arguments
	)
	if err != nil {
		return nil, fmt.Errorf("failed to bind queue: %w", err)
	}

	// Set QoS if specified
	if cfg.Channel.PrefetchCount > 0 {
		err = ch.Qos(cfg.Channel.PrefetchCount, 0, false)
		if err != nil {
			return nil, fmt.Errorf("failed to set QoS: %w", err)
		}
	}

	return ch, nil
}

// RetryConnection continuously monitors the RabbitMQ connection and automatically
// re-establishes it if it fails. This method is typically run in a goroutine.
//
// Parameters:
//   - logger: Logger for recording reconnection events and errors
//   - cfg: Configuration for establishing new connections
//
// The method will:
//   - Monitor the connection for closure events
//   - Attempt to reconnect when the connection is lost
//   - Re-establish channels and their configurations
//   - Continue monitoring until the shutdownSignal is received
//
// This provides resilience against network issues and RabbitMQ server restarts.
func (rb *RabbitClient) RetryConnection(cfg Config) {
	defer rb.closeShutdownOnce.Do(func() {
		close(rb.shutdownSignal)
	})
outerLoop:
	for {
		errChan := make(chan *amqp.Error, 1)
		rb.conn.NotifyClose(errChan)

		select {
		case <-rb.shutdownSignal:
			rb.logInfo(context.Background(), "Stopping RetryConnection loop due to shutdown signal", nil)
			return

		case err := <-errChan:
			rb.logWarn(context.Background(), "RabbitMQ connection closed, retrying", map[string]interface{}{
				"error": err.Error(),
			})
		reconnectLoop:
			for {
				select {
				case <-rb.shutdownSignal:
					rb.logInfo(context.Background(), "Stopping RetryConnection loop due to shutdown signal", nil)
					return
				default:
					newConn, err := newConnection(cfg)
					if err != nil {
						rb.logError(context.Background(), "RabbitMQ reconnection failed", map[string]interface{}{
							"error": err.Error(),
						})
						time.Sleep(time.Second)
						continue reconnectLoop
					}

					rb.mu.Lock()
					rb.conn = newConn
					if rb.Channel != nil {
						_ = rb.Channel.Close()
					}
					rb.Channel, err = connectToChannel(newConn, cfg)
					rb.mu.Unlock()

					if err != nil {
						rb.logError(context.Background(), "Failed to re-establish RabbitMQ channel", map[string]interface{}{
							"error": err.Error(),
						})
						continue reconnectLoop
					}

					rb.logInfo(context.Background(), "Successfully reconnected to RabbitMQ", nil)
					continue outerLoop
				}
			}
		}
	}
}

// newConnection establishes a connection to the RabbitMQ server with retry logic.
// This function handles different connection scenarios, including TLS/SSL configurations.
//
// Parameters:
//   - cfg: Configuration containing connection details
//   - logger: Logger for recording connection events and errors
//
// Returns:
//   - *amqp.Connection: Established connection to RabbitMQ
//   - error: Any error encountered during connection
//
// The function supports three connection modes:
//   - SSL with client certificates (full TLS authentication)
//   - SSL without client certificates (server authentication only)
//   - Plain AMQP (no SSL/TLS)
//
// All connections use a 2-second heartbeat interval to detect disconnections quickly.
func newConnection(cfg Config) (*amqp.Connection, error) {

	if cfg.Connection.IsSSLEnabled && cfg.Connection.UseCert {
		hostURL := fmt.Sprintf("amqps://%v:%v@%v:%v", cfg.Connection.User, cfg.Connection.Password, cfg.Connection.Host, cfg.Connection.Port)
		caCert, err := os.ReadFile(cfg.Connection.CACertPath)
		if err != nil {
			return nil, err
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		cert, err := tls.LoadX509KeyPair(cfg.Connection.ClientCertPath, cfg.Connection.ClientKeyPath)
		if err != nil {
			return nil, err
		}

		tlsConfig := &tls.Config{
			RootCAs:      caCertPool,
			Certificates: []tls.Certificate{cert},
			ServerName:   cfg.Connection.ServerName,
		}
		conn, err := amqp.DialConfig(hostURL, amqp.Config{
			Heartbeat:       2 * time.Second,
			TLSClientConfig: tlsConfig,
		})
		if err == nil {
			return conn, nil
		}
	} else if !cfg.Connection.IsSSLEnabled {
		hostURL := fmt.Sprintf("amqp://%v:%v@%v:%v", cfg.Connection.User, cfg.Connection.Password, cfg.Connection.Host, cfg.Connection.Port)
		conn, err := amqp.DialConfig(hostURL, amqp.Config{
			Heartbeat: 2 * time.Second,
		})
		if err == nil {
			return conn, nil
		}
	} else {
		hostURL := fmt.Sprintf("amqps://%v:%v@%v:%v", cfg.Connection.User, cfg.Connection.Password, cfg.Connection.Host, cfg.Connection.Port)
		conn, err := amqp.DialConfig(hostURL, amqp.Config{
			Heartbeat: 2 * time.Second,
		})
		if err == nil {
			return conn, nil
		}
	}
	return nil, fmt.Errorf("failed to connect to Rabbit")
}

// WithObserver attaches an observer to the RabbitMQ client for observability hooks.
// This method uses the builder pattern and returns the client for method chaining.
//
// The observer will be notified of all publish and consume operations, allowing
// external systems to track metrics, traces, or other observability data.
//
// This is useful for non-FX usage where you want to attach an observer after
// creating the client. When using FX, the observer is automatically injected via NewClientWithDI.
//
// Example:
//
//	client, err := rabbit.NewClient(config)
//	if err != nil {
//	    return err
//	}
//	client = client.WithObserver(myObserver)
//	defer client.GracefulShutdown()
func (rb *RabbitClient) WithObserver(observer observability.Observer) *RabbitClient {
	rb.observer = observer
	return rb
}

// WithLogger attaches a logger to the RabbitMQ client for internal logging.
// This method uses the builder pattern and returns the client for method chaining.
//
// The logger will be used for lifecycle events, background worker logs, and cleanup errors.
// This is particularly useful for debugging and monitoring reconnection behavior.
//
// This is useful for non-FX usage where you want to enable logging after
// creating the client. When using FX, the logger is automatically injected via NewClientWithDI.
//
// Example:
//
//	client, err := rabbit.NewClient(config)
//	if err != nil {
//	    return err
//	}
//	client = client.WithLogger(myLogger)
//	defer client.GracefulShutdown()
func (rb *RabbitClient) WithLogger(logger Logger) *RabbitClient {
	rb.logger = logger
	return rb
}

// logInfo logs an informational message using the configured logger if available.
// This is used for lifecycle and background operation logging.
func (rb *RabbitClient) logInfo(ctx context.Context, msg string, fields map[string]interface{}) {
	if rb.logger != nil {
		rb.logger.InfoWithContext(ctx, msg, nil, fields)
	}
	// Silently skip if no logger configured
}

// logWarn logs a warning message using the configured logger if available.
// This is used for non-critical issues during shutdown or background operations.
func (rb *RabbitClient) logWarn(ctx context.Context, msg string, fields map[string]interface{}) {
	if rb.logger != nil {
		rb.logger.WarnWithContext(ctx, msg, nil, fields)
	}
	// Silently skip if no logger configured
}

// logError logs an error message using the configured logger if available.
// This is only used for errors in background goroutines that can't be returned to the caller.
func (rb *RabbitClient) logError(ctx context.Context, msg string, fields map[string]interface{}) {
	if rb.logger != nil {
		rb.logger.ErrorWithContext(ctx, msg, nil, fields)
	}
	// Silently skip if no logger configured
}
