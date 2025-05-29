package rabbit

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	amqp "github.com/rabbitmq/amqp091-go"
	"os"
	"sync"
	"time"
)

// Logger defines the interface for logging operations in the rabbit package.
// This interface allows the package to use any logging implementation that
// conforms to these methods.
//
//go:generate mockgen -source=setup.go -destination=mock_logger.go -package=rabbit
type Logger interface {
	// Info logs informational messages, optionally with error and contextual fields
	Info(msg string, err error, fields ...map[string]interface{})

	// Debug logs debug-level messages, optionally with error and contextual fields
	Debug(msg string, err error, fields ...map[string]interface{})

	// Warn logs warning messages, optionally with error and contextual fields
	Warn(msg string, err error, fields ...map[string]interface{})

	// Error logs error messages with the associated error and optional contextual fields
	Error(msg string, err error, fields ...map[string]interface{})

	// Fatal logs critical errors that should terminate the application
	Fatal(msg string, err error, fields ...map[string]interface{})
}

// Rabbit represents a client for interacting with RabbitMQ.
// It manages connections, channels, and provides methods for publishing
// and consuming messages with automatic reconnection capabilities.
type Rabbit struct {
	// cfg stores the configuration for this RabbitMQ client
	cfg Config

	// Channel is the main AMQP channel used for publishing and consuming messages.
	// It's exposed publicly to allow direct operations when needed.
	Channel *amqp.Channel

	// conn is the underlying AMQP connection to the RabbitMQ server
	conn *amqp.Connection

	// logger is used for logging operations and errors
	logger Logger

	// mu protects concurrent access to connection and channel
	mu sync.RWMutex

	// shutdownSignal is closed when the client is being shut down
	shutdownSignal chan struct{}
}

