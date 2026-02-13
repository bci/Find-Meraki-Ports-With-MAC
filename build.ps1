#!/usr/bin/env pwsh
# Build script for Find-Meraki-Ports-With-MAC
# Produces STATIC binaries with no runtime library dependencies
# CGO_ENABLED=0 ensures pure Go compilation
# -ldflags "-s -w" strips symbols for smaller binaries

$ErrorActionPreference = "Stop"

# Clear any cross-compilation environment variables
$env:GOOS = ""
$env:GOARCH = ""
$env:CGO_ENABLED = ""

$AppName = "Find-Meraki-Ports-With-MAC"
$Version = "1.0.0"
$OutputDir = "bin"

# Run unit tests
Write-Host "Running unit tests..." -ForegroundColor Cyan
go test -v ./...
if ($LASTEXITCODE -ne 0) {
    Write-Host "Tests failed!" -ForegroundColor Red
    exit 1
}
Write-Host "Tests passed" -ForegroundColor Green
Write-Host ""

# Run linter
Write-Host "Running linter (go vet)..." -ForegroundColor Cyan
go vet ./...
if ($LASTEXITCODE -ne 0) {
    Write-Host "Linting failed!" -ForegroundColor Red
    exit 1
}
Write-Host "Linting passed" -ForegroundColor Green
Write-Host ""

# Run golangci-lint
Write-Host "Running golangci-lint..." -ForegroundColor Cyan
golangci-lint run ./...
if ($LASTEXITCODE -ne 0) {
    Write-Host "golangci-lint failed!" -ForegroundColor Red
    exit 1
}
Write-Host "golangci-lint passed" -ForegroundColor Green
Write-Host ""

# Create output directory if it doesn't exist
if (!(Test-Path -Path $OutputDir)) {
    New-Item -ItemType Directory -Path $OutputDir | Out-Null
}

Write-Host "Building $AppName v$Version for multiple platforms..." -ForegroundColor Cyan
Write-Host "Static builds (no C runtime, no external dependencies)" -ForegroundColor Cyan
Write-Host ""

# Build configurations: OS, Architecture, Extension
$builds = @(
    @{OS="windows"; Arch="amd64"; Ext=".exe"},
    @{OS="windows"; Arch="arm64"; Ext=".exe"},
    @{OS="darwin"; Arch="amd64"; Ext=""},
    @{OS="darwin"; Arch="arm64"; Ext=""},
    @{OS="linux"; Arch="amd64"; Ext=""},
    @{OS="linux"; Arch="arm64"; Ext=""}
)

# Get git metadata once for all builds
$commit = git rev-parse --short HEAD 2>$null
if (!$commit) {
    $commit = "unknown"
}
$buildTime = Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ"

foreach ($build in $builds) {
    $os = $build.OS
    $arch = $build.Arch
    $ext = $build.Ext
    $outputName = "$OutputDir/$AppName-$os-$arch$ext"
    
    Write-Host "Building for $os/$arch..." -ForegroundColor Yellow
    
    # Static build: disable CGO, strip symbols
    $env:GOOS = $os
    $env:GOARCH = $arch
    $env:CGO_ENABLED = "0"
    
    # Inject version metadata at build time
    $ldflags = "-s -w -X main.Version=$Version -X main.Commit=$commit -X main.BuildTime=$buildTime"
    
    go build -ldflags $ldflags -o $outputName .
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "  $outputName" -ForegroundColor Green
    } else {
        Write-Host "  Failed to build for $os/$arch" -ForegroundColor Red
        exit 1
    }
}

Write-Host ""
Write-Host "Build complete! Static binaries in the $OutputDir/ directory." -ForegroundColor Cyan
Write-Host "No runtime library dependencies or C runtime required." -ForegroundColor Green
