package minio

import (
	"errors"
	"net/url"
	"strings"

	"github.com/minio/minio-go/v7"
)

// Common object storage error types that can be used by consumers of this package.
// These provide a standardized set of errors that abstract away the
// underlying MinIO-specific error details.
var (
	// ErrObjectNotFound is returned when an object doesn't exist in the bucket
	ErrObjectNotFound = errors.New("object not found")

	// ErrBucketNotFound is returned when a bucket doesn't exist
	ErrBucketNotFound = errors.New("bucket not found")

	// ErrBucketAlreadyExists is returned when trying to create a bucket that already exists
	ErrBucketAlreadyExists = errors.New("bucket already exists")

	// ErrInvalidBucketName is returned when bucket name is invalid
	ErrInvalidBucketName = errors.New("invalid bucket name")

	// ErrInvalidObjectName is returned when object name is invalid
	ErrInvalidObjectName = errors.New("invalid object name")

	// ErrAccessDenied is returned when access is denied to bucket or object
	ErrAccessDenied = errors.New("access denied")

	// ErrInsufficientPermissions is returned when user lacks necessary permissions
	ErrInsufficientPermissions = errors.New("insufficient permissions")

	// ErrInvalidCredentials is returned when credentials are invalid
	ErrInvalidCredentials = errors.New("invalid credentials")

	// ErrCredentialsExpired is returned when credentials have expired
	ErrCredentialsExpired = errors.New("credentials expired")

	// ErrConnectionFailed is returned when connection to MinIO server fails
	ErrConnectionFailed = errors.New("connection failed")

	// ErrConnectionLost is returned when connection to MinIO server is lost
	ErrConnectionLost = errors.New("connection lost")

	// ErrTimeout is returned when operation times out
	ErrTimeout = errors.New("operation timeout")

	// ErrInvalidRange is returned when byte range is invalid
	ErrInvalidRange = errors.New("invalid range")

	// ErrObjectTooLarge is returned when object exceeds size limits
	ErrObjectTooLarge = errors.New("object too large")

	// ErrInvalidChecksum is returned when object checksum doesn't match
	ErrInvalidChecksum = errors.New("invalid checksum")

	// ErrInvalidMetadata is returned when object metadata is invalid
	ErrInvalidMetadata = errors.New("invalid metadata")

	// ErrInvalidContentType is returned when content type is invalid
	ErrInvalidContentType = errors.New("invalid content type")

	// ErrInvalidEncryption is returned when encryption parameters are invalid
	ErrInvalidEncryption = errors.New("invalid encryption")

	// ErrServerError is returned for internal MinIO server errors
	ErrServerError = errors.New("server error")

	// ErrServiceUnavailable is returned when MinIO service is unavailable
	ErrServiceUnavailable = errors.New("service unavailable")

	// ErrTooManyRequests is returned when rate limit is exceeded
	ErrTooManyRequests = errors.New("too many requests")

	// ErrInvalidResponse is returned when server response is invalid
	ErrInvalidResponse = errors.New("invalid response")

	// ErrNetworkError is returned for network-related errors
	ErrNetworkError = errors.New("network error")

	// ErrInvalidURL is returned when URL is malformed
	ErrInvalidURL = errors.New("invalid URL")

	// ErrQuotaExceeded is returned when storage quota is exceeded
	ErrQuotaExceeded = errors.New("quota exceeded")

	// ErrBucketNotEmpty is returned when trying to delete non-empty bucket
	ErrBucketNotEmpty = errors.New("bucket not empty")

	// ErrPreconditionFailed is returned when precondition check fails
	ErrPreconditionFailed = errors.New("precondition failed")

	// ErrInvalidPart is returned when multipart upload part is invalid
	ErrInvalidPart = errors.New("invalid part")

	// ErrInvalidUploadID is returned when multipart upload ID is invalid
	ErrInvalidUploadID = errors.New("invalid upload ID")

	// ErrPartTooSmall is returned when multipart upload part is too small
	ErrPartTooSmall = errors.New("part too small")

	// ErrNotImplemented is returned for unsupported operations
	ErrNotImplemented = errors.New("operation not implemented")

	// ErrInvalidArgument is returned when function arguments are invalid
	ErrInvalidArgument = errors.New("invalid argument")

	// ErrCancelled is returned when operation is cancelled
	ErrCancelled = errors.New("operation cancelled")

	// ErrConfigurationError is returned for configuration-related errors
	ErrConfigurationError = errors.New("configuration error")

	// ErrVersioningNotEnabled is returned when versioning is required but not enabled
	ErrVersioningNotEnabled = errors.New("versioning not enabled")

	// ErrInvalidVersion is returned when object version is invalid
	ErrInvalidVersion = errors.New("invalid version")

	// ErrLockConfigurationError is returned for object lock configuration errors
	ErrLockConfigurationError = errors.New("lock configuration error")

	// ErrObjectLocked is returned when object is locked and cannot be deleted
	ErrObjectLocked = errors.New("object locked")

	// ErrRetentionPolicyError is returned for retention policy errors
	ErrRetentionPolicyError = errors.New("retention policy error")

	// ErrMalformedXML is returned when XML is malformed
	ErrMalformedXML = errors.New("malformed XML")

	// ErrInvalidStorageClass is returned when storage class is invalid
	ErrInvalidStorageClass = errors.New("invalid storage class")

	// ErrReplicationError is returned for replication-related errors
	ErrReplicationError = errors.New("replication error")

	// ErrLifecycleError is returned for lifecycle configuration errors
	ErrLifecycleError = errors.New("lifecycle error")

	// ErrNotificationError is returned for notification configuration errors
	ErrNotificationError = errors.New("notification error")

	// ErrTaggingError is returned for object tagging errors
	ErrTaggingError = errors.New("tagging error")

	// ErrPolicyError is returned for bucket policy errors
	ErrPolicyError = errors.New("policy error")

	// ErrWebsiteError is returned for website configuration errors
	ErrWebsiteError = errors.New("website configuration error")

	// ErrCORSError is returned for CORS configuration errors
	ErrCORSError = errors.New("CORS configuration error")

	// ErrLoggingError is returned for logging configuration errors
	ErrLoggingError = errors.New("logging configuration error")

	// ErrEncryptionError is returned for encryption configuration errors
	ErrEncryptionError = errors.New("encryption configuration error")

	// ErrUnknownError is returned for unknown/unhandled errors
	ErrUnknownError = errors.New("unknown error")
)

