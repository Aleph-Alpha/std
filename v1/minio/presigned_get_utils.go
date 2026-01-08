package minio

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
)

// MultipartPresignedGetInfo contains information for downloading an object in parts
type MultipartPresignedGetInfo struct {
	// ObjectKey is the path and name of the object in the bucket
	ObjectKey string `json:"objectKey"`

	// PresignedUrls is a slice of temporary URLs for downloading each part
	PresignedUrls []string `json:"presignedUrls"`

	// PartRanges contains the byte ranges for each part
	PartRanges []string `json:"partRanges"`

	// ExpiresAt is the Unix timestamp when the presigned URLs will expire
	ExpiresAt int64 `json:"expiresAt"`

	// TotalSize is the total size of the object in bytes
	TotalSize int64 `json:"totalSize"`

	// ContentType is the MIME type of the object
	ContentType string `json:"contentType"`

	// ETag is the entity tag of the object
	ETag string `json:"etag"`
}

// MultipartPresignedGet is an interface for accessing multipart download info
type MultipartPresignedGet interface {
	// GetObjectKey returns the object key for this download
	GetObjectKey() string

	// GetPresignedURLs returns all presigned URLs for this download
	GetPresignedURLs() []string

	// GetPartRanges returns all byte ranges corresponding to the URLs
	GetPartRanges() []string

	// GetExpiryTimestamp returns the Unix timestamp when these URLs expire
	GetExpiryTimestamp() int64

	// GetTotalSize returns the total size of the object in bytes
	GetTotalSize() int64

	// GetContentType returns the content type of the object
	GetContentType() string

	// GetETag returns the ETag of the object
	GetETag() string

	// IsExpired checks if the download URLs have expired
	IsExpired() bool
}

// multipartPresignedGetImpl implements the MultipartPresignedGet interface
type multipartPresignedGetImpl struct {
	info *MultipartPresignedGetInfo
}

// NewMultipartPresignedGet creates a new MultipartPresignedGet from info
func newMultipartPresignedGet(info *MultipartPresignedGetInfo) MultipartPresignedGet {
	return &multipartPresignedGetImpl{
		info: info,
	}
}

// Implement all interface methods for multipartPresignedGetImpl...

// GenerateMultipartPresignedGetURLs generates URLs for downloading an object in parts
// This method is useful for large objects that may benefit from parallel downloads
// or resumable downloads by clients.
//
// Parameters:
//   - ctx: Context for the operation
//   - objectKey: Path and name of the object in the bucket
//   - partSize: Size of each part in bytes (must be at least 5 MiB)
//   - expiry: Optional custom expiration duration for the presigned URLs
//
// Returns:
//   - MultipartPresignedGet: Interface providing access to download details
//   - error: Any error that occurred during setup
//
// Example:
//
//	download, err := minioClient.GenerateMultipartPresignedGetURLs(
//	    ctx,
//	    "documents/large-file.zip",
//	    10*1024*1024, // 10 MiB parts
//	    2*time.Hour,
//	)
func (m *MinioClient) GenerateMultipartPresignedGetURLs(
	ctx context.Context,
	bucket, objectKey string,
	partSize int64,
	expiry ...time.Duration,
) (MultipartPresignedGet, error) {
	c := m.client.Load()
	if c == nil {
		return nil, ErrConnectionFailed
	}

	// Determine expiry
	expiryDuration := m.cfg.PresignedConfig.ExpiryDuration
	if len(expiry) > 0 && expiry[0] > 0 {
		expiryDuration = expiry[0]
	}

	// Get object stats to determine size and other metadata
	objInfo, err := c.StatObject(ctx, bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object info: %w", err)
	}

	// Ensure part size is reasonable
	if partSize == 0 {
		partSize = MultipartDownloadDefaultPartSize // Minimum 5 MiB
	}

	// Calculate the number of parts
	totalSize := objInfo.Size
	partCount := int(math.Ceil(float64(totalSize) / float64(partSize)))

	// Generate presigned URLs for each part
	presignedUrls := make([]string, partCount)
	partRanges := make([]string, partCount)

	for i := 0; i < partCount; i++ {
		startByte := int64(i) * partSize
		endByte := startByte + partSize - 1
		if endByte >= totalSize {
			endByte = totalSize - 1
		}

		// Create range header value
		byteRange := fmt.Sprintf("bytes=%d-%d", startByte, endByte)
		partRanges[i] = byteRange

		// Create request parameters with range
		reqParams := make(url.Values)
		// Add range parameter
		reqParams.Set("response-content-range", byteRange)

		// Generate presigned URL for this part
		presignedURL, err := c.Presign(ctx, "GET", bucket, objectKey, expiryDuration, reqParams)
		if err != nil {
			return nil, fmt.Errorf("failed to generate presigned URL for part %d: %w", i+1, err)
		}

		// Apply base URL override if configured
		finalURL := presignedURL.String()
		if m.cfg.PresignedConfig.BaseURL != "" {
			finalURL, err = urlGenerator(presignedURL, m.cfg.PresignedConfig.BaseURL)
			if err != nil {
				return nil, err
			}
		}

		presignedUrls[i] = finalURL
	}

	// Calculate expiration time
	expiresAt := time.Now().Add(expiryDuration).Unix()

	// Create the info struct
	info := &MultipartPresignedGetInfo{
		ObjectKey:     objectKey,
		PresignedUrls: presignedUrls,
		PartRanges:    partRanges,
		ExpiresAt:     expiresAt,
		TotalSize:     totalSize,
		ContentType:   objInfo.ContentType,
		ETag:          objInfo.ETag,
	}

	// Wrap with interface implementation
	return newMultipartPresignedGet(info), nil
}

