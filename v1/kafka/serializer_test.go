package kafka

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test types
type TestStruct struct {
	Name  string `json:"name"`
	Age   int    `json:"age"`
	Email string `json:"email"`
}

// ==================== JSON Serializer Tests ====================

func TestJSONSerializer(t *testing.T) {
	serializer := &JSONSerializer{}

	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
	}{
		{
			name: "serialize struct",
			input: TestStruct{
				Name:  "John Doe",
				Age:   30,
				Email: "john@example.com",
			},
			wantErr: false,
		},
		{
			name: "serialize map",
			input: map[string]interface{}{
				"key":   "value",
				"count": 42,
			},
			wantErr: false,
		},
		{
			name:    "serialize string",
			input:   "hello world",
			wantErr: false,
		},
		{
			name:    "serialize bytes",
			input:   []byte("raw bytes"),
			wantErr: false,
		},
		{
			name: "serialize slice",
			input: []string{
				"item1",
				"item2",
				"item3",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := serializer.Serialize(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, bytes)
			assert.Greater(t, len(bytes), 0)
		})
	}
}

func TestJSONDeserializer(t *testing.T) {
	deserializer := &JSONDeserializer{}

	t.Run("deserialize to struct", func(t *testing.T) {
		data := []byte(`{"name":"John Doe","age":30,"email":"john@example.com"}`)
		var result TestStruct
		err := deserializer.Deserialize(data, &result)
		require.NoError(t, err)
		assert.Equal(t, "John Doe", result.Name)
		assert.Equal(t, 30, result.Age)
		assert.Equal(t, "john@example.com", result.Email)
	})

	t.Run("deserialize to map", func(t *testing.T) {
		data := []byte(`{"key":"value","count":42}`)
		var result map[string]interface{}
		err := deserializer.Deserialize(data, &result)
		require.NoError(t, err)
		assert.Equal(t, "value", result["key"])
		assert.Equal(t, float64(42), result["count"]) // JSON numbers are float64
	})

	t.Run("deserialize invalid JSON", func(t *testing.T) {
		data := []byte(`{invalid json}`)
		var result TestStruct
		err := deserializer.Deserialize(data, &result)
		assert.Error(t, err)
	})
}

func TestJSONRoundTrip(t *testing.T) {
	serializer := &JSONSerializer{}
	deserializer := &JSONDeserializer{}

	original := TestStruct{
		Name:  "Jane Smith",
		Age:   25,
		Email: "jane@example.com",
	}

	// Serialize
	bytes, err := serializer.Serialize(original)
	require.NoError(t, err)

	// Deserialize
	var result TestStruct
	err = deserializer.Deserialize(bytes, &result)
	require.NoError(t, err)

	assert.Equal(t, original, result)
}

// ==================== String Serializer Tests ====================

func TestStringSerializer(t *testing.T) {
	serializer := &StringSerializer{}

	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "serialize string",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "serialize bytes",
			input:    []byte("raw bytes"),
			expected: "raw bytes",
		},
		{
			name:     "serialize int",
			input:    42,
			expected: "42",
		},
		{
			name:     "serialize float",
			input:    3.14,
			expected: "3.14",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := serializer.Serialize(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(bytes))
		})
	}
}

func TestStringDeserializer(t *testing.T) {
	deserializer := &StringDeserializer{}

	t.Run("deserialize to string", func(t *testing.T) {
		data := []byte("hello world")
		var result string
		err := deserializer.Deserialize(data, &result)
		require.NoError(t, err)
		assert.Equal(t, "hello world", result)
	})

	t.Run("deserialize to bytes", func(t *testing.T) {
		data := []byte("hello world")
		var result []byte
		err := deserializer.Deserialize(data, &result)
		require.NoError(t, err)
		assert.Equal(t, data, result)
	})

	t.Run("deserialize to invalid target", func(t *testing.T) {
		data := []byte("hello world")
		var result int
		err := deserializer.Deserialize(data, &result)
		assert.Error(t, err)
	})
}

