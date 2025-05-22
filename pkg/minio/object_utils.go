package minio

import (
	"context"
	"fmt"
	"github.com/minio/minio-go/v7"
	"io"
)

// Put uploads an object to the specified bucket.
// This method handles the direct upload of data to MinIO, managing both the transfer
// and proper error handling.
//
// Parameters:
//   - ctx: Context for controlling the upload operation
//   - objectKey: Path and name of the object in the bucket
//   - reader: Source of the data to upload
//   - size: Optional parameter specifying the size of the data in bytes.
//     If not provided or set to 0, an unknown size is assumed, which may affect
//     upload performance as chunking decisions can't be optimized
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
//	size, err := minioClient.Put(ctx, "documents/document.pdf", file, fileInfo.Size())
//	if err == nil {
//	    fmt.Printf("Uploaded %d bytes\n", size)
//	}
func (m *Minio) Put(ctx context.Context, objectKey string, reader io.Reader, size ...int64) (int64, error) {
	actualSize := unknownSize
	// If size was provided, use it
	if len(size) > 0 && size[0] != 0 {
		actualSize = size[0]
	}

	response, err := m.Client.PutObject(ctx, m.cfg.Connection.BucketName, objectKey, reader, actualSize, minio.PutObjectOptions{
		PartSize: m.cfg.UploadConfig.MinPartSize,
	})

	if err != nil {
		return 0, err
	}
	return response.Size, nil
}

// Get retrieves an object from the bucket and returns its contents as a byte slice.
// This method is optimized for both small and large files with safety considerations
// for buffer use. For small files, it uses direct allocation, while for larger files
// it leverages a buffer pool to reduce memory pressure.
//
// Parameters:
//   - ctx: Context for controlling the download operation
//   - objectKey: Path and name of the object in the bucket
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
//	data, err := minioClient.Get(ctx, "documents/report.pdf")
//	if err == nil {
//	    fmt.Printf("Downloaded %d bytes\n", len(data))
//	    ioutil.WriteFile("report.pdf", data, 0644)
//	}
func (m *Minio) Get(ctx context.Context, objectKey string) ([]byte, error) {
	// Get the object with a single call
	reader, err := m.Client.GetObject(ctx, m.cfg.Connection.BucketName, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}
	defer func(reader io.ReadCloser) {
		err := reader.Close()
		if err != nil {
			m.logger.Error("failed to close object reader", err, map[string]interface{}{})
		}
	}(reader)

	// Get object stats directly from the reader
	objectInfo, err := reader.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get object stats: %w", err)
	}

	// Get the size of the object
	size := objectInfo.Size

	// For small files, use direct allocation for simplicity and performance
	if size < m.cfg.DownloadConfig.SmallFileThreshold {
		data := make([]byte, size)
		_, err = io.ReadFull(reader, data)
		if err != nil {
			return nil, fmt.Errorf("failed to read object data: %w", err)
		}
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
		return nil, fmt.Errorf("failed to read large object: %w", err)
	}

	// Create a new slice to copy the buffer data into
	// This ensures we return an independent copy that won't be affected by buffer reuse
	result := make([]byte, buffer.Len())
	copy(result, buffer.Bytes())

	// Return the buffer to the pool for reuse
	m.bufferPool.Put(buffer)

	// Return the independent copy
	return result, nil
}

// Delete removes an object from the specified bucket.
// This method permanently deletes the object from MinIO storage.
//
// Parameters:
//   - ctx: Context for controlling the delete operation
//   - objectKey: Path and name of the object to delete
//
// Returns an error if the deletion fails.
//
// Example:
//
//	err := minioClient.Delete(ctx, "documents/old-report.pdf")
//	if err == nil {
//	    fmt.Println("Object successfully deleted")
//	}
func (m *Minio) Delete(ctx context.Context, objectKey string) error {
	err := m.Client.RemoveObject(ctx, m.cfg.Connection.BucketName, objectKey, minio.RemoveObjectOptions{})
	if err != nil {
		return err
	}
	return nil
}