// TranslateError converts MinIO-specific errors into standardized application errors.
// This function provides abstraction from the underlying MinIO implementation details,
// allowing application code to handle errors in a MinIO-agnostic way.
//
// It maps common MinIO errors to the standardized error types defined above.
// If an error doesn't match any known type, it's returned unchanged.
func (m *MinioClient) TranslateError(err error) error {
	if err == nil {
		return nil
	}

	// Check for MinIO specific errors
	var minioErr minio.ErrorResponse
	if errors.As(err, &minioErr) {
		return m.translateMinIOError(minioErr)
	}

	// Check for URL errors
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return m.translateURLError(urlErr)
	}

	// Check error message for common patterns (fallback for string matching)
	errMsg := strings.ToLower(err.Error())
	return m.translateByErrorMessage(errMsg, err)
}

// translateMinIOError maps MinIO error responses to custom errors
func (m *MinioClient) translateMinIOError(minioErr minio.ErrorResponse) error {
	switch minioErr.Code {
	// 4xx Client Errors
	case "NoSuchBucket":
		return ErrBucketNotFound
	case "NoSuchKey":
		return ErrObjectNotFound
	case "BucketAlreadyExists":
		return ErrBucketAlreadyExists
	case "BucketAlreadyOwnedByYou":
		return ErrBucketAlreadyExists
	case "InvalidBucketName":
		return ErrInvalidBucketName
	case "InvalidObjectName":
		return ErrInvalidObjectName
	case "AccessDenied":
		return ErrAccessDenied
	case "InvalidAccessKeyId":
		return ErrInvalidCredentials
	case "SignatureDoesNotMatch":
		return ErrInvalidCredentials
	case "TokenRefreshRequired":
		return ErrCredentialsExpired
	case "ExpiredToken":
		return ErrCredentialsExpired
	case "InvalidRange":
		return ErrInvalidRange
	case "EntityTooLarge":
		return ErrObjectTooLarge
	case "InvalidDigest":
		return ErrInvalidChecksum
	case "BadDigest":
		return ErrInvalidChecksum
	case "InvalidArgument":
		return ErrInvalidArgument
	case "MalformedXML":
		return ErrMalformedXML
	case "InvalidStorageClass":
		return ErrInvalidStorageClass
	case "InvalidPart":
		return ErrInvalidPart
	case "InvalidPartOrder":
		return ErrInvalidPart
	case "NoSuchUpload":
		return ErrInvalidUploadID
	case "PreconditionFailed":
		return ErrPreconditionFailed
	case "BucketNotEmpty":
		return ErrBucketNotEmpty
	case "TooManyBuckets":
		return ErrQuotaExceeded
	case "InvalidRequest":
		return ErrInvalidArgument
	case "MetadataTooLarge":
		return ErrInvalidMetadata
	case "InvalidEncryptionAlgorithm":
		return ErrInvalidEncryption
	case "InvalidObjectState":
		return ErrObjectLocked
	case "ObjectLockConfigurationNotFoundError":
		return ErrLockConfigurationError
	case "InvalidRetentionDate":
		return ErrRetentionPolicyError
	case "InvalidVersionId":
		return ErrInvalidVersion
	case "NoSuchVersion":
		return ErrInvalidVersion
	case "InvalidLocationConstraint":
		return ErrConfigurationError
	case "CrossLocationLoggingProhibited":
		return ErrLoggingError
	case "InvalidTargetBucketForLogging":
		return ErrLoggingError
	case "InvalidPolicyDocument":
		return ErrPolicyError
	case "MalformedPolicy":
		return ErrPolicyError
	case "InvalidWebsiteConfiguration":
		return ErrWebsiteError
	case "InvalidCORSConfiguration":
		return ErrCORSError
	case "InvalidNotificationConfiguration":
		return ErrNotificationError
	case "InvalidLifecycleConfiguration":
		return ErrLifecycleError
	case "InvalidReplicationConfiguration":
		return ErrReplicationError
	case "ReplicationConfigurationNotFoundError":
		return ErrReplicationError
	case "InvalidTagError":
		return ErrTaggingError
	case "BadRequest":
		return ErrInvalidArgument
	case "RequestTimeout":
		return ErrTimeout
	case "EntityTooSmall":
		return ErrPartTooSmall
	case "IncompleteBody":
		return ErrInvalidArgument
	case "InvalidContentType":
		return ErrInvalidContentType
	case "KeyTooLong":
		return ErrInvalidObjectName
	case "MissingContentLength":
		return ErrInvalidArgument
	case "MissingRequestBodyError":
		return ErrInvalidArgument
	case "RequestTimeTooSkewed":
		return ErrInvalidArgument
	case "SlowDown":
		return ErrTooManyRequests
	case "TemporaryRedirect":
		return ErrNetworkError
	case "RequestHeaderSectionTooLarge":
		return ErrInvalidArgument
	case "TooManyRequests":
		return ErrTooManyRequests

	// 5xx Server Errors
	case "InternalError":
		return ErrServerError
	case "ServiceUnavailable":
		return ErrServiceUnavailable
	case "NotImplemented":
		return ErrNotImplemented
	case "BandwidthLimitExceeded":
		return ErrTooManyRequests
	case "ReducedRedundancyLostFraction":
		return ErrServerError
	case "InsufficientStorage":
		return ErrQuotaExceeded
	case "XNotImplemented":
		return ErrNotImplemented

	default:
		// For unknown MinIO error codes, check status code
		switch minioErr.StatusCode {
		case 400:
			return ErrInvalidArgument
		case 401:
			return ErrInvalidCredentials
		case 403:
			return ErrAccessDenied
		case 404:
			return ErrObjectNotFound
		case 409:
			return ErrBucketAlreadyExists
		case 412:
			return ErrPreconditionFailed
		case 413:
			return ErrObjectTooLarge
		case 416:
			return ErrInvalidRange
		case 429:
			return ErrTooManyRequests
		case 500:
			return ErrServerError
		case 501:
			return ErrNotImplemented
		case 503:
			return ErrServiceUnavailable
		default:
			return ErrUnknownError
		}
	}
}

