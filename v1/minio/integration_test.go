package minio

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

type minioContainerInstance struct {
	testcontainers.Container
	Host   string
	Port   string
	Config Config
}

func setupMinioContainer(ctx context.Context) (_ *minioContainerInstance, err error) {
	const (
		rootUser     = "minioadmin"
		rootPassword = "minioadmin"
		bucket       = "testbucket"
	)

	// testcontainers-go may panic when Docker is not available (e.g. no daemon, rootless not configured).
	// Convert that into a regular error so the test can be skipped gracefully.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("docker not available: %v", r)
		}
	}()

	req := testcontainers.ContainerRequest{
		Image:        "minio/minio:latest",
		ExposedPorts: []string{"9000/tcp", "9001/tcp"},
		Env: map[string]string{
			"MINIO_ROOT_USER":     rootUser,
			"MINIO_ROOT_PASSWORD": rootPassword,
		},
		Cmd: []string{"server", "/data", "--console-address", ":9001"},
		WaitingFor: wait.ForHTTP("/minio/health/ready").
			WithPort("9000/tcp").
			WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, err
	}

	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, err
	}

	mappedPort, err := container.MappedPort(ctx, "9000/tcp")
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, err
	}

	cfg := Config{
		Connection: ConnectionConfig{
			Endpoint:        host + ":" + mappedPort.Port(),
			AccessKeyID:     rootUser,
			SecretAccessKey: rootPassword,
			UseSSL:          false,
			Region:          "us-east-1",
		},
		UploadConfig: UploadConfig{
			MinPartSize: uint64(minPartSizeForUpload),
		},
		DownloadConfig: DownloadConfig{
			SmallFileThreshold: 64 * 1024,
			InitialBufferSize:  64 * 1024,
		},
		PresignedConfig: PresignedConfig{
			Enabled:        true,
			ExpiryDuration: 15 * time.Minute,
		},
	}

	return &minioContainerInstance{
		Container: container,
		Host:      host,
		Port:      mappedPort.Port(),
		Config:    cfg,
	}, nil
}

