package minio

import (
	"bytes"
	"context"
	"fmt"
	"github.com/minio/minio-go/v7"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/mock/gomock"
)

// createMinIOContainer sets up and starts a MinIO Docker container for testing
func createMinIOContainer(ctx context.Context) (testcontainers.Container, string, string, error) {
	// Get a random free port
	port, err := getFreePort()
	if err != nil {
		return nil, "", "", fmt.Errorf("could not get free port: %w", err)
	}

	portStr := fmt.Sprintf("%d", port)
	portBindings := nat.PortMap{
		"9000/tcp": []nat.PortBinding{{HostPort: portStr}},
	}

	req := testcontainers.ContainerRequest{
		Image: "minio/minio:RELEASE.2024-01-16T16-07-38Z",
		Cmd:   []string{"server", "/data"},
		Env: map[string]string{
			"MINIO_ACCESS_KEY": "minio_admin",
			"MINIO_SECRET_KEY": "minio_admin",
		},
		ExposedPorts: []string{
			"9000/tcp",
		},
		HostConfigModifier: func(cfg *container.HostConfig) {
			cfg.PortBindings = portBindings
		},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("9000/tcp").WithStartupTimeout(20*time.Second),
			wait.ForHTTP("/minio/health/ready").WithPort("9000/tcp").WithStartupTimeout(20*time.Second),
		),
	}

	containerInstance, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to start MinIO container: %w", err)
	}

	host, err := containerInstance.Host(ctx)
	if err != nil {
		_ = containerInstance.Terminate(ctx)
		return nil, "", "", fmt.Errorf("failed to get host: %w", err)
	}

	return containerInstance, host, portStr, nil
}

// getFreePort gets a free port from the OS
func getFreePort() (int, error) {
	addr, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}
	defer func(addr net.Listener) {
		err := addr.Close()
		if err != nil {
			fmt.Printf("Failed to close listener: %v", err)
		}
	}(addr)

	return addr.Addr().(*net.TCPAddr).Port, nil
}

// waitForMinioReady polls MinIO endpoint until it's ready or times out
func waitForMinioReady(host, port string, timeout time.Duration) error {
	endpoint := fmt.Sprintf("http://%s:%s/minio/health/ready", host, port)
	client := http.Client{
		Timeout: 1 * time.Second,
	}

	startTime := time.Now()
	for {
		if time.Since(startTime) > timeout {
			return fmt.Errorf("timed out waiting for MinIO to be ready")
		}

		resp, err := client.Get(endpoint)
		if err == nil && resp.StatusCode == http.StatusOK {
			err := resp.Body.Close()
			if err != nil {
				return err
			}
			return nil
		}

		if resp != nil {
			err := resp.Body.Close()
			if err != nil {
				return err
			}
		}

		time.Sleep(500 * time.Millisecond)
	}
}

// TestNewMinioClient_BucketCreation_Success tests successful MinIO client creation
func TestNewMinioClient_BucketCreation_Success(t *testing.T) {
	// Set up context
	ctx := context.Background()

	// Start MinIO containerInstance
	containerInstance, host, port, err := createMinIOContainer(ctx)
	require.NoError(t, err)

	// Create mock controller and logger
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockLogger := NewMockLogger(ctrl)

	// Set logger expectations
	mockLogger.EXPECT().Info("Connecting to MinIO", nil, gomock.Any()).MinTimes(1)
	mockLogger.EXPECT().Info("Creating MinIO Core client", nil, gomock.Any()).Times(1)
	mockLogger.EXPECT().Info("Bucket does not exist, creating it", nil, gomock.Any()).Times(1)
	mockLogger.EXPECT().Info("Successfully created bucket", nil, gomock.Any()).Times(1)
	mockLogger.EXPECT().Info("closing minio client...", nil, gomock.Any()).Times(1)
	mockLogger.EXPECT().Info("Stopping MinIO connection retry loop due to shutdown signal", nil, gomock.Any()).Times(1)

	// Create MinIO client config with the correct structure
	cfg := Config{
		Connection: ConnectionConfig{
			Endpoint:             fmt.Sprintf("%s:%s", host, port),
			AccessKeyID:          "minio_admin",
			SecretAccessKey:      "minio_admin",
			UseSSL:               false,
			BucketName:           "test-bucket",
			Region:               "us-east-1",
			AccessBucketCreation: true,
		},
		UploadConfig: UploadConfig{
			MaxObjectSize:      5 * 1024 * 1024 * 1024, // 5 GiB
			MinPartSize:        5 * 1024 * 1024,        // 5 MiB
			MultipartThreshold: 50 * 1024 * 1024,       // 50 MiB
		},
		PresignedConfig: PresignedConfig{
			Enabled:        true,
			ExpiryDuration: 15 * time.Minute,
		},
	}

	var client *Minio

	app := fxtest.New(t,
		FXModule,
		fx.Provide(
			func() Config {
				return cfg
			},
			func() Logger {
				return mockLogger
			},
		),
		fx.Populate(&client),
	)

	require.NoError(t, app.Start(ctx))
	require.NoError(t, app.Stop(ctx))
	require.NoError(t, containerInstance.Terminate(ctx))
}

// TestNewMinioClient_ConnectionFailure_InvalidEndpoint tests MinIO client creation with connection failure
func TestNewMinioClient_ConnectionFailure_InvalidEndpoint(t *testing.T) {
	ctx := context.Background()

	// Create mock controller and logger
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockLogger := NewMockLogger(ctrl)

	// Set logger expectations for failed connection - use more flexible matching
	mockLogger.EXPECT().Info("Connecting to MinIO", nil, gomock.Any()).Times(1)
	mockLogger.EXPECT().Info("Creating MinIO Core client", nil, gomock.Any()).Times(1)
	mockLogger.EXPECT().Error("failed to validate minio connection", gomock.Any(), gomock.Any()).Times(1)

	// Create MinIO client config with an invalid endpoint
	cfg := Config{
		Connection: ConnectionConfig{
			Endpoint:             "invalid-endpoint:9000",
			AccessKeyID:          "minio_admin",
			SecretAccessKey:      "minio_admin",
			UseSSL:               false,
			BucketName:           "test-bucket",
			Region:               "us-east-1",
			AccessBucketCreation: true,
		},
	}

	var client *Minio

	app := fx.New(
		FXModule,
		fx.Provide(
			func() Config {
				return cfg
			},
			func() Logger {
				return mockLogger
			},
		),
		fx.Populate(&client),
		fx.NopLogger,
	)

	require.Error(t, app.Start(ctx), "invalid-endpoint")
	require.NoError(t, app.Stop(ctx))
}

// TestEnsureBucketExists_EmptyBucketName tests when the bucket name is empty
func TestEnsureBucketExists_EmptyBucketName(t *testing.T) {
	// Set up context
	ctx := context.Background()

	// Start MinIO containerInstance
	containerInstance, host, port, err := createMinIOContainer(ctx)
	require.NoError(t, err)

	// Create mock controller and logger
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockLogger := NewMockLogger(ctrl)

	// Set logger expectations
	mockLogger.EXPECT().Info("Connecting to MinIO", nil, gomock.Any()).Times(1)
	mockLogger.EXPECT().Info("Creating MinIO Core client", nil, gomock.Any()).Times(1)
	mockLogger.EXPECT().Error("failed to verify bucket", gomock.Any(), gomock.Any()).Times(1)

	// Create MinIO client config with an empty bucket name
	cfg := Config{
		Connection: ConnectionConfig{
			Endpoint:             fmt.Sprintf("%s:%s", host, port),
			AccessKeyID:          "minio_admin",
			SecretAccessKey:      "minio_admin",
			UseSSL:               false,
			BucketName:           "", // Empty bucket name
			Region:               "us-east-1",
			AccessBucketCreation: true,
		},
	}

	var client *Minio

	app := fx.New(
		FXModule,
		fx.Provide(
			func() Config {
				return cfg
			},
			func() Logger {
				return mockLogger
			},
		),
		fx.Populate(&client),
		fx.NopLogger,
	)

	require.Error(t, app.Start(ctx), "bucket name is empty")
	require.NoError(t, app.Stop(ctx))
	require.NoError(t, containerInstance.Terminate(ctx))
}

