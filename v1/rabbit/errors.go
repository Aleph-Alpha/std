package rabbit

import (
	"errors"
	"net"
	"strings"
	"syscall"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Common RabbitMQ error types that can be used by consumers of this package.
// These provide a standardized set of errors that abstract away the
// underlying AMQP-specific error details.
var (
	// ErrConnectionFailed is returned when connection to RabbitMQ cannot be established
	ErrConnectionFailed = errors.New("connection failed")

	// ErrConnectionLost is returned when connection to RabbitMQ is lost
	ErrConnectionLost = errors.New("connection lost")

	// ErrConnectionClosed is returned when connection is closed
	ErrConnectionClosed = errors.New("connection closed")

	// ErrChannelClosed is returned when channel is closed
	ErrChannelClosed = errors.New("channel closed")

	// ErrChannelException is returned when channel encounters an exception
	ErrChannelException = errors.New("channel exception")

	// ErrAuthenticationFailed is returned when authentication fails
	ErrAuthenticationFailed = errors.New("authentication failed")

	// ErrAccessDenied is returned when access is denied to a resource
	ErrAccessDenied = errors.New("access denied")

	// ErrInsufficientPermissions is returned when user lacks necessary permissions
	ErrInsufficientPermissions = errors.New("insufficient permissions")

	// ErrInvalidCredentials is returned when credentials are invalid
	ErrInvalidCredentials = errors.New("invalid credentials")

	// ErrExchangeNotFound is returned when exchange doesn't exist
	ErrExchangeNotFound = errors.New("exchange not found")

	// ErrQueueNotFound is returned when queue doesn't exist
	ErrQueueNotFound = errors.New("queue not found")

	// ErrQueueEmpty is returned when queue is empty
	ErrQueueEmpty = errors.New("queue empty")

	// ErrQueueExists is returned when queue already exists with different properties
	ErrQueueExists = errors.New("queue already exists")

	// ErrExchangeExists is returned when exchange already exists with different properties
	ErrExchangeExists = errors.New("exchange already exists")

	// ErrResourceLocked is returned when resource is locked
	ErrResourceLocked = errors.New("resource locked")

	// ErrPreconditionFailed is returned when precondition check fails
	ErrPreconditionFailed = errors.New("precondition failed")

	// ErrInvalidArgument is returned when argument is invalid
	ErrInvalidArgument = errors.New("invalid argument")

	// ErrInvalidFrameFormat is returned when frame format is invalid
	ErrInvalidFrameFormat = errors.New("invalid frame format")

	// ErrInvalidFrameSize is returned when frame size is invalid
	ErrInvalidFrameSize = errors.New("invalid frame size")

	// ErrFrameError is returned for frame-related errors
	ErrFrameError = errors.New("frame error")

	// ErrSyntaxError is returned for syntax errors in AMQP protocol
	ErrSyntaxError = errors.New("syntax error")

	// ErrCommandInvalid is returned when command is invalid
	ErrCommandInvalid = errors.New("command invalid")

	// ErrChannelError is returned for channel-related errors
	ErrChannelError = errors.New("channel error")

	// ErrUnexpectedFrame is returned when unexpected frame is received
	ErrUnexpectedFrame = errors.New("unexpected frame")

	// ErrResourceError is returned for resource-related errors
	ErrResourceError = errors.New("resource error")

	// ErrNotAllowed is returned when operation is not allowed
	ErrNotAllowed = errors.New("not allowed")

	// ErrNotImplemented is returned when feature is not implemented
	ErrNotImplemented = errors.New("not implemented")

	// ErrInternalError is returned for internal errors
	ErrInternalError = errors.New("internal error")

	// ErrTimeout is returned when operation times out
	ErrTimeout = errors.New("timeout")

	// ErrNetworkError is returned for network-related errors
	ErrNetworkError = errors.New("network error")

	// ErrTLSError is returned for TLS/SSL errors
	ErrTLSError = errors.New("TLS error")

	// ErrCertificateError is returned for certificate-related errors
	ErrCertificateError = errors.New("certificate error")

	// ErrHandshakeFailed is returned when handshake fails
	ErrHandshakeFailed = errors.New("handshake failed")

	// ErrProtocolError is returned for protocol-related errors
	ErrProtocolError = errors.New("protocol error")

	// ErrVersionMismatch is returned when version mismatch occurs
	ErrVersionMismatch = errors.New("version mismatch")

	// ErrServerError is returned for server-side errors
	ErrServerError = errors.New("server error")

	// ErrClientError is returned for client-side errors
	ErrClientError = errors.New("client error")

	// ErrMessageTooLarge is returned when message exceeds size limits
	ErrMessageTooLarge = errors.New("message too large")

	// ErrInvalidMessage is returned when message format is invalid
	ErrInvalidMessage = errors.New("invalid message")

	// ErrMessageNacked is returned when message is negatively acknowledged
	ErrMessageNacked = errors.New("message nacked")

	// ErrMessageReturned is returned when message is returned by broker
	ErrMessageReturned = errors.New("message returned")

	// ErrPublishFailed is returned when publish operation fails
	ErrPublishFailed = errors.New("publish failed")

	// ErrConsumeFailed is returned when consume operation fails
	ErrConsumeFailed = errors.New("consume failed")

	// ErrAckFailed is returned when acknowledge operation fails
	ErrAckFailed = errors.New("acknowledge failed")

	// ErrNackFailed is returned when negative acknowledge operation fails
	ErrNackFailed = errors.New("negative acknowledge failed")

	// ErrRejectFailed is returned when reject operation fails
	ErrRejectFailed = errors.New("reject failed")

	// ErrQoSFailed is returned when QoS operation fails
	ErrQoSFailed = errors.New("QoS failed")

	// ErrBindFailed is returned when bind operation fails
	ErrBindFailed = errors.New("bind failed")

	// ErrUnbindFailed is returned when unbind operation fails
	ErrUnbindFailed = errors.New("unbind failed")

	// ErrDeclareFailed is returned when declare operation fails
	ErrDeclareFailed = errors.New("declare failed")

	// ErrDeleteFailed is returned when delete operation fails
	ErrDeleteFailed = errors.New("delete failed")

	// ErrPurgeFailed is returned when purge operation fails
	ErrPurgeFailed = errors.New("purge failed")

	// ErrTransactionFailed is returned when transaction fails
	ErrTransactionFailed = errors.New("transaction failed")

	// ErrCancelled is returned when operation is cancelled
	ErrCancelled = errors.New("operation cancelled")

	// ErrShutdown is returned when system is shutting down
	ErrShutdown = errors.New("shutdown")

	// ErrConfigurationError is returned for configuration-related errors
	ErrConfigurationError = errors.New("configuration error")

	// ErrVirtualHostNotFound is returned when virtual host doesn't exist
	ErrVirtualHostNotFound = errors.New("virtual host not found")

	// ErrUserNotFound is returned when user doesn't exist
	ErrUserNotFound = errors.New("user not found")

	// ErrClusterError is returned for cluster-related errors
	ErrClusterError = errors.New("cluster error")

	// ErrNodeDown is returned when node is down
	ErrNodeDown = errors.New("node down")

	// ErrMemoryAlarm is returned when memory alarm is triggered
	ErrMemoryAlarm = errors.New("memory alarm")

	// ErrDiskAlarm is returned when disk alarm is triggered
	ErrDiskAlarm = errors.New("disk alarm")

	// ErrResourceAlarm is returned when resource alarm is triggered
	ErrResourceAlarm = errors.New("resource alarm")

	// ErrFlowControl is returned when flow control is active
	ErrFlowControl = errors.New("flow control")

	// ErrQuotaExceeded is returned when quota is exceeded
	ErrQuotaExceeded = errors.New("quota exceeded")

	// ErrRateLimit is returned when rate limit is exceeded
	ErrRateLimit = errors.New("rate limit exceeded")

	// ErrBackpressure is returned when backpressure is applied
	ErrBackpressure = errors.New("backpressure")

	// ErrUnknownError is returned for unknown/unhandled errors
	ErrUnknownError = errors.New("unknown error")
)

// TranslateError converts AMQP/RabbitMQ-specific errors into standardized application errors.
// This function provides abstraction from the underlying AMQP implementation details,
// allowing application code to handle errors in a RabbitMQ-agnostic way.
//
// It maps common RabbitMQ errors to the standardized error types defined above.
// If an error doesn't match any known type, it's returned unchanged.
func (r *Rabbit) TranslateError(err error) error {
	if err == nil {
		return nil
	}

	// Check for AMQP specific errors
	var amqpErr *amqp.Error
	if errors.As(err, &amqpErr) {
		return r.translateAMQPError(amqpErr)
	}

	// Check for network errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		return r.translateNetworkError(netErr)
	}

	// Check for syscall errors
	var syscallErr syscall.Errno
	if errors.As(err, &syscallErr) {
		return r.translateSyscallError(syscallErr)
	}

	// Check error message for common patterns (fallback for string matching)
	errMsg := strings.ToLower(err.Error())
	return r.translateByErrorMessage(errMsg, err)
}

