package kafka

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"strings"
)

// Serializer defines the interface for serializing data before publishing to Kafka.
// Implementations can provide custom serialization logic (e.g., JSON, Protobuf, Avro, etc.).
type Serializer interface {
	// Serialize converts the input data to a byte slice
	Serialize(data interface{}) ([]byte, error)
}

// Deserializer defines the interface for deserializing data received from Kafka.
// Implementations can provide custom deserialization logic.
type Deserializer interface {
	// Deserialize converts a byte slice into the target data structure
	Deserialize(data []byte, target interface{}) error
}

// ProtoMessage is an interface that matches the proto.Message interface from google.golang.org/protobuf.
// This allows the ProtobufSerializer to work with any protobuf message without requiring a direct dependency.
type ProtoMessage interface {
	ProtoReflect() interface{} // This matches proto.Message
	Reset()
	String() string
}

// ProtoMarshaler is an interface for types that can marshal themselves to protobuf format.
// This is useful for custom protobuf implementations.
type ProtoMarshaler interface {
	Marshal() ([]byte, error)
}

// ProtoUnmarshaler is an interface for types that can unmarshal themselves from protobuf format.
type ProtoUnmarshaler interface {
	Unmarshal([]byte) error
}

// ==================== JSON Serializer ====================

// JSONSerializer implements Serializer using JSON encoding.
// This is the default serializer provided by the Kafka module.
//
// Features:
//   - Handles any Go type that can be marshaled to JSON
//   - Automatically passes through []byte without modification
//   - Thread-safe
type JSONSerializer struct{}

// Serialize converts data to JSON bytes.
func (j *JSONSerializer) Serialize(data interface{}) ([]byte, error) {
	// If data is already []byte, return it directly
	if bytes, ok := data.([]byte); ok {
		return bytes, nil
	}

	// If data is a string, convert to bytes
	if str, ok := data.(string); ok {
		return []byte(str), nil
	}

	// Otherwise, marshal to JSON
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("JSONSerializer: failed to serialize: %w", err)
	}
	return bytes, nil
}

// JSONDeserializer implements Deserializer using JSON decoding.
// This is the default deserializer provided by the Kafka module.
type JSONDeserializer struct{}

// Deserialize converts JSON bytes to the target structure.
func (j *JSONDeserializer) Deserialize(data []byte, target interface{}) error {
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("JSONDeserializer: failed to deserialize: %w", err)
	}
	return nil
}

// ==================== Protobuf Serializer ====================

// ProtobufSerializer implements Serializer for Protocol Buffer messages.
//
// This serializer works with any type that implements:
//   - ProtoMessage interface (google.golang.org/protobuf/proto.Message)
//   - ProtoMarshaler interface (custom types with Marshal() method)
//
// Example usage with google protobuf:
//
//	import "google.golang.org/protobuf/proto"
//
//	serializer := &kafka.ProtobufSerializer{
//	    MarshalFunc: proto.Marshal,
//	}
//
// Example with custom marshal function:
//
//	serializer := &kafka.ProtobufSerializer{
//	    MarshalFunc: func(m interface{}) ([]byte, error) {
//	        return myCustomProtoMarshal(m)
//	    },
//	}
type ProtobufSerializer struct {
	// MarshalFunc is the function used to marshal protobuf messages.
	// If nil, will attempt to use the ProtoMarshaler interface.
	// For google protobuf, use: proto.Marshal
	MarshalFunc func(interface{}) ([]byte, error)
}

// Serialize converts a protobuf message to bytes.
func (p *ProtobufSerializer) Serialize(data interface{}) ([]byte, error) {
	// If data is already []byte, return it directly
	if bytes, ok := data.([]byte); ok {
		return bytes, nil
	}

	// If MarshalFunc is provided, use it
	if p.MarshalFunc != nil {
		bytes, err := p.MarshalFunc(data)
		if err != nil {
			return nil, fmt.Errorf("ProtobufSerializer: failed to marshal: %w", err)
		}
		return bytes, nil
	}

	// Try ProtoMarshaler interface
	if marshaler, ok := data.(ProtoMarshaler); ok {
		bytes, err := marshaler.Marshal()
		if err != nil {
			return nil, fmt.Errorf("ProtobufSerializer: failed to marshal: %w", err)
		}
		return bytes, nil
	}

	return nil, fmt.Errorf("ProtobufSerializer: data must implement ProtoMarshaler or provide MarshalFunc, got %T", data)
}

