.PHONY: all clean build-windows build-darwin build-linux build-all help test lint

APP_NAME := Find-Meraki-Ports-With-MAC
VERSION := 1.0.0
OUTPUT_DIR := bin
# Static build flags: no CGO, strip symbols, inject version metadata at build time
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME)"

all: build-all

help:
	@echo "Available targets:"
	@echo "  make test           - Run unit tests with race detection"
	@echo "  make lint           - Run linter (go vet)"
	@echo "  make build-all      - Build for all platforms (static, no RTL)"
	@echo "  make build-windows  - Build for Windows (amd64, arm64)"
	@echo "  make build-darwin   - Build for macOS (amd64, arm64)"
	@echo "  make build-linux    - Build for Linux (amd64, arm64)"
	@echo "  make clean          - Remove build artifacts"
	@echo ""
	@echo "Static builds: CGO_ENABLED=0, no C runtime dependencies"

test:
	@echo "Running unit tests with race detection..."
	unset GOOS GOARCH CGO_ENABLED; go test -v ./...

lint:
	@echo "Running linter (go vet)..."
	unset GOOS GOARCH CGO_ENABLED; go vet ./...
	@echo "Running golangci-lint..."
	unset GOOS GOARCH CGO_ENABLED; golangci-lint run ./...

$(OUTPUT_DIR):
	mkdir -p $(OUTPUT_DIR)

build-windows: $(OUTPUT_DIR)
	@echo "Building for Windows amd64 (static)..."
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(OUTPUT_DIR)/$(APP_NAME)-windows-amd64.exe .
	@echo "Building for Windows arm64 (static)..."
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o $(OUTPUT_DIR)/$(APP_NAME)-windows-arm64.exe .

build-darwin: $(OUTPUT_DIR)
	@echo "Building for macOS amd64 (static)..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(OUTPUT_DIR)/$(APP_NAME)-darwin-amd64 .
	@echo "Building for macOS arm64 (static)..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(OUTPUT_DIR)/$(APP_NAME)-darwin-arm64 .

build-linux: $(OUTPUT_DIR)
	@echo "Building for Linux amd64 (static)..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(OUTPUT_DIR)/$(APP_NAME)-linux-amd64 .
	@echo "Building for Linux arm64 (static)..."
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(OUTPUT_DIR)/$(APP_NAME)-linux-arm64 .

build-all: test lint build-windows build-darwin build-linux
	@echo ""
	@echo "Build complete! Static binaries in the $(OUTPUT_DIR)/ directory."
	@echo "No runtime library dependencies or C runtime required."

clean:
	rm -rf $(OUTPUT_DIR)
	@echo "Cleaned build artifacts."