// TestEnsureBucketExists_NoBucket_NoPermission tests when the bucket doesn't exist and the user doesn't have permission to create it
func TestEnsureBucketExists_NoBucket_NoPermission(t *testing.T) {
	// Set up context
	ctx := context.Background()

	// Start MinIO containerInstance
	containerInstance, host, port, err := createMinIOContainer(ctx)
	require.NoError(t, err)

	// Create mock controller and logger
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockLogger := NewMockLogger(ctrl)

	// Set logger expectations
	mockLogger.EXPECT().Info("Connecting to MinIO", nil, gomock.Any()).Times(1)
	mockLogger.EXPECT().Info("Creating MinIO Core client", nil, gomock.Any()).Times(1)
	mockLogger.EXPECT().Error("failed to verify bucket", gomock.Any(), gomock.Any()).Times(1)

	// Create MinIO client config with an empty bucket name
	cfg := Config{
		Connection: ConnectionConfig{
			Endpoint:             fmt.Sprintf("%s:%s", host, port),
			AccessKeyID:          "minio_admin",
			SecretAccessKey:      "minio_admin",
			UseSSL:               false,
			BucketName:           "hello", // Empty bucket name
			Region:               "us-east-1",
			AccessBucketCreation: false,
		},
	}

	var client *Minio

	app := fx.New(
		FXModule,
		fx.Provide(
			func() Config {
				return cfg
			},
			func() Logger {
				return mockLogger
			},
		),
		fx.Populate(&client),
		fx.NopLogger,
	)

	require.Error(t, app.Start(ctx), "bucket does not exist, please create it manually")
	require.NoError(t, app.Stop(ctx))
	require.NoError(t, containerInstance.Terminate(ctx))
}

// TestPut tests the Put method with an actual MinIO container
func TestPut_Success(t *testing.T) {
	// Set up context
	ctx := context.Background()

	// Start MinIO containerInstance
	containerInstance, host, port, err := createMinIOContainer(ctx)
	require.NoError(t, err)

	// Create mock controller and logger
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockLogger := NewMockLogger(ctrl)

	// Set logger expectations
	mockLogger.EXPECT().Info("Connecting to MinIO", nil, gomock.Any()).MinTimes(1)
	mockLogger.EXPECT().Info("Creating MinIO Core client", nil, gomock.Any()).Times(1)
	mockLogger.EXPECT().Info("Bucket does not exist, creating it", nil, gomock.Any()).Times(1)
	mockLogger.EXPECT().Info("Successfully created bucket", nil, gomock.Any()).Times(1)
	mockLogger.EXPECT().Info("closing minio client...", nil, gomock.Any()).Times(1)
	mockLogger.EXPECT().Info("Stopping MinIO connection retry loop due to shutdown signal", nil, gomock.Any()).Times(1)
	mockLogger.EXPECT().Error("failed to close object reader", gomock.Any(), gomock.Any()).AnyTimes()

	// Wait for MinIO to be ready
	err = waitForMinioReady(host, port, 10*time.Second)
	require.NoError(t, err)

	// Create MinIO client config
	cfg := Config{
		Connection: ConnectionConfig{
			Endpoint:             fmt.Sprintf("%s:%s", host, port),
			AccessKeyID:          "minio_admin",
			SecretAccessKey:      "minio_admin",
			UseSSL:               false,
			BucketName:           "test-bucket",
			Region:               "us-east-1",
			AccessBucketCreation: true,
		},
		UploadConfig: UploadConfig{
			MinPartSize: 5 * 1024 * 1024, // 5MB
		},
		DownloadConfig: DownloadConfig{
			SmallFileThreshold: 1 * 1024 * 1024, // 1MB
			InitialBufferSize:  1 * 1024 * 1024, // 1MB initial buffer for large files
		},
	}

	var client *Minio

	app := fxtest.New(t,
		FXModule,
		fx.Provide(
			func() Config {
				return cfg
			},
			func() Logger {
				return mockLogger
			},
		),
		fx.Populate(&client),
	)

	require.NoError(t, app.Start(ctx))
	defer func() {
		require.NoError(t, app.Stop(ctx))
	}()

	// Define test content strings first to ensure consistency
	smallContent := "This is a small test file"
	smallUnknownContent := "This is a small test file with unknown size"
	emptyContent := ""

	// Create medium and large data at once
	mediumData := bytes.Repeat([]byte("1234567890"), 10000) // 100KB
	largeFileSize := int64(100 * 1024 * 1024)               // 100MB
	largeData := bytes.Repeat([]byte("L"), int(largeFileSize))

	// Test cases
	testCases := []struct {
		name           string
		objectKey      string
		getContent     func() (io.Reader, string) // Returns reader and the expected content string
		size           []int64
		expectedSize   int64
		verifyFullData bool
	}{
		{
			name:      "Small file with known size",
			objectKey: "small-file.txt",
			getContent: func() (io.Reader, string) {
				return strings.NewReader(smallContent), smallContent
			},
			size:           []int64{int64(len(smallContent))},
			expectedSize:   int64(len(smallContent)),
			verifyFullData: true,
		},
		{
			name:      "Small file with unknown size",
			objectKey: "small-file-unknown-size.txt",
			getContent: func() (io.Reader, string) {
				return strings.NewReader(smallUnknownContent), smallUnknownContent
			},
			size:           []int64{},
			expectedSize:   int64(len(smallUnknownContent)),
			verifyFullData: true,
		},
		{
			name:      "Zero-sized file",
			objectKey: "zero-sized-file.txt",
			getContent: func() (io.Reader, string) {
				return strings.NewReader(emptyContent), emptyContent
			},
			size:           []int64{0},
			expectedSize:   0,
			verifyFullData: true,
		},
		{
			name:      "Medium file with known size",
			objectKey: "medium-file.bin",
			getContent: func() (io.Reader, string) {
				return bytes.NewReader(mediumData), string(mediumData)
			},
			size:           []int64{int64(len(mediumData))},
			expectedSize:   int64(len(mediumData)),
			verifyFullData: true,
		},
		{
			name:      "Medium file with unknown size",
			objectKey: "medium-file-unknown-size.bin",
			getContent: func() (io.Reader, string) {
				return bytes.NewReader(mediumData), string(mediumData)
			},
			size:           []int64{},
			expectedSize:   int64(len(mediumData)),
			verifyFullData: true,
		},
		{
			name:      "Large file with known size",
			objectKey: "large-file.bin",
			getContent: func() (io.Reader, string) {
				return bytes.NewReader(largeData), string(largeData)
			},
			size:           []int64{largeFileSize},
			expectedSize:   largeFileSize,
			verifyFullData: true,
		},
		{
			name:      "Large file with unknown size",
			objectKey: "large-file-unknown-size.bin",
			getContent: func() (io.Reader, string) {
				return bytes.NewReader(largeData), string(largeData)
			},
			size:           []int64{},
			expectedSize:   largeFileSize,
			verifyFullData: true,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Get fresh reader and expected content for this test case
			reader, expectedContent := tc.getContent()

			// Upload the object using your custom Put method
			size, err := client.Put(ctx, tc.objectKey, reader, tc.size...)
			require.NoError(t, err)

			// Verify the uploaded size is correct
			require.Equal(t, tc.expectedSize, size, "Uploaded size doesn't match expected size")

			// Get the object to verify it was uploaded correctly using the new Get method
			data, err := client.Get(ctx, tc.objectKey)
			require.NoError(t, err)

			// Verify the size
			require.Equal(t, tc.expectedSize, int64(len(data)), "Downloaded size doesn't match expected size")

			// For smaller files, also verify the content
			if tc.verifyFullData {
				actualContent := string(data)
				t.Logf("Expected content length: %d, Actual content length: %d", len(expectedContent), len(actualContent))

				// Print content in hex if they don't match (to detect invisible characters)
				if expectedContent != actualContent {
					t.Logf("Expected content (hex): %x", []byte(expectedContent))
					t.Logf("Actual content (hex): %x", []byte(actualContent))
				}

				require.Equal(t, expectedContent, actualContent, "Content verification failed")
			}
		})
	}

	require.NoError(t, containerInstance.Terminate(ctx))
}

