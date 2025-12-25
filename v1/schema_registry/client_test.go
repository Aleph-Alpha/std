package schema_registry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient(t *testing.T) {
	// Mock schema registry server
	schemaID := 1
	avroSchema := `{"type":"record","name":"User","fields":[{"name":"name","type":"string"},{"name":"age","type":"int"}]}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/schemas/ids/1":
			// GetSchemaByID
			w.Header().Set("Content-Type", "application/vnd.schemaregistry.v1+json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"schema": avroSchema,
			})

		case "/subjects/test-subject/versions/latest":
			// GetLatestSchema
			w.Header().Set("Content-Type", "application/vnd.schemaregistry.v1+json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":      schemaID,
				"version": 1,
				"schema":  avroSchema,
			})

		case "/subjects/test-subject/versions":
			// RegisterSchema
			w.Header().Set("Content-Type", "application/vnd.schemaregistry.v1+json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": schemaID,
			})

		case "/compatibility/subjects/test-subject/versions/latest":
			// CheckCompatibility
			w.Header().Set("Content-Type", "application/vnd.schemaregistry.v1+json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"is_compatible": true,
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create schema registry client
	registry, err := NewClient(Config{
		URL: server.URL,
	})
	require.NoError(t, err)
	require.NotNil(t, registry)

	t.Run("get schema by id", func(t *testing.T) {
		schema, err := registry.GetSchemaByID(schemaID)
		require.NoError(t, err)
		assert.Equal(t, avroSchema, schema)

		// Test cache - second call should also work
		schema2, err := registry.GetSchemaByID(schemaID)
		require.NoError(t, err)
		assert.Equal(t, schema, schema2)
	})

	t.Run("get latest schema", func(t *testing.T) {
		metadata, err := registry.GetLatestSchema("test-subject")
		require.NoError(t, err)
		assert.Equal(t, schemaID, metadata.ID)
		assert.Equal(t, 1, metadata.Version)
		assert.Equal(t, avroSchema, metadata.Schema)
		assert.Equal(t, "test-subject", metadata.Subject)
	})

	t.Run("register schema", func(t *testing.T) {
		id, err := registry.RegisterSchema("test-subject", avroSchema, "AVRO")
		require.NoError(t, err)
		assert.Equal(t, schemaID, id)

		// Test cache - second registration should return cached ID
		id2, err := registry.RegisterSchema("test-subject", avroSchema, "AVRO")
		require.NoError(t, err)
		assert.Equal(t, id, id2)
	})

	t.Run("check compatibility", func(t *testing.T) {
		compatible, err := registry.CheckCompatibility("test-subject", avroSchema, "AVRO")
		require.NoError(t, err)
		assert.True(t, compatible)
	})
}

func TestEncodeDecodeSchemaID(t *testing.T) {
	t.Run("encode and decode schema id", func(t *testing.T) {
		schemaID := 123

		// Encode
		encoded := EncodeSchemaID(schemaID)
		assert.Len(t, encoded, 5)
		assert.Equal(t, byte(0x0), encoded[0]) // Magic byte

		// Decode
		decoded, payload, err := DecodeSchemaID(encoded)
		require.NoError(t, err)
		assert.Equal(t, schemaID, decoded)
		assert.Empty(t, payload)
	})

	t.Run("decode with payload", func(t *testing.T) {
		schemaID := 456
		payloadData := []byte("hello world")

		// Create message with schema ID and payload
		encoded := EncodeSchemaID(schemaID)
		message := append(encoded, payloadData...)

		// Decode
		decoded, payload, err := DecodeSchemaID(message)
		require.NoError(t, err)
		assert.Equal(t, schemaID, decoded)
		assert.Equal(t, payloadData, payload)
	})

	t.Run("decode invalid magic byte", func(t *testing.T) {
		invalidData := []byte{0x1, 0x0, 0x0, 0x0, 0x1}
		_, _, err := DecodeSchemaID(invalidData)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid magic byte")
	})

	t.Run("decode too short data", func(t *testing.T) {
		shortData := []byte{0x0, 0x0, 0x0}
		_, _, err := DecodeSchemaID(shortData)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "data too short")
	})
}

func TestWrapperSerializer(t *testing.T) {
	// Mock schema registry
	schemaID := 42
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/subjects/test-value/versions" {
			w.Header().Set("Content-Type", "application/vnd.schemaregistry.v1+json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": schemaID,
			})
		}
	}))
	defer server.Close()

	registry, err := NewClient(Config{
		URL: server.URL,
	})
	require.NoError(t, err)

	t.Run("serialize with schema registry", func(t *testing.T) {
		serializer, err := NewWrapperSerializer(WrapperSerializerConfig{
			Registry:   registry,
			Subject:    "test-value",
			SchemaType: "AVRO",
			Schema:     `{"type":"string"}`,
		})
		require.NoError(t, err)

		// Serialize raw bytes (already encoded)
		data := []byte("test data")
		encoded, err := serializer.Serialize(data)
		require.NoError(t, err)

		// Should have 5-byte header + payload
		assert.Greater(t, len(encoded), 5)

		// Verify header
		assert.Equal(t, byte(0x0), encoded[0]) // Magic byte

		// Decode to verify
		decodedID, payload, err := DecodeSchemaID(encoded)
		require.NoError(t, err)
		assert.Equal(t, schemaID, decodedID)
		assert.Equal(t, data, payload)
	})
}

func TestWrapperDeserializer(t *testing.T) {
	avroSchema := `{"type":"record","name":"User","fields":[{"name":"name","type":"string"}]}`
	schemaID := 99

	// Mock schema registry
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/schemas/ids/99" {
			w.Header().Set("Content-Type", "application/vnd.schemaregistry.v1+json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"schema": avroSchema,
			})
		}
	}))
	defer server.Close()

	registry, err := NewClient(Config{
		URL: server.URL,
	})
	require.NoError(t, err)

	t.Run("deserialize with schema registry", func(t *testing.T) {
		deserializer, err := NewWrapperDeserializer(WrapperDeserializerConfig{
			Registry: registry,
		})
		require.NoError(t, err)

		// Create encoded message
		payload := []byte("test payload")
		header := EncodeSchemaID(schemaID)
		message := append(header, payload...)

		// Deserialize to []byte
		var result []byte
		err = deserializer.Deserialize(message, &result)
		require.NoError(t, err)
		assert.Equal(t, payload, result)
	})
}

func TestJSONSerializer(t *testing.T) {
	schemaID := 100

	// Mock schema registry
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/subjects/test-json/versions" {
			json.NewEncoder(w).Encode(map[string]interface{}{"id": schemaID})
		}
		if r.URL.Path == "/schemas/ids/100" {
			json.NewEncoder(w).Encode(map[string]interface{}{"schema": `{"type":"object"}`})
		}
	}))
	defer server.Close()

	registry, err := NewClient(Config{
		URL: server.URL,
	})
	require.NoError(t, err)

	t.Run("json serializer", func(t *testing.T) {
		serializer, err := NewJSONSerializer(JSONSerializerConfig{
			Registry: registry,
			Subject:  "test-json",
			Schema:   `{"type":"object"}`,
		})
		require.NoError(t, err)
		require.NotNil(t, serializer)

		data := map[string]string{"key": "value"}
		encoded, err := serializer.Serialize(data)
		require.NoError(t, err)
		assert.Greater(t, len(encoded), 5)
	})

	t.Run("json deserializer", func(t *testing.T) {
		deserializer, err := NewJSONDeserializer(JSONDeserializerConfig{
			Registry: registry,
		})
		require.NoError(t, err)
		require.NotNil(t, deserializer)
	})
}

func TestConfig(t *testing.T) {
	t.Run("missing url", func(t *testing.T) {
		_, err := NewClient(Config{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "URL is required")
	})

	t.Run("with authentication", func(t *testing.T) {
		registry, err := NewClient(Config{
			URL:      "http://localhost:8081",
			Username: "user",
			Password: "pass",
		})
		require.NoError(t, err)

		// Verify credentials are stored (no type assertion needed, already *Client)
		assert.Equal(t, "user", registry.username)
		assert.Equal(t, "pass", registry.password)
	})
}
