package kafka

import (
	"errors"
	"strings"
)

// Common Kafka error types that can be used by consumers of this package.
// These provide a standardized set of errors that abstract away the
// underlying Kafka-specific error details.
var (
	// ErrConnectionFailed is returned when connection to Kafka cannot be established
	ErrConnectionFailed = errors.New("connection failed")

	// ErrConnectionLost is returned when connection to Kafka is lost
	ErrConnectionLost = errors.New("connection lost")

	// ErrBrokerNotAvailable is returned when broker is not available
	ErrBrokerNotAvailable = errors.New("broker not available")

	// ErrReplicaNotAvailable is returned when replica is not available
	ErrReplicaNotAvailable = errors.New("replica not available")

	// ErrAuthenticationFailed is returned when authentication fails
	ErrAuthenticationFailed = errors.New("authentication failed")

	// ErrAuthorizationFailed is returned when authorization fails
	ErrAuthorizationFailed = errors.New("authorization failed")

	// ErrInvalidCredentials is returned when credentials are invalid
	ErrInvalidCredentials = errors.New("invalid credentials")

	// ErrTopicNotFound is returned when topic doesn't exist
	ErrTopicNotFound = errors.New("topic not found")

	// ErrTopicAlreadyExists is returned when topic already exists
	ErrTopicAlreadyExists = errors.New("topic already exists")

	// ErrPartitionNotFound is returned when partition doesn't exist
	ErrPartitionNotFound = errors.New("partition not found")

	// ErrInvalidPartition is returned when partition is invalid
	ErrInvalidPartition = errors.New("invalid partition")

	// ErrGroupNotFound is returned when consumer group doesn't exist
	ErrGroupNotFound = errors.New("consumer group not found")

	// ErrGroupCoordinatorNotAvailable is returned when group coordinator is not available
	ErrGroupCoordinatorNotAvailable = errors.New("group coordinator not available")

	// ErrNotGroupCoordinator is returned when broker is not the group coordinator
	ErrNotGroupCoordinator = errors.New("not group coordinator")

	// ErrInvalidGroupID is returned when group ID is invalid
	ErrInvalidGroupID = errors.New("invalid group id")

	// ErrUnknownMemberID is returned when member ID is unknown
	ErrUnknownMemberID = errors.New("unknown member id")

	// ErrInvalidSessionTimeout is returned when session timeout is invalid
	ErrInvalidSessionTimeout = errors.New("invalid session timeout")

	// ErrRebalanceInProgress is returned when rebalance is in progress
	ErrRebalanceInProgress = errors.New("rebalance in progress")

	// ErrInvalidCommitOffset is returned when commit offset is invalid
	ErrInvalidCommitOffset = errors.New("invalid commit offset")

	// ErrMessageTooLarge is returned when message exceeds size limits
	ErrMessageTooLarge = errors.New("message too large")

	// ErrInvalidMessage is returned when message format is invalid
	ErrInvalidMessage = errors.New("invalid message")

	// ErrInvalidMessageSize is returned when message size is invalid
	ErrInvalidMessageSize = errors.New("invalid message size")

	// ErrOffsetOutOfRange is returned when offset is out of range
	ErrOffsetOutOfRange = errors.New("offset out of range")

	// ErrInvalidFetchSize is returned when fetch size is invalid
	ErrInvalidFetchSize = errors.New("invalid fetch size")

	// ErrLeaderNotAvailable is returned when leader is not available
	ErrLeaderNotAvailable = errors.New("leader not available")

	// ErrNotLeaderForPartition is returned when broker is not the leader for partition
	ErrNotLeaderForPartition = errors.New("not leader for partition")

	// ErrRequestTimedOut is returned when request times out
	ErrRequestTimedOut = errors.New("request timed out")

	// ErrNetworkError is returned for network-related errors
	ErrNetworkError = errors.New("network error")

	// ErrProducerFenced is returned when producer is fenced
	ErrProducerFenced = errors.New("producer fenced")

	// ErrTransactionCoordinatorFenced is returned when transaction coordinator is fenced
	ErrTransactionCoordinatorFenced = errors.New("transaction coordinator fenced")

	// ErrInvalidTransactionState is returned when transaction state is invalid
	ErrInvalidTransactionState = errors.New("invalid transaction state")

	// ErrInvalidProducerEpoch is returned when producer epoch is invalid
	ErrInvalidProducerEpoch = errors.New("invalid producer epoch")

	// ErrInvalidTxnState is returned when transaction state is invalid
	ErrInvalidTxnState = errors.New("invalid transaction state")

	// ErrInvalidProducerIDMapping is returned when producer ID mapping is invalid
	ErrInvalidProducerIDMapping = errors.New("invalid producer id mapping")

	// ErrInvalidTransaction is returned when transaction is invalid
	ErrInvalidTransaction = errors.New("invalid transaction")

	// ErrUnsupportedVersion is returned when version is not supported
	ErrUnsupportedVersion = errors.New("unsupported version")

	// ErrUnsupportedForMessageFormat is returned when message format is not supported
	ErrUnsupportedForMessageFormat = errors.New("unsupported for message format")

	// ErrInvalidRequest is returned when request is invalid
	ErrInvalidRequest = errors.New("invalid request")

	// ErrInvalidConfig is returned when configuration is invalid
	ErrInvalidConfig = errors.New("invalid config")

	// ErrNotController is returned when broker is not the controller
	ErrNotController = errors.New("not controller")

	// ErrInvalidReplicationFactor is returned when replication factor is invalid
	ErrInvalidReplicationFactor = errors.New("invalid replication factor")

	// ErrInvalidReplica is returned when replica is invalid
	ErrInvalidReplica = errors.New("invalid replica")

	// ErrReplicaNotAvailableError is returned when replica is not available
	ErrReplicaNotAvailableError = errors.New("replica not available")

	// ErrPolicyViolation is returned when policy is violated
	ErrPolicyViolation = errors.New("policy violation")

	// ErrOutOfOrderSequence is returned when sequence is out of order
	ErrOutOfOrderSequence = errors.New("out of order sequence")

	// ErrDuplicateSequence is returned when sequence is duplicated
	ErrDuplicateSequence = errors.New("duplicate sequence")

	// ErrTooManyInFlightRequests is returned when there are too many in-flight requests
	ErrTooManyInFlightRequests = errors.New("too many in-flight requests")

	// ErrWriterNotInitialized is returned when writer is not initialized
	ErrWriterNotInitialized = errors.New("writer not initialized")

	// ErrReaderNotInitialized is returned when reader is not initialized
	ErrReaderNotInitialized = errors.New("reader not initialized")

	// ErrContextCanceled is returned when context is canceled
	ErrContextCanceled = errors.New("context canceled")

	// ErrContextDeadlineExceeded is returned when context deadline is exceeded
	ErrContextDeadlineExceeded = errors.New("context deadline exceeded")

	// ErrUnknownError is returned for unknown/unhandled errors
	ErrUnknownError = errors.New("unknown error")
)