// TestMinioReconnection tests the automatic reconnection functionality
// when the MinIO server goes down and comes back up
func TestMinioReconnection(t *testing.T) {
	// Set up context with cancellation capability
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start a MinIO container
	containerInstance, host, port, err := createMinIOContainer(ctx)
	require.NoError(t, err)

	// Create mock controller and logger
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockLogger := NewMockLogger(ctrl)

	// Set logger expectations (more flexible to accommodate reconnection logs)
	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Warn("MinIO connection issue detected, attempting reconnection",
		gomock.Any(), gomock.Any()).MinTimes(1)
	mockLogger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	// Wait for MinIO to be ready
	err = waitForMinioReady(host, port, 10*time.Second)
	require.NoError(t, err)

	// Create MinIO client config with a shorter monitoring interval for testing
	cfg := Config{
		Connection: ConnectionConfig{
			Endpoint:             fmt.Sprintf("%s:%s", host, port),
			AccessKeyID:          "minio_admin",
			SecretAccessKey:      "minio_admin",
			UseSSL:               false,
			BucketName:           "test-bucket",
			Region:               "us-east-1",
			AccessBucketCreation: true,
		},
		UploadConfig: UploadConfig{
			MinPartSize: 5 * 1024 * 1024, // 5MB
		},
		DownloadConfig: DownloadConfig{
			SmallFileThreshold: 1 * 1024 * 1024, // 1MB
			InitialBufferSize:  1 * 1024 * 1024, // 1MB initial buffer
		},
	}

	// Create the app with the MinIO client
	var client *Minio
	app := fxtest.New(t,
		FXModule,
		fx.Provide(
			func() Config {
				return cfg
			},
			func() Logger {
				return mockLogger
			},
		),
		fx.Populate(&client),
	)

	require.NoError(t, app.Start(ctx))
	defer func() {
		require.NoError(t, app.Stop(ctx))
	}()

	// Prepare test content
	preRestartContent := "Content before server restart"
	postRestartContent := "Content after server restart"
	preRestartKey := "pre-restart.txt"
	postRestartKey := "post-restart.txt"

	// Upload initial content before server restart
	size, err := client.Put(ctx, preRestartKey, strings.NewReader(preRestartContent), int64(len(preRestartContent)))
	require.NoError(t, err)
	require.Equal(t, int64(len(preRestartContent)), size)

	// Verify the initial upload
	data, err := client.Get(ctx, preRestartKey)
	require.NoError(t, err)
	require.Equal(t, preRestartContent, string(data))
	t.Log("Successfully verified pre-restart upload before server restart")

	// Determine if the container setup uses persistent storage
	// This information should come from your actual implementation
	isPersistentStorage := false // Set this based on your actual container setup
	t.Logf("Container is using %s storage", map[bool]string{false: "ephemeral", true: "persistent"}[isPersistentStorage])

	// Create a channel to signal when the server is restarted
	serverRestarted := make(chan struct{})

	// Create a channel to signal when the upload completes
	uploadComplete := make(chan struct{})

	// Stop the container to simulate a server outage
	t.Log("Stopping MinIO container to simulate server outage")
	err = containerInstance.Stop(ctx, nil)
	require.NoError(t, err)

	// Start a goroutine to restart the server after a delay
	go func() {
		// Wait before restarting the server
		time.Sleep(8 * time.Second)

		t.Log("Restarting MinIO container")
		err := containerInstance.Start(ctx)
		if err != nil {
			t.Logf("Failed to restart container: %v", err)
			return
		}

		// Wait for MinIO to be ready after restart
		err = waitForMinioReady(host, port, 15*time.Second)
		if err != nil {
			t.Logf("MinIO not ready after restart: %v", err)
			return
		}

		t.Log("MinIO container restarted and ready")
		close(serverRestarted)
	}()

	// Start uploading concurrently to test auto-reconnection during an operation
	go func() {
		// Wait a moment to ensure we're uploading while the server is down
		time.Sleep(2 * time.Second)

		t.Log("Attempting upload while server is down (should automatically reconnect)")
		// Try repeatedly until successful or timeout
		for attempt := 1; attempt <= 10; attempt++ {
			size, err := client.Put(ctx, postRestartKey, strings.NewReader(postRestartContent), int64(len(postRestartContent)))
			if err == nil {
				require.Equal(t, int64(len(postRestartContent)), size)
				t.Log("Upload succeeded after reconnection")
				close(uploadComplete)
				return
			}
			t.Logf("Upload attempt %d failed: %v (expected during outage)", attempt, err)
			time.Sleep(2 * time.Second)
		}
		t.Log("Upload failed after maximum attempts")
		close(uploadComplete)
	}()

	// Wait for upload to complete with timeout
	select {
	case <-uploadComplete:
		t.Log("Upload operation completed (success or failure)")
	case <-time.After(30 * time.Second):
		t.Fatal("Test timed out waiting for upload completion")
	}

	// Wait for the server to fully restart
	select {
	case <-serverRestarted:
		t.Log("Server restart confirmed")
	case <-time.After(20 * time.Second):
		t.Log("Warning: Server restart signal not received within timeout")
	}

	// Additional wait to ensure reconnection completes
	time.Sleep(5 * time.Second)

	// Check for a pre-restart file (an expected result depends on storage persistence)
	preRestartData, err := client.Get(ctx, preRestartKey)
	if isPersistentStorage {
		// For persistent storage, a pre-restart file should be available
		require.NoError(t, err, "Pre-restart file should be available with persistent storage")
		require.Equal(t, preRestartContent, string(preRestartData),
			"Content verification failed for pre-restart file")
		t.Log("Successfully retrieved pre-restart file after reconnection (persistent storage)")
	} else {
		// For ephemeral storage, we expect the file to be gone, but don't fail the test on this
		if err == nil {
			t.Log("Note: Pre-restart file still exists after restart (unexpected with ephemeral storage)")
			require.Equal(t, preRestartContent, string(preRestartData),
				"Content verification failed for pre-restart file")
		} else {
			t.Log("Pre-restart file not available after restart as expected with ephemeral storage")
		}

		// If a file not found, re-upload it for next tests
		if err != nil {
			_, err = client.Put(ctx, preRestartKey, strings.NewReader(preRestartContent), int64(len(preRestartContent)))
			require.NoError(t, err, "Re-upload of pre-restart file should succeed")
			t.Log("Re-uploaded pre-restart file")
		}
	}

	// Verify file uploaded during reconnection attempts
	postRestartData, err := client.Get(ctx, postRestartKey)
	require.NoError(t, err, "Should be able to download file uploaded during reconnection")
	require.Equal(t, postRestartContent, string(postRestartData),
		"Content verification failed for post-restart file")
	t.Log("Successfully retrieved post-restart file")

	// Test creating a new file after confirmed reconnection
	finalContent := "Final content after confirmed reconnection"
	finalKey := "final-file.txt"

	size, err = client.Put(ctx, finalKey, strings.NewReader(finalContent), int64(len(finalContent)))
	require.NoError(t, err, "Upload after confirmed reconnection should succeed")
	require.Equal(t, int64(len(finalContent)), size)

	// Verify the final file
	finalData, err := client.Get(ctx, finalKey)
	require.NoError(t, err)
	require.Equal(t, finalContent, string(finalData))
	t.Log("Successfully created and retrieved final file after reconnection")

	require.NoError(t, containerInstance.Terminate(ctx))
}

