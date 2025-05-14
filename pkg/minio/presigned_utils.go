package minio

import (
	"context"
	"fmt"
	"github.com/minio/minio-go/v7"
	"math"
	"net/url"
	"sort"
	"strconv"
	"time"
)

// MultipartUploadInfo contains all information needed for a multipart upload
type MultipartUploadInfo struct {
	UploadID            string   `json:"uploadId"`            // Unique identifier for the multipart upload
	ObjectKey           string   `json:"objectKey"`           // The object key in MinIO
	PresignedUrls       []string `json:"presignedUrls"`       // Presigned URLs for uploading each part
	PartNumbers         []int    `json:"partNumbers"`         // Part numbers corresponding to each URL
	ExpiresAt           int64    `json:"expiresAt"`           // Expiration timestamp
	RecommendedPartSize int64    `json:"recommendedPartSize"` // Recommended size for each part
	MaxParts            int      `json:"maxParts"`            // Maximum number of parts allowed
	ContentType         string   `json:"contentType"`         // Content type of the object
	TotalSize           int64    `json:"totalSize"`           // Total size of the object
}

// MultipartUpload represents a multipart upload session
// This is the public interface that hides the internal implementation
type MultipartUpload interface {
	// GetUploadID returns the unique identifier for this multipart upload
	GetUploadID() string

	// GetObjectKey returns the object key for this upload
	GetObjectKey() string

	// GetPresignedURLs returns all presigned URLs for this upload
	GetPresignedURLs() []string

	// GetPartNumbers returns all part numbers corresponding to the URLs
	GetPartNumbers() []int

	// GetExpiryTimestamp returns the Unix timestamp when this upload expires
	GetExpiryTimestamp() int64

	// GetRecommendedPartSize returns the recommended size for each part in bytes
	GetRecommendedPartSize() int64

	// GetMaxParts returns the maximum number of parts allowed
	GetMaxParts() int

	// GetContentType returns the content type of the object
	GetContentType() string

	// GetTotalSize returns the total size of the object in bytes
	GetTotalSize() int64

	// IsExpired checks if the upload has expired
	IsExpired() bool
}

// multipartUploadImpl is the internal implementation of MultipartUpload interface
// This is private and not exported
type multipartUploadImpl struct {
	info *MultipartUploadInfo
}

// NewMultipartUpload creates a new MultipartUpload from a MultipartUploadInfo
// This function would be internal to your package
func newMultipartUpload(info *MultipartUploadInfo) MultipartUpload {
	return &multipartUploadImpl{
		info: info,
	}
}

// GetUploadID Implement the MultipartUpload interface methods
func (m *multipartUploadImpl) GetUploadID() string {
	return m.info.UploadID
}

func (m *multipartUploadImpl) GetObjectKey() string {
	return m.info.ObjectKey
}

func (m *multipartUploadImpl) GetPresignedURLs() []string {
	// Return a copy to prevent modification
	result := make([]string, len(m.info.PresignedUrls))
	copy(result, m.info.PresignedUrls)
	return result
}

func (m *multipartUploadImpl) GetPartNumbers() []int {
	// Return a copy to prevent modification
	result := make([]int, len(m.info.PartNumbers))
	copy(result, m.info.PartNumbers)
	return result
}

func (m *multipartUploadImpl) GetExpiryTimestamp() int64 {
	return m.info.ExpiresAt
}

func (m *multipartUploadImpl) GetRecommendedPartSize() int64 {
	return m.info.RecommendedPartSize
}

func (m *multipartUploadImpl) GetMaxParts() int {
	return m.info.MaxParts
}

func (m *multipartUploadImpl) GetContentType() string {
	return m.info.ContentType
}

func (m *multipartUploadImpl) GetTotalSize() int64 {
	return m.info.TotalSize
}

func (m *multipartUploadImpl) IsExpired() bool {
	return time.Now().Unix() > m.info.ExpiresAt
}