// translateAMQPError maps AMQP error codes to custom errors
func (r *Rabbit) translateAMQPError(amqpErr *amqp.Error) error {
	switch amqpErr.Code {
	// Connection-level errors (300-399)
	case amqp.ConnectionForced:
		return ErrConnectionClosed
	case amqp.InvalidPath:
		return ErrVirtualHostNotFound
	case amqp.AccessRefused:
		return ErrAccessDenied
	case amqp.NotFound:
		return ErrVirtualHostNotFound
	case amqp.ResourceLocked:
		return ErrResourceLocked
	case amqp.PreconditionFailed:
		return ErrPreconditionFailed

	// Channel-level errors (400-499)
	case amqp.ContentTooLarge:
		return ErrMessageTooLarge
	case amqp.NoRoute:
		return ErrPublishFailed
	case amqp.NoConsumers:
		return ErrPublishFailed
	case amqp.ChannelError:
		return ErrChannelError
	case amqp.UnexpectedFrame:
		return ErrUnexpectedFrame
	case amqp.ResourceError:
		return ErrResourceError
	case amqp.NotAllowed:
		return ErrNotAllowed
	case amqp.NotImplemented:
		return ErrNotImplemented
	case amqp.InternalError:
		return ErrInternalError

	// Frame-level errors (500-599)
	case amqp.SyntaxError:
		return ErrSyntaxError
	case amqp.CommandInvalid:
		return ErrCommandInvalid
	case amqp.FrameError:
		return ErrFrameError

	default:
		// Handle based on error reason/message
		return r.translateByAMQPReason(amqpErr.Reason, amqpErr)
	}
}