// TestPreSignedPut tests the generation of pre-signed URLs for PUT operations
// with both small and large files
func TestPreSignedPut(t *testing.T) {
	// Set up context with cancellation capability
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start MinIO container
	containerInstance, host, port, err := createMinIOContainer(ctx)
	require.NoError(t, err)
	defer func() {
		if containerInstance != nil {
			_ = containerInstance.Terminate(ctx)
		}
	}()

	// Wait for MinIO to be ready
	err = waitForMinioReady(host, port, 10*time.Second)
	require.NoError(t, err)

	// Create mock controller and logger
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockLogger := NewMockLogger(ctrl)

	// Set logger expectations
	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	// Define file sizes for testing
	smallFileSize := int64(100 * 1024)        // 100KB
	largeFileSize := int64(100 * 1024 * 1024) // 100MB

	// Run test cases
	testCases := []struct {
		name        string
		config      Config
		objectKey   string
		fileSize    int64
		expectError bool
		validateURL func(t *testing.T, url string)
	}{
		{
			name: "Standard PreSigned URL - Small File",
			config: Config{
				Connection: ConnectionConfig{
					Endpoint:             fmt.Sprintf("%s:%s", host, port),
					AccessKeyID:          "minio_admin",
					SecretAccessKey:      "minio_admin",
					UseSSL:               false,
					BucketName:           "test-bucket",
					Region:               "us-east-1",
					AccessBucketCreation: true,
				},
				PresignedConfig: PresignedConfig{
					ExpiryDuration: 15 * time.Minute,
				},
			},
			objectKey:   "small-file.txt",
			fileSize:    smallFileSize,
			expectError: false,
			validateURL: func(t *testing.T, url string) {
				require.Contains(t, url, "test-bucket", "URL should contain the bucket name")
				require.Contains(t, url, "small-file.txt", "URL should contain the object key")
				require.Contains(t, url, "X-Amz-Signature=", "URL should contain signature")
			},
		},
		{
			name: "Standard PreSigned URL - Large File",
			config: Config{
				Connection: ConnectionConfig{
					Endpoint:        fmt.Sprintf("%s:%s", host, port),
					AccessKeyID:     "minio_admin",
					SecretAccessKey: "minio_admin",
					UseSSL:          false,
					BucketName:      "test-bucket",
					Region:          "us-east-1",
				},
				PresignedConfig: PresignedConfig{
					ExpiryDuration: 30 * time.Minute, // Longer expiry for a large file
				},
			},
			objectKey:   "large-file.bin",
			fileSize:    largeFileSize,
			expectError: false,
			validateURL: func(t *testing.T, url string) {
				require.Contains(t, url, "test-bucket", "URL should contain the bucket name")
				require.Contains(t, url, "large-file.bin", "URL should contain the object key")
				require.Contains(t, url, "X-Amz-Signature=", "URL should contain signature")
			},
		},
		{
			name: "Empty Object Key",
			config: Config{
				Connection: ConnectionConfig{
					Endpoint:        fmt.Sprintf("%s:%s", host, port),
					AccessKeyID:     "minio_admin",
					SecretAccessKey: "minio_admin",
					UseSSL:          false,
					BucketName:      "test-bucket",
					Region:          "us-east-1",
				},
				PresignedConfig: PresignedConfig{
					ExpiryDuration: 15 * time.Minute,
				},
			},
			objectKey:   "",
			fileSize:    smallFileSize, // Not used for an error case
			expectError: true,          // Expecting an error for an empty object key
			validateURL: nil,           // No validation needed for the error case
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Skip large file tests if running in short mode
			if testing.Short() && tc.fileSize == largeFileSize {
				t.Skip("Skipping large file test in short mode")
			}

			// Create a client for this test case
			var client *Minio
			app := fxtest.New(t,
				FXModule,
				fx.Provide(
					func() Config {
						return tc.config
					},
					func() Logger {
						return mockLogger
					},
				),
				fx.Populate(&client),
			)

			require.NoError(t, app.Start(ctx))
			defer func() {
				require.NoError(t, app.Stop(ctx))
			}()

			// Generate pre-signed URL
			url, err := client.PreSignedPut(ctx, tc.objectKey)

			// Check error expectations
			if tc.expectError {
				require.Error(t, err, "Expected an error for test case: %s", tc.name)
				return
			}

			require.NoError(t, err, "Unexpected error for test case: %s", tc.name)
			require.NotEmpty(t, url, "URL should not be empty for test case: %s", tc.name)

			t.Logf("Generated URL: %s", url)

			// Validate URL if validation function provided
			if tc.validateURL != nil {
				tc.validateURL(t, url)
			}

			// Skip the empty object key test for the actual upload
			if tc.objectKey == "" {
				return
			}

			// Verify the pre-signed URL works for uploading
			httpClient := &http.Client{
				Timeout: 5 * time.Minute, // Extended timeout for large file uploads
			}

			// Prepare test data based on file size
			var testData io.Reader
			var expectedSize int64

			if tc.fileSize == smallFileSize {
				// For small files, use a simple string pattern
				testContent := generateTestContent(tc.fileSize)
				testData = strings.NewReader(testContent)
				expectedSize = int64(len(testContent))
				t.Logf("Using generated test content of %d bytes", expectedSize)
			} else {
				// For large files, use a deterministic random data generator
				testData = newRandomDataReader(tc.fileSize)
				expectedSize = tc.fileSize
				t.Logf("Using random data generator for %d bytes", expectedSize)
			}

			// Log upload start
			t.Logf("Starting upload for %s (%d bytes)", tc.name, tc.fileSize)
			startTime := time.Now()

			// Create a request using the pre-signed URL
			req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, testData)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			// Set Content-Length header - crucial for large file uploads
			req.ContentLength = expectedSize

			// Execute the request
			resp, err := httpClient.Do(req)
			if err != nil {
				t.Fatalf("Failed to execute PUT request: %v", err)
			}
			defer func(Body io.ReadCloser) {
				err := Body.Close()
				if err != nil {
					t.Logf("Failed to close response body: %v", err)
				}
			}(resp.Body)

			respBody, _ := io.ReadAll(resp.Body)

			// Log upload completion
			uploadDuration := time.Since(startTime)
			t.Logf("Upload request completed in %v with status %d", uploadDuration, resp.StatusCode)
			if resp.StatusCode != http.StatusOK {
				t.Logf("Response body: %s", string(respBody))
			}

			// Check response status
			require.Equal(t, http.StatusOK, resp.StatusCode, "PUT request should return OK")

			// Verify the object exists and has the correct size
			waitForObjectToBeAvailable(t, ctx, client, tc.objectKey, expectedSize)

			// For small files, verify content integrity
			if tc.fileSize == smallFileSize {
				// Retrieve the object and verify its content
				data, err := client.Get(ctx, tc.objectKey)
				require.NoError(t, err, "Failed to retrieve uploaded object")
				require.Equal(t, expectedSize, int64(len(data)), "Retrieved data size should match expected size")
				t.Logf("Successfully verified data integrity for small file")
			}
		})
	}
}

