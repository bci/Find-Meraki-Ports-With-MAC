#!/bin/bash
# release.sh — Build all platform binaries and publish a GitHub release.
#
# Copyright (C) 2025 Kent Behrends — GPL v3
#
# Prerequisites:
#   - gh CLI authenticated (gh auth login)
#   - git tag matching VERSION must exist locally (created by this script if not)
#
# Usage:
#   ./release.sh              # build + release using VERSION from this script
#   ./release.sh --dry-run    # build only, skip gh release commands
#   ./release.sh --notes "Release notes text"   # custom release notes
#
# The script will:
#   1. Run tests and linting
#   2. Build static binaries for all 6 platforms into ./bin/
#   3. Create (or update) the git tag v$VERSION on the current commit
#   4. Create (or update) the GitHub release with all 6 binaries attached

set -e

# ── Configuration ──────────────────────────────────────────────────────────────
APP_NAME="Find-Meraki-Ports-With-MAC"
VERSION="1.3.1"
TAG="v$VERSION"
OUTPUT_DIR="bin"
DRY_RUN=false
RELEASE_NOTES=""

# ── Argument parsing ───────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
    case "$1" in
        --dry-run)   DRY_RUN=true ;;
        --notes)     RELEASE_NOTES="$2"; shift ;;
        --notes=*)   RELEASE_NOTES="${1#--notes=}" ;;
        *)           echo "Unknown argument: $1"; exit 1 ;;
    esac
    shift
done

# ── Checks ─────────────────────────────────────────────────────────────────────
if ! command -v gh &>/dev/null; then
    echo "ERROR: gh CLI not found. Install from https://cli.github.com/" >&2
    exit 1
fi

if [ "$DRY_RUN" = false ] && ! gh auth status &>/dev/null; then
    echo "ERROR: gh CLI not authenticated. Run: gh auth login" >&2
    exit 1
fi

unset GOOS GOARCH CGO_ENABLED 2>/dev/null || true

# ── Tests & lint ───────────────────────────────────────────────────────────────
echo "Running unit tests..."
go test -race -count=1 ./...
echo "Tests passed"
echo ""

echo "Running go vet..."
go vet ./...
echo "go vet passed"
echo ""

if command -v golangci-lint &>/dev/null; then
    echo "Running golangci-lint..."
    golangci-lint run ./...
    echo "golangci-lint passed"
    echo ""
else
    echo "golangci-lint not found — skipping (install for full lint coverage)"
    echo ""
fi

# ── Version metadata ───────────────────────────────────────────────────────────
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS="-s -w -X main.Version=$VERSION -X main.Commit=$COMMIT -X main.BuildTime=$BUILD_TIME"

echo "Building $APP_NAME $TAG (commit $COMMIT)..."
echo ""

# ── Cross-platform builds ──────────────────────────────────────────────────────
mkdir -p "$OUTPUT_DIR"

declare -a BUILDS=(
    "windows:amd64:.exe"
    "windows:arm64:.exe"
    "darwin:amd64:"
    "darwin:arm64:"
    "linux:amd64:"
    "linux:arm64:"
)

ARTIFACTS=()
for build in "${BUILDS[@]}"; do
    IFS=':' read -r os arch ext <<< "$build"
    output="$OUTPUT_DIR/$APP_NAME-$os-$arch$ext"

    echo "  Building $os/$arch → $output"
    CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -ldflags "$LDFLAGS" -o "$output" .
    ARTIFACTS+=("$output")
done

echo ""
echo "All binaries built in ./$OUTPUT_DIR/"
echo ""

if [ "$DRY_RUN" = true ]; then
    echo "DRY RUN — skipping git tag and gh release steps."
    echo "Artifacts that would be uploaded:"
    for a in "${ARTIFACTS[@]}"; do echo "  $a"; done
    exit 0
fi

# ── Git tag ────────────────────────────────────────────────────────────────────
echo "Tagging $TAG on $(git rev-parse HEAD)..."
git tag -f "$TAG"
git push origin -f "$TAG"
echo ""

# ── GitHub release ─────────────────────────────────────────────────────────────
# Build default release notes from CHANGELOG if none provided
if [ -z "$RELEASE_NOTES" ] && [ -f "CHANGELOG.md" ]; then
    # Extract the first version block from CHANGELOG.md
    RELEASE_NOTES=$(awk "/^## \[$VERSION\]/{found=1; next} found && /^## \[/{exit} found{print}" CHANGELOG.md)
fi
if [ -z "$RELEASE_NOTES" ]; then
    RELEASE_NOTES="Release $TAG"
fi

# Delete existing release if present (allows re-running the script)
if gh release view "$TAG" &>/dev/null; then
    echo "Deleting existing GitHub release $TAG..."
    gh release delete "$TAG" --yes
    echo ""
fi

echo "Creating GitHub release $TAG..."
gh release create "$TAG" \
    --title "$APP_NAME $TAG" \
    --notes "$RELEASE_NOTES" \
    "${ARTIFACTS[@]}"

echo ""
echo "✅ Release $TAG published."
echo "   https://github.com/$(gh repo view --json nameWithOwner -q .nameWithOwner)/releases/tag/$TAG"
