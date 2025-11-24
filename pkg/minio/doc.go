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
//   - Multipart upload and download support for large files
//   - Notification configuration
//   - Integration with the Logger package for structured logging
//
// Basic Usage:
//
//		import (
//			"github.com/Aleph-Alpha/std/pkg/minio"
//			"github.com/Aleph-Alpha/std/pkg/logger"
//		)
//
//		// Create a logger (optional)
//		log, _ := logger.NewLoggerClient(logger.Config{Level: "info"})
//		loggerAdapter := minio.NewLoggerAdapter(log)
//
//		// Create a new MinIO client with logger
//		client, err := minio.NewClient(minio.Config{
//			Connection: minio.ConnectionConfig{
//				Endpoint:        "play.min.io",
//				AccessKeyID:     "minioadmin",
//				SecretAccessKey: "minioadmin",
//				UseSSL:          true,
//				BucketName:      "mybucket",
//	         AccessBucketCreation: true,
//			},
//			PresignedConfig: minio.PresignedConfig{
//				ExpiryDuration: 1 * time.Hour,
//			},
//		}, loggerAdapter)
//		if err != nil {
//			log.Fatal("Failed to create MinIO client", err, nil)
//		}
//
//		// Alternative: Create MinIO client without logger (uses fallback)
//		client, err := minio.NewClient(config, nil)
//		if err != nil {
//			return fmt.Errorf("failed to initialize MinIO client: %w", err)
//		}
//
//		// Create a bucket if it doesn't exist
//		err = client.CreateBucket(context.Background(), "mybucket")
//		if err != nil {
//			log.Error("Failed to create bucket", err, nil)
//		}
//
//		// Upload a file
//		file, _ := os.Open("/local/path/file.txt")
//		defer file.Close()
//		fileInfo, _ := file.Stat()
//		_, err = client.Put(context.Background(), "path/to/object.txt", file, fileInfo.Size())
//		if err != nil {
//			log.Error("Failed to upload file", err, nil)
//		}
//
//		// Generate a presigned URL for downloading
//		url, err := client.PreSignedGet(context.Background(), "path/to/object.txt")
//		if err != nil {
//			log.Error("Failed to generate presigned URL", err, nil)
//		}
//
// Multipart Operations:
//
// For large files, the package provides multipart upload and download capabilities:
//
// Multipart Upload Example:
//
//	// Generate multipart upload URLs for a 1GB file
//	upload, err := client.GenerateMultipartUploadURLs(
//		ctx,
//		"large-file.zip",
//		1024*1024*1024, // 1GB
//		"application/zip",
//		2*time.Hour, // URLs valid for 2 hours
//	)
//	if err != nil {
//		log.Error("Failed to generate upload URLs", err, nil)
//	}
//
//	// Use the returned URLs to upload each part
//	urls := upload.GetPresignedURLs()
//	partNumbers := upload.GetPartNumbers()
//
//	// After uploading parts, complete the multipart upload
//	err = client.CompleteMultipartUpload(
//		ctx,
//		upload.GetObjectKey(),
//		upload.GetUploadID(),
//		partNumbers,
//		etags, // ETags returned from each part upload
//	)
//
// Multipart Download Example:
//
//	// Generate multipart download URLs for a large file
//	download, err := client.GenerateMultipartPresignedGetURLs(
//		ctx,
//		"large-file.zip",
//		10*1024*1024, // 10MB parts
//		1*time.Hour,  // URLs valid for 1 hour
//	)
//	if err != nil {
//		log.Error("Failed to generate download URLs", err, nil)
//	}
//
//	// Use the returned URLs to download each part
//	urls := download.GetPresignedURLs()
//	ranges := download.GetPartRanges()
//
//	// Each URL can be used with an HTTP client to download a specific part
//	// by setting the Range header to the corresponding value from ranges
//
// FX Module Integration:
//
// This package provides an fx module for easy integration with optional logger injection:
//
//	// With logger module (recommended)
//	app := fx.New(
//		logger.FXModule,  // Provides structured logger
//		fx.Provide(func(log *logger.Logger) minio.MinioLogger {
//			return minio.NewLoggerAdapter(log)
//		}),
//		minio.FXModule,   // Will automatically use the provided logger
//		// ... other modules
//	)
//	app.Run()
//
//	// Without logger module (uses fallback logger)
//	app := fx.New(
//		minio.FXModule,   // Will use fallback logger
//		// ... other modules
//	)
//	app.Run()
//
// Security Considerations:
//
// When using this package, follow these security best practices:
//   - Use environment variables or a secure secrets manager for credentials
//   - Always enable TLS (UseSSL=true) in production environments
//   - Use presigned URLs with the shortest viable expiration time
//   - Set appropriate bucket policies and access controls
//   - Consider using server-side encryption for sensitive data
//
// Error Handling:
//
// All operations return clear error messages that can be logged:
//
//	data, err := client.Get(ctx, "object.txt")
//	if err != nil {
//		if strings.Contains(err.Error(), "The specified key does not exist") {
//			// Handle not found case
//		} else {
//			// Handle other errors
//			log.Error("Download failed", err, nil)
//		}
//	}
//
// Thread Safety:
//
// All methods on the Minio type are safe for concurrent use by multiple goroutines.
package minio