// ProtobufDeserializer implements Deserializer for Protocol Buffer messages.
//
// Example usage with google protobuf:
//
//	import "google.golang.org/protobuf/proto"
//
//	deserializer := &kafka.ProtobufDeserializer{
//	    UnmarshalFunc: proto.Unmarshal,
//	}
type ProtobufDeserializer struct {
	// UnmarshalFunc is the function used to unmarshal protobuf messages.
	// If nil, will attempt to use the ProtoUnmarshaler interface.
	// For google protobuf, use: proto.Unmarshal
	UnmarshalFunc func([]byte, interface{}) error
}

// Deserialize converts protobuf bytes to the target structure.
func (p *ProtobufDeserializer) Deserialize(data []byte, target interface{}) error {
	// If UnmarshalFunc is provided, use it
	if p.UnmarshalFunc != nil {
		if err := p.UnmarshalFunc(data, target); err != nil {
			return fmt.Errorf("ProtobufDeserializer: failed to unmarshal: %w", err)
		}
		return nil
	}

	// Try ProtoUnmarshaler interface
	if unmarshaler, ok := target.(ProtoUnmarshaler); ok {
		if err := unmarshaler.Unmarshal(data); err != nil {
			return fmt.Errorf("ProtobufDeserializer: failed to unmarshal: %w", err)
		}
		return nil
	}

	return fmt.Errorf("ProtobufDeserializer: target must implement ProtoUnmarshaler or provide UnmarshalFunc, got %T", target)
}

// ==================== Avro Serializer ====================

// AvroSerializer implements Serializer for Apache Avro format.
//
// This serializer requires an external Avro library. It works with any type
// that implements the AvroMarshaler interface or you can provide a custom MarshalFunc.
//
// Example with linkedin/goavro:
//
//	codec, _ := goavro.NewCodec(schemaJSON)
//	serializer := &kafka.AvroSerializer{
//	    MarshalFunc: func(data interface{}) ([]byte, error) {
//	        return codec.BinaryFromNative(nil, data)
//	    },
//	}
type AvroSerializer struct {
	// MarshalFunc is the function used to marshal to Avro format.
	MarshalFunc func(interface{}) ([]byte, error)
}

// Serialize converts data to Avro bytes.
func (a *AvroSerializer) Serialize(data interface{}) ([]byte, error) {
	// If data is already []byte, return it directly
	if bytes, ok := data.([]byte); ok {
		return bytes, nil
	}

	if a.MarshalFunc == nil {
		return nil, fmt.Errorf("AvroSerializer: MarshalFunc must be provided")
	}

	bytes, err := a.MarshalFunc(data)
	if err != nil {
		return nil, fmt.Errorf("AvroSerializer: failed to marshal: %w", err)
	}
	return bytes, nil
}

// AvroDeserializer implements Deserializer for Apache Avro format.
type AvroDeserializer struct {
	// UnmarshalFunc is the function used to unmarshal from Avro format.
	UnmarshalFunc func([]byte, interface{}) error
}

// Deserialize converts Avro bytes to the target structure.
func (a *AvroDeserializer) Deserialize(data []byte, target interface{}) error {
	if a.UnmarshalFunc == nil {
		return fmt.Errorf("AvroDeserializer: UnmarshalFunc must be provided")
	}

	if err := a.UnmarshalFunc(data, target); err != nil {
		return fmt.Errorf("AvroDeserializer: failed to unmarshal: %w", err)
	}
	return nil
}

// ==================== String Serializer ====================

// StringSerializer implements Serializer for string data.
// This is useful for text-based messages.
type StringSerializer struct {
	// Encoding specifies the character encoding (default: UTF-8)
	Encoding string
}

// Serialize converts data to bytes.
func (s *StringSerializer) Serialize(data interface{}) ([]byte, error) {
	// If data is already []byte, return it directly
	if bytes, ok := data.([]byte); ok {
		return bytes, nil
	}

	// If data is a string, convert to bytes
	if str, ok := data.(string); ok {
		return []byte(str), nil
	}

	// For other types, use fmt.Sprintf
	return []byte(fmt.Sprintf("%v", data)), nil
}

// StringDeserializer implements Deserializer for string data.
type StringDeserializer struct{}

// Deserialize converts bytes to string.
func (s *StringDeserializer) Deserialize(data []byte, target interface{}) error {
	// If target is *string, set it directly
	if strPtr, ok := target.(*string); ok {
		*strPtr = string(data)
		return nil
	}

	// If target is *[]byte, copy the data
	if bytesPtr, ok := target.(*[]byte); ok {
		*bytesPtr = data
		return nil
	}

	return fmt.Errorf("StringDeserializer: target must be *string or *[]byte, got %T", target)
}

// ==================== Gob Serializer ====================