// NewClient creates and initializes a new RabbitMQ client with the provided configuration.
// This function establishes the initial connection to RabbitMQ, sets up channels,
// and configures exchanges and queues as specified in the configuration.
//
// Parameters:
//   - cfg: Configuration for connecting to RabbitMQ and setting up channels
//   - logger: Logger implementation for recording events and errors
//
// Returns a new Rabbit client instance that is ready to use.
// If connection fails after all retries or channel setup fails, it will log a fatal error.
//
// Example:
//
//	client := rabbit.NewClient(config, myLogger)
//	defer client.Close()
func NewClient(config Config, logger Logger) *Rabbit {
	con, err := newConnection(config, logger)
	if err != nil {
		logger.Fatal("error in connecting to rabbit after all retries", nil, nil)
	}

	ch, err := connectToChannel(con, config, logger)
	if ch == nil || err != nil {
		logger.Fatal("error in declaring channel", nil, nil)
	}

	return &Rabbit{
		cfg:            config,
		conn:           con,
		Channel:        ch,
		logger:         logger,
		shutdownSignal: make(chan struct{}),
	}
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
func connectToChannel(rb *amqp.Connection, cfg Config, logger Logger) (*amqp.Channel, error) {
	ch, err := rb.Channel()
	if err != nil {
		logger.Error("failed to create channel", err, nil)
		return nil, fmt.Errorf("failed to create channel: %w", err)
	}

	if err = ch.Confirm(false); err != nil {
		logger.Error("failed to enable publisher confirms", err, nil)
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
		logger.Error("failed to declare exchange", err, map[string]interface{}{
			"exchange": cfg.Channel.ExchangeName,
		})
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
			logger.Error("failed to declare dead letter exchange", err, map[string]interface{}{
				"exchange": cfg.DeadLetter.ExchangeName,
			})
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
			logger.Error("failed to declare dead letter queue", err, map[string]interface{}{
				"queue": cfg.DeadLetter.QueueName,
			})
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
			logger.Error("failed to bind dead letter queue", err, map[string]interface{}{
				"queue":    cfg.DeadLetter.QueueName,
				"exchange": cfg.DeadLetter.ExchangeName,
			})
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
		logger.Error("failed to declare queue", err, map[string]interface{}{
			"queue": cfg.Channel.QueueName,
		})
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
		logger.Error("failed to bind queue", err, map[string]interface{}{
			"queue":    cfg.Channel.QueueName,
			"exchange": cfg.Channel.ExchangeName,
		})
		return nil, fmt.Errorf("failed to bind queue: %w", err)
	}

	// Set QoS if specified
	if cfg.Channel.PrefetchCount > 0 {
		err = ch.Qos(cfg.Channel.PrefetchCount, 0, false)
		if err != nil {
			logger.Error("failed to set QoS", err, map[string]interface{}{
				"prefetch_count": cfg.Channel.PrefetchCount,
			})
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
func (rb *Rabbit) RetryConnection(logger Logger, cfg Config) {
outerLoop:
	for {
		errChan := make(chan *amqp.Error, 1)
		rb.conn.NotifyClose(errChan)

		select {
		case <-rb.shutdownSignal:
			logger.Info("Stopping RetryConnection loop due to shutdown signal", nil, nil)
			return

		case err := <-errChan:
			logger.Warn("RabbitMQ connection closed, retrying...", err, nil)
		reconnectLoop:
			for {
				select {
				case <-rb.shutdownSignal:
					logger.Info("Stopping RetryConnection loop due to shutdown signal inside reconnect", nil, nil)
					return
				default:
					newConn, err := newConnection(cfg, logger)
					if err != nil {
						logger.Error("Reconnection failed", err, nil)
						time.Sleep(time.Second)
						continue reconnectLoop
					}

					rb.mu.Lock()
					rb.conn = newConn
					if rb.Channel != nil {
						_ = rb.Channel.Close()
					}
					rb.Channel, err = connectToChannel(newConn, cfg, logger)
					rb.mu.Unlock()

					if err != nil {
						logger.Error("Failed to reopen channel, retrying...", err, nil)
						continue reconnectLoop
					}

					logger.Info("Reconnected to RabbitMQ", nil, nil)
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
func newConnection(cfg Config, logger Logger) (*amqp.Connection, error) {

	logger.Info("Connecting to Rabbit", nil, nil)

	if cfg.Connection.IsSSLEnabled && cfg.Connection.UseCert {
		hostURL := fmt.Sprintf("amqps://%v:%v@%v:%v", cfg.Connection.User, cfg.Connection.Password, cfg.Connection.Host, cfg.Connection.Port)
		caCert, err := os.ReadFile(cfg.Connection.CACertPath)
		if err != nil {
			logger.Error("failed to read CA certificate", err, nil)
			return nil, err
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		cert, err := tls.LoadX509KeyPair(cfg.Connection.ClientCertPath, cfg.Connection.ClientKeyPath)
		if err != nil {
			logger.Error("failed to load client cert/key", err, nil)
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
			logger.Info("Connected to Rabbit", nil, map[string]interface{}{
				"rabbit_addr": hostURL,
			})
			return conn, nil
		}
		logger.Error("error in connecting to rabbit", nil, map[string]interface{}{
			"rabbit_addr": hostURL,
			"error":       err,
		})
	} else if !cfg.Connection.IsSSLEnabled {
		hostURL := fmt.Sprintf("amqp://%v:%v@%v:%v", cfg.Connection.User, cfg.Connection.Password, cfg.Connection.Host, cfg.Connection.Port)
		//conn, err := amqp.Dial(hostURL)
		conn, err := amqp.DialConfig(hostURL, amqp.Config{
			Heartbeat: 2 * time.Second,
		})
		if err == nil {
			logger.Info("Connected to Rabbit", nil, map[string]interface{}{
				"rabbit_addr": hostURL,
			})
			return conn, nil
		}
		logger.Error("error in connecting to rabbit", nil, map[string]interface{}{
			"rabbit_addr": hostURL,
			"error":       err,
		})
	} else {
		hostURL := fmt.Sprintf("amqps://%v:%v@%v:%v", cfg.Connection.User, cfg.Connection.Password, cfg.Connection.Host, cfg.Connection.Port)
		conn, err := amqp.DialConfig(hostURL, amqp.Config{
			Heartbeat: 2 * time.Second,
		})
		if err == nil {
			logger.Info("Connected to Rabbit", nil, map[string]interface{}{
				"rabbit_addr": hostURL,
			})
			return conn, nil
		}
		logger.Error("error in connecting to rabbit", nil, map[string]interface{}{
			"rabbit_addr": hostURL,
			"error":       err,
		})
	}
	return nil, fmt.Errorf("failed to connect to Rabbit")
}
