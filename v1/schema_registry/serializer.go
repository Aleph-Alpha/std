package schema_registry

import (
	"fmt"
)

// Serializer is the interface for encoding data
type Serializer interface {
	Serialize(data interface{}) ([]byte, error)
}

// Deserializer is the interface for decoding data
type Deserializer interface {
	Deserialize(data []byte, target interface{}) error
}

// WrapperSerializer wraps any serializer with schema registry support.
// It automatically registers schemas and prepends the Confluent wire format header.
type WrapperSerializer struct {
	registry   Registry
	subject    string
	schemaType string // "AVRO", "PROTOBUF", "JSON"
	schema     string // The schema definition

	// Optional: underlying serializer for the actual data encoding
	// If nil, assumes data is already encoded
	innerSerializer Serializer
}

// WrapperSerializerConfig holds configuration for schema registry serializer
type WrapperSerializerConfig struct {
	Registry        Registry
	Subject         string
	SchemaType      string // "AVRO", "PROTOBUF", "JSON"
	Schema          string
	InnerSerializer Serializer // Optional: for encoding data before schema registry wrapping
}

// NewWrapperSerializer creates a new schema registry-aware serializer
func NewWrapperSerializer(config WrapperSerializerConfig) (*WrapperSerializer, error) {
	if config.Registry == nil {
		return nil, fmt.Errorf("schema registry is required")
	}
	if config.Subject == "" {
		return nil, fmt.Errorf("subject is required")
	}
	if config.Schema == "" {
		return nil, fmt.Errorf("schema is required")
	}
	if config.SchemaType == "" {
		config.SchemaType = "AVRO" // Default to Avro
	}

	return &WrapperSerializer{
		registry:        config.Registry,
		subject:         config.Subject,
		schemaType:      config.SchemaType,
		schema:          config.Schema,
		innerSerializer: config.InnerSerializer,
	}, nil
}

// Serialize encodes data and prepends the schema registry header
func (s *WrapperSerializer) Serialize(data interface{}) ([]byte, error) {
	// If data is already []byte, pass through
	var encoded []byte
	if bytes, ok := data.([]byte); ok {
		encoded = bytes
	} else if s.innerSerializer != nil {
		// Use inner serializer to encode the data
		var err error
		encoded, err = s.innerSerializer.Serialize(data)
		if err != nil {
			return nil, fmt.Errorf("inner serializer failed: %w", err)
		}
	} else {
		return nil, fmt.Errorf("cannot serialize non-[]byte data without an inner serializer")
	}

	// Register schema and get ID
	schemaID, err := s.registry.RegisterSchema(s.subject, s.schema, s.schemaType)
	if err != nil {
		return nil, fmt.Errorf("failed to register schema: %w", err)
	}

	// Prepend Confluent wire format header
	header := EncodeSchemaID(schemaID)
	result := append(header, encoded...)

	return result, nil
}

// WrapperDeserializer wraps any deserializer with schema registry support.
// It automatically retrieves schemas and strips the Confluent wire format header.
type WrapperDeserializer struct {
	registry Registry

	// Optional: underlying deserializer for the actual data decoding
	// If nil, returns raw bytes
	innerDeserializer Deserializer
}

// WrapperDeserializerConfig holds configuration for schema registry deserializer
type WrapperDeserializerConfig struct {
	Registry          Registry
	InnerDeserializer Deserializer // Optional: for decoding data after schema registry unwrapping
}

// NewWrapperDeserializer creates a new schema registry-aware deserializer
func NewWrapperDeserializer(config WrapperDeserializerConfig) (*WrapperDeserializer, error) {
	if config.Registry == nil {
		return nil, fmt.Errorf("schema registry is required")
	}

	return &WrapperDeserializer{
		registry:          config.Registry,
		innerDeserializer: config.InnerDeserializer,
	}, nil
}

// Deserialize strips the schema registry header and decodes data
func (d *WrapperDeserializer) Deserialize(data []byte, target interface{}) error {
	// Decode schema ID and extract payload
	schemaID, payload, err := DecodeSchemaID(data)
	if err != nil {
		return fmt.Errorf("failed to decode schema ID: %w", err)
	}

	// Optionally retrieve schema for validation/info
	// (This could be used for schema evolution checks)
	_, err = d.registry.GetSchemaByID(schemaID)
	if err != nil {
		return fmt.Errorf("failed to retrieve schema %d: %w", schemaID, err)
	}

	// Use inner deserializer if provided
	if d.innerDeserializer != nil {
		return d.innerDeserializer.Deserialize(payload, target)
	}

	// Otherwise, if target is *[]byte, just copy the payload
	if bytesPtr, ok := target.(*[]byte); ok {
		*bytesPtr = payload
		return nil
	}

	return fmt.Errorf("cannot deserialize without an inner deserializer, target type: %T", target)
}
