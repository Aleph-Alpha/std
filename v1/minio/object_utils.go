package minio

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
)

// Put uploads an object to the specified bucket.
// This method handles the direct upload of data to MinIO, managing both the transfer
// and proper error handling.
//
// Parameters:
//   - ctx: Context for controlling the upload operation
//   - bucket: Name of the bucket to upload to
//   - objectKey: Path and name of the object in the bucket
//   - reader: Source of the data to upload
//   - opts: Optional functional options for configuring the upload (size, content type, etc.)
//
// Returns:
//   - int64: The number of bytes uploaded
//   - error: Any error that occurred during the upload
//
// Example:
//
//	file, _ := os.Open("document.pdf")
//	defer file.Close()
//
//	fileInfo, _ := file.Stat()
//	size, err := minioClient.Put(ctx, "my-bucket", "documents/document.pdf", file,
//	    WithSize(fileInfo.Size()),
//	    WithContentType("application/pdf"))
//	if err == nil {
//	    fmt.Printf("Uploaded %d bytes\n", size)
//	}
func (m *MinioClient) Put(ctx context.Context, bucket, objectKey string, reader io.Reader, opts ...PutOption) (int64, error) {
	start := time.Now()
	c := m.client.Load()
	if c == nil {
		m.observeOperation("put", bucket, objectKey, time.Since(start), ErrConnectionFailed, 0, nil)
		return 0, ErrConnectionFailed
	}

	// Apply options
	options := &PutOptions{
		Size: unknownSize,
	}
	for _, opt := range opts {
		opt(options)
	}

	// Extract tracing information from context or generate new IDs
	traceMetadata := extractTraceMetadataFromContext(ctx)

	// Merge user metadata with trace metadata
	if options.Metadata != nil {
		for k, v := range options.Metadata {
			traceMetadata[k] = v
		}
	}

	putOpts := minio.PutObjectOptions{
		PartSize:     m.cfg.UploadConfig.MinPartSize,
		UserMetadata: traceMetadata,
	}

	// Set content type if provided
	if options.ContentType != "" {
		putOpts.ContentType = options.ContentType
	}

	// Override part size if provided
	if options.PartSize > 0 {
		putOpts.PartSize = options.PartSize
	}

	response, err := c.PutObject(ctx, bucket, objectKey, reader, options.Size, putOpts)

	m.observeOperation("put", bucket, objectKey, time.Since(start), err, response.Size, nil)
	if err != nil {
		return 0, err
	}
	return response.Size, nil
}

