package minio

import (
	"context"
	"fmt"
	"github.com/minio/minio-go/v7"
	"io"
)

// Put uploads an object to the specified bucket
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

// Get retrieves an object from the bucket and returns its contents as a byte slice
// Optimized for both small and large files with safety considerations for buffer use
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

// Delete removes an object from the specified bucket
func (m *Minio) Delete(ctx context.Context, objectKey string) error {
	err := m.Client.RemoveObject(ctx, m.cfg.Connection.BucketName, objectKey, minio.RemoveObjectOptions{})
	if err != nil {
		return err
	}
	return nil
}