// translateByAMQPReason translates errors based on AMQP reason field
func (r *Rabbit) translateByAMQPReason(reason string, originalErr *amqp.Error) error {
	reason = strings.ToLower(reason)

	switch {
	// Authentication and authorization
	case strings.Contains(reason, "access refused"):
		return ErrAccessDenied
	case strings.Contains(reason, "login refused"):
		return ErrAuthenticationFailed
	case strings.Contains(reason, "authentication failed"):
		return ErrAuthenticationFailed
	case strings.Contains(reason, "invalid credentials"):
		return ErrInvalidCredentials
	case strings.Contains(reason, "permission denied"):
		return ErrInsufficientPermissions
	case strings.Contains(reason, "access denied"):
		return ErrAccessDenied

	// Exchange and queue errors
	case strings.Contains(reason, "exchange") && strings.Contains(reason, "not found"):
		return ErrExchangeNotFound
	case strings.Contains(reason, "queue") && strings.Contains(reason, "not found"):
		return ErrQueueNotFound
	case strings.Contains(reason, "queue") && strings.Contains(reason, "empty"):
		return ErrQueueEmpty
	case strings.Contains(reason, "queue") && strings.Contains(reason, "exists"):
		return ErrQueueExists
	case strings.Contains(reason, "exchange") && strings.Contains(reason, "exists"):
		return ErrExchangeExists

	// Resource errors
	case strings.Contains(reason, "resource locked"):
		return ErrResourceLocked
	case strings.Contains(reason, "precondition failed"):
		return ErrPreconditionFailed
	case strings.Contains(reason, "invalid argument"):
		return ErrInvalidArgument

	// Message errors
	case strings.Contains(reason, "message too large"):
		return ErrMessageTooLarge
	case strings.Contains(reason, "content too large"):
		return ErrMessageTooLarge
	case strings.Contains(reason, "no route"):
		return ErrPublishFailed
	case strings.Contains(reason, "no consumers"):
		return ErrPublishFailed
	case strings.Contains(reason, "message returned"):
		return ErrMessageReturned
	case strings.Contains(reason, "message nacked"):
		return ErrMessageNacked

	// Channel errors
	case strings.Contains(reason, "channel closed"):
		return ErrChannelClosed
	case strings.Contains(reason, "channel error"):
		return ErrChannelError
	case strings.Contains(reason, "unexpected frame"):
		return ErrUnexpectedFrame

	// Connection errors
	case strings.Contains(reason, "connection closed"):
		return ErrConnectionClosed
	case strings.Contains(reason, "connection forced"):
		return ErrConnectionClosed
	case strings.Contains(reason, "connection lost"):
		return ErrConnectionLost

	// Protocol errors
	case strings.Contains(reason, "frame error"):
		return ErrFrameError
	case strings.Contains(reason, "syntax error"):
		return ErrSyntaxError
	case strings.Contains(reason, "command invalid"):
		return ErrCommandInvalid
	case strings.Contains(reason, "protocol error"):
		return ErrProtocolError

	// Server errors
	case strings.Contains(reason, "internal error"):
		return ErrInternalError
	case strings.Contains(reason, "server error"):
		return ErrServerError
	case strings.Contains(reason, "not implemented"):
		return ErrNotImplemented
	case strings.Contains(reason, "not allowed"):
		return ErrNotAllowed

	// Virtual host errors
	case strings.Contains(reason, "virtual host") && strings.Contains(reason, "not found"):
		return ErrVirtualHostNotFound
	case strings.Contains(reason, "vhost") && strings.Contains(reason, "not found"):
		return ErrVirtualHostNotFound

	// User errors
	case strings.Contains(reason, "user") && strings.Contains(reason, "not found"):
		return ErrUserNotFound

	// Alarms and resource limits
	case strings.Contains(reason, "memory alarm"):
		return ErrMemoryAlarm
	case strings.Contains(reason, "disk alarm"):
		return ErrDiskAlarm
	case strings.Contains(reason, "resource alarm"):
		return ErrResourceAlarm
	case strings.Contains(reason, "flow control"):
		return ErrFlowControl
	case strings.Contains(reason, "quota exceeded"):
		return ErrQuotaExceeded
	case strings.Contains(reason, "rate limit"):
		return ErrRateLimit
	case strings.Contains(reason, "backpressure"):
		return ErrBackpressure

	// Cluster errors
	case strings.Contains(reason, "cluster"):
		return ErrClusterError
	case strings.Contains(reason, "node down"):
		return ErrNodeDown

	default:
		return ErrUnknownError
	}
}