// Get retrieves an object from the specified bucket and returns its contents as a byte slice.
// This method is optimized for both small and large files with safety considerations
// for buffer use. For small files, it uses direct allocation, while for larger files
// it leverages a buffer pool to reduce memory pressure.
//
// Parameters:
//   - ctx: Context for controlling the download operation
//   - bucket: Name of the bucket to download from
//   - objectKey: Path and name of the object in the bucket
//   - opts: Optional functional options for configuring the download (version, range, etc.)
//
// Returns:
//   - []byte: The object's contents as a byte slice
//   - error: Any error that occurred during the download
//
// Note: For very large files, consider using a streaming approach rather than
// loading the entire file into memory with this method.
//
// Example:
//
//	data, err := minioClient.Get(ctx, "my-bucket", "documents/report.pdf")
//	if err == nil {
//	    fmt.Printf("Downloaded %d bytes\n", len(data))
//	    ioutil.WriteFile("report.pdf", data, 0644)
//	}
func (m *MinioClient) Get(ctx context.Context, bucket, objectKey string, opts ...GetOption) ([]byte, error) {
	start := time.Now()
	c := m.client.Load()
	if c == nil {
		m.observeOperation("get", bucket, objectKey, time.Since(start), ErrConnectionFailed, 0, nil)
		return nil, ErrConnectionFailed
	}

	// Apply options
	options := &GetOptions{}
	for _, opt := range opts {
		opt(options)
	}

	getOpts := minio.GetObjectOptions{}
	if options.VersionID != "" {
		getOpts.VersionID = options.VersionID
	}

	// Get the object with a single call
	reader, err := c.GetObject(ctx, bucket, objectKey, getOpts)
	if err != nil {
		m.observeOperation("get", bucket, objectKey, time.Since(start), err, 0, nil)
		return nil, fmt.Errorf("failed to get object: %w", err)
	}
	defer func(reader io.ReadCloser) {
		err := reader.Close()
		if err != nil {
			m.logError(ctx, "Failed to close object reader", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}(reader)

	// Get object stats directly from the reader
	objectInfo, err := reader.Stat()
	if err != nil {
		m.observeOperation("get", bucket, objectKey, time.Since(start), err, 0, nil)
		return nil, fmt.Errorf("failed to get object stats: %w", err)
	}

	// Get the size of the object
	size := objectInfo.Size

	// For small files, use direct allocation for simplicity and performance
	if size < m.cfg.DownloadConfig.SmallFileThreshold {
		data := make([]byte, size)
		_, err = io.ReadFull(reader, data)
		if err != nil {
			m.observeOperation("get", bucket, objectKey, time.Since(start), err, 0, nil)
			return nil, fmt.Errorf("failed to read object data: %w", err)
		}
		m.observeOperation("get", bucket, objectKey, time.Since(start), nil, size, nil)
		return data, nil
	}

	// For larger files, use the buffer pool
	// Get a buffer from the pool with appropriate capacity
	bufferSize := min(size, int64(m.cfg.DownloadConfig.InitialBufferSize))
	buffer := m.bufferPool.Get()
	if buffer.Cap() < int(bufferSize) {
		// If the buffer is too small, resize it
		buffer.Grow(int(bufferSize) - buffer.Cap())
	}
	buffer.Reset() // Clear the buffer but keep the capacity

	// Copy data from the reader to the buffer
	_, err = io.Copy(buffer, reader)
	if err != nil {
		// Return the buffer to the pool even on error
		m.bufferPool.Put(buffer)
		m.observeOperation("get", bucket, objectKey, time.Since(start), err, 0, nil)
		return nil, fmt.Errorf("failed to read large object: %w", err)
	}

	// Create a new slice to copy the buffer data into
	// This ensures we return an independent copy that won't be affected by buffer reuse
	result := make([]byte, buffer.Len())
	copy(result, buffer.Bytes())

	// Return the buffer to the pool for reuse
	m.bufferPool.Put(buffer)

	// Return the independent copy
	m.observeOperation("get", bucket, objectKey, time.Since(start), nil, int64(len(result)), nil)
	return result, nil
}

// StreamGet downloads a file from the specified bucket and sends chunks through a channel.
//
// Parameters:
//   - ctx: Context for controlling the download operation
//   - bucket: Name of the bucket to download from
//   - objectKey: Path and name of the object in the bucket
//   - chunkSize: Size of each chunk in bytes
//
// Returns:
//   - <-chan []byte: Channel that receives data chunks
//   - <-chan error: Channel that receives any error that occurred
func (m *MinioClient) StreamGet(ctx context.Context, bucket, objectKey string, chunkSize int) (<-chan []byte, <-chan error) {
	start := time.Now()
	dataCh := make(chan []byte)
	errCh := make(chan error, 1) // buffered to avoid goroutine leaks

	go func() {
		defer close(dataCh)
		defer close(errCh)

		var totalBytes int64

		// Get the object
		c := m.client.Load()
		if c == nil {
			m.observeOperation("stream_get", bucket, objectKey, time.Since(start), ErrConnectionFailed, 0, map[string]interface{}{
				"chunk_size": chunkSize,
			})
			errCh <- ErrConnectionFailed
			return
		}

		reader, err := c.GetObject(ctx, bucket, objectKey, minio.GetObjectOptions{})
		if err != nil {
			m.observeOperation("stream_get", bucket, objectKey, time.Since(start), err, 0, map[string]interface{}{
				"chunk_size": chunkSize,
			})
			errCh <- fmt.Errorf("failed to get object: %w", err)
			return
		}
		defer func() {
			if err := reader.Close(); err != nil {
				m.logError(ctx, "Failed to close object reader", map[string]interface{}{
					"error": err.Error(),
				})
			}
		}()

		// Read the object in chunks
		buffer := make([]byte, chunkSize)
		for {
			// Check if context is done
			select {
			case <-ctx.Done():
				m.observeOperation("stream_get", bucket, objectKey, time.Since(start), ctx.Err(), totalBytes, map[string]interface{}{
					"chunk_size": chunkSize,
				})
				errCh <- ctx.Err()
				return
			default:
				// Continue processing
			}

			// Read a chunk
			n, err := reader.Read(buffer)

			if n > 0 {
				totalBytes += int64(n)
				// Create a copy of the data to send through the channel
				// This is important, so the buffer can be reused for the next read
				chunk := make([]byte, n)
				copy(chunk, buffer[:n])

				// Send the chunk through the channel
				select {
				case dataCh <- chunk:
					// Chunk sent successfully
				case <-ctx.Done():
					m.observeOperation("stream_get", bucket, objectKey, time.Since(start), ctx.Err(), totalBytes, map[string]interface{}{
						"chunk_size": chunkSize,
					})
					errCh <- ctx.Err()
					return
				}
			}

			// Handle the end of a file or errors
			if err != nil {
				if err == io.EOF {
					// Normal end of a file, not an error
					m.observeOperation("stream_get", bucket, objectKey, time.Since(start), nil, totalBytes, map[string]interface{}{
						"chunk_size": chunkSize,
					})
					return
				}
				m.observeOperation("stream_get", bucket, objectKey, time.Since(start), err, totalBytes, map[string]interface{}{
					"chunk_size": chunkSize,
				})
				errCh <- fmt.Errorf("error reading object: %w", err)
				return
			}
		}
	}()

	return dataCh, errCh
}

// Delete removes an object from the specified bucket.
// This method permanently deletes the object from MinIO storage.
//
// Parameters:
//   - ctx: Context for controlling the delete operation
//   - bucket: Name of the bucket containing the object
//   - objectKey: Path and name of the object to delete
//
// Returns an error if the deletion fails.
//
// Example:
//
//	err := minioClient.Delete(ctx, "my-bucket", "documents/old-report.pdf")
//	if err == nil {
//	    fmt.Println("Object successfully deleted")
//	}
func (m *MinioClient) Delete(ctx context.Context, bucket, objectKey string) error {
	start := time.Now()
	c := m.client.Load()
	if c == nil {
		m.observeOperation("delete", bucket, objectKey, time.Since(start), ErrConnectionFailed, 0, nil)
		return ErrConnectionFailed
	}
	err := c.RemoveObject(ctx, bucket, objectKey, minio.RemoveObjectOptions{})
	m.observeOperation("delete", bucket, objectKey, time.Since(start), err, 0, nil)
	return err
}
