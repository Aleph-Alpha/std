<div align="center">
  <img src="logo.png" alt="Pharia Data Standard Library Logo" width="200"/>
  <h1>Pharia Data Standard Library</h1>

[![Go Report Card](https://goreportcard.com/badge/github.com/Aleph-Alpha/std)](https://goreportcard.com/report/github.com/Aleph-Alpha/std)
[![GoDoc](https://godoc.org/github.com/Aleph-Alpha/std?status.svg)](https://godoc.org/github.com/Aleph-Alpha/std)
[![License](https://img.shields.io/github/license/Aleph-Alpha/std.svg)](https://github.com/Aleph-Alpha/std/blob/main/LICENSE)

</div>

Standard library packages for Go services at Pharia Data (Aleph Alpha). This repository provides common utilities and clients for services such as logging, metrics, database access, and message queues.

## Adding as a Dependency

To use this package in your Go project, add it as a dependency:

```bash
go get github.com/Aleph-Alpha/std
```

## Importing Packages

This module contains several packages that you can import into your project:

```go
// Import the logger package
import "github.com/Aleph-Alpha/std/v1/logger"

// Import the minio package
import "github.com/Aleph-Alpha/std/v1/minio"

// Import the rabbit package
import "github.com/Aleph-Alpha/std/v1/rabbit"
```

# Running Tests

This project includes several methods for running tests, with special handling for environments using Colima for Docker containerization.

## Standard Tests

To run basic tests without container support:

```bash
make test
```

## Tests with Containers

For tests that require Docker containers (like integration tests), use:

```bash
make test-with-containers
```

This command automatically detects if Colima is running on your system:

- If Colima is detected, it will configure the proper Docker socket path and disable Ryuk
- If Colima is not detected, it falls back to standard Docker configuration

## Manual Testing

If you need more control, you can run tests manually:

### With Colima

```bash
DOCKER_HOST=unix://$HOME/.colima/default/docker.sock TESTCONTAINERS_RYUK_DISABLED=true go test -v ./...
```

# Go Packages Documentation

> [!NOTE]
> The documentation is generated using [gomarkdoc](https://github.com/princjef/gomarkdoc). To generate the documentation, run `make docs`.

> [!TODO]
> Add gomarkdoc as a go tool in the project and update the Makefile to use the tool instead of installing it manually.

Generated on Mon Dec 8 12:49:20 CET 2025

## Packages

- [tracer](docs/v1/tracer.md)
- [metrics](docs/v1/metrics.md)
- [logger](docs/v1/logger.md)
- [embedding](docs/v1/embedding.md)
- [minio](docs/v1/minio.md)
- [postgres](docs/v1/postgres.md)
- [qdrant](docs/v1/qdrant.md)
- [sparseembedding](docs/v1/sparseembedding.md)
- [rabbit](docs/v1/rabbit.md)
