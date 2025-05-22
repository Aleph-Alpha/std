// Package minio provides functionality for interacting with MinIO/S3-compatible object storage.
//
// The minio package offers a simplified interface for working with object storage
// systems that are compatible with the S3 API, including MinIO, Amazon S3, and others.
// It provides functionality for object operations, bucket management, and presigned URL
// generation with a focus on ease of use and reliability.
//
// Core Features:
//   - Bucket operations (create, check existence)
//   - Object operations (upload, download, delete)
//   - Presigned URL generation for temporary access
//   - Notification configuration
//   - Integration with the Logger package for structured logging
//
// Basic Usage:
//
//	import (
//		"gitlab.aleph-alpha.de/engineering/pharia-data-search/data-go-packages/pkg/minio"
//		"gitlab.aleph-alpha.de/engineering/pharia-data-search/data-go-packages/pkg/logger"
//	)
//
//	// Create a logger
//	log, _ := logger.NewLogger(logger.Config{Level: "info"})
//
//	// Create a new MinIO client
//	client, err := minio.New(minio.Config{
//		Endpoint:  "play.min.io",
//		AccessKey: "minioadmin",
//		SecretKey: "minioadmin",
//		Secure:    true,
//	}, log)
//	if err != nil {
//		log.Fatal("Failed to create MinIO client", err, nil)
//	}
//
//	// Create a bucket if it doesn't exist
//	err = client.CreateBucketIfNotExists(context.Background(), "mybucket")
//	if err != nil {
//		log.Error("Failed to create bucket", err, nil)
//	}
//
//	// Upload a file
//	_, err = client.UploadFile(context.Background(), "mybucket", "path/to/object.txt", "/local/path/file.txt", nil)
//	if err != nil {
//		log.Error("Failed to upload file", err, nil)
//	}
//
//	// Generate a presigned URL valid for 1 hour
//	url, err := client.GetPresignedURL(context.Background(), "mybucket", "path/to/object.txt", time.Hour)
//	if err != nil {
//		log.Error("Failed to generate presigned URL", err, nil)
//	}
//
// FX Module Integration:
//
// This package provides an fx module for easy integration:
//
//	app := fx.New(
//		logger.Module,
//		minio.Module,
//		// ... other modules
//	)
//	app.Run()
//
// Security Considerations:
//
// When using this package, follow these security best practices:
//   - Use environment variables or a secure secrets manager for credentials
//   - Always enable TLS (Secure=true) in production environments
//   - Use presigned URLs with the shortest viable expiration time
//   - Set appropriate bucket policies and access controls
//   - Consider using server-side encryption for sensitive data
//
// Error Handling:
//
// All operations return clear error messages that can be logged:
//
//	_, err := client.DownloadFile(ctx, "mybucket", "object.txt", "/local/path/download.txt")
//	if err != nil {
//		if strings.Contains(err.Error(), "The specified key does not exist") {
//			// Handle not found case
//		} else {
//			// Handle other errors
//			log.Error("Download failed", err, nil)
//		}
//	}
package minio
