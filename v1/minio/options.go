package minio

// Option constructors for functional options pattern.
// These functions allow users to configure operations without importing the underlying MinIO SDK.

// WithSize sets the size of the object being uploaded.
// This is important for optimal upload performance and progress tracking.
//
// Example:
//
//	client.Put(ctx, bucket, key, reader, minio.WithSize(fileSize))
func WithSize(size int64) PutOption {
	return func(opts *PutOptions) {
		opts.Size = size
	}
}

// WithContentType sets the MIME type of the object being uploaded.
//
// Example:
//
//	client.Put(ctx, bucket, key, reader,
//	    minio.WithSize(fileSize),
//	    minio.WithContentType("application/json"))
func WithContentType(contentType string) PutOption {
	return func(opts *PutOptions) {
		opts.ContentType = contentType
	}
}

// WithMetadata adds custom metadata to the object being uploaded.
// Metadata is stored as key-value pairs and can be retrieved later.
//
// Example:
//
//	client.Put(ctx, bucket, key, reader,
//	    minio.WithMetadata(map[string]string{
//	        "user-id": "12345",
//	        "version": "v2",
//	    }))
func WithMetadata(metadata map[string]string) PutOption {
	return func(opts *PutOptions) {
		opts.Metadata = metadata
	}
}

// WithPartSize sets a custom part size for multipart uploads.
// This overrides the default part size from the configuration.
//
// Example:
//
//	client.Put(ctx, bucket, key, reader,
//	    minio.WithPartSize(10 * 1024 * 1024)) // 10 MB parts
func WithPartSize(partSize uint64) PutOption {
	return func(opts *PutOptions) {
		opts.PartSize = partSize
	}
}

// WithVersionID retrieves a specific version of an object (for versioned buckets).
//
// Example:
//
//	data, err := client.Get(ctx, bucket, key,
//	    minio.WithVersionID("version-id-here"))
func WithVersionID(versionID string) GetOption {
	return func(opts *GetOptions) {
		opts.VersionID = versionID
	}
}

// WithByteRange retrieves only a specific byte range of an object.
// Useful for partial downloads or streaming specific portions.
//
// Example:
//
//	data, err := client.Get(ctx, bucket, key,
//	    minio.WithByteRange(0, 1023)) // First 1KB
func WithByteRange(start, end int64) GetOption {
	return func(opts *GetOptions) {
		opts.Range = &ByteRange{
			Start: start,
			End:   end,
		}
	}
}

// WithRegion sets the region for bucket creation.
//
// Example:
//
//	err := client.CreateBucket(ctx, "my-bucket",
//	    minio.WithRegion("us-west-2"))
func WithRegion(region string) BucketOption {
	return func(opts *BucketOptions) {
		opts.Region = region
	}
}

// WithObjectLocking enables object locking for the bucket.
// Object locking prevents objects from being deleted or overwritten.
//
// Example:
//
//	err := client.CreateBucket(ctx, "my-bucket",
//	    minio.WithObjectLocking(true))
func WithObjectLocking(enabled bool) BucketOption {
	return func(opts *BucketOptions) {
		opts.ObjectLocking = enabled
	}
}
