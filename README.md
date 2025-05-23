# Prerequisites

## Adding as a Dependency

To use this package in your Go project, add it as a dependency:

```bash
go get gitlab.aleph-alpha.de/engineering/pharia-data-search/data-go-packages
```

## Authentication Setup
Since this is a private repository, you'll need to configure Go to authenticate with GitLab. You have several options:

### Option 1: Git Configuration

```bash
go env -w GOPRIVATE=gitlab.aleph-alpha.de
git config --global url."https://YOUR_USERNAME:YOUR_ACCESS_TOKEN@gitlab.aleph-alpha.de/".insteadOf "https://gitlab.aleph-alpha.de/"
```
Replace and with your GitLab credentials. `YOUR_USERNAME``YOUR_ACCESS_TOKEN`

### Option 2: Using .netrc File (Recommended)
Create or edit `~/.netrc` file (or `_netrc` on Windows) with the following content:
```vim
machine gitlab.aleph-alpha.de login YOUR_USERNAME password YOUR_ACCESS_TOKEN
```

Make sure to set proper permissions:
```bash
chmod 600 ~/.netrc
```

### Option 3: Environment Variables in Shell Profile
You can add the required configurations to your shell profile:
For Bash users (`~/.bash_profile` or `~/.bashrc`):

#### Add to ~/.bash_profile or ~/.bashrc

```text
export GOPRIVATE=gitlab.aleph-alpha.de
```

## IDE Configuration
If using an IDE like GoLand or VS Code, you may need additional configuration:
1. **GoLand**: Ensure environment variables are correctly set in:
   1. Go to **Settings/Preferences** (⌘,)
   2. Navigate to **Version Control** → **Git**
   3. Make sure "Use credential helper" is enabled
   4. Go to **Settings/Preferences** (⌘,)
   5. Navigate to **Tools** → **Terminal**
   6. Add the following to the "Environment Variables" field: GOPRIVATE=gitlab.aleph-alpha.de
   7. Click "Apply" and "OK"
   8. Restart GoLand's Terminal

2. **VS Code**: Configure git authentication in your settings.json or use the integrated terminal with the correct profile loaded


## Importing Packages
This module contains several packages that you can import into your project:

```go
// Import the logger package
import "gitlab.aleph-alpha.de/engineering/pharia-data-search/data-go-packages/pkg/logger"

// Import the minio package
import "gitlab.aleph-alpha.de/engineering/pharia-data-search/data-go-packages/pkg/minio"

// Import the rabbit package
import "gitlab.aleph-alpha.de/engineering/pharia-data-search/data-go-packages/pkg/rabbit"
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

Generated on Fri May 23 17:01:51 CEST 2025

## Packages
- [tracer](docs/pkg/tracer.md)
- [logger](docs/pkg/logger.md)
- [minio](docs/pkg/minio.md)
- [postgres](docs/pkg/postgres.md)
- [rabbit](docs/pkg/rabbit.md)
