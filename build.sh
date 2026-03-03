#!/bin/bash
# Build script for Find-Meraki-Ports-With-MAC
# Usage:
#   ./build.sh           - run tests + lint, then build ./findmac
#   ./build.sh --package - same as above, then also build static binaries for all platforms in ./bin

PACKAGE=false
for arg in "$@"; do
    if [ "$arg" = "--package" ]; then
        PACKAGE=true
    fi
done

set -e

# Clear any cross-compilation environment variables
unset GOOS GOARCH CGO_ENABLED 2>/dev/null || true

APP_NAME="Find-Meraki-Ports-With-MAC"
VERSION="1.3.1"
OUTPUT_DIR="bin"

# Run unit tests
echo "Running unit tests..."
go test -v ./...
echo "Tests passed"
echo ""

# Run linter
echo "Running linter (go vet)..."
go vet ./...
echo "Linting passed"
echo ""

# Install golangci-lint if not present
if ! command -v golangci-lint &>/dev/null; then
    echo "golangci-lint not found, installing..."
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "$(go env GOPATH)/bin"
fi

# Run golangci-lint
echo "Running golangci-lint..."
golangci-lint run ./...
echo "golangci-lint passed"
echo ""

# Get git metadata for version injection
commit=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
buildTime=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
ldflags="-s -w -X main.Version=$VERSION -X main.Commit=$commit -X main.BuildTime=$buildTime"

# Always build local native executable
echo "Building ./findmac..."
CGO_ENABLED=0 go build -ldflags "$ldflags" -o "./findmac" .
echo "  ./findmac"
echo ""

if [ "$PACKAGE" = false ]; then
    echo "Done. Run './build.sh --package' to also build all platform binaries in ./bin."
    exit 0
fi

# --package: build static binaries for all platforms
mkdir -p "$OUTPUT_DIR"

echo "Building $APP_NAME v$VERSION for all platforms..."
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

for build in "${builds[@]}"; do
    IFS=':' read -r os arch ext <<< "$build"
    output_name="$OUTPUT_DIR/$APP_NAME-$os-$arch$ext"

    echo "Building for $os/$arch..."

    CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build \
        -ldflags "$ldflags" \
        -o "$output_name" .

    echo "  $output_name"
done

echo ""
echo "Build complete! Static binaries in the $OUTPUT_DIR/ directory."
echo "No runtime library dependencies or C runtime required."