// TestGenerateMultipartUploadURLs tests the functionality of generating presigned URLs
// for multipart uploads with various file sizes and configurations
func TestGenerateMultipartUploadURLs(t *testing.T) {
	// Set up context with cancellation capability
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start MinIO container
	containerInstance, host, port, err := createMinIOContainer(ctx)
	require.NoError(t, err)
	defer func() {
		if containerInstance != nil {
			_ = containerInstance.Terminate(ctx)
		}
	}()

	// Wait for MinIO to be ready
	err = waitForMinioReady(host, port, 10*time.Second)
	require.NoError(t, err)

	// Create mock controller and logger
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockLogger := NewMockLogger(ctrl)

	// Set logger expectations
	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	// Define file sizes for testing
	smallFileSize := int64(10 * 1024 * 1024)   // 10MB
	mediumFileSize := int64(100 * 1024 * 1024) // 100MB
	largeFileSize := int64(1024 * 1024 * 1024) // 1GB

	// Run test cases
	testCases := []struct {
		name            string
		config          Config
		objectKey       string
		fileSize        int64
		contentType     string
		customExpiry    time.Duration
		expectError     bool
		validateInfo    func(t *testing.T, upload MultipartUpload)
		validateUpload  bool // Whether to test actual upload using the URLs
		skipInShortMode bool
	}{
		{
			name: "Basic Multipart Upload",
			config: Config{
				Connection: ConnectionConfig{
					Endpoint:             fmt.Sprintf("%s:%s", host, port),
					AccessKeyID:          "minio_admin",
					SecretAccessKey:      "minio_admin",
					UseSSL:               false,
					BucketName:           "test-bucket",
					Region:               "us-east-1",
					AccessBucketCreation: true,
				},
				UploadConfig: UploadConfig{
					MinPartSize:        5 * 1024 * 1024,               // 5MB
					MultipartThreshold: 10 * 1024 * 1024,              // 10MB
					MaxObjectSize:      5 * 1024 * 1024 * 1024 * 1024, // 5TB
				},
				PresignedConfig: PresignedConfig{
					ExpiryDuration: 15 * time.Minute,
				},
			},
			objectKey:      "test-multipart-small.dat",
			fileSize:       smallFileSize,
			contentType:    "application/octet-stream",
			validateUpload: true,
			validateInfo: func(t *testing.T, upload MultipartUpload) {
				require.NotEmpty(t, upload.GetUploadID(), "Upload ID should not be empty")
				require.Equal(t, "test-multipart-small.dat", upload.GetObjectKey())
				require.Equal(t, smallFileSize, upload.GetTotalSize())
				require.Equal(t, "application/octet-stream", upload.GetContentType())

				// Check the number of parts (10MB file with 5MB min part size = 2 parts)
				require.Equal(t, 2, len(upload.GetPresignedURLs()))
				require.Equal(t, 2, len(upload.GetPartNumbers()))

				// Check that part numbers are sequential
				partNumbers := upload.GetPartNumbers()
				require.Equal(t, 1, partNumbers[0])
				require.Equal(t, 2, partNumbers[1])

				// Check expiry time is in the future
				require.Greater(t, upload.GetExpiryTimestamp(), time.Now().Unix())

				// Check recommended part size
				require.Equal(t, int64(5*1024*1024), upload.GetRecommendedPartSize())

				// Check max parts
				require.Equal(t, 10000, upload.GetMaxParts())

				// Check URLs contain necessary query parameters
				urls := upload.GetPresignedURLs()
				for i, url := range urls {
					require.Contains(t, url, "partNumber="+strconv.Itoa(i+1))
					require.Contains(t, url, "uploadId="+upload.GetUploadID())
					require.Contains(t, url, "X-Amz-Signature=")
				}

				// Test IsExpired() function
				require.False(t, upload.IsExpired(), "Upload should not be expired yet")
			},
		},
		{
			name: "Medium File Multipart Upload with Custom Expiry",
			config: Config{
				Connection: ConnectionConfig{
					Endpoint:             fmt.Sprintf("%s:%s", host, port),
					AccessKeyID:          "minio_admin",
					SecretAccessKey:      "minio_admin",
					UseSSL:               false,
					BucketName:           "test-bucket",
					Region:               "us-east-1",
					AccessBucketCreation: true,
				},
				UploadConfig: UploadConfig{
					MinPartSize:        10 * 1024 * 1024,              // 10MB
					MultipartThreshold: 5 * 1024 * 1024,               // 5MB
					MaxObjectSize:      5 * 1024 * 1024 * 1024 * 1024, // 5TB
				},
				PresignedConfig: PresignedConfig{
					ExpiryDuration: 15 * time.Minute, // Default, but we'll override
				},
			},
			objectKey:      "test-multipart-medium.dat",
			fileSize:       mediumFileSize,
			contentType:    "application/binary",
			customExpiry:   60 * time.Minute, // Custom 1-hour expiry
			validateUpload: true,
			validateInfo: func(t *testing.T, upload MultipartUpload) {
				// Check that expiry time is approximately 1 hour in the future
				expectedExpiry := time.Now().Add(60 * time.Minute).Unix()
				require.InDelta(t, expectedExpiry, upload.GetExpiryTimestamp(), 5) // Allow 5-second tolerance

				// Check URL count (100MB with 10MB parts = 10 parts)
				require.Equal(t, 10, len(upload.GetPresignedURLs()))
			},
		},
		{
			name: "Large File Multipart Upload",
			config: Config{
				Connection: ConnectionConfig{
					Endpoint:             fmt.Sprintf("%s:%s", host, port),
					AccessKeyID:          "minio_admin",
					SecretAccessKey:      "minio_admin",
					UseSSL:               false,
					BucketName:           "test-bucket",
					Region:               "us-east-1",
					AccessBucketCreation: true,
				},
				UploadConfig: UploadConfig{
					MinPartSize:        16 * 1024 * 1024,              // 16MB
					MultipartThreshold: 5 * 1024 * 1024,               // 5MB
					MaxObjectSize:      5 * 1024 * 1024 * 1024 * 1024, // 5TB
				},
				PresignedConfig: PresignedConfig{
					ExpiryDuration: 30 * time.Minute,
				},
			},
			objectKey:      "test-multipart-large.dat",
			fileSize:       largeFileSize,
			contentType:    "application/binary",
			validateUpload: true, // Skip actual upload for a large file to save time
			validateInfo: func(t *testing.T, upload MultipartUpload) {
				// Check URL count (1GB with 16MB parts = 64 parts)
				require.Equal(t, 64, len(upload.GetPresignedURLs()))
				require.Equal(t, 64, len(upload.GetPartNumbers()))
			},
			skipInShortMode: true,
		},
		{
			name: "With Custom Base URL",
			config: Config{
				Connection: ConnectionConfig{
					Endpoint:             fmt.Sprintf("%s:%s", host, port),
					AccessKeyID:          "minio_admin",
					SecretAccessKey:      "minio_admin",
					UseSSL:               false,
					BucketName:           "test-bucket",
					Region:               "us-east-1",
					AccessBucketCreation: true,
				},
				UploadConfig: UploadConfig{
					MinPartSize:        5 * 1024 * 1024,               // 5MB
					MultipartThreshold: 5 * 1024 * 1024,               // 5MB
					MaxObjectSize:      5 * 1024 * 1024 * 1024 * 1024, // 5TB
				},
				PresignedConfig: PresignedConfig{
					ExpiryDuration: 15 * time.Minute,
					BaseURL:        "https://custom-endpoint.example.com",
				},
			},
			objectKey:      "test-multipart-custom-url.dat",
			fileSize:       smallFileSize,
			contentType:    "application/octet-stream",
			validateUpload: false, // Skip upload as custom URL won't be reachable
			validateInfo: func(t *testing.T, upload MultipartUpload) {
				// Check that URLs use the custom domain
				urls := upload.GetPresignedURLs()
				for _, url := range urls {
					require.Contains(t, url, "https://custom-endpoint.example.com")
					require.NotContains(t, url, host)
				}
			},
		},
		{
			name: "Error - Empty Object Key",
			config: Config{
				Connection: ConnectionConfig{
					Endpoint:             fmt.Sprintf("%s:%s", host, port),
					AccessKeyID:          "minio_admin",
					SecretAccessKey:      "minio_admin",
					UseSSL:               false,
					BucketName:           "test-bucket",
					AccessBucketCreation: true,
				},
				UploadConfig: UploadConfig{
					MinPartSize:        5 * 1024 * 1024,
					MultipartThreshold: 5 * 1024 * 1024,
					MaxObjectSize:      5 * 1024 * 1024 * 1024 * 1024,
				},
			},
			objectKey:      "",
			fileSize:       smallFileSize,
			expectError:    true,
			validateUpload: false,
		},
		{
			name: "Error - Exceeds Maximum Object Size",
			config: Config{
				Connection: ConnectionConfig{
					Endpoint:             fmt.Sprintf("%s:%s", host, port),
					AccessKeyID:          "minio_admin",
					SecretAccessKey:      "minio_admin",
					UseSSL:               false,
					BucketName:           "test-bucket",
					AccessBucketCreation: true,
				},
				UploadConfig: UploadConfig{
					MinPartSize:        5 * 1024 * 1024,
					MultipartThreshold: 5 * 1024 * 1024,
					MaxObjectSize:      50 * 1024 * 1024, // 50MB max
				},
			},
			objectKey:      "too-large.dat",
			fileSize:       100 * 1024 * 1024, // 100MB (exceeds max)
			expectError:    true,
			validateUpload: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Skip large file tests in short mode
			if testing.Short() && tc.skipInShortMode {
				t.Skip("Skipping large file test in short mode")
			}

			// Create a client for this test case
			var client *Minio
			app := fxtest.New(t,
				FXModule,
				fx.Provide(
					func() Config {
						return tc.config
					},
					func() Logger {
						return mockLogger
					},
				),
				fx.Populate(&client),
			)

			require.NoError(t, app.Start(ctx))
			defer func() {
				require.NoError(t, app.Stop(ctx))
			}()

			// Generate multipart upload URLs
			var upload MultipartUpload
			var err error

			// Call with custom expiry if specified
			if tc.customExpiry > 0 {
				upload, err = client.GenerateMultipartUploadURLs(
					ctx, tc.objectKey, tc.fileSize, tc.contentType, tc.customExpiry)
			} else {
				upload, err = client.GenerateMultipartUploadURLs(
					ctx, tc.objectKey, tc.fileSize, tc.contentType)
			}

			// Verify error handling
			if tc.expectError {
				require.Error(t, err, "Expected an error for test case: %s", tc.name)
				require.Nil(t, upload, "Upload should be nil when an error occurs")
				return
			}

			require.NoError(t, err, "Failed to generate multipart upload URLs")
			require.NotNil(t, upload, "MultipartUpload should not be nil")

			// Log the upload ID for reference
			t.Logf("Generated multipart upload with ID: %s", upload.GetUploadID())
			t.Logf("Total URLs generated: %d", len(upload.GetPresignedURLs()))

			// Validate the returned info
			if tc.validateInfo != nil {
				tc.validateInfo(t, upload)
			}

			// Skip the actual upload test if not required
			if !tc.validateUpload {
				// Cleanup: abort the multipart upload
				err = client.AbortMultipartUpload(ctx, upload.GetObjectKey(), upload.GetUploadID())
				require.NoError(t, err, "Failed to abort multipart upload")
				return
			}

			// Test uploading a small number of parts (just the first 2 parts for any file size)
			urls := upload.GetPresignedURLs()
			partNumbers := upload.GetPartNumbers()
			numPartsToTest := min(20, len(urls))

			// Instead of using CompletePart struct, track part numbers and ETags separately
			uploadPartNumbers := make([]int, 0, numPartsToTest)
			uploadPartETags := make([]string, 0, numPartsToTest)

			httpClient := &http.Client{Timeout: 30 * time.Second}

			// Upload each test part
			for i := 0; i < numPartsToTest; i++ {
				partNumber := partNumbers[i]

				// Calculate part size
				var partSize int64
				if i == len(urls)-1 {
					// For the last part, might be smaller
					partSize = tc.fileSize - int64(i)*upload.GetRecommendedPartSize()
				} else {
					partSize = upload.GetRecommendedPartSize()
				}

				// Generate test data for this part
				partData := make([]byte, partSize)
				for j := range partData {
					partData[j] = byte(j % 256) // Simple pattern
				}

				// Create a request to upload this part
				req, err := http.NewRequestWithContext(
					ctx, "PUT", urls[i], bytes.NewReader(partData))
				require.NoError(t, err, "Failed to create request for part %d", partNumber)

				// Set content length explicitly
				req.ContentLength = partSize

				// Upload the part
				t.Logf("Uploading part %d of %d (size: %d bytes)",
					partNumber, numPartsToTest, partSize)
				resp, err := httpClient.Do(req)
				require.NoError(t, err, "Failed to upload part %d", partNumber)

				// Get ETag from response header
				etag := resp.Header.Get("ETag")
				require.NotEmpty(t, etag, "ETag missing from response for part %d", partNumber)

				// Remove quotes from ETag
				etag = strings.Trim(etag, "\"")

				// Store part info for completion (using separate slices)
				uploadPartNumbers = append(uploadPartNumbers, partNumber)
				uploadPartETags = append(uploadPartETags, etag)

				// Check status code
				require.Equal(t, http.StatusOK, resp.StatusCode,
					"Part upload failed with status %d", resp.StatusCode)

				_ = resp.Body.Close()
				t.Logf("Successfully uploaded part %d with ETag: %s", partNumber, etag)
			}

			// Complete the multipart upload using the new interface with separate slices
			err = client.CompleteMultipartUpload(ctx, upload.GetObjectKey(), upload.GetUploadID(), uploadPartNumbers, uploadPartETags)
			require.NoError(t, err, "Failed to complete multipart upload")

			// Verify the object exists
			objInfo, err := client.Client.StatObject(ctx, client.cfg.Connection.BucketName,
				upload.GetObjectKey(), minio.StatObjectOptions{})
			require.NoError(t, err, "Object not found after upload")
			t.Logf("Successfully completed multipart upload: %s (size: %d bytes)",
				objInfo.Key, objInfo.Size)

			// Cleanup
			err = client.Delete(ctx, upload.GetObjectKey())
			require.NoError(t, err, "Failed to clean up test object")
		})
	}
}