// translateNetworkError maps network errors to custom errors
func (r *Rabbit) translateNetworkError(netErr net.Error) error {
	if netErr.Timeout() {
		return ErrTimeout
	}
	return ErrNetworkError
}

// translateSyscallError maps syscall errors to custom errors
func (r *Rabbit) translateSyscallError(syscallErr syscall.Errno) error {
	switch syscallErr {
	case syscall.ECONNREFUSED:
		return ErrConnectionFailed
	case syscall.ECONNRESET:
		return ErrConnectionLost
	case syscall.ECONNABORTED:
		return ErrConnectionLost
	case syscall.ETIMEDOUT:
		return ErrTimeout
	case syscall.ENETUNREACH:
		return ErrNetworkError
	case syscall.EHOSTUNREACH:
		return ErrNetworkError
	case syscall.EHOSTDOWN:
		return ErrNetworkError
	case syscall.EPIPE:
		return ErrConnectionLost
	case syscall.ENOTCONN:
		return ErrConnectionLost
	case syscall.EACCES:
		return ErrAccessDenied
	case syscall.EPERM:
		return ErrAccessDenied
	case syscall.EINVAL:
		return ErrInvalidArgument
	case syscall.EMFILE:
		return ErrResourceError
	case syscall.ENFILE:
		return ErrResourceError
	case syscall.ENOBUFS:
		return ErrResourceError
	case syscall.ENOMEM:
		return ErrResourceError
	default:
		return ErrNetworkError
	}
}

