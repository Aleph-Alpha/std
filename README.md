# Prerequisites

## Adding as a Dependency

To use this package in your Go project, add it as a dependency:

```bash
go get github.com/Aleph-Alpha/data-go-packages
```

## Authentication Setup
Since this is a private repository hosted on GitHub, you'll need to configure Go to authenticate properly. The repository has been moved from GitLab to GitHub under the Aleph-Alpha organization.

### Prerequisites
Ensure you have SSH access configured for GitHub and the Aleph-Alpha organization. Your Git should already be configured with:
```bash
git config --global url."git@github.com:Aleph-Alpha/".insteadOf "https://github.com/Aleph-Alpha/"
```

### Required Configuration
Configure Go to treat Aleph-Alpha repositories as private by setting these environment variables:

#### Option 1: Using Go's Built-in Configuration (Recommended)
```bash
go env -w GOPRIVATE="gitlab.aleph-alpha.de,github.com/Aleph-Alpha"
go env -w GONOPROXY="gitlab.aleph-alpha.de,github.com/Aleph-Alpha"
go env -w GONOSUMDB="gitlab.aleph-alpha.de,github.com/Aleph-Alpha"
```

#### Option 2: Environment Variables in Shell Profile
Add these lines to your shell profile (`~/.zshrc` for zsh, `~/.bash_profile` or `~/.bashrc` for bash):

```bash
export GOPRIVATE="gitlab.aleph-alpha.de,github.com/Aleph-Alpha"
export GONOPROXY="gitlab.aleph-alpha.de,github.com/Aleph-Alpha"
export GONOSUMDB="gitlab.aleph-alpha.de,github.com/Aleph-Alpha"
```

Then reload your shell configuration:
```bash
source ~/.zshrc  # or ~/.bash_profile
```

### What These Variables Do
- **GOPRIVATE**: Tells Go these are private repositories that should bypass the module proxy
- **GONOPROXY**: Prevents Go from using the public proxy (proxy.golang.org) for these repositories
- **GONOSUMDB**: Prevents Go from trying to verify checksums using the public checksum database (sum.golang.org)

### Verification
After configuration, verify the setup works:

```bash
# Check environment variables are set
echo "GOPRIVATE: $GOPRIVATE"
echo "GONOPROXY: $GONOPROXY" 
echo "GONOSUMDB: $GONOSUMDB"

# Test module download
go mod download github.com/Aleph-Alpha/data-go-packages
go mod tidy
```


## IDE Configuration
If using an IDE like GoLand or VS Code, you may need additional configuration:

### GoLand
1. Go to **Settings/Preferences** (⌘,)
2. Navigate to **Go** → **Go Modules**
3. Ensure "Environment" includes the GOPRIVATE variables:

```text
GOPRIVATE=gitlab.aleph-alpha.de,github.com/Aleph-Alpha
GONOPROXY=gitlab.aleph-alpha.de,github.com/Aleph-Alpha
GONOSUMDB=gitlab.aleph-alpha.de,github.com/Aleph-Alpha
```
4. Navigate to **Tools** → **Terminal**
5. Ensure "Shell path" uses your configured shell profile
6. Click "Apply" and "OK"
7. Restart GoLand

### VS Code
1. Ensure your integrated terminal loads your shell profile with the environment variables
2. Alternatively, add the variables to your VS Code settings.json:
```json
{
 "go.toolsEnvVars": {
   "GOPRIVATE": "gitlab.aleph-alpha.de,github.com/Aleph-Alpha",
   "GONOPROXY": "gitlab.aleph-alpha.de,github.com/Aleph-Alpha",
   "GONOSUMDB": "gitlab.aleph-alpha.de,github.com/Aleph-Alpha"
 }
}
```

## Importing Packages
This module contains several packages that you can import into your project:

```go
// Import the logger package
import "github.com/Aleph-Alpha/data-go-packages/pkg/logger"

// Import the minio package
import "github.com/Aleph-Alpha/data-go-packages/pkg/minio"

// Import the rabbit package
import "github.com/Aleph-Alpha/data-go-packages/pkg/rabbit"
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

Generated on Mon Nov 10 11:21:51 CET 2025

## Packages
- [tracer](docs/pkg/tracer.md)
- [metrics](docs/pkg/metrics.md)
- [logger](docs/pkg/logger.md)
- [minio](docs/pkg/minio.md)
- [postgres](docs/pkg/postgres.md)
- [qdrant](docs/pkg/qdrant.md)
- [rabbit](docs/pkg/rabbit.md)