// updateUploadConfig updates the UploadConfig with multipart-specific settings
func (m *Minio) updateUploadConfig() {
	// Set default values if not provided
	if m.cfg.UploadConfig.MaxObjectSize == 0 {
		m.cfg.UploadConfig.MaxObjectSize = MaxObjectSize
	}

	if m.cfg.UploadConfig.MinPartSize == 0 {
		m.cfg.UploadConfig.MinPartSize = uint64(minPartSizeForUpload) // 5 MiB default
	}

	if m.cfg.UploadConfig.MultipartThreshold == 0 {
		m.cfg.UploadConfig.MultipartThreshold = MultipartThreshold // 50 MiB default
	}
}

// calculateOptimalPartSize determines the best part size and part count for a given file size
func (m *Minio) calculateOptimalPartSize(fileSize int64) (partSize int64, partCount int) {
	// Default to MinIO minimum (5 MiB)
	minPartSize := minPartSizeForUpload
	if m.cfg.UploadConfig.MinPartSize > 0 {
		minPartSize = int64(m.cfg.UploadConfig.MinPartSize)
	}

	// If the file is small, use the minimum part size
	if fileSize < 10*minPartSize {
		partSize = minPartSize
		partCount = int(math.Ceil(float64(fileSize) / float64(partSize)))
		return
	}

	// Calculate a part size that divides evenly
	partSize = int64(math.Ceil(float64(fileSize) / float64(minioLimitPartCount)))

	// Round up to the nearest 1 MiB
	partSize = ((partSize + 1024*1024 - 1) / (1024 * 1024)) * (1024 * 1024)

	// Ensure part size is at least the minimum
	if partSize < minPartSize {
		partSize = minPartSize
	}

	// Recalculate part count with the final part size
	partCount = int(math.Ceil(float64(fileSize) / float64(partSize)))

	return
}

// GenerateMultipartUploadURLs initiates a multipart upload and generates all necessary URLs
func (m *Minio) GenerateMultipartUploadURLs(
	ctx context.Context,
	objectKey string,
	fileSize int64,
	contentType string,
	expiry ...time.Duration,
) (MultipartUpload, error) {
	// Determine expiry
	expiryDuration := m.cfg.PresignedConfig.ExpiryDuration
	if len(expiry) > 0 && expiry[0] > 0 {
		expiryDuration = expiry[0]
	}

	// Ensure multipart config is set
	m.updateUploadConfig()

	// Check if file size exceeds maximum
	if fileSize > m.cfg.UploadConfig.MaxObjectSize {
		return nil, fmt.Errorf("file size %d exceeds maximum allowed size %d", fileSize, m.cfg.UploadConfig.MaxObjectSize)
	}

	// Check if the file size is below a multipart threshold, in which case suggest single-part upload
	if fileSize < m.cfg.UploadConfig.MultipartThreshold {
		m.logger.Info("File size is below multipart threshold, consider using single-part upload", nil, map[string]interface{}{
			"fileSize":  fileSize,
			"threshold": m.cfg.UploadConfig.MultipartThreshold,
		})
	}

	// Calculate optimal part size and count
	partSize, partCount := m.calculateOptimalPartSize(fileSize)

	// Initiate multipart upload
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	uploadID, err := m.initiateMultipartUpload(ctx, objectKey, contentType)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate multipart upload: %w", err)
	}

	// Generate presigned URLs for each part
	presignedUrls := make([]string, partCount)
	partNumbers := make([]int, partCount)

	for i := 0; i < partCount; i++ {
		partNumber := i + 1
		presignedURL, err := m.generatePartPresignedURL(ctx, objectKey, uploadID, partNumber, expiryDuration)
		if err != nil {
			// Try to abort the upload if we fail to generate URLs
			_ = m.AbortMultipartUpload(ctx, objectKey, uploadID)
			return nil, fmt.Errorf("failed to generate presigned URL for part %d: %w", partNumber, err)
		}

		presignedUrls[i] = presignedURL
		partNumbers[i] = partNumber
	}

	// Calculate expiration time
	expiresAt := time.Now().Add(expiryDuration).Unix()

	// Create the internal struct
	info := &MultipartUploadInfo{
		UploadID:            uploadID,
		ObjectKey:           objectKey,
		PresignedUrls:       presignedUrls,
		PartNumbers:         partNumbers,
		ExpiresAt:           expiresAt,
		RecommendedPartSize: partSize,
		MaxParts:            int(minioLimitPartCount), // Fixed by S3 spec
		ContentType:         contentType,
		TotalSize:           fileSize,
	}

	// Wrap with the interface implementation
	return newMultipartUpload(info), nil
}

