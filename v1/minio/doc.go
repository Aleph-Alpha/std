// Package minio provides functionality for interacting with MinIO/S3-compatible object storage.
//
// The minio package offers a simplified interface for working with object storage
// systems that are compatible with the S3 API, including MinIO, Amazon S3, and others.
// It provides functionality for object operations, bucket management, and presigned URL
// generation with a focus on ease of use and reliability.
//
// # Architecture
//
// This package follows the "accept interfaces, return structs" design pattern:
//   - Client interface: Defines the contract for MinIO/S3 operations
//   - MinioClient struct: Concrete implementation of the Client interface
//   - NewClient constructor: Returns *MinioClient (concrete type)
//   - FX module: Provides both *MinioClient and Client interface for dependency injection
//
// Core Features:
//   - Bucket operations (create, check existence)
//   - Object operations (upload, download, delete)
//   - Presigned URL generation for temporary access
//   - Multipart upload and download support for large files
//   - Notification configuration
//   - Integration with the Logger package for structured logging
//
// # Direct Usage (Without FX)
//
// For simple applications or tests, create a client directly:
//
//	import (
//		"github.com/Aleph-Alpha/std/v1/minio"
//		"context"
//	)
//
//	// Create a new MinIO client (returns concrete *MinioClient)
//	client, err := minio.NewClient(minio.Config{
//		Connection: minio.ConnectionConfig{
//			Endpoint:        "play.min.io",
//			AccessKeyID:     "minioadmin",
//			SecretAccessKey: "minioadmin",
//			UseSSL:          true,
//			BucketName:      "mybucket",
//			AccessBucketCreation: true,
//		},
//	}, nil) // Pass logger if needed
//	if err != nil {
//		return err
//	}
//
//	// Use the client
//	ctx := context.Background()
//	_, err = client.Put(ctx, "path/to/object.txt", file, fileInfo.Size())
//
// # FX Module Integration
//
// For production applications using Uber's fx, use the FXModule which provides
// both the concrete type and interface:
//
//	import (
//		"github.com/Aleph-Alpha/std/v1/minio"
//		"github.com/Aleph-Alpha/std/v1/logger"
//		"go.uber.org/fx"
//	)
//
//	app := fx.New(
//		logger.FXModule, // Optional: provides std logger
//		minio.FXModule,  // Provides *MinioClient and minio.Client interface
//		fx.Provide(func() minio.Config {
//			return minio.Config{
//				Connection: minio.ConnectionConfig{
//					Endpoint:        "play.min.io",
//					AccessKeyID:     "minioadmin",
//					SecretAccessKey: "minioadmin",
//					BucketName:      "mybucket",
//				},
//			}
//		}),
//		fx.Invoke(func(client *minio.MinioClient) {
//			// Use concrete type directly
//			ctx := context.Background()
//			client.Put(ctx, "key", reader, size)
//		}),
//		// ... other modules
//	)
//	app.Run()
//
// # Observability (Observer Hook)
//
// MinIO supports optional observability through the Observer interface from the observability package.
// This allows external systems to track operations without coupling the MinIO package to specific
// metrics/tracing implementations.
//
// Using WithObserver (non-FX usage):
//
//	client, err := minio.NewClient(config)
//	if err != nil {
//	    return err
//	}
//	client = client.WithObserver(myObserver).WithLogger(myLogger)
//	defer client.GracefulShutdown()
//
// Using FX (automatic injection):
//
//	app := fx.New(
//	    minio.FXModule,
//	    logger.FXModule,  // Optional: provides logger
//	    fx.Provide(
//	        func() minio.Config { return loadConfig() },
//	        func() observability.Observer { return myObserver },  // Optional
//	    ),
//	)
//
// The observer receives events for all storage operations:
//   - Component: "minio"
//   - Operations: "put", "get", "stream_get", "delete", "presigned_get",
//     "presigned_put", "presigned_head"
//   - Resource: bucket name
//   - SubResource: object key
//   - Duration: operation duration
//   - Error: any error that occurred
//   - Size: bytes transferred (for put/get operations)
//   - Metadata: operation-specific details (e.g., chunk_size for stream_get)
//
// # Type Aliases in Consumer Code
//
// To simplify your code and make it storage-agnostic, use type aliases:
//
//	package myapp
//
//	import stdMinio "github.com/Aleph-Alpha/std/v1/minio"
//
//	// Use type alias to reference std's interface
//	type MinioClient = stdMinio.Client
//
//	// Now use MinioClient throughout your codebase
//	func MyFunction(client MinioClient) {
//		client.Get(ctx, "key")
//	}
//
// This eliminates the need for adapters and allows you to switch implementations
// by only changing the alias definition.
//
// # Basic Operations
//
//	// Create a bucket
//	err = client.CreateBucket(ctx, "mybucket")
//
//	// Upload a file
//	file, _ := os.Open("/local/path/file.txt")
//	defer file.Close()
//	fileInfo, _ := file.Stat()
//	_, err = client.Put(ctx, "path/to/object.txt", file, fileInfo.Size())
//
//	// Generate a presigned URL for downloading
//	url, err := client.PreSignedGet(ctx, "path/to/object.txt")
//
// # Multipart Operations
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
// All methods on the MinioClient type are safe for concurrent use by multiple goroutines.
// The underlying `*minio.Client` and `*minio.Core` pointers are stored in atomic pointers and
// can be swapped during reconnection without racing with concurrent operations.
package minio