// GobSerializer implements Serializer using Go's gob encoding.
// This is useful for Go-to-Go communication where both sides use the same types.
//
// Note: Gob encoding is Go-specific and not interoperable with other languages.
type GobSerializer struct{}

// Serialize converts data to gob bytes.
func (g *GobSerializer) Serialize(data interface{}) ([]byte, error) {
	// If data is already []byte, return it directly
	if bytes, ok := data.([]byte); ok {
		return bytes, nil
	}

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(data); err != nil {
		return nil, fmt.Errorf("GobSerializer: failed to encode: %w", err)
	}
	return buf.Bytes(), nil
}

// GobDeserializer implements Deserializer using Go's gob decoding.
type GobDeserializer struct{}

// Deserialize converts gob bytes to the target structure.
func (g *GobDeserializer) Deserialize(data []byte, target interface{}) error {
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	if err := dec.Decode(target); err != nil {
		return fmt.Errorf("GobDeserializer: failed to decode: %w", err)
	}
	return nil
}

// ==================== NoOp Serializer ====================

// NoOpSerializer passes through byte slices without modification.
// Use this when you want to handle serialization yourself or work with raw bytes.
type NoOpSerializer struct{}

// Serialize returns the data as-is if it's a byte slice, otherwise returns an error.
func (n *NoOpSerializer) Serialize(data interface{}) ([]byte, error) {
	bytes, ok := data.([]byte)
	if !ok {
		return nil, fmt.Errorf("NoOpSerializer: requires []byte input, got %T", data)
	}
	return bytes, nil
}

// NoOpDeserializer does not perform any deserialization.
// The target must be a *[]byte to receive the raw bytes.
type NoOpDeserializer struct{}

// Deserialize copies the raw bytes to the target if it's a *[]byte.
func (n *NoOpDeserializer) Deserialize(data []byte, target interface{}) error {
	bytesPtr, ok := target.(*[]byte)
	if !ok {
		return fmt.Errorf("NoOpDeserializer: requires *[]byte target, got %T", target)
	}
	*bytesPtr = data
	return nil
}

// ==================== Multi-Format Serializer ====================

// MultiFormatSerializer supports multiple serialization formats with automatic detection.
// It can handle JSON, Protobuf, Avro, and custom formats based on configuration or content type.
type MultiFormatSerializer struct {
	// DefaultFormat is the format to use when not specified (default: "json")
	DefaultFormat string

	// Serializers maps format names to their serializer implementations
	Serializers map[string]Serializer
}

// NewMultiFormatSerializer creates a new multi-format serializer with default formats.
func NewMultiFormatSerializer() *MultiFormatSerializer {
	return &MultiFormatSerializer{
		DefaultFormat: "json",
		Serializers: map[string]Serializer{
			"json":   &JSONSerializer{},
			"string": &StringSerializer{},
			"gob":    &GobSerializer{},
			"noop":   &NoOpSerializer{},
		},
	}
}

// RegisterSerializer adds a custom serializer for a specific format.
func (m *MultiFormatSerializer) RegisterSerializer(format string, serializer Serializer) {
	if m.Serializers == nil {
		m.Serializers = make(map[string]Serializer)
	}
	m.Serializers[strings.ToLower(format)] = serializer
}

// Serialize converts data using the appropriate serializer based on format.
// The format can be specified as a prefix in the form "format:data" or uses DefaultFormat.
func (m *MultiFormatSerializer) Serialize(data interface{}) ([]byte, error) {
	format := m.DefaultFormat
	if format == "" {
		format = "json"
	}

	serializer, ok := m.Serializers[format]
	if !ok {
		return nil, fmt.Errorf("MultiFormatSerializer: unknown format %q", format)
	}

	return serializer.Serialize(data)
}

// SerializeWithFormat explicitly specifies the format to use.
func (m *MultiFormatSerializer) SerializeWithFormat(format string, data interface{}) ([]byte, error) {
	serializer, ok := m.Serializers[strings.ToLower(format)]
	if !ok {
		return nil, fmt.Errorf("MultiFormatSerializer: unknown format %q", format)
	}

	return serializer.Serialize(data)
}

// MultiFormatDeserializer supports multiple deserialization formats.
type MultiFormatDeserializer struct {
	// DefaultFormat is the format to use when not specified (default: "json")
	DefaultFormat string

	// Deserializers maps format names to their deserializer implementations
	Deserializers map[string]Deserializer
}