// Generate a string of the specified size using a repeating pattern
func generateTestContent(size int64) string {
	pattern := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	var builder strings.Builder
	builder.Grow(int(size))

	for i := int64(0); i < size; i++ {
		builder.WriteByte(pattern[i%int64(len(pattern))])
	}

	return builder.String()
}

// randomDataReader provides a deterministic stream of random data
type randomDataReader struct {
	size      int64
	remaining int64
	rng       *rand.Rand
}

// Create a new random data reader with the specified size
func newRandomDataReader(size int64) *randomDataReader {
	// Use a fixed seed for deterministic output
	return &randomDataReader{
		size:      size,
		remaining: size,
		rng:       rand.New(rand.NewSource(42)),
	}
}

// Implement io.Reader interface
func (r *randomDataReader) Read(p []byte) (n int, err error) {
	if r.remaining <= 0 {
		return 0, io.EOF
	}

	toRead := int64(len(p))
	if toRead > r.remaining {
		toRead = r.remaining
	}

	for i := int64(0); i < toRead; i++ {
		p[i] = byte(r.rng.Intn(256))
	}

	r.remaining -= toRead
	return int(toRead), nil
}

// Helper function to wait for an object to be available with the correct size
func waitForObjectToBeAvailable(t *testing.T, ctx context.Context, client *Minio, objectKey string, expectedSize int64) {
	t.Helper()

	maxRetries := 10
	retryDelay := 2 * time.Second

	for i := 0; i < maxRetries; i++ {
		objInfo, err := client.Client.StatObject(ctx, client.cfg.Connection.BucketName, objectKey, minio.StatObjectOptions{})
		if err == nil && objInfo.Size == expectedSize {
			t.Logf("Object verified with correct size of %d bytes", objInfo.Size)
			return
		}

		if err != nil {
			t.Logf("Retry %d/%d: Object not yet available: %v", i+1, maxRetries, err)
		} else {
			t.Logf("Size mismatch. Expected: %d, Actual: %d", expectedSize, objInfo.Size)
		}

		if i < maxRetries-1 {
			time.Sleep(retryDelay)
		}
	}

	t.Fatalf("Object verification failed after %d retries", maxRetries)
}