func TestMinioWithFXModule(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	containerInstance, err := setupMinioContainer(ctx)
	if err != nil {
		t.Skipf("Skipping MinIO integration test: %v", err)
	}
	defer func() {
		if err := containerInstance.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	}()

	t.Logf("Using MinIO on %s:%s", containerInstance.Host, containerInstance.Port)

	var client *MinioClient
	app := fxtest.New(t,
		fx.Provide(func() Config { return containerInstance.Config }),
		FXModule,
		fx.Populate(&client),
	)

	err = app.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = app.Stop(ctx) })

	require.NotNil(t, client)

	// Create the test bucket
	const testBucket = "testbucket"
	err = client.CreateBucket(ctx, testBucket)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.DeleteBucket(ctx, testBucket) })

	t.Run("PutGetDelete", func(t *testing.T) {
		key := "it/hello.txt"
		payload := []byte("hello minio integration test")

		n, err := client.Put(ctx, testBucket, key, bytes.NewReader(payload), WithSize(int64(len(payload))))
		require.NoError(t, err)
		require.Equal(t, int64(len(payload)), n)

		got, err := client.Get(ctx, testBucket, key)
		require.NoError(t, err)
		require.Equal(t, payload, got)

		require.NoError(t, client.Delete(ctx, testBucket, key))

		_, err = client.Get(ctx, testBucket, key)
		require.Error(t, err)
	})

	t.Run("GetLargeObjectUsesBufferPool", func(t *testing.T) {
		key := "it/large.bin"
		// Ensure this is above SmallFileThreshold (64 KiB in test config)
		payload := []byte(strings.Repeat("0123456789", 25000)) // 250kB

		_, err := client.Put(ctx, testBucket, key, bytes.NewReader(payload), WithSize(int64(len(payload))))
		require.NoError(t, err)
		t.Cleanup(func() { _ = client.Delete(ctx, testBucket, key) })

		got, err := client.Get(ctx, testBucket, key)
		require.NoError(t, err)
		require.Equal(t, payload, got)
	})

	t.Run("StreamGet", func(t *testing.T) {
		key := "it/stream.txt"
		payload := []byte(strings.Repeat("abc", 1024))

		_, err := client.Put(ctx, testBucket, key, bytes.NewReader(payload), WithSize(int64(len(payload))))
		require.NoError(t, err)
		t.Cleanup(func() { _ = client.Delete(ctx, testBucket, key) })

		dataCh, errCh := client.StreamGet(ctx, testBucket, key, 128)

		var out []byte
		for chunk := range dataCh {
			out = append(out, chunk...)
		}
		err = <-errCh
		// StreamGet reports EOF as nil, but may also return ctx errors if cancelled.
		require.NoError(t, err)
		require.Equal(t, payload, out)
	})

	t.Run("StreamGetCancellation", func(t *testing.T) {
		key := "it/stream-cancel.txt"
		payload := []byte(strings.Repeat("x", 128*1024))

		_, err := client.Put(ctx, testBucket, key, bytes.NewReader(payload), WithSize(int64(len(payload))))
		require.NoError(t, err)
		t.Cleanup(func() { _ = client.Delete(ctx, testBucket, key) })

		cctx, cancel := context.WithCancel(ctx)
		dataCh, errCh := client.StreamGet(cctx, testBucket, key, 1024)

		// Consume a little then cancel.
		for range dataCh {
			cancel()
			break
		}

		err = <-errCh
		require.Error(t, err)
	})

	t.Run("PresignedURLs", func(t *testing.T) {
		key := "it/presigned.txt"

		putURL, err := client.PreSignedPut(ctx, testBucket, key)
		require.NoError(t, err)
		_, err = url.Parse(putURL)
		require.NoError(t, err)

		getURL, err := client.PreSignedGet(ctx, testBucket, key)
		require.NoError(t, err)
		_, err = url.Parse(getURL)
		require.NoError(t, err)

		headURL, err := client.PreSignedHeadObject(ctx, testBucket, key)
		require.NoError(t, err)
		_, err = url.Parse(headURL)
		require.NoError(t, err)

		assert.NotEmpty(t, putURL)
		assert.NotEmpty(t, getURL)
		assert.NotEmpty(t, headURL)
	})

	t.Run("PresignedBaseURLOverride", func(t *testing.T) {
		cfg := containerInstance.Config
		cfg.PresignedConfig.BaseURL = "https://cdn.example.com/base"

		c2, err := NewClient(cfg)
		require.NoError(t, err)

		// Create bucket for the new client
		err = c2.CreateBucket(ctx, testBucket)
		require.NoError(t, err)

		u, err := c2.PreSignedGet(ctx, testBucket, "it/override.txt")
		require.NoError(t, err)

		parsed, err := url.Parse(u)
		require.NoError(t, err)
		require.Equal(t, "cdn.example.com", parsed.Host)
	})

	t.Run("MultipartUploadURLsAndAbort", func(t *testing.T) {
		key := "it/multipart-upload.bin"
		fileSize := int64(25 * 1024 * 1024) // 25MiB -> multiple parts

		upload, err := client.GenerateMultipartUploadURLs(ctx, testBucket, key, fileSize, "application/octet-stream", 15*time.Minute)
		require.NoError(t, err)

		require.Equal(t, key, upload.GetObjectKey())
		require.NotEmpty(t, upload.GetUploadID())
		require.Greater(t, len(upload.GetPresignedURLs()), 1)
		require.Equal(t, len(upload.GetPresignedURLs()), len(upload.GetPartNumbers()))

		// Abort the multipart upload and ensure no error.
		err = client.AbortMultipartUpload(ctx, testBucket, upload.GetObjectKey(), upload.GetUploadID())
		require.NoError(t, err)
	})

	t.Run("MultipartDownloadURLs", func(t *testing.T) {
		key := "it/multipart-download.bin"
		payload := []byte(strings.Repeat("z", 12*1024*1024)) // 12MiB

		_, err := client.Put(ctx, testBucket, key, bytes.NewReader(payload), WithSize(int64(len(payload))))
		require.NoError(t, err)
		t.Cleanup(func() { _ = client.Delete(ctx, testBucket, key) })

		download, err := client.GenerateMultipartPresignedGetURLs(ctx, testBucket, key, 5*1024*1024, 15*time.Minute)
		require.NoError(t, err)

		urls := download.GetPresignedURLs()
		ranges := download.GetPartRanges()
		require.Equal(t, len(urls), len(ranges))
		require.GreaterOrEqual(t, len(urls), 3) // 12MiB with 5MiB parts => 3 parts

		for _, u := range urls {
			_, err := url.Parse(u)
			require.NoError(t, err)
		}
	})
}