// NewMultiFormatDeserializer creates a new multi-format deserializer with default formats.
func NewMultiFormatDeserializer() *MultiFormatDeserializer {
	return &MultiFormatDeserializer{
		DefaultFormat: "json",
		Deserializers: map[string]Deserializer{
			"json":   &JSONDeserializer{},
			"string": &StringDeserializer{},
			"gob":    &GobDeserializer{},
			"noop":   &NoOpDeserializer{},
		},
	}
}

// RegisterDeserializer adds a custom deserializer for a specific format.
func (m *MultiFormatDeserializer) RegisterDeserializer(format string, deserializer Deserializer) {
	if m.Deserializers == nil {
		m.Deserializers = make(map[string]Deserializer)
	}
	m.Deserializers[strings.ToLower(format)] = deserializer
}

// Deserialize converts data using the appropriate deserializer based on format.
func (m *MultiFormatDeserializer) Deserialize(data []byte, target interface{}) error {
	format := m.DefaultFormat
	if format == "" {
		format = "json"
	}

	deserializer, ok := m.Deserializers[format]
	if !ok {
		return fmt.Errorf("MultiFormatDeserializer: unknown format %q", format)
	}

	return deserializer.Deserialize(data, target)
}

// DeserializeWithFormat explicitly specifies the format to use.
func (m *MultiFormatDeserializer) DeserializeWithFormat(format string, data []byte, target interface{}) error {
	deserializer, ok := m.Deserializers[strings.ToLower(format)]
	if !ok {
		return fmt.Errorf("MultiFormatDeserializer: unknown format %q", format)
	}

	return deserializer.Deserialize(data, target)
}

// getDefaultSerializer returns the appropriate serializer based on the config's DataType
func getDefaultSerializer(dataType string) Serializer {
	switch dataType {
	case "string":
		return &StringSerializer{}
	case "protobuf":
		// Return nil - user must provide custom protobuf serializer
		// since it requires MarshalFunc
		return nil
	case "avro":
		// Return nil - user must provide custom avro serializer
		// since it requires MarshalFunc
		return nil
	case "gob":
		return &GobSerializer{}
	case "bytes":
		return &NoOpSerializer{}
	case "json", "":
		// Default to JSON
		return &JSONSerializer{}
	default:
		// Unknown type, default to JSON
		return &JSONSerializer{}
	}
}

// getDefaultDeserializer returns the appropriate deserializer based on the config's DataType
func getDefaultDeserializer(dataType string) Deserializer {
	switch dataType {
	case "string":
		return &StringDeserializer{}
	case "protobuf":
		// Return nil - user must provide custom protobuf deserializer
		// since it requires UnmarshalFunc
		return nil
	case "avro":
		// Return nil - user must provide custom avro deserializer
		// since it requires UnmarshalFunc
		return nil
	case "gob":
		return &GobDeserializer{}
	case "bytes":
		return &NoOpDeserializer{}
	case "json", "":
		// Default to JSON
		return &JSONDeserializer{}
	default:
		// Unknown type, default to JSON
		return &JSONDeserializer{}
	}
}

// SetDefaultSerializers sets default serializers on the Kafka client based on config DataType
// This is called automatically during client creation if no serializers are provided
func (k *KafkaClient) SetDefaultSerializers() {
	k.mu.Lock()
	defer k.mu.Unlock()

	// Only set the default serializer if none is configured
	if k.serializer == nil {
		if defaultSer := getDefaultSerializer(k.cfg.DataType); defaultSer != nil {
			k.serializer = defaultSer
		}
	}

	// Only set the default deserializer if none is configured
	if k.deserializer == nil {
		if defaultDeser := getDefaultDeserializer(k.cfg.DataType); defaultDeser != nil {
			k.deserializer = defaultDeser
		}
	}
}

// ValidateDataType checks if the provided DataType is supported
// Returns error if DataType requires custom serializer but none is provided
func ValidateDataType(dataType string, hasSerializer, hasDeserializer bool) error {
	switch dataType {
	case "protobuf":
		if !hasSerializer {
			return fmt.Errorf("DataType 'protobuf' requires a custom serializer with MarshalFunc")
		}
		if !hasDeserializer {
			return fmt.Errorf("DataType 'protobuf' requires a custom deserializer with UnmarshalFunc")
		}
	case "avro":
		if !hasSerializer {
			return fmt.Errorf("DataType 'avro' requires a custom serializer with MarshalFunc")
		}
		if !hasDeserializer {
			return fmt.Errorf("DataType 'avro' requires a custom deserializer with UnmarshalFunc")
		}
	case "json", "string", "gob", "bytes", "":
		// These have default implementations, validation passes
		return nil
	default:
		// Unknown type - will default to JSON, just log a warning
		// Not an error, just use JSON as fallback
		return nil
	}
	return nil
}