// translateByErrorMessage translates errors based on error message patterns (fallback)
func (r *Rabbit) translateByErrorMessage(errMsg string, originalErr error) error {
	switch {
	// Connection related
	case strings.Contains(errMsg, "connection refused"):
		return ErrConnectionFailed
	case strings.Contains(errMsg, "connection reset"):
		return ErrConnectionLost
	case strings.Contains(errMsg, "connection closed"):
		return ErrConnectionClosed
	case strings.Contains(errMsg, "connection lost"):
		return ErrConnectionLost
	case strings.Contains(errMsg, "connection failed"):
		return ErrConnectionFailed
	case strings.Contains(errMsg, "connection timeout"):
		return ErrTimeout
	case strings.Contains(errMsg, "no route to host"):
		return ErrNetworkError
	case strings.Contains(errMsg, "network is unreachable"):
		return ErrNetworkError
	case strings.Contains(errMsg, "host is down"):
		return ErrNetworkError

	// Channel related
	case strings.Contains(errMsg, "channel closed"):
		return ErrChannelClosed
	case strings.Contains(errMsg, "channel error"):
		return ErrChannelError
	case strings.Contains(errMsg, "channel exception"):
		return ErrChannelException

	// Authentication related
	case strings.Contains(errMsg, "authentication failed"):
		return ErrAuthenticationFailed
	case strings.Contains(errMsg, "login failed"):
		return ErrAuthenticationFailed
	case strings.Contains(errMsg, "invalid credentials"):
		return ErrInvalidCredentials
	case strings.Contains(errMsg, "access denied"):
		return ErrAccessDenied
	case strings.Contains(errMsg, "access refused"):
		return ErrAccessDenied
	case strings.Contains(errMsg, "permission denied"):
		return ErrInsufficientPermissions
	case strings.Contains(errMsg, "unauthorized"):
		return ErrInsufficientPermissions

	// TLS/SSL related
	case strings.Contains(errMsg, "tls"):
		return ErrTLSError
	case strings.Contains(errMsg, "ssl"):
		return ErrTLSError
	case strings.Contains(errMsg, "certificate"):
		return ErrCertificateError
	case strings.Contains(errMsg, "handshake"):
		return ErrHandshakeFailed

	// Protocol related
	case strings.Contains(errMsg, "protocol error"):
		return ErrProtocolError
	case strings.Contains(errMsg, "frame error"):
		return ErrFrameError
	case strings.Contains(errMsg, "syntax error"):
		return ErrSyntaxError
	case strings.Contains(errMsg, "command invalid"):
		return ErrCommandInvalid
	case strings.Contains(errMsg, "unexpected frame"):
		return ErrUnexpectedFrame
	case strings.Contains(errMsg, "version mismatch"):
		return ErrVersionMismatch

	// Queue and exchange related
	case strings.Contains(errMsg, "queue") && strings.Contains(errMsg, "not found"):
		return ErrQueueNotFound
	case strings.Contains(errMsg, "exchange") && strings.Contains(errMsg, "not found"):
		return ErrExchangeNotFound
	case strings.Contains(errMsg, "queue") && strings.Contains(errMsg, "empty"):
		return ErrQueueEmpty
	case strings.Contains(errMsg, "queue") && strings.Contains(errMsg, "exists"):
		return ErrQueueExists
	case strings.Contains(errMsg, "exchange") && strings.Contains(errMsg, "exists"):
		return ErrExchangeExists

	// Message related
	case strings.Contains(errMsg, "message too large"):
		return ErrMessageTooLarge
	case strings.Contains(errMsg, "content too large"):
		return ErrMessageTooLarge
	case strings.Contains(errMsg, "invalid message"):
		return ErrInvalidMessage
	case strings.Contains(errMsg, "message nacked"):
		return ErrMessageNacked
	case strings.Contains(errMsg, "message returned"):
		return ErrMessageReturned
	case strings.Contains(errMsg, "publish failed"):
		return ErrPublishFailed
	case strings.Contains(errMsg, "no route"):
		return ErrPublishFailed
	case strings.Contains(errMsg, "no consumers"):
		return ErrPublishFailed

	// Operation related - Order matters here! More specific patterns first
	case strings.Contains(errMsg, "unbind failed"):
		return ErrUnbindFailed
	case strings.Contains(errMsg, "bind failed"):
		return ErrBindFailed
	case strings.Contains(errMsg, "declare failed"):
		return ErrDeclareFailed
	case strings.Contains(errMsg, "delete failed"):
		return ErrDeleteFailed
	case strings.Contains(errMsg, "purge failed"):
		return ErrPurgeFailed
	case strings.Contains(errMsg, "consume failed"):
		return ErrConsumeFailed
	case strings.Contains(errMsg, "nack failed"):
		return ErrNackFailed
	case strings.Contains(errMsg, "ack failed"):
		return ErrAckFailed
	case strings.Contains(errMsg, "reject failed"):
		return ErrRejectFailed
	case strings.Contains(errMsg, "qos failed"):
		return ErrQoSFailed
	case strings.Contains(errMsg, "transaction failed"):
		return ErrTransactionFailed

	// Resource related
	case strings.Contains(errMsg, "resource locked"):
		return ErrResourceLocked
	case strings.Contains(errMsg, "precondition failed"):
		return ErrPreconditionFailed
	case strings.Contains(errMsg, "invalid argument"):
		return ErrInvalidArgument
	case strings.Contains(errMsg, "not allowed"):
		return ErrNotAllowed
	case strings.Contains(errMsg, "not implemented"):
		return ErrNotImplemented

	// Server related
	case strings.Contains(errMsg, "internal error"):
		return ErrInternalError
	case strings.Contains(errMsg, "server error"):
		return ErrServerError
	case strings.Contains(errMsg, "client error"):
		return ErrClientError

	// Timeout related
	case strings.Contains(errMsg, "timeout"):
		return ErrTimeout
	case strings.Contains(errMsg, "deadline exceeded"):
		return ErrTimeout

	// Virtual host related
	case strings.Contains(errMsg, "virtual host") && strings.Contains(errMsg, "not found"):
		return ErrVirtualHostNotFound
	case strings.Contains(errMsg, "vhost") && strings.Contains(errMsg, "not found"):
		return ErrVirtualHostNotFound

	// User related
	case strings.Contains(errMsg, "user") && strings.Contains(errMsg, "not found"):
		return ErrUserNotFound

	// Alarms and limits
	case strings.Contains(errMsg, "memory alarm"):
		return ErrMemoryAlarm
	case strings.Contains(errMsg, "disk alarm"):
		return ErrDiskAlarm
	case strings.Contains(errMsg, "resource alarm"):
		return ErrResourceAlarm
	case strings.Contains(errMsg, "flow control"):
		return ErrFlowControl
	case strings.Contains(errMsg, "quota exceeded"):
		return ErrQuotaExceeded
	case strings.Contains(errMsg, "rate limit"):
		return ErrRateLimit
	case strings.Contains(errMsg, "backpressure"):
		return ErrBackpressure

	// Cluster related
	case strings.Contains(errMsg, "cluster"):
		return ErrClusterError
	case strings.Contains(errMsg, "node down"):
		return ErrNodeDown

	// Cancellation and shutdown
	case strings.Contains(errMsg, "canceled"):
		return ErrCancelled
	case strings.Contains(errMsg, "cancelled"):
		return ErrCancelled
	case strings.Contains(errMsg, "shutdown"):
		return ErrShutdown

	// Configuration related
	case strings.Contains(errMsg, "configuration"):
		return ErrConfigurationError
	case strings.Contains(errMsg, "config"):
		return ErrConfigurationError

	default:
		// Return the original error if no pattern matches
		return originalErr
	}
}

