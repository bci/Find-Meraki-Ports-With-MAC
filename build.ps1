#!/usr/bin/env pwsh

<#
.SYNOPSIS
    Build script for Find-Meraki-Ports-With-MAC.

.DESCRIPTION
    Usage:
      .\build.ps1            - run tests + lint, then build .\findmac.exe
      .\build.ps1 -package   - same as above, then also build static binaries for all platforms in .\bin

    Runs "go test -v ./...", "go vet ./..." and, when it is on PATH,
    "golangci-lint run ./...". It then builds .\findmac.exe for
    windows/amd64 (on every host OS) with CGO disabled and version metadata
    injected through -ldflags: main.Version from $Version below, main.Commit
    from git, and main.BuildTime.

    With -package it also cross-compiles six static binaries
    (windows/darwin/linux x amd64/arm64) into .\bin\.

    $Version must be kept in step with build.sh, release.sh and the Makefile
    by hand.

.PARAMETER package
    Also build the six cross-platform binaries into .\bin\.

.EXAMPLE
    .\build.ps1
    Runs tests and lint, then builds .\findmac.exe.

.EXAMPLE
    .\build.ps1 -package
    Additionally builds all platform binaries into .\bin\.

.INPUTS
    None. This script does not accept pipeline input.

.OUTPUTS
    None. Writes progress to the host and binaries to disk. Exits 1 on the
    first failing step.

.NOTES
    Requires go on PATH; git is optional (commit becomes "unknown").
    $env:GOOS, $env:GOARCH and $env:CGO_ENABLED are modified and left set to
    the last build's values when the script ends.
    BuildTime is UTC, matching build.sh and the Makefile.
#>

param(
    [switch]$package
)

$ErrorActionPreference = "Stop"

# Clear any cross-compilation environment variables
$env:GOOS = ""
$env:GOARCH = ""
$env:CGO_ENABLED = ""

$AppName = "Find-Meraki-Ports-With-MAC"
$Version = "1.3.1"
$OutputDir = "bin"

# Run unit tests
Write-Host "Running unit tests..." -ForegroundColor Cyan
try {
    go test -v ./...
    $testExitCode = $LASTEXITCODE
    Write-Host "Test command completed with exit code: $testExitCode" -ForegroundColor Yellow
    if ($testExitCode -ne 0) {
        Write-Host "Tests failed with exit code $testExitCode!" -ForegroundColor Red
        exit 1
    }
} catch {
    Write-Host "Tests failed with exception: $($_.Exception.Message)" -ForegroundColor Red
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

# Run golangci-lint (if available)
Write-Host "Running golangci-lint..." -ForegroundColor Cyan
if (Get-Command golangci-lint -ErrorAction SilentlyContinue) {
    golangci-lint run ./...
    if ($LASTEXITCODE -ne 0) {
        Write-Host "golangci-lint failed!" -ForegroundColor Red
        exit 1
    }
    Write-Host "golangci-lint passed" -ForegroundColor Green
} else {
    Write-Host "golangci-lint not found, skipping..." -ForegroundColor Yellow
}
Write-Host ""

# Get git metadata
$commit = git rev-parse --short HEAD 2>$null
if (!$commit) { $commit = "unknown" }
$buildTime = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$ldflags = "-s -w -X main.Version=$Version -X main.Commit=$commit -X main.BuildTime=$buildTime"

# Always build local Windows executable
Write-Host "Building .\findmac.exe..." -ForegroundColor Cyan
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -ldflags $ldflags -o ".\findmac.exe" .
if ($LASTEXITCODE -eq 0) {
    Write-Host "  .\findmac.exe" -ForegroundColor Green
} else {
    Write-Host "  Failed to build .\findmac.exe" -ForegroundColor Red
    exit 1
}

if (-not $package) {
    Write-Host ""
    Write-Host "Done. Run '.\build.ps1 -package' to also build all platform binaries in .\bin." -ForegroundColor Cyan
    exit 0
}

# -package: build static binaries for all platforms
Write-Host ""
if (!(Test-Path -Path $OutputDir)) {
    New-Item -ItemType Directory -Path $OutputDir | Out-Null
}

Write-Host "Building $AppName v$Version for all platforms..." -ForegroundColor Cyan
Write-Host "Static builds (no C runtime, no external dependencies)" -ForegroundColor Cyan
Write-Host ""

$builds = @(
    @{OS="windows"; Arch="amd64"; Ext=".exe"},
    @{OS="windows"; Arch="arm64"; Ext=".exe"},
    @{OS="darwin";  Arch="amd64"; Ext=""},
    @{OS="darwin";  Arch="arm64"; Ext=""},
    @{OS="linux";   Arch="amd64"; Ext=""},
    @{OS="linux";   Arch="arm64"; Ext=""}
)

foreach ($build in $builds) {
    $os   = $build.OS
    $arch = $build.Arch
    $ext  = $build.Ext
    $outputName = "$OutputDir/$AppName-$os-$arch$ext"

    Write-Host "Building for $os/$arch..." -ForegroundColor Yellow

    $env:GOOS = $os
    $env:GOARCH = $arch
    $env:CGO_ENABLED = "0"

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