// TestGenerateMultipartPresignedGetURLs tests the functionality of generating presigned URLs
// for multipart downloads with various file sizes and configurations
func TestGenerateMultipartPresignedGetURLs(t *testing.T) {
	// Set up context with cancellation capability
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start MinIO container
	containerInstance, host, port, err := createMinIOContainer(ctx)
	require.NoError(t, err)
	defer func() {
		if containerInstance != nil {
			_ = containerInstance.Terminate(ctx)
		}
	}()

	// Wait for MinIO to be ready
	err = waitForMinioReady(host, port, 10*time.Second)
	require.NoError(t, err)

	// Create mock controller and logger
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockLogger := NewMockLogger(ctrl)

	// Set logger expectations
	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	// Define file sizes for testing
	smallFileSize := int64(10 * 1024 * 1024)   // 10MB
	mediumFileSize := int64(100 * 1024 * 1024) // 100MB
	largeFileSize := int64(256 * 1024 * 1024)  // 256MB (to keep test duration reasonable)

	// Run test cases
	testCases := []struct {
		name             string
		config           Config
		objectKey        string
		fileSize         int64
		contentType      string
		partSize         int64
		customExpiry     time.Duration
		expectError      bool
		validateInfo     func(t *testing.T, download MultipartPresignedGet)
		validateDownload bool // Whether to test actual download using the URLs
		skipInShortMode  bool
	}{
		{
			name: "Basic Multipart Download",
			config: Config{
				Connection: ConnectionConfig{
					Endpoint:             fmt.Sprintf("%s:%s", host, port),
					AccessKeyID:          "minio_admin",
					SecretAccessKey:      "minio_admin",
					UseSSL:               false,
					BucketName:           "test-bucket",
					Region:               "us-east-1",
					AccessBucketCreation: true,
				},
				PresignedConfig: PresignedConfig{
					ExpiryDuration: 15 * time.Minute,
				},
			},
			objectKey:        "test-multipart-get-small.dat",
			fileSize:         smallFileSize,
			contentType:      "application/octet-stream",
			partSize:         5 * 1024 * 1024, // 5MB parts
			validateDownload: true,
			validateInfo: func(t *testing.T, download MultipartPresignedGet) {
				require.Equal(t, "test-multipart-get-small.dat", download.GetObjectKey())
				require.Equal(t, smallFileSize, download.GetTotalSize())
				require.Equal(t, "application/octet-stream", download.GetContentType())

				// Check URL and range count (10MB file with 5MB parts = 2 parts)
				urls := download.GetPresignedURLs()
				ranges := download.GetPartRanges()
				require.Equal(t, 2, len(urls))
				require.Equal(t, 2, len(ranges))

				// Check that ranges are correct
				require.Equal(t, "bytes=0-5242879", ranges[0])
				require.Equal(t, fmt.Sprintf("bytes=5242880-%d", smallFileSize-1), ranges[1])

				// Check expiry time is in the future
				require.Greater(t, download.GetExpiryTimestamp(), time.Now().Unix())

				// Check URLs contain necessary query parameters
				for _, url := range urls {
					require.Contains(t, url, "X-Amz-Signature=")
					require.Contains(t, url, "response-content-range=bytes")
				}

				// Test IsExpired() function
				require.False(t, download.IsExpired(), "Download URLs should not be expired yet")
			},
		},
		{
			name: "Medium File Download with Custom Expiry",
			config: Config{
				Connection: ConnectionConfig{
					Endpoint:             fmt.Sprintf("%s:%s", host, port),
					AccessKeyID:          "minio_admin",
					SecretAccessKey:      "minio_admin",
					UseSSL:               false,
					BucketName:           "test-bucket",
					Region:               "us-east-1",
					AccessBucketCreation: true,
				},
				PresignedConfig: PresignedConfig{
					ExpiryDuration: 15 * time.Minute, // Default, but we'll override
				},
			},
			objectKey:        "test-multipart-get-medium.dat",
			fileSize:         mediumFileSize,
			contentType:      "application/binary",
			partSize:         10 * 1024 * 1024, // 10MB parts
			customExpiry:     60 * time.Minute, // Custom 1-hour expiry
			validateDownload: true,
			validateInfo: func(t *testing.T, download MultipartPresignedGet) {
				// Check that expiry time is approximately 1 hour in the future
				expectedExpiry := time.Now().Add(60 * time.Minute).Unix()
				require.InDelta(t, expectedExpiry, download.GetExpiryTimestamp(), 5) // Allow 5-second tolerance

				// Check URL count (100MB with 10MB parts = 10 parts)
				require.Equal(t, 10, len(download.GetPresignedURLs()))
				require.Equal(t, 10, len(download.GetPartRanges()))

				// Check ETag is not empty
				require.NotEmpty(t, download.GetETag())
			},
		},
		{
			name: "Large File Download",
			config: Config{
				Connection: ConnectionConfig{
					Endpoint:             fmt.Sprintf("%s:%s", host, port),
					AccessKeyID:          "minio_admin",
					SecretAccessKey:      "minio_admin",
					UseSSL:               false,
					BucketName:           "test-bucket",
					Region:               "us-east-1",
					AccessBucketCreation: true,
				},
				PresignedConfig: PresignedConfig{
					ExpiryDuration: 30 * time.Minute,
				},
			},
			objectKey:        "test-multipart-get-large.dat",
			fileSize:         largeFileSize,
			contentType:      "application/binary",
			partSize:         16 * 1024 * 1024, // 16MB parts
			validateDownload: false,            // Skip full download for large file to save time
			validateInfo: func(t *testing.T, download MultipartPresignedGet) {
				// Check URL count (256MB with 16MB parts = 16 parts)
				require.Equal(t, 16, len(download.GetPresignedURLs()))
				require.Equal(t, 16, len(download.GetPartRanges()))

				// Validate just the first and last range
				ranges := download.GetPartRanges()
				require.Equal(t, "bytes=0-16777215", ranges[0])
				require.Equal(t, fmt.Sprintf("bytes=%d-%d", (len(ranges)-1)*16*1024*1024, largeFileSize-1), ranges[len(ranges)-1])
			},
			skipInShortMode: true,
		},
		{
			name: "With Custom Base URL",
			config: Config{
				Connection: ConnectionConfig{
					Endpoint:             fmt.Sprintf("%s:%s", host, port),
					AccessKeyID:          "minio_admin",
					SecretAccessKey:      "minio_admin",
					UseSSL:               false,
					BucketName:           "test-bucket",
					Region:               "us-east-1",
					AccessBucketCreation: true,
				},
				PresignedConfig: PresignedConfig{
					ExpiryDuration: 15 * time.Minute,
					BaseURL:        "https://custom-endpoint.example.com",
				},
			},
			objectKey:        "test-multipart-get-custom-url.dat",
			fileSize:         smallFileSize,
			partSize:         5 * 1024 * 1024, // 5MB parts
			contentType:      "application/octet-stream",
			validateDownload: false, // Skip download as custom URL won't be reachable
			validateInfo: func(t *testing.T, download MultipartPresignedGet) {
				// Check that URLs use the custom domain
				urls := download.GetPresignedURLs()
				for _, url := range urls {
					require.Contains(t, url, "https://custom-endpoint.example.com")
					require.NotContains(t, url, host)
				}
			},
		},
		{
			name: "Small Part Size (below minimum)",
			config: Config{
				Connection: ConnectionConfig{
					Endpoint:             fmt.Sprintf("%s:%s", host, port),
					AccessKeyID:          "minio_admin",
					SecretAccessKey:      "minio_admin",
					UseSSL:               false,
					BucketName:           "test-bucket",
					Region:               "us-east-1",
					AccessBucketCreation: true,
				},
				PresignedConfig: PresignedConfig{
					ExpiryDuration: 15 * time.Minute,
				},
			},
			objectKey:        "test-multipart-get-small-parts.dat",
			fileSize:         smallFileSize,
			contentType:      "application/octet-stream",
			partSize:         1 * 1024 * 1024, // 1MB parts (below minimum)
			validateDownload: false,
			validateInfo: func(t *testing.T, download MultipartPresignedGet) {
				// Should adjust to minimum part size (5MB)
				// 10MB file with 5MB parts = 2 parts
				require.Equal(t, 2, len(download.GetPresignedURLs()))
			},
		},
		{
			name: "Error - Object Doesn't Exist",
			config: Config{
				Connection: ConnectionConfig{
					Endpoint:             fmt.Sprintf("%s:%s", host, port),
					AccessKeyID:          "minio_admin",
					SecretAccessKey:      "minio_admin",
					UseSSL:               false,
					BucketName:           "test-bucket",
					AccessBucketCreation: true,
				},
				PresignedConfig: PresignedConfig{
					ExpiryDuration: 15 * time.Minute,
				},
			},
			objectKey:        "non-existent-object.dat",
			partSize:         5 * 1024 * 1024,
			expectError:      true,
			validateDownload: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Skip large file tests in short mode
			if testing.Short() && tc.skipInShortMode {
				t.Skip("Skipping large file test in short mode")
			}

			// Create a client for this test case
			var client *Minio
			app := fxtest.New(t,
				FXModule,
				fx.Provide(
					func() Config {
						return tc.config
					},
					func() Logger {
						return mockLogger
					},
				),
				fx.Populate(&client),
			)

			require.NoError(t, app.Start(ctx))
			defer func() {
				require.NoError(t, app.Stop(ctx))
			}()

			// Skip setup for the error test case with non-existent object
			if tc.objectKey != "non-existent-object.dat" {
				// Create and upload a test file first
				uploadTestFile(t, ctx, client, tc.objectKey, tc.fileSize, tc.contentType)
			}

			// Generate multipart download URLs
			var download MultipartPresignedGet
			var err error

			// Call with custom expiry if specified
			if tc.customExpiry > 0 {
				download, err = client.GenerateMultipartPresignedGetURLs(
					ctx, tc.objectKey, tc.partSize, tc.customExpiry)
			} else {
				download, err = client.GenerateMultipartPresignedGetURLs(
					ctx, tc.objectKey, tc.partSize)
			}

			// Verify error handling
			if tc.expectError {
				require.Error(t, err, "Expected an error for test case: %s", tc.name)
				require.Nil(t, download, "Download should be nil when an error occurs")
				return
			}

			require.NoError(t, err, "Failed to generate multipart download URLs")
			require.NotNil(t, download, "MultipartPresignedGet should not be nil")

			// Log basic info for reference
			t.Logf("Generated multipart download for object: %s", download.GetObjectKey())
			t.Logf("Total URLs generated: %d", len(download.GetPresignedURLs()))
			t.Logf("Total size: %d bytes", download.GetTotalSize())

			// Validate the returned info
			if tc.validateInfo != nil {
				tc.validateInfo(t, download)
			}

			// Skip the actual download test if not required
			if !tc.validateDownload {
				return
			}

			// Test downloading parts
			urls := download.GetPresignedURLs()
			ranges := download.GetPartRanges()

			// Create a byte slice to store the reassembled content
			totalBytes := make([]byte, download.GetTotalSize())
			var bytesDownloaded int64

			// Use a simple HTTP client for downloads
			httpClient := &http.Client{Timeout: 30 * time.Second}

			// Download each part
			for i, url := range urls {
				t.Logf("Downloading part %d of %d with range %s", i+1, len(urls), ranges[i])

				// Create request with range header
				req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
				require.NoError(t, err, "Failed to create request for part %d", i+1)
				req.Header.Set("Range", ranges[i])

				// Download the part
				resp, err := httpClient.Do(req)
				require.NoError(t, err, "Failed to download part %d", i+1)
				defer resp.Body.Close()

				// Verify response status code (should be 206 Partial Content)
				require.Equal(t, http.StatusPartialContent, resp.StatusCode,
					"Expected 206 Partial Content status for part %d, got %d",
					i+1, resp.StatusCode)

				// Parse the range to determine where in the buffer to put the data
				rangeStr := ranges[i]
				startEnd := strings.Split(strings.Split(rangeStr, "=")[1], "-")
				start, _ := strconv.ParseInt(startEnd[0], 10, 64)

				// Read the part data directly into the correct position in the buffer
				n, err := io.ReadFull(resp.Body, totalBytes[start:start+resp.ContentLength])
				require.NoError(t, err, "Failed to read part data")
				require.Equal(t, resp.ContentLength, int64(n), "Downloaded bytes mismatch")

				bytesDownloaded += int64(n)
			}

			// Verify total bytes downloaded
			require.Equal(t, download.GetTotalSize(), bytesDownloaded,
				"Total downloaded size mismatch")

			// For small files, verify content integrity by downloading the full file and comparing
			if tc.fileSize <= smallFileSize {
				fullData, err := client.Get(ctx, tc.objectKey)
				require.NoError(t, err, "Failed to download full file for comparison")
				require.Equal(t, len(fullData), len(totalBytes), "Full download size mismatch")

				// Compare content
				require.Equal(t, fullData, totalBytes, "Multipart download content differs from full download")
				t.Log("Successfully verified data integrity by comparing with full download")
			}

			// Clean up
			err = client.Delete(ctx, tc.objectKey)
			require.NoError(t, err, "Failed to clean up test object")
		})
	}
}