// TranslateError converts Kafka-specific errors into standardized application errors.
// This function provides abstraction from the underlying Kafka implementation details,
// allowing application code to handle errors in a Kafka-agnostic way.
//
// It maps common Kafka errors to the standardized error types defined above.
// If an error doesn't match any known type, it's returned unchanged.
func (k *KafkaClient) TranslateError(err error) error {
	if err == nil {
		return nil
	}

	// Check error message for common patterns
	errMsg := strings.ToLower(err.Error())
	return translateByErrorMessage(errMsg, err)
}

// translateByErrorMessage translates errors based on error message patterns
func translateByErrorMessage(errMsg string, originalErr error) error {
	switch {
	// Connection related
	case strings.Contains(errMsg, "connection refused"):
		return ErrConnectionFailed
	case strings.Contains(errMsg, "connection reset"):
		return ErrConnectionLost
	case strings.Contains(errMsg, "connection closed"):
		return ErrConnectionLost
	case strings.Contains(errMsg, "broker not available"):
		return ErrBrokerNotAvailable
	case strings.Contains(errMsg, "replica not available"):
		return ErrReplicaNotAvailable

	// Authentication and authorization
	case strings.Contains(errMsg, "authentication failed"):
		return ErrAuthenticationFailed
	case strings.Contains(errMsg, "sasl authentication failed"):
		return ErrAuthenticationFailed
	case strings.Contains(errMsg, "authorization failed"):
		return ErrAuthorizationFailed
	case strings.Contains(errMsg, "invalid credentials"):
		return ErrInvalidCredentials

	// Topic and partition errors
	case strings.Contains(errMsg, "topic not found"):
		return ErrTopicNotFound
	case strings.Contains(errMsg, "unknown topic"):
		return ErrTopicNotFound
	case strings.Contains(errMsg, "topic already exists"):
		return ErrTopicAlreadyExists
	case strings.Contains(errMsg, "partition not found"):
		return ErrPartitionNotFound
	case strings.Contains(errMsg, "unknown partition"):
		return ErrPartitionNotFound
	case strings.Contains(errMsg, "invalid partition"):
		return ErrInvalidPartition

	// Consumer group errors
	case strings.Contains(errMsg, "group coordinator not available"):
		return ErrGroupCoordinatorNotAvailable
	case strings.Contains(errMsg, "not coordinator for group"):
		return ErrNotGroupCoordinator
	case strings.Contains(errMsg, "not group coordinator"):
		return ErrNotGroupCoordinator
	case strings.Contains(errMsg, "invalid group id"):
		return ErrInvalidGroupID
	case strings.Contains(errMsg, "unknown member id"):
		return ErrUnknownMemberID
	case strings.Contains(errMsg, "invalid session timeout"):
		return ErrInvalidSessionTimeout
	case strings.Contains(errMsg, "rebalance in progress"):
		return ErrRebalanceInProgress

	// Offset errors
	case strings.Contains(errMsg, "offset out of range"):
		return ErrOffsetOutOfRange
	case strings.Contains(errMsg, "invalid commit offset"):
		return ErrInvalidCommitOffset

	// Message errors
	case strings.Contains(errMsg, "message too large"):
		return ErrMessageTooLarge
	case strings.Contains(errMsg, "record too large"):
		return ErrMessageTooLarge
	case strings.Contains(errMsg, "invalid message"):
		return ErrInvalidMessage
	case strings.Contains(errMsg, "invalid message size"):
		return ErrInvalidMessageSize

	// Leader errors
	case strings.Contains(errMsg, "leader not available"):
		return ErrLeaderNotAvailable
	case strings.Contains(errMsg, "not leader for partition"):
		return ErrNotLeaderForPartition

	// Timeout errors
	case strings.Contains(errMsg, "request timed out"):
		return ErrRequestTimedOut
	case strings.Contains(errMsg, "timeout"):
		return ErrRequestTimedOut
	case strings.Contains(errMsg, "deadline exceeded"):
		return ErrContextDeadlineExceeded

	// Network errors
	case strings.Contains(errMsg, "network"):
		return ErrNetworkError
	case strings.Contains(errMsg, "dial"):
		return ErrNetworkError
	case strings.Contains(errMsg, "i/o timeout"):
		return ErrNetworkError

	// Producer errors
	case strings.Contains(errMsg, "producer fenced"):
		return ErrProducerFenced
	case strings.Contains(errMsg, "invalid producer epoch"):
		return ErrInvalidProducerEpoch

	// Transaction errors
	case strings.Contains(errMsg, "transaction coordinator fenced"):
		return ErrTransactionCoordinatorFenced
	case strings.Contains(errMsg, "invalid transaction state"):
		return ErrInvalidTransactionState
	case strings.Contains(errMsg, "invalid txn state"):
		return ErrInvalidTxnState

	// Version errors
	case strings.Contains(errMsg, "unsupported version"):
		return ErrUnsupportedVersion
	case strings.Contains(errMsg, "unsupported for message format"):
		return ErrUnsupportedForMessageFormat

	// Configuration errors
	case strings.Contains(errMsg, "invalid request"):
		return ErrInvalidRequest
	case strings.Contains(errMsg, "invalid config"):
		return ErrInvalidConfig
	case strings.Contains(errMsg, "invalid replication factor"):
		return ErrInvalidReplicationFactor

	// Sequence errors
	case strings.Contains(errMsg, "out of order sequence"):
		return ErrOutOfOrderSequence
	case strings.Contains(errMsg, "duplicate sequence"):
		return ErrDuplicateSequence

	// Context errors
	case strings.Contains(errMsg, "context canceled"):
		return ErrContextCanceled
	case strings.Contains(errMsg, "context cancelled"):
		return ErrContextCanceled

	default:
		// Return the original error if no pattern matches
		return originalErr
	}
}