// ErrorCategory represents different categories of RabbitMQ errors
type ErrorCategory int

const (
	CategoryUnknown ErrorCategory = iota
	CategoryConnection
	CategoryChannel
	CategoryAuthentication
	CategoryPermission
	CategoryResource
	CategoryMessage
	CategoryProtocol
	CategoryNetwork
	CategoryServer
	CategoryConfiguration
	CategoryCluster
	CategoryOperation
	CategoryAlarm
	CategoryTimeout
)

// GetErrorCategory returns the category of the given error
func (r *Rabbit) GetErrorCategory(err error) ErrorCategory {
	switch {
	case errors.Is(err, ErrConnectionFailed), errors.Is(err, ErrConnectionLost), errors.Is(err, ErrConnectionClosed):
		return CategoryConnection
	case errors.Is(err, ErrChannelClosed), errors.Is(err, ErrChannelError), errors.Is(err, ErrChannelException):
		return CategoryChannel
	case errors.Is(err, ErrAuthenticationFailed), errors.Is(err, ErrInvalidCredentials):
		return CategoryAuthentication
	case errors.Is(err, ErrAccessDenied), errors.Is(err, ErrInsufficientPermissions):
		return CategoryPermission
	case errors.Is(err, ErrExchangeNotFound), errors.Is(err, ErrQueueNotFound), errors.Is(err, ErrQueueEmpty), errors.Is(err, ErrQueueExists), errors.Is(err, ErrExchangeExists), errors.Is(err, ErrResourceLocked):
		return CategoryResource
	case errors.Is(err, ErrMessageTooLarge), errors.Is(err, ErrInvalidMessage), errors.Is(err, ErrMessageNacked), errors.Is(err, ErrMessageReturned), errors.Is(err, ErrPublishFailed):
		return CategoryMessage
	case errors.Is(err, ErrFrameError), errors.Is(err, ErrSyntaxError), errors.Is(err, ErrCommandInvalid), errors.Is(err, ErrUnexpectedFrame), errors.Is(err, ErrProtocolError), errors.Is(err, ErrVersionMismatch):
		return CategoryProtocol
	case errors.Is(err, ErrNetworkError), errors.Is(err, ErrTLSError), errors.Is(err, ErrCertificateError), errors.Is(err, ErrHandshakeFailed):
		return CategoryNetwork
	case errors.Is(err, ErrInternalError), errors.Is(err, ErrServerError), errors.Is(err, ErrNotImplemented):
		return CategoryServer
	case errors.Is(err, ErrConfigurationError), errors.Is(err, ErrVirtualHostNotFound), errors.Is(err, ErrUserNotFound):
		return CategoryConfiguration
	case errors.Is(err, ErrClusterError), errors.Is(err, ErrNodeDown):
		return CategoryCluster
	case errors.Is(err, ErrBindFailed), errors.Is(err, ErrUnbindFailed), errors.Is(err, ErrDeclareFailed), errors.Is(err, ErrDeleteFailed), errors.Is(err, ErrPurgeFailed), errors.Is(err, ErrConsumeFailed), errors.Is(err, ErrAckFailed), errors.Is(err, ErrNackFailed), errors.Is(err, ErrRejectFailed), errors.Is(err, ErrQoSFailed), errors.Is(err, ErrTransactionFailed):
		return CategoryOperation
	case errors.Is(err, ErrMemoryAlarm), errors.Is(err, ErrDiskAlarm), errors.Is(err, ErrResourceAlarm), errors.Is(err, ErrFlowControl), errors.Is(err, ErrQuotaExceeded), errors.Is(err, ErrRateLimit), errors.Is(err, ErrBackpressure):
		return CategoryAlarm
	case errors.Is(err, ErrTimeout):
		return CategoryTimeout
	default:
		return CategoryUnknown
	}
}

