// Package schema_registry provides integration with Confluent Schema Registry.
//
// This package enables schema management, validation, and evolution for
// Apache Kafka messages and other streaming data platforms. It supports
// multiple serialization formats including Avro, Protobuf, and JSON Schema.
//
// # Architecture
//
// This package follows the "accept interfaces, return structs" design pattern:
//   - Registry interface: Defines the contract for schema registry operations
//   - Client struct: Concrete implementation of the Registry interface
//   - NewClient constructor: Returns *Client (concrete type)
//   - FX module: Provides both *Client and Registry interface for dependency injection
//
// Core Features:
//   - HTTP client for Confluent Schema Registry
//   - Schema registration and retrieval with caching
//   - Compatibility checking for schema evolution
//   - Confluent wire format encoding/decoding
//   - Serializers for Avro, Protobuf, and JSON Schema
//   - Generic wrapper for custom serializers
//
// # Direct Usage (Without FX)
//
// For simple applications or tests, create a client directly:
//
//	import "github.com/Aleph-Alpha/std/v1/schema_registry"
//
//	// Create schema registry client (returns concrete *Client)
//	client, err := schema_registry.NewClient(schema_registry.Config{
//	    URL:      "http://localhost:8081",
//	    Username: "user",     // Optional
//	    Password: "password", // Optional
//	    Timeout:  10 * time.Second,
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Register a schema
//	avroSchema := `{
//	    "type": "record",
//	    "name": "User",
//	    "fields": [
//	        {"name": "name", "type": "string"},
//	        {"name": "age", "type": "int"}
//	    ]
//	}`
//
//	schemaID, err := client.RegisterSchema("users-value", avroSchema, "AVRO")
//
// # FX Module Integration
//
// For production applications using Uber's fx, use the FXModule which provides
// both the concrete type and interface:
//
//	import (
//	    "go.uber.org/fx"
//	    "github.com/Aleph-Alpha/std/v1/schema_registry"
//	)
//
//	app := fx.New(
//	    schema_registry.FXModule, // Provides *Client and Registry interface
//	    fx.Provide(
//	        func() schema_registry.Config {
//	            return schema_registry.Config{
//	                URL:      os.Getenv("SCHEMA_REGISTRY_URL"),
//	                Username: os.Getenv("SCHEMA_REGISTRY_USER"),
//	                Password: os.Getenv("SCHEMA_REGISTRY_PASSWORD"),
//	                Timeout:  30 * time.Second,
//	            }
//	        },
//	    ),
//	    fx.Invoke(func(client *schema_registry.Client) {
//	        // Use concrete type directly
//	        schemaID, _ := client.RegisterSchema("subject", schema, "AVRO")
//	    }),
//	)
//
// # Type Aliases in Consumer Code
//
// To simplify your code and make it registry-agnostic, use type aliases:
//
//	package myapp
//
//	import stdRegistry "github.com/Aleph-Alpha/std/v1/schema_registry"
//
//	// Use type alias to reference std's interface
//	type SchemaRegistry = stdRegistry.Registry
//
//	// Now use SchemaRegistry throughout your codebase
//	func MyFunction(registry SchemaRegistry) {
//	    registry.GetSchemaByID(schemaID)
//	}
//
// This eliminates the need for adapters and allows you to switch implementations
// by only changing the alias definition.
//
// # Schema Operations
//
//	import "github.com/linkedin/goavro/v2"
//
//	// Create Avro codec
//	codec, err := goavro.NewCodec(avroSchema)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Create Avro serializer
//	serializer, err := schema_registry.NewAvroSerializer(
//	    schema_registry.AvroSerializerConfig{
//	        Registry: client,
//	        Subject:  "users-value",
//	        Schema:   avroSchema,
//	        MarshalFunc: func(data interface{}) ([]byte, error) {
//	            return codec.BinaryFromNative(nil, data)
//	        },
//	    },
//	)
//
//	// Serialize data
//	user := map[string]interface{}{
//	    "name": "John Doe",
//	    "age":  30,
//	}
//	encoded, err := serializer.Serialize(user)
//	// encoded contains: [magic_byte][schema_id][avro_payload]
//
//	// Create Avro deserializer
//	deserializer, err := schema_registry.NewAvroDeserializer(
//	    schema_registry.AvroDeserializerConfig{
//	        Registry: client,
//	        UnmarshalFunc: func(data []byte, target interface{}) error {
//	            native, _, err := codec.NativeFromBinary(data)
//	            if err != nil {
//	                return err
//	            }
//	            // Handle conversion to target type
//	            return nil
//	        },
//	    },
//	)
//
//	// Deserialize data
//	var result map[string]interface{}
//	err = deserializer.Deserialize(encoded, &result)
//
// # Using with Protobuf
//
//	import "google.golang.org/protobuf/proto"
//
//	// Create Protobuf serializer
//	serializer, err := schema_registry.NewProtobufSerializer(
//	    schema_registry.ProtobufSerializerConfig{
//	        Registry:    client,
//	        Subject:     "users-value",
//	        Schema:      protoSchema, // .proto file content as string
//	        MarshalFunc: proto.Marshal,
//	    },
//	)
//
//	// Serialize protobuf message
//	protoMsg := &pb.User{Name: "Jane", Age: 25}
//	encoded, err := serializer.Serialize(protoMsg)
//
//	// Create Protobuf deserializer
//	deserializer, err := schema_registry.NewProtobufDeserializer(
//	    schema_registry.ProtobufDeserializerConfig{
//	        Registry:      client,
//	        UnmarshalFunc: proto.Unmarshal,
//	    },
//	)
//
//	// Deserialize
//	var user pb.User
//	err = deserializer.Deserialize(encoded, &user)
//
// # Using with JSON Schema
//
//	// Create JSON serializer
//	serializer, err := schema_registry.NewJSONSerializer(
//	    schema_registry.JSONSerializerConfig{
//	        Registry: client,
//	        Subject:  "users-value",
//	        Schema: `{
//	            "$schema": "http://json-schema.org/draft-07/schema#",
//	            "type": "object",
//	            "properties": {
//	                "name": {"type": "string"},
//	                "age": {"type": "integer"}
//	            }
//	        }`,
//	    },
//	)
//
//	// Serialize JSON
//	user := struct {
//	    Name string `json:"name"`
//	    Age  int    `json:"age"`
//	}{Name: "Alice", Age: 28}
//	encoded, err := serializer.Serialize(user)
//
//	// Deserialize
//	deserializer, err := schema_registry.NewJSONDeserializer(
//	    schema_registry.JSONDeserializerConfig{
//	        Registry: client,
//	    },
//	)
//	var result struct {
//	    Name string `json:"name"`
//	    Age  int    `json:"age"`
//	}
//	err = deserializer.Deserialize(encoded, &result)
//
// # Wire Format
//
// All serializers produce messages in Confluent wire format:
//
//	[magic_byte (1 byte)] [schema_id (4 bytes, big-endian)] [payload]
//
// The magic byte is always 0x0, followed by the schema ID, then the
// serialized payload. This format is compatible with all Confluent tools.
//
// # Schema Caching
//
// The client automatically caches schemas by ID and subject to minimize
// network calls to the Schema Registry. Caches are thread-safe and
// maintained in-memory for the lifetime of the client.
//
// For more information, see the SCHEMA_REGISTRY.md documentation file.
package schema_registry
