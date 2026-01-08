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
//
// Multi-bucket support: All object operations now require an explicit bucket parameter,
// enabling operations across multiple buckets with a single client instance.
type Client interface {
	// Object operations

	// Put uploads an object to the specified bucket in MinIO storage.
	Put(ctx context.Context, bucket, objectKey string, reader io.Reader, opts ...PutOption) (int64, error)

	// Get retrieves an object from the specified bucket in MinIO storage and returns its contents.
	Get(ctx context.Context, bucket, objectKey string, opts ...GetOption) ([]byte, error)

	// StreamGet retrieves an object from the specified bucket in chunks, useful for large files.
	StreamGet(ctx context.Context, bucket, objectKey string, chunkSize int) (<-chan []byte, <-chan error)

	// Delete removes an object from the specified bucket in MinIO storage.
	Delete(ctx context.Context, bucket, objectKey string) error

	// Bucket management operations

	// CreateBucket creates a new bucket with the specified name and options.
	CreateBucket(ctx context.Context, bucket string, opts ...BucketOption) error

	// DeleteBucket removes an empty bucket.
	DeleteBucket(ctx context.Context, bucket string) error

	// BucketExists checks if a bucket exists.
	BucketExists(ctx context.Context, bucket string) (bool, error)

	// ListBuckets returns a list of all buckets.
	ListBuckets(ctx context.Context) ([]BucketInfo, error)

	// Presigned URL operations

	// PreSignedPut generates a presigned URL for uploading an object to the specified bucket.
	PreSignedPut(ctx context.Context, bucket, objectKey string) (string, error)

	// PreSignedGet generates a presigned URL for downloading an object from the specified bucket.
	PreSignedGet(ctx context.Context, bucket, objectKey string) (string, error)

	// PreSignedHeadObject generates a presigned URL for retrieving object metadata from the specified bucket.
	PreSignedHeadObject(ctx context.Context, bucket, objectKey string) (string, error)

	// Multipart upload operations

	// GenerateMultipartUploadURLs generates presigned URLs for multipart upload to the specified bucket.
	GenerateMultipartUploadURLs(
		ctx context.Context,
		bucket, objectKey string,
		fileSize int64,
		contentType string,
		expiry ...time.Duration,
	) (MultipartUpload, error)

	// CompleteMultipartUpload finalizes a multipart upload in the specified bucket.
	CompleteMultipartUpload(ctx context.Context, bucket, objectKey, uploadID string, partNumbers []int, etags []string) error

	// AbortMultipartUpload cancels a multipart upload in the specified bucket.
	AbortMultipartUpload(ctx context.Context, bucket, objectKey, uploadID string) error

	// ListIncompleteUploads lists all incomplete multipart uploads in the specified bucket.
	ListIncompleteUploads(ctx context.Context, bucket, prefix string) ([]minio.ObjectMultipartInfo, error)

	// CleanupIncompleteUploads removes stale incomplete multipart uploads in the specified bucket.
	CleanupIncompleteUploads(ctx context.Context, bucket, prefix string, olderThan time.Duration) error

	// Multipart download operations

	// GenerateMultipartPresignedGetURLs generates presigned URLs for downloading parts of an object from the specified bucket.
	GenerateMultipartPresignedGetURLs(
		ctx context.Context,
		bucket, objectKey string,
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

	// Convenience

	// Bucket returns a BucketClient that automatically uses the specified bucket for all operations.
	// This is a convenience wrapper for when you perform multiple operations on the same bucket.
	Bucket(name string) *BucketClient
}
