package rabbit

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Rabbit represents a client for interacting with RabbitMQ.
// It manages connections, channels, and provides methods for publishing
// and consuming messages with automatic reconnection capabilities.
type RabbitClient struct {
	// cfg stores the configuration for this RabbitMQ client
	cfg Config

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
		log.Printf("ERROR: error in connecting to rabbit after all retries: %v", err)
		return nil, err
	}

	ch, err := connectToChannel(con, config)
	if ch == nil || err != nil {
		log.Printf("ERROR: error in declaring channel: %v", err)
		return nil, err
	}

	return &RabbitClient{
		cfg:            config,
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
		log.Printf("ERROR: error in creating channel: %v", err)
		return nil, fmt.Errorf("failed to create channel: %w", err)
	}

	if err = ch.Confirm(false); err != nil {
		log.Printf("ERROR: error in enabling publisher confirms: %v", err)
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
		log.Printf("ERROR: error in declaring exchange: %v", err)
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
			log.Printf("ERROR: error in declaring dead letter exchange: %v", err)
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
			log.Printf("ERROR: error in declaring dead letter queue: %v", err)
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
			log.Printf("ERROR: error in binding dead letter queue: %v", err)
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
		log.Printf("ERROR: error in declaring queue: %v", err)
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
		log.Printf("ERROR: error in binding queue: %v", err)
		return nil, fmt.Errorf("failed to bind queue: %w", err)
	}

	// Set QoS if specified
	if cfg.Channel.PrefetchCount > 0 {
		err = ch.Qos(cfg.Channel.PrefetchCount, 0, false)
		if err != nil {
			log.Printf("ERROR: error in setting QoS: %v", err)
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
			log.Println("INFO: Stopping RetryConnection loop due to shutdown signal")
			return

		case err := <-errChan:
			log.Printf("WARNING: RabbitMQ connection closed, retrying... %v", err)
		reconnectLoop:
			for {
				select {
				case <-rb.shutdownSignal:
					log.Println("INFO: Stopping RetryConnection loop due to shutdown signal")
					return
				default:
					newConn, err := newConnection(cfg)
					if err != nil {
						log.Printf("ERROR: RabbitMQ reconnection failed: %v", err)
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
						log.Printf("ERROR: Failed to re-establish RabbitMQ channel: %v", err)
						continue reconnectLoop
					}

					log.Println("INFO: Successfully reconnected to RabbitMQ")
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
			log.Printf("ERROR: failed to read CA cert: %v", err)
			return nil, err
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		cert, err := tls.LoadX509KeyPair(cfg.Connection.ClientCertPath, cfg.Connection.ClientKeyPath)
		if err != nil {
			log.Printf("ERROR: failed to load client cert: %v", err)
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
			log.Println("INFO: Connected to Rabbit")
			return conn, nil
		}
		log.Printf("ERROR: error in connecting to rabbit: %v", err)
	} else if !cfg.Connection.IsSSLEnabled {
		hostURL := fmt.Sprintf("amqp://%v:%v@%v:%v", cfg.Connection.User, cfg.Connection.Password, cfg.Connection.Host, cfg.Connection.Port)
		//conn, err := amqp.Dial(hostURL)
		conn, err := amqp.DialConfig(hostURL, amqp.Config{
			Heartbeat: 2 * time.Second,
		})
		if err == nil {
			log.Println("INFO: Connected to Rabbit")
			return conn, nil
		}
		log.Printf("ERROR: error in connecting to rabbit: %v", err)
	} else {
		hostURL := fmt.Sprintf("amqps://%v:%v@%v:%v", cfg.Connection.User, cfg.Connection.Password, cfg.Connection.Host, cfg.Connection.Port)
		conn, err := amqp.DialConfig(hostURL, amqp.Config{
			Heartbeat: 2 * time.Second,
		})
		if err == nil {
			log.Println("INFO: Connected to Rabbit")
			return conn, nil
		}
		log.Printf("ERROR: error in connecting to rabbit: %v", err)
	}
	return nil, fmt.Errorf("failed to connect to Rabbit")
}
