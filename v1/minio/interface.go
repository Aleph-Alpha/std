package minio

import (
	"context"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
)

// Client provides a high-level interface for interacting with MinIO/S3-compatible storage.
// It abstracts object storage operations with features like multipart uploads, presigned URLs,
// and resource monitoring.
//
// This interface is implemented by the concrete *MinioClient type.
type Client interface {
	// Object operations

	// Put uploads an object to MinIO storage.
	Put(ctx context.Context, objectKey string, reader io.Reader, size ...int64) (int64, error)

	// Get retrieves an object from MinIO storage and returns its contents.
	Get(ctx context.Context, objectKey string) ([]byte, error)

	// StreamGet retrieves an object in chunks, useful for large files.
	StreamGet(ctx context.Context, objectKey string, chunkSize int) (<-chan []byte, <-chan error)

	// Delete removes an object from MinIO storage.
	Delete(ctx context.Context, objectKey string) error

	// Presigned URL operations

	// PreSignedPut generates a presigned URL for uploading an object.
	PreSignedPut(ctx context.Context, objectKey string) (string, error)

	// PreSignedGet generates a presigned URL for downloading an object.
	PreSignedGet(ctx context.Context, objectKey string) (string, error)

	// PreSignedHeadObject generates a presigned URL for retrieving object metadata.
	PreSignedHeadObject(ctx context.Context, objectKey string) (string, error)

	// Multipart upload operations

	// GenerateMultipartUploadURLs generates presigned URLs for multipart upload.
	GenerateMultipartUploadURLs(
		ctx context.Context,
		objectKey string,
		fileSize int64,
		contentType string,
		expiry ...time.Duration,
	) (MultipartUpload, error)

	// CompleteMultipartUpload finalizes a multipart upload.
	CompleteMultipartUpload(ctx context.Context, objectKey, uploadID string, partNumbers []int, etags []string) error

	// AbortMultipartUpload cancels a multipart upload.
	AbortMultipartUpload(ctx context.Context, objectKey, uploadID string) error

	// ListIncompleteUploads lists all incomplete multipart uploads.
	ListIncompleteUploads(ctx context.Context, prefix string) ([]minio.ObjectMultipartInfo, error)

	// CleanupIncompleteUploads removes stale incomplete multipart uploads.
	CleanupIncompleteUploads(ctx context.Context, prefix string, olderThan time.Duration) error

	// Multipart download operations

	// GenerateMultipartPresignedGetURLs generates presigned URLs for downloading parts of an object.
	GenerateMultipartPresignedGetURLs(
		ctx context.Context,
		objectKey string,
		partSize int64,
		expiry ...time.Duration,
	) (MultipartPresignedGet, error)

	// Resource monitoring

	// GetBufferPoolStats returns buffer pool statistics.
	GetBufferPoolStats() BufferPoolStats

	// CleanupResources performs cleanup of buffer pools and forces garbage collection.
	CleanupResources()

	// Error handling

	// TranslateError converts MinIO-specific errors into standardized application errors.
	TranslateError(err error) error

	// GetErrorCategory returns the category of an error.
	GetErrorCategory(err error) ErrorCategory

	// IsRetryableError checks if an error can be retried.
	IsRetryableError(err error) bool

	// IsTemporaryError checks if an error is temporary.
	IsTemporaryError(err error) bool

	// IsPermanentError checks if an error is permanent.
	IsPermanentError(err error) bool

	// Lifecycle

	// GracefulShutdown safely terminates all MinIO client operations.
	GracefulShutdown()
}