// initiateMultipartUpload starts a new multipart upload and returns the upload ID
func (m *Minio) initiateMultipartUpload(ctx context.Context, objectKey, contentType string) (string, error) {
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Use a standard client to start multipart upload
	opts := minio.PutObjectOptions{ContentType: contentType}
	uploadID, err := m.CoreClient.NewMultipartUpload(ctx, m.cfg.Connection.BucketName, objectKey, opts)
	if err != nil {
		return "", fmt.Errorf("failed to initialize multipart upload: %w", err)
	}

	return uploadID, nil
}

// generatePartPresignedURL creates a presigned URL for uploading a specific part
func (m *Minio) generatePartPresignedURL(ctx context.Context, objectKey, uploadID string, partNumber int, expiry time.Duration) (string, error) {
	// Create request parameters for part upload
	reqParams := make(url.Values)
	reqParams.Set("partNumber", strconv.Itoa(partNumber))
	reqParams.Set("uploadId", uploadID)

	// Use Presign method for more control over the request
	presignedURL, err := m.Client.Presign(ctx, "PUT", m.cfg.Connection.BucketName, objectKey, expiry, reqParams)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL for part %d: %w", partNumber, err)
	}

	// Apply base URL override if configured
	finalURL := presignedURL.String()
	if m.cfg.PresignedConfig.BaseURL != "" {
		finalURL, err = urlGenerator(presignedURL, m.cfg.PresignedConfig.BaseURL)
		if err != nil {
			return "", err
		}
	}

	return finalURL, nil
}

// AbortMultipartUpload aborts a multipart upload
func (m *Minio) AbortMultipartUpload(ctx context.Context, objectKey, uploadID string) error {
	err := m.CoreClient.AbortMultipartUpload(ctx, m.cfg.Connection.BucketName, objectKey, uploadID)
	if err != nil {
		return fmt.Errorf("failed to abort multipart upload: %w", err)
	}

	m.logger.Info("Aborted multipart upload", nil, map[string]interface{}{
		"objectKey": objectKey,
		"uploadID":  uploadID,
	})

	return nil
}

// CompleteMultipartUpload finalizes a multipart upload by combining all parts
func (m *Minio) CompleteMultipartUpload(ctx context.Context, objectKey, uploadID string, partNumbers []int, etags []string) error {
	if len(partNumbers) == 0 || len(etags) == 0 {
		return fmt.Errorf("cannot complete multipart upload with no parts")
	}

	if len(partNumbers) != len(etags) {
		return fmt.Errorf("mismatched part numbers and etags: got %d part numbers and %d etags", len(partNumbers), len(etags))
	}

	// Create complete parts and ensure they're in order
	type partInfo struct {
		idx        int
		partNumber int
		etag       string
	}

	parts := make([]partInfo, len(partNumbers))
	for i := range partNumbers {
		parts[i] = partInfo{idx: i, partNumber: partNumbers[i], etag: etags[i]}
	}

	// Sort by part number
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].partNumber < parts[j].partNumber
	})

	// Convert to minio.CompletePart
	completeParts := make([]minio.CompletePart, len(parts))
	for i, part := range parts {
		completeParts[i] = minio.CompletePart{
			PartNumber: part.partNumber,
			ETag:       part.etag,
		}
	}

	// Complete the multipart upload
	_, err := m.CoreClient.CompleteMultipartUpload(ctx, m.cfg.Connection.BucketName, objectKey, uploadID, completeParts, minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to complete multipart upload: %w", err)
	}

	return nil
}

