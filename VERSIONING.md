# Versioning Implementation Guide

## Overview

This project uses Go's build-time flag injection to embed versioning information into binaries at compile time. This approach ensures accurate version tracking without runtime overhead.

## Version Information Embedded

The following metadata is injected at build time:

- **Version**: Application version (e.g., `1.0.0`)
- **Commit**: Short git commit SHA for traceability
- **BuildTime**: UTC timestamp when the binary was built
- **GoVersion**: Go version used for compilation

## Build Targets

All build scripts automatically inject version metadata:

### PowerShell (build.ps1)
```powershell
$ldflags = "-s -w -X main.Version=$Version -X main.Commit=$commit -X main.BuildTime=$buildTime"
go build -ldflags $ldflags -o $outputName .
```

### Bash (build.sh)
```bash
ldflags="-s -w -X main.Version=$VERSION -X main.Commit=$commit -X main.BuildTime=$buildTime"
CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -ldflags "$ldflags" -o "$output_name" .
```

### Makefile
```makefile
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME)"
```

### GitHub Actions (.github/workflows/test.yml)
```yaml
COMMIT=$(git rev-parse --short HEAD)
BUILD_TIME=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS="-s -w -X main.Version=1.1.0 -X main.Commit=$COMMIT -X main.BuildTime=$BUILD_TIME"
```

## Usage

### Check Version

```bash
./Find-Meraki-Ports-With-MAC --version
```

Output:
```
Find-Meraki-Ports-With-MAC version 1.1.0
  Commit:     a1b2c3d
  Build Time: 2024-01-15T10:30:00Z
  Go Version: go1.21
```

### Manual Build with Custom Version

```bash
VERSION="1.1.0"
COMMIT=$(git rev-parse --short HEAD)
BUILDTIME=$(date -u '+%Y-%m-%dT%H:%M:%SZ')

go build -ldflags \
  "-s -w \
   -X main.Version=$VERSION \
   -X main.Commit=$COMMIT \
   -X main.BuildTime=$BUILDTIME" \
  -o Find-Meraki-Ports-With-MAC .
```

### Build for Release

Use the build scripts which automatically inject version metadata:

```bash
# Windows (PowerShell)
.\build.ps1

# Unix-like systems
./build.sh

# Using Make
make build-all
```

## Version Variables in Code

The following variables in [main.go](main.go#L38-L45) are populated at build time:

```go
var (
    Version   = "dev"         // Injected: -X main.Version=1.1.0
    Commit    = "unknown"     // Injected: -X main.Commit=abc1234
    BuildTime = "unknown"     // Injected: -X main.BuildTime=2024-01-15T10:30:00Z
    GoVersion = "go1.21"      // Set to build Go version
)
```

## Static Build Optimization

All builds include optimization flags for minimal binary size:
- `-s` : Strip symbol table
- `-w` : Strip DWARF debug info
- `CGO_ENABLED=0` : Pure Go, no C runtime dependencies

Resulting binaries are typically 5.9-6.4 MB with zero external runtime requirements.

## CI/CD Integration

GitHub Actions automatically injects version metadata during builds:
1. Extracts short commit SHA from git
2. Captures current UTC timestamp
3. Injects into all platform builds
4. Produces reproducible binaries

## Release Process

When creating a release:

1. Update version in build scripts (VERSION variable)
2. Build all platforms: `make build-all`
3. Tag release: `git tag -a v1.1.0 -m "Release v1.1.0"`
4. Push changes and tags: `git push origin master v1.1.0`
5. Create GitHub release with binaries from `bin/` directory

Each binary will automatically contain the correct version metadata from build-time injection.