// ==================== Gob Serializer Tests ====================

func TestGobSerializer(t *testing.T) {
	serializer := &GobSerializer{}

	t.Run("serialize struct", func(t *testing.T) {
		input := TestStruct{
			Name:  "Alice",
			Age:   35,
			Email: "alice@example.com",
		}
		bytes, err := serializer.Serialize(input)
		require.NoError(t, err)
		assert.Greater(t, len(bytes), 0)
	})

	t.Run("serialize bytes passthrough", func(t *testing.T) {
		input := []byte("raw bytes")
		bytes, err := serializer.Serialize(input)
		require.NoError(t, err)
		assert.Equal(t, input, bytes)
	})
}

func TestGobDeserializer(t *testing.T) {
	serializer := &GobSerializer{}
	deserializer := &GobDeserializer{}

	t.Run("round trip struct", func(t *testing.T) {
		original := TestStruct{
			Name:  "Bob",
			Age:   40,
			Email: "bob@example.com",
		}

		// Serialize
		bytes, err := serializer.Serialize(original)
		require.NoError(t, err)

		// Deserialize
		var result TestStruct
		err = deserializer.Deserialize(bytes, &result)
		require.NoError(t, err)

		assert.Equal(t, original, result)
	})
}

// ==================== NoOp Serializer Tests ====================

func TestNoOpSerializer(t *testing.T) {
	serializer := &NoOpSerializer{}

	t.Run("serialize bytes", func(t *testing.T) {
		input := []byte("test data")
		bytes, err := serializer.Serialize(input)
		require.NoError(t, err)
		assert.Equal(t, input, bytes)
	})

	t.Run("serialize non-bytes", func(t *testing.T) {
		input := "string"
		_, err := serializer.Serialize(input)
		assert.Error(t, err)
	})
}

func TestNoOpDeserializer(t *testing.T) {
	deserializer := &NoOpDeserializer{}

	t.Run("deserialize to bytes", func(t *testing.T) {
		data := []byte("test data")
		var result []byte
		err := deserializer.Deserialize(data, &result)
		require.NoError(t, err)
		assert.Equal(t, data, result)
	})

	t.Run("deserialize to invalid target", func(t *testing.T) {
		data := []byte("test data")
		var result string
		err := deserializer.Deserialize(data, &result)
		assert.Error(t, err)
	})
}

// ==================== Protobuf Serializer Tests ====================

// Mock protobuf message for testing
type mockProtoMessage struct {
	data []byte
}

func (m *mockProtoMessage) Marshal() ([]byte, error) {
	return m.data, nil
}

func (m *mockProtoMessage) Unmarshal(data []byte) error {
	m.data = data
	return nil
}

func TestProtobufSerializer(t *testing.T) {
	t.Run("serialize with custom marshal func", func(t *testing.T) {
		serializer := &ProtobufSerializer{
			MarshalFunc: func(data interface{}) ([]byte, error) {
				return []byte("marshaled"), nil
			},
		}

		bytes, err := serializer.Serialize("any data")
		require.NoError(t, err)
		assert.Equal(t, []byte("marshaled"), bytes)
	})

	t.Run("serialize with ProtoMarshaler interface", func(t *testing.T) {
		serializer := &ProtobufSerializer{}
		msg := &mockProtoMessage{data: []byte("proto data")}

		bytes, err := serializer.Serialize(msg)
		require.NoError(t, err)
		assert.Equal(t, []byte("proto data"), bytes)
	})

	t.Run("serialize bytes passthrough", func(t *testing.T) {
		serializer := &ProtobufSerializer{}
		input := []byte("raw bytes")

		bytes, err := serializer.Serialize(input)
		require.NoError(t, err)
		assert.Equal(t, input, bytes)
	})

	t.Run("serialize without marshal func or interface", func(t *testing.T) {
		serializer := &ProtobufSerializer{}
		_, err := serializer.Serialize("invalid")
		assert.Error(t, err)
	})
}