// ListIncompleteUploads lists all incomplete multipart uploads for cleanup
func (m *Minio) ListIncompleteUploads(ctx context.Context, prefix string) ([]minio.ObjectMultipartInfo, error) {
	var incompleteUploads []minio.ObjectMultipartInfo

	// List incomplete uploads
	objectCh := m.CoreClient.ListIncompleteUploads(ctx, m.cfg.Connection.BucketName, prefix, true)

	for object := range objectCh {
		if object.Err != nil {
			return nil, fmt.Errorf("error listing incomplete uploads: %w", object.Err)
		}
		incompleteUploads = append(incompleteUploads, object)
	}

	return incompleteUploads, nil
}

// CleanupIncompleteUploads aborts any incomplete uploads older than the given duration
func (m *Minio) CleanupIncompleteUploads(ctx context.Context, prefix string, olderThan time.Duration) error {
	uploads, err := m.ListIncompleteUploads(ctx, prefix)
	if err != nil {
		return err
	}

	cutoffTime := time.Now().Add(-olderThan)

	for _, upload := range uploads {
		// Check if upload is older than cutoff
		if upload.Initiated.Before(cutoffTime) {
			err := m.AbortMultipartUpload(ctx, upload.Key, upload.UploadID)
			if err != nil {
				m.logger.Error("Failed to abort incomplete upload during cleanup", err, map[string]interface{}{
					"objectKey": upload.Key,
					"uploadID":  upload.UploadID,
				})
				// Continue with other uploads even if one fails
				continue
			}

			m.logger.Info("Cleaned up incomplete upload", nil, map[string]interface{}{
				"objectKey":   upload.Key,
				"uploadID":    upload.UploadID,
				"initiatedOn": upload.Initiated,
				"ageInHours":  time.Since(upload.Initiated).Hours(),
			})
		}
	}

	return nil
}

// PreSignedHeadObject generates a pre-signed URL for HeadObject operations
func (m *Minio) PreSignedHeadObject(ctx context.Context, objectKey string) (string, error) {
	presignedUrl, err := m.Client.PresignedHeadObject(ctx, m.cfg.Connection.BucketName, objectKey, m.cfg.PresignedConfig.ExpiryDuration, nil)
	if err != nil {
		return "", err
	}

	if m.cfg.PresignedConfig.BaseURL != "" {
		return urlGenerator(presignedUrl, m.cfg.PresignedConfig.BaseURL)
	}
	return presignedUrl.String(), nil
}

// PreSignedGet generates a pre-signed URL for GetObject operations
func (m *Minio) PreSignedGet(ctx context.Context, objectKey string) (string, error) {
	presignedUrl, err := m.Client.PresignedGetObject(ctx, m.cfg.Connection.BucketName, objectKey, m.cfg.PresignedConfig.ExpiryDuration, nil)
	if err != nil {
		return "", err
	}

	if m.cfg.PresignedConfig.BaseURL != "" {
		return urlGenerator(presignedUrl, m.cfg.PresignedConfig.BaseURL)
	}
	return presignedUrl.String(), nil
}

// PreSignedPut generates a pre-signed URL for PutObject operations
func (m *Minio) PreSignedPut(ctx context.Context, objectKey string) (string, error) {
	presignedUrl, err := m.Client.PresignedPutObject(ctx, m.cfg.Connection.BucketName, objectKey, m.cfg.PresignedConfig.ExpiryDuration)
	if err != nil {
		return "", err
	}

	if m.cfg.PresignedConfig.BaseURL != "" {
		return urlGenerator(presignedUrl, m.cfg.PresignedConfig.BaseURL)
	}
	return presignedUrl.String(), nil
}

// urlGenerator replaces the host of a presigned URL with a custom base URL
func urlGenerator(presignedUrl *url.URL, baseUrl string) (string, error) {
	base, err := url.Parse(baseUrl)
	if err != nil {
		return "", fmt.Errorf("invalid BaseURL format: %v", err)
	}

	finalURL, err := url.Parse(presignedUrl.String())
	if err != nil {
		return "", fmt.Errorf("failed to parse presigned URL: %v", err)
	}
	finalURL.Scheme = base.Scheme
	finalURL.Host = base.Host
	if base.Path != "" && base.Path != "/" {
		finalURL.Path = base.ResolveReference(&url.URL{Path: finalURL.Path}).Path
	}
	return finalURL.String(), nil
}