// translateURLError maps URL errors to custom errors
func (m *MinioClient) translateURLError(urlErr *url.Error) error {
	switch urlErr.Op {
	case "dial":
		return ErrConnectionFailed
	case "read":
		return ErrConnectionLost
	case "write":
		return ErrConnectionLost
	default:
		if strings.Contains(urlErr.Error(), "timeout") {
			return ErrTimeout
		}
		if strings.Contains(urlErr.Error(), "connection refused") {
			return ErrConnectionFailed
		}
		if strings.Contains(urlErr.Error(), "connection reset") {
			return ErrConnectionLost
		}
		return ErrNetworkError
	}
}

// translateByErrorMessage translates errors based on error message patterns (fallback)
func (m *MinioClient) translateByErrorMessage(errMsg string, originalErr error) error {
	switch {
	// Connection related
	case strings.Contains(errMsg, "connection refused"):
		return ErrConnectionFailed
	case strings.Contains(errMsg, "connection reset"):
		return ErrConnectionLost
	case strings.Contains(errMsg, "connection timeout"):
		return ErrTimeout
	case strings.Contains(errMsg, "connection failed"):
		return ErrConnectionFailed
	case strings.Contains(errMsg, "network is unreachable"):
		return ErrNetworkError
	case strings.Contains(errMsg, "no route to host"):
		return ErrNetworkError
	case strings.Contains(errMsg, "host is down"):
		return ErrNetworkError

	// Timeout related
	case strings.Contains(errMsg, "timeout"):
		return ErrTimeout
	case strings.Contains(errMsg, "deadline exceeded"):
		return ErrTimeout
	case strings.Contains(errMsg, "request timeout"):
		return ErrTimeout
	case strings.Contains(errMsg, "client timeout"):
		return ErrTimeout

	// Object/Bucket related
	case strings.Contains(errMsg, "bucket does not exist"):
		return ErrBucketNotFound
	case strings.Contains(errMsg, "bucket not found"):
		return ErrBucketNotFound
	case strings.Contains(errMsg, "key does not exist"):
		return ErrObjectNotFound
	case strings.Contains(errMsg, "object not found"):
		return ErrObjectNotFound
	case strings.Contains(errMsg, "no such bucket"):
		return ErrBucketNotFound
	case strings.Contains(errMsg, "no such key"):
		return ErrObjectNotFound
	case strings.Contains(errMsg, "bucket already exists"):
		return ErrBucketAlreadyExists
	case strings.Contains(errMsg, "bucket not empty"):
		return ErrBucketNotEmpty

	// Access/Permission related
	case strings.Contains(errMsg, "access denied"):
		return ErrAccessDenied
	case strings.Contains(errMsg, "forbidden"):
		return ErrAccessDenied
	case strings.Contains(errMsg, "unauthorized"):
		return ErrInvalidCredentials
	case strings.Contains(errMsg, "invalid credentials"):
		return ErrInvalidCredentials
	case strings.Contains(errMsg, "signature mismatch"):
		return ErrInvalidCredentials
	case strings.Contains(errMsg, "invalid access key"):
		return ErrInvalidCredentials
	case strings.Contains(errMsg, "expired token"):
		return ErrCredentialsExpired
	case strings.Contains(errMsg, "token expired"):
		return ErrCredentialsExpired

	// Data/Content related
	case strings.Contains(errMsg, "invalid checksum"):
		return ErrInvalidChecksum
	case strings.Contains(errMsg, "bad digest"):
		return ErrInvalidChecksum
	case strings.Contains(errMsg, "entity too large"):
		return ErrObjectTooLarge
	case strings.Contains(errMsg, "file too large"):
		return ErrObjectTooLarge
	case strings.Contains(errMsg, "invalid range"):
		return ErrInvalidRange
	case strings.Contains(errMsg, "invalid part"):
		return ErrInvalidPart
	case strings.Contains(errMsg, "part too small"):
		return ErrPartTooSmall
	case strings.Contains(errMsg, "invalid upload id"):
		return ErrInvalidUploadID
	case strings.Contains(errMsg, "malformed xml"):
		return ErrMalformedXML
	case strings.Contains(errMsg, "invalid xml"):
		return ErrMalformedXML

	// Server related
	case strings.Contains(errMsg, "internal server error"):
		return ErrServerError
	case strings.Contains(errMsg, "service unavailable"):
		return ErrServiceUnavailable
	case strings.Contains(errMsg, "server error"):
		return ErrServerError
	case strings.Contains(errMsg, "bad gateway"):
		return ErrServerError
	case strings.Contains(errMsg, "gateway timeout"):
		return ErrTimeout

	// Rate limiting
	case strings.Contains(errMsg, "too many requests"):
		return ErrTooManyRequests
	case strings.Contains(errMsg, "rate limit"):
		return ErrTooManyRequests
	case strings.Contains(errMsg, "slow down"):
		return ErrTooManyRequests

	// Configuration related
	case strings.Contains(errMsg, "invalid bucket name"):
		return ErrInvalidBucketName
	case strings.Contains(errMsg, "invalid object name"):
		return ErrInvalidObjectName
	case strings.Contains(errMsg, "invalid key name"):
		return ErrInvalidObjectName
	case strings.Contains(errMsg, "invalid argument"):
		return ErrInvalidArgument
	case strings.Contains(errMsg, "invalid request"):
		return ErrInvalidArgument
	case strings.Contains(errMsg, "bad request"):
		return ErrInvalidArgument

	// Quota/Storage related
	case strings.Contains(errMsg, "quota exceeded"):
		return ErrQuotaExceeded
	case strings.Contains(errMsg, "insufficient storage"):
		return ErrQuotaExceeded
	case strings.Contains(errMsg, "storage quota"):
		return ErrQuotaExceeded

	// Cancellation
	case strings.Contains(errMsg, "canceled"):
		return ErrCancelled
	case strings.Contains(errMsg, "cancelled"):
		return ErrCancelled
	case strings.Contains(errMsg, "context canceled"):
		return ErrCancelled
	case strings.Contains(errMsg, "context cancelled"):
		return ErrCancelled

	// Versioning related
	case strings.Contains(errMsg, "invalid version"):
		return ErrInvalidVersion
	case strings.Contains(errMsg, "no such version"):
		return ErrInvalidVersion
	case strings.Contains(errMsg, "versioning not enabled"):
		return ErrVersioningNotEnabled

	// Object locking related
	case strings.Contains(errMsg, "object locked"):
		return ErrObjectLocked
	case strings.Contains(errMsg, "retention policy"):
		return ErrRetentionPolicyError
	case strings.Contains(errMsg, "lock configuration"):
		return ErrLockConfigurationError

	default:
		// Return the original error if no pattern matches
		return originalErr
	}
}

