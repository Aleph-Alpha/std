// Package schema_registry provides integration with Confluent Schema Registry.
//
// This package enables schema management, validation, and evolution for
// Apache Kafka messages and other streaming data platforms. It supports
// multiple serialization formats including Avro, Protobuf, and JSON Schema.
//
// Core Features:
//   - HTTP client for Confluent Schema Registry
//   - Schema registration and retrieval with caching
//   - Compatibility checking for schema evolution
//   - Confluent wire format encoding/decoding
//   - Serializers for Avro, Protobuf, and JSON Schema
//   - Generic wrapper for custom serializers
//
// Basic Usage:
//
//	import "github.com/Aleph-Alpha/std/v1/schema_registry"
//
//	// Create schema registry client
//	registry, err := schema_registry.NewClient(schema_registry.Config{
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
//	schemaID, err := registry.RegisterSchema("users-value", avroSchema, "AVRO")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Retrieve a schema
//	schema, err := registry.GetSchemaByID(schemaID)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Check compatibility
//	compatible, err := registry.CheckCompatibility("users-value", newSchema, "AVRO")
//	if !compatible {
//	    log.Println("Schema is not compatible!")
//	}
//
// Using with FX:
//
//	import (
//	    "go.uber.org/fx"
//	    "github.com/Aleph-Alpha/std/v1/schema_registry"
//	)
//
//	app := fx.New(
//	    schema_registry.FXModule,
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
//	    // Your application code that uses schema_registry.Registry
//	)
//
// Using with Avro:
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
//	        Registry: registry,
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
//	        Registry: registry,
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
// Using with Protobuf:
//
//	import "google.golang.org/protobuf/proto"
//
//	// Create Protobuf serializer
//	serializer, err := schema_registry.NewProtobufSerializer(
//	    schema_registry.ProtobufSerializerConfig{
//	        Registry:    registry,
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
//	        Registry:      registry,
//	        UnmarshalFunc: proto.Unmarshal,
//	    },
//	)
//
//	// Deserialize
//	var user pb.User
//	err = deserializer.Deserialize(encoded, &user)
//
// Using with JSON Schema:
//
//	// Create JSON serializer
//	serializer, err := schema_registry.NewJSONSerializer(
//	    schema_registry.JSONSerializerConfig{
//	        Registry: registry,
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
//	        Registry: registry,
//	    },
//	)
//	var result struct {
//	    Name string `json:"name"`
//	    Age  int    `json:"age"`
//	}
//	err = deserializer.Deserialize(encoded, &result)
//
// Wire Format:
//
// All serializers produce messages in Confluent wire format:
//
//	[magic_byte (1 byte)] [schema_id (4 bytes, big-endian)] [payload]
//
// The magic byte is always 0x0, followed by the schema ID, then the
// serialized payload. This format is compatible with all Confluent tools.
//
// Schema Caching:
//
// The client automatically caches schemas by ID and subject to minimize
// network calls to the Schema Registry. Caches are thread-safe and
// maintained in-memory for the lifetime of the client.
//
// For more information, see the SCHEMA_REGISTRY.md documentation file.
package schema_registry