func TestProtobufDeserializer(t *testing.T) {
	t.Run("deserialize with custom unmarshal func", func(t *testing.T) {
		deserializer := &ProtobufDeserializer{
			UnmarshalFunc: func(data []byte, target interface{}) error {
				if ptr, ok := target.(*[]byte); ok {
					*ptr = data
				}
				return nil
			},
		}

		var result []byte
		err := deserializer.Deserialize([]byte("test"), &result)
		require.NoError(t, err)
		assert.Equal(t, []byte("test"), result)
	})

	t.Run("deserialize with ProtoUnmarshaler interface", func(t *testing.T) {
		deserializer := &ProtobufDeserializer{}
		msg := &mockProtoMessage{}

		err := deserializer.Deserialize([]byte("proto data"), msg)
		require.NoError(t, err)
		assert.Equal(t, []byte("proto data"), msg.data)
	})
}

// ==================== Multi-Format Serializer Tests ====================

func TestMultiFormatSerializer(t *testing.T) {
	serializer := NewMultiFormatSerializer()

	t.Run("serialize with default format (json)", func(t *testing.T) {
		input := map[string]interface{}{"key": "value"}
		bytes, err := serializer.Serialize(input)
		require.NoError(t, err)
		assert.Contains(t, string(bytes), "key")
	})

	t.Run("serialize with explicit format", func(t *testing.T) {
		input := "hello"
		bytes, err := serializer.SerializeWithFormat("string", input)
		require.NoError(t, err)
		assert.Equal(t, "hello", string(bytes))
	})

	t.Run("register custom serializer", func(t *testing.T) {
		customSerializer := &NoOpSerializer{}
		serializer.RegisterSerializer("custom", customSerializer)

		input := []byte("test")
		bytes, err := serializer.SerializeWithFormat("custom", input)
		require.NoError(t, err)
		assert.Equal(t, input, bytes)
	})

	t.Run("unknown format", func(t *testing.T) {
		_, err := serializer.SerializeWithFormat("unknown", "data")
		assert.Error(t, err)
	})
}

func TestMultiFormatDeserializer(t *testing.T) {
	deserializer := NewMultiFormatDeserializer()

	t.Run("deserialize with default format (json)", func(t *testing.T) {
		data := []byte(`{"name":"test"}`)
		var result map[string]interface{}
		err := deserializer.Deserialize(data, &result)
		require.NoError(t, err)
		assert.Equal(t, "test", result["name"])
	})

	t.Run("deserialize with explicit format", func(t *testing.T) {
		data := []byte("hello")
		var result string
		err := deserializer.DeserializeWithFormat("string", data, &result)
		require.NoError(t, err)
		assert.Equal(t, "hello", result)
	})

	t.Run("register custom deserializer", func(t *testing.T) {
		customDeserializer := &NoOpDeserializer{}
		deserializer.RegisterDeserializer("custom", customDeserializer)

		data := []byte("test")
		var result []byte
		err := deserializer.DeserializeWithFormat("custom", data, &result)
		require.NoError(t, err)
		assert.Equal(t, data, result)
	})
}

// ==================== Integration Tests ====================

func TestSerializerIntegrationWithKafka(t *testing.T) {
	// Test that serializers work correctly with the Kafka client
	t.Run("kafka client with json serializer", func(t *testing.T) {
		// Test serializer directly (no Kafka instance needed)
		serializer := &JSONSerializer{}
		assert.NotNil(t, serializer)

		// Test serialization
		data := TestStruct{Name: "Test", Age: 30, Email: "test@example.com"}
		bytes, err := serializer.Serialize(data)
		require.NoError(t, err)
		assert.Contains(t, string(bytes), "Test")
	})
}