// ErrorCategory represents different categories of MinIO errors
type ErrorCategory int

const (
	CategoryUnknown ErrorCategory = iota
	CategoryConnection
	CategoryAuthentication
	CategoryPermission
	CategoryNotFound
	CategoryConflict
	CategoryValidation
	CategoryServer
	CategoryNetwork
	CategoryTimeout
	CategoryQuota
	CategoryConfiguration
	CategoryVersioning
	CategoryLocking
	CategoryOperation
)

// GetErrorCategory returns the category of the given error
func (m *MinioClient) GetErrorCategory(err error) ErrorCategory {
	switch {
	case errors.Is(err, ErrConnectionFailed), errors.Is(err, ErrConnectionLost):
		return CategoryConnection
	case errors.Is(err, ErrInvalidCredentials), errors.Is(err, ErrCredentialsExpired):
		return CategoryAuthentication
	case errors.Is(err, ErrAccessDenied), errors.Is(err, ErrInsufficientPermissions):
		return CategoryPermission
	case errors.Is(err, ErrObjectNotFound), errors.Is(err, ErrBucketNotFound):
		return CategoryNotFound
	case errors.Is(err, ErrBucketAlreadyExists), errors.Is(err, ErrBucketNotEmpty):
		return CategoryConflict
	case errors.Is(err, ErrInvalidBucketName), errors.Is(err, ErrInvalidObjectName), errors.Is(err, ErrInvalidArgument), errors.Is(err, ErrInvalidRange), errors.Is(err, ErrInvalidChecksum), errors.Is(err, ErrInvalidMetadata), errors.Is(err, ErrInvalidContentType), errors.Is(err, ErrMalformedXML):
		return CategoryValidation
	case errors.Is(err, ErrServerError), errors.Is(err, ErrServiceUnavailable), errors.Is(err, ErrInvalidResponse):
		return CategoryServer
	case errors.Is(err, ErrNetworkError), errors.Is(err, ErrInvalidURL):
		return CategoryNetwork
	case errors.Is(err, ErrTimeout):
		return CategoryTimeout
	case errors.Is(err, ErrQuotaExceeded), errors.Is(err, ErrObjectTooLarge):
		return CategoryQuota
	case errors.Is(err, ErrConfigurationError), errors.Is(err, ErrInvalidStorageClass):
		return CategoryConfiguration
	case errors.Is(err, ErrVersioningNotEnabled), errors.Is(err, ErrInvalidVersion):
		return CategoryVersioning
	case errors.Is(err, ErrObjectLocked), errors.Is(err, ErrLockConfigurationError), errors.Is(err, ErrRetentionPolicyError):
		return CategoryLocking
	case errors.Is(err, ErrNotImplemented), errors.Is(err, ErrCancelled):
		return CategoryOperation
	default:
		return CategoryUnknown
	}
}

