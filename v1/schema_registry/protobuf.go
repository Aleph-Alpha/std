package schema_registry

import (
	"fmt"
)

// ProtobufSerializer is a convenience wrapper for Protobuf with schema registry
type ProtobufSerializer struct {
	*WrapperSerializer
}

// ProtobufSerializerConfig holds configuration for Protobuf serializer
type ProtobufSerializerConfig struct {
	Registry    Registry
	Subject     string
	Schema      string
	MarshalFunc func(interface{}) ([]byte, error) // Protobuf encoding function
}

// NewProtobufSerializer creates a Protobuf serializer with schema registry support
func NewProtobufSerializer(config ProtobufSerializerConfig) (*ProtobufSerializer, error) {
	inner, err := NewWrapperSerializer(WrapperSerializerConfig{
		Registry:   config.Registry,
		Subject:    config.Subject,
		SchemaType: "PROTOBUF",
		Schema:     config.Schema,
		InnerSerializer: &protobufInnerSerializer{
			marshalFunc: config.MarshalFunc,
		},
	})
	if err != nil {
		return nil, err
	}

	return &ProtobufSerializer{
		WrapperSerializer: inner,
	}, nil
}

// protobufInnerSerializer implements Serializer for Protobuf
type protobufInnerSerializer struct {
	marshalFunc func(interface{}) ([]byte, error)
}

func (p *protobufInnerSerializer) Serialize(data interface{}) ([]byte, error) {
	if bytes, ok := data.([]byte); ok {
		return bytes, nil
	}
	if p.marshalFunc != nil {
		return p.marshalFunc(data)
	}
	return nil, fmt.Errorf("protobuf marshal function not provided")
}

// ProtobufDeserializer is a convenience wrapper for Protobuf with schema registry
type ProtobufDeserializer struct {
	*WrapperDeserializer
}

// ProtobufDeserializerConfig holds configuration for Protobuf deserializer
type ProtobufDeserializerConfig struct {
	Registry      Registry
	UnmarshalFunc func([]byte, interface{}) error // Protobuf decoding function
}

// NewProtobufDeserializer creates a Protobuf deserializer with schema registry support
func NewProtobufDeserializer(config ProtobufDeserializerConfig) (*ProtobufDeserializer, error) {
	inner, err := NewWrapperDeserializer(WrapperDeserializerConfig{
		Registry: config.Registry,
		InnerDeserializer: &protobufInnerDeserializer{
			unmarshalFunc: config.UnmarshalFunc,
		},
	})
	if err != nil {
		return nil, err
	}

	return &ProtobufDeserializer{
		WrapperDeserializer: inner,
	}, nil
}

// protobufInnerDeserializer implements Deserializer for Protobuf
type protobufInnerDeserializer struct {
	unmarshalFunc func([]byte, interface{}) error
}

func (p *protobufInnerDeserializer) Deserialize(data []byte, target interface{}) error {
	return p.unmarshalFunc(data, target)
}
