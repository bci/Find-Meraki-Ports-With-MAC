#!/bin/bash
# Build script for Find-Meraki-Ports-With-MAC
# Produces STATIC binaries with no runtime library dependencies
# CGO_ENABLED=0 ensures pure Go compilation
# -ldflags "-s -w" strips symbols for smaller binaries

set -e

# Clear any cross-compilation environment variables
unset GOOS GOARCH CGO_ENABLED 2>/dev/null || true

APP_NAME="Find-Meraki-Ports-With-MAC"
VERSION="1.2.0"
OUTPUT_DIR="bin"

# Run unit tests
echo "Running unit tests..."
go test -v ./...
if [ $? -ne 0 ]; then
    echo "Tests failed!"
    exit 1
fi
echo "Tests passed"
echo ""

# Run linter
echo "Running linter (go vet)..."
go vet ./...
if [ $? -ne 0 ]; then
    echo "Linting failed!"
    exit 1
fi
echo "✓ Linting passed"
echo ""
# Run golangci-lint
echo "Running golangci-lint..."
golangci-lint run ./...
if [ $? -ne 0 ]; then
    echo "golangci-lint failed!"
    exit 1
fi
echo "golangci-lint passed"
echo ""
# Create output directory if it doesn't exist
mkdir -p "$OUTPUT_DIR"

echo "Building $APP_NAME v$VERSION for multiple platforms..."
echo "Static builds (no C runtime, no external dependencies)"
echo ""

# Build configurations: OS, Architecture, Extension
declare -a builds=(
    "windows:amd64:.exe"
    "windows:arm64:.exe"
    "darwin:amd64:"
    "darwin:arm64:"
    "linux:amd64:"
    "linux:arm64:"
)

# Get git metadata for version injection
commit=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
buildTime=$(date -u '+%Y-%m-%dT%H:%M:%SZ')

for build in "${builds[@]}"; do
    IFS=':' read -r os arch ext <<< "$build"
    output_name="$OUTPUT_DIR/$APP_NAME-$os-$arch$ext"
    
    echo "Building for $os/$arch..."
    
    # Static build: disable CGO, strip symbols
    # Inject version metadata at build time
    ldflags="-s -w -X main.Version=$VERSION -X main.Commit=$commit -X main.BuildTime=$buildTime"
    
    CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build \
        -ldflags "$ldflags" \
        -o "$output_name" .
    
    if [ $? -eq 0 ]; then
        echo "  ✓ $output_name"
    else
        echo "  ✗ Failed to build for $os/$arch"
        exit 1
    fi
done

echo ""
echo "Build complete! Static binaries in the $OUTPUT_DIR/ directory."
echo "No runtime library dependencies or C runtime required."