// IsRetryableError returns true if the error is retryable
func (m *MinioClient) IsRetryableError(err error) bool {
	switch {
	case errors.Is(err, ErrConnectionFailed),
		errors.Is(err, ErrConnectionLost),
		errors.Is(err, ErrTimeout),
		errors.Is(err, ErrNetworkError),
		errors.Is(err, ErrServerError),
		errors.Is(err, ErrServiceUnavailable),
		errors.Is(err, ErrTooManyRequests),
		errors.Is(err, ErrInvalidResponse):
		return true
	default:
		return false
	}
}

// IsTemporaryError returns true if the error is temporary
func (m *MinioClient) IsTemporaryError(err error) bool {
	return m.IsRetryableError(err) || errors.Is(err, ErrCredentialsExpired)
}

// IsPermanentError returns true if the error is permanent and should not be retried
func (m *MinioClient) IsPermanentError(err error) bool {
	switch {
	case errors.Is(err, ErrObjectNotFound),
		errors.Is(err, ErrBucketNotFound),
		errors.Is(err, ErrInvalidCredentials),
		errors.Is(err, ErrAccessDenied),
		errors.Is(err, ErrInvalidBucketName),
		errors.Is(err, ErrInvalidObjectName),
		errors.Is(err, ErrInvalidArgument),
		errors.Is(err, ErrInvalidRange),
		errors.Is(err, ErrInvalidChecksum),
		errors.Is(err, ErrInvalidMetadata),
		errors.Is(err, ErrInvalidContentType),
		errors.Is(err, ErrMalformedXML),
		errors.Is(err, ErrNotImplemented),
		errors.Is(err, ErrCancelled):
		return true
	default:
		return false
	}
}