// IsRetryableError returns true if the error is retryable
func (k *KafkaClient) IsRetryableError(err error) bool {
	switch {
	case errors.Is(err, ErrConnectionFailed),
		errors.Is(err, ErrConnectionLost),
		errors.Is(err, ErrBrokerNotAvailable),
		errors.Is(err, ErrReplicaNotAvailable),
		errors.Is(err, ErrLeaderNotAvailable),
		errors.Is(err, ErrNotLeaderForPartition),
		errors.Is(err, ErrRequestTimedOut),
		errors.Is(err, ErrNetworkError),
		errors.Is(err, ErrGroupCoordinatorNotAvailable),
		errors.Is(err, ErrNotGroupCoordinator),
		errors.Is(err, ErrRebalanceInProgress):
		return true
	default:
		return false
	}
}

// IsTemporaryError returns true if the error is temporary
func (k *KafkaClient) IsTemporaryError(err error) bool {
	return k.IsRetryableError(err) ||
		errors.Is(err, ErrRebalanceInProgress) ||
		errors.Is(err, ErrLeaderNotAvailable)
}

// IsPermanentError returns true if the error is permanent and should not be retried
func (k *KafkaClient) IsPermanentError(err error) bool {
	switch {
	case errors.Is(err, ErrAuthenticationFailed),
		errors.Is(err, ErrAuthorizationFailed),
		errors.Is(err, ErrInvalidCredentials),
		errors.Is(err, ErrTopicNotFound),
		errors.Is(err, ErrTopicAlreadyExists),
		errors.Is(err, ErrPartitionNotFound),
		errors.Is(err, ErrInvalidPartition),
		errors.Is(err, ErrInvalidGroupID),
		errors.Is(err, ErrInvalidMessage),
		errors.Is(err, ErrInvalidRequest),
		errors.Is(err, ErrInvalidConfig),
		errors.Is(err, ErrUnsupportedVersion),
		errors.Is(err, ErrContextCanceled):
		return true
	default:
		return false
	}
}

// IsAuthenticationError returns true if the error is authentication-related
func (k *KafkaClient) IsAuthenticationError(err error) bool {
	switch {
	case errors.Is(err, ErrAuthenticationFailed),
		errors.Is(err, ErrAuthorizationFailed),
		errors.Is(err, ErrInvalidCredentials):
		return true
	default:
		return false
	}
}
