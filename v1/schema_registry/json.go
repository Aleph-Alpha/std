package schema_registry

import (
	"encoding/json"
	"fmt"
)

// JSONSerializer is a convenience wrapper for JSON Schema with schema registry
type JSONSerializer struct {
	*WrapperSerializer
}

// JSONSerializerConfig holds configuration for JSON serializer
type JSONSerializerConfig struct {
	Registry Registry
	Subject  string
	Schema   string
}

// NewJSONSerializer creates a JSON serializer with schema registry support
func NewJSONSerializer(config JSONSerializerConfig) (*JSONSerializer, error) {
	inner, err := NewWrapperSerializer(WrapperSerializerConfig{
		Registry:        config.Registry,
		Subject:         config.Subject,
		SchemaType:      "JSON",
		Schema:          config.Schema,
		InnerSerializer: &jsonInnerSerializer{},
	})
	if err != nil {
		return nil, err
	}

	return &JSONSerializer{
		WrapperSerializer: inner,
	}, nil
}

// jsonInnerSerializer implements Serializer for JSON
type jsonInnerSerializer struct{}

func (j *jsonInnerSerializer) Serialize(data interface{}) ([]byte, error) {
	if bytes, ok := data.([]byte); ok {
		return bytes, nil
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return bytes, nil
}

// JSONDeserializer is a convenience wrapper for JSON Schema with schema registry
type JSONDeserializer struct {
	*WrapperDeserializer
}

// JSONDeserializerConfig holds configuration for JSON deserializer
type JSONDeserializerConfig struct {
	Registry Registry
}

// NewJSONDeserializer creates a JSON deserializer with schema registry support
func NewJSONDeserializer(config JSONDeserializerConfig) (*JSONDeserializer, error) {
	inner, err := NewWrapperDeserializer(WrapperDeserializerConfig{
		Registry:          config.Registry,
		InnerDeserializer: &jsonInnerDeserializer{},
	})
	if err != nil {
		return nil, err
	}

	return &JSONDeserializer{
		WrapperDeserializer: inner,
	}, nil
}

// jsonInnerDeserializer implements Deserializer for JSON
type jsonInnerDeserializer struct{}

func (j *jsonInnerDeserializer) Deserialize(data []byte, target interface{}) error {
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	return nil
}