// uploadTestFile creates and uploads a test file with the specified size and content type
func uploadTestFile(t *testing.T, ctx context.Context, client *Minio, objectKey string, fileSize int64, contentType string) {
	t.Helper()

	// Generate test data
	var reader io.Reader
	if fileSize <= 10*1024*1024 { // 10MB or less
		// For small files, use a string pattern
		pattern := generateTestContent(fileSize)
		reader = strings.NewReader(pattern)
	} else {
		// For larger files, use a deterministic random data generator
		reader = newRandomDataReader(fileSize)
	}

	// Create a temporary file
	tempFile, err := os.CreateTemp("", "minio-test-*")
	require.NoError(t, err, "Failed to create temporary file")
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Write data to the temp file
	written, err := io.Copy(tempFile, reader)
	require.NoError(t, err, "Failed to write to temporary file")
	require.Equal(t, fileSize, written, "Written file size mismatch")

	// Rewind the file for reading
	_, err = tempFile.Seek(0, 0)
	require.NoError(t, err, "Failed to rewind temporary file")

	// Upload the file using the Put method with size
	t.Logf("Uploading test file %s (%d bytes)", objectKey, fileSize)
	uploadedSize, err := client.Put(ctx, objectKey, tempFile, fileSize)
	require.NoError(t, err, "Failed to upload test file")
	require.Equal(t, fileSize, uploadedSize, "Uploaded size mismatch")

	// Verify the file exists and has correct size and content type
	objInfo, err := client.Client.StatObject(ctx, client.cfg.Connection.BucketName,
		objectKey, minio.StatObjectOptions{})
	require.NoError(t, err, "Failed to verify uploaded file")
	require.Equal(t, fileSize, objInfo.Size, "Uploaded file size mismatch")

	// Set the content type if needed using CopyObject with metadata
	if contentType != "" && objInfo.ContentType != contentType {
		// Create source options
		srcOpts := minio.CopySrcOptions{
			Bucket: client.cfg.Connection.BucketName,
			Object: objectKey,
		}

		// Create destination options with a new content type
		dstOpts := minio.CopyDestOptions{
			Bucket: client.cfg.Connection.BucketName,
			Object: objectKey,
			UserMetadata: map[string]string{
				"Content-Type": contentType,
			},
			ReplaceMetadata: true,
		}

		// Copy an object to itself with new metadata
		_, err = client.Client.CopyObject(ctx, dstOpts, srcOpts)
		require.NoError(t, err, "Failed to set content type")

		// Verify the content type was set
		objInfo, err = client.Client.StatObject(ctx, client.cfg.Connection.BucketName,
			objectKey, minio.StatObjectOptions{})
		require.NoError(t, err, "Failed to verify content type")
		require.Equal(t, contentType, objInfo.ContentType, "Content type mismatch")
	}

	t.Logf("Successfully uploaded test file: %s (%d bytes)", objInfo.Key, objInfo.Size)
}