// GetObjectKey returns the object key for this download.
// This is the full path where the object is stored in the bucket.
func (m *multipartPresignedGetImpl) GetObjectKey() string {
	return m.info.ObjectKey
}

// GetPresignedURLs returns all presigned URLs for this download.
// Returns a copy of the URLs to prevent modification of the internal state.
// These URLs can be used to download individual parts directly by a client.
// Each URL corresponds to a specific byte range of the object.
func (m *multipartPresignedGetImpl) GetPresignedURLs() []string {
	// Return a copy to prevent modification
	result := make([]string, len(m.info.PresignedUrls))
	copy(result, m.info.PresignedUrls)
	return result
}

// GetPartRanges returns all byte ranges corresponding to the URLs.
// Returns a copy of the ranges to prevent modification of the internal state.
// Each range is in the format "bytes=start-end" and corresponds to the
// same-indexed URL in the presigned URLs slice.
func (m *multipartPresignedGetImpl) GetPartRanges() []string {
	// Return a copy to prevent modification
	result := make([]string, len(m.info.PartRanges))
	copy(result, m.info.PartRanges)
	return result
}

// GetExpiryTimestamp returns the Unix timestamp when these URLs expire.
// After this time, the presigned URLs will no longer be valid for downloading parts.
func (m *multipartPresignedGetImpl) GetExpiryTimestamp() int64 {
	return m.info.ExpiresAt
}

// GetTotalSize returns the total size of the object in bytes.
// This is the complete size of the object being downloaded.
func (m *multipartPresignedGetImpl) GetTotalSize() int64 {
	return m.info.TotalSize
}

// GetContentType returns the content type (MIME type) of the object.
// This value was determined from the object's metadata in MinIO/S3.
func (m *multipartPresignedGetImpl) GetContentType() string {
	return m.info.ContentType
}

// GetETag returns the ETag of the object.
// The ETag is typically an MD5 hash of the object and can be used for
// validation after downloading all parts and reassembling the object.
func (m *multipartPresignedGetImpl) GetETag() string {
	return m.info.ETag
}

// IsExpired checks if the download URLs have expired.
// Returns true if the current time is past the expiration timestamp.
func (m *multipartPresignedGetImpl) IsExpired() bool {
	return time.Now().Unix() > m.info.ExpiresAt
}

// PreSignedGet generates a pre-signed URL for GetObject operations.
// This method creates a temporary URL that allows downloading an object
// without additional authentication.
//
// Parameters:
//   - ctx: Context for the operation
//   - objectKey: Path and name of the object in the bucket
//
// Returns:
//   - string: The presigned URL for downloading the object
//   - error: Any error that occurred during URL generation
//
// Example:
//
//	url, err := minioClient.PreSignedGet(ctx, "documents/report.pdf")
//	if err == nil {
//	    fmt.Printf("Download link: %s\n", url)
//	}
func (m *MinioClient) PreSignedGet(ctx context.Context, bucket, objectKey string) (string, error) {
	start := time.Now()
	c := m.client.Load()
	if c == nil {
		m.observeOperation("presigned_get", bucket, objectKey, time.Since(start), ErrConnectionFailed, 0, nil)
		return "", ErrConnectionFailed
	}

	presignedUrl, err := c.PresignedGetObject(ctx, bucket, objectKey, m.cfg.PresignedConfig.ExpiryDuration, nil)
	if err != nil {
		m.observeOperation("presigned_get", bucket, objectKey, time.Since(start), err, 0, nil)
		return "", err
	}

	var result string
	if m.cfg.PresignedConfig.BaseURL != "" {
		result, err = urlGenerator(presignedUrl, m.cfg.PresignedConfig.BaseURL)
	} else {
		result = presignedUrl.String()
	}
	m.observeOperation("presigned_get", bucket, objectKey, time.Since(start), err, 0, nil)
	return result, err
}
