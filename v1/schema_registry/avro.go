package schema_registry

import (
	"fmt"
)

// AvroSerializer is a convenience wrapper for Avro with schema registry
type AvroSerializer struct {
	*WrapperSerializer
}

// AvroSerializerConfig holds configuration for Avro serializer
type AvroSerializerConfig struct {
	Registry    Registry
	Subject     string
	Schema      string
	MarshalFunc func(interface{}) ([]byte, error) // Avro encoding function
}

// NewAvroSerializer creates an Avro serializer with schema registry support
func NewAvroSerializer(config AvroSerializerConfig) (*AvroSerializer, error) {
	if config.MarshalFunc == nil {
		return nil, fmt.Errorf("marshalFunc is required for Avro serialization")
	}

	inner, err := NewWrapperSerializer(WrapperSerializerConfig{
		Registry:   config.Registry,
		Subject:    config.Subject,
		SchemaType: "AVRO",
		Schema:     config.Schema,
		InnerSerializer: &avroInnerSerializer{
			marshalFunc: config.MarshalFunc,
		},
	})
	if err != nil {
		return nil, err
	}

	return &AvroSerializer{
		WrapperSerializer: inner,
	}, nil
}

// avroInnerSerializer implements Serializer for Avro
type avroInnerSerializer struct {
	marshalFunc func(interface{}) ([]byte, error)
}

func (a *avroInnerSerializer) Serialize(data interface{}) ([]byte, error) {
	if bytes, ok := data.([]byte); ok {
		return bytes, nil
	}
	return a.marshalFunc(data)
}

// AvroDeserializer is a convenience wrapper for Avro with schema registry
type AvroDeserializer struct {
	*WrapperDeserializer
}

// AvroDeserializerConfig holds configuration for Avro deserializer
type AvroDeserializerConfig struct {
	Registry      Registry
	UnmarshalFunc func([]byte, interface{}) error // Avro decoding function
}

// NewAvroDeserializer creates an Avro deserializer with schema registry support
func NewAvroDeserializer(config AvroDeserializerConfig) (*AvroDeserializer, error) {
	if config.UnmarshalFunc == nil {
		return nil, fmt.Errorf("unmarshalFunc is required for Avro deserialization")
	}

	inner, err := NewWrapperDeserializer(WrapperDeserializerConfig{
		Registry: config.Registry,
		InnerDeserializer: &avroInnerDeserializer{
			unmarshalFunc: config.UnmarshalFunc,
		},
	})
	if err != nil {
		return nil, err
	}

	return &AvroDeserializer{
		WrapperDeserializer: inner,
	}, nil
}

// avroInnerDeserializer implements Deserializer for Avro
type avroInnerDeserializer struct {
	unmarshalFunc func([]byte, interface{}) error
}

func (a *avroInnerDeserializer) Deserialize(data []byte, target interface{}) error {
	return a.unmarshalFunc(data, target)
}