// IsRetryableError returns true if the error is retryable
func (r *Rabbit) IsRetryableError(err error) bool {
	switch {
	case errors.Is(err, ErrConnectionFailed),
		errors.Is(err, ErrConnectionLost),
		errors.Is(err, ErrChannelClosed),
		errors.Is(err, ErrChannelError),
		errors.Is(err, ErrTimeout),
		errors.Is(err, ErrNetworkError),
		errors.Is(err, ErrServerError),
		errors.Is(err, ErrInternalError),
		errors.Is(err, ErrTLSError),
		errors.Is(err, ErrHandshakeFailed),
		errors.Is(err, ErrMemoryAlarm),
		errors.Is(err, ErrDiskAlarm),
		errors.Is(err, ErrResourceAlarm),
		errors.Is(err, ErrFlowControl),
		errors.Is(err, ErrRateLimit),
		errors.Is(err, ErrBackpressure),
		errors.Is(err, ErrClusterError),
		errors.Is(err, ErrNodeDown):
		return true
	default:
		return false
	}
}

// IsTemporaryError returns true if the error is temporary
func (r *Rabbit) IsTemporaryError(err error) bool {
	return r.IsRetryableError(err) ||
		errors.Is(err, ErrResourceLocked) ||
		errors.Is(err, ErrQuotaExceeded) ||
		errors.Is(err, ErrMessageReturned) ||
		errors.Is(err, ErrMessageNacked)
}

// IsPermanentError returns true if the error is permanent and should not be retried
func (r *Rabbit) IsPermanentError(err error) bool {
	switch {
	case errors.Is(err, ErrAuthenticationFailed),
		errors.Is(err, ErrInvalidCredentials),
		errors.Is(err, ErrAccessDenied),
		errors.Is(err, ErrInsufficientPermissions),
		errors.Is(err, ErrExchangeNotFound),
		errors.Is(err, ErrQueueNotFound),
		errors.Is(err, ErrVirtualHostNotFound),
		errors.Is(err, ErrUserNotFound),
		errors.Is(err, ErrInvalidArgument),
		errors.Is(err, ErrSyntaxError),
		errors.Is(err, ErrCommandInvalid),
		errors.Is(err, ErrNotAllowed),
		errors.Is(err, ErrNotImplemented),
		errors.Is(err, ErrVersionMismatch),
		errors.Is(err, ErrCertificateError),
		errors.Is(err, ErrConfigurationError),
		errors.Is(err, ErrCancelled),
		errors.Is(err, ErrShutdown):
		return true
	default:
		return false
	}
}

// IsConnectionError returns true if the error is connection-related
func (r *Rabbit) IsConnectionError(err error) bool {
	switch {
	case errors.Is(err, ErrConnectionFailed),
		errors.Is(err, ErrConnectionLost),
		errors.Is(err, ErrConnectionClosed):
		return true
	default:
		return false
	}
}

// IsChannelError returns true if the error is channel-related
func (r *Rabbit) IsChannelError(err error) bool {
	switch {
	case errors.Is(err, ErrChannelClosed),
		errors.Is(err, ErrChannelError),
		errors.Is(err, ErrChannelException):
		return true
	default:
		return false
	}
}

// IsAuthenticationError returns true if the error is authentication-related
func (r *Rabbit) IsAuthenticationError(err error) bool {
	switch {
	case errors.Is(err, ErrAuthenticationFailed),
		errors.Is(err, ErrInvalidCredentials),
		errors.Is(err, ErrAccessDenied),
		errors.Is(err, ErrInsufficientPermissions):
		return true
	default:
		return false
	}
}

// IsResourceError returns true if the error is resource-related
func (r *Rabbit) IsResourceError(err error) bool {
	switch {
	case errors.Is(err, ErrExchangeNotFound),
		errors.Is(err, ErrQueueNotFound),
		errors.Is(err, ErrQueueEmpty),
		errors.Is(err, ErrQueueExists),
		errors.Is(err, ErrExchangeExists),
		errors.Is(err, ErrResourceLocked),
		errors.Is(err, ErrResourceError):
		return true
	default:
		return false
	}
}

// IsAlarmError returns true if the error is alarm-related
func (r *Rabbit) IsAlarmError(err error) bool {
	switch {
	case errors.Is(err, ErrMemoryAlarm),
		errors.Is(err, ErrDiskAlarm),
		errors.Is(err, ErrResourceAlarm),
		errors.Is(err, ErrFlowControl),
		errors.Is(err, ErrQuotaExceeded),
		errors.Is(err, ErrRateLimit),
		errors.Is(err, ErrBackpressure):
		return true
	default:
		return false
	}
}
