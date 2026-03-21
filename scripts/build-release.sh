#!/bin/bash
#
# DevContext AI - Build Release Script
# Cross-compiles Go binary for all platforms
#

set -e

VERSION="${1:-dev}"
BUILD_DIR="dist"
CLI_DIR="cli"

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

info() {
    echo -e "${BLUE}[BUILD]${NC} $1"
}

success() {
    echo -e "${GREEN}[DONE]${NC} $1"
}

# Platforms to build
PLATFORMS=(
    "darwin/amd64"
    "darwin/arm64"
    "linux/amd64"
    "linux/arm64"
    "windows/amd64"
)

# Clean build directory
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

info "Building DevContext CLI v$VERSION"
echo ""

for PLATFORM in "${PLATFORMS[@]}"; do
    GOOS="${PLATFORM%/*}"
    GOARCH="${PLATFORM#*/}"

    OUTPUT_NAME="devctx_${VERSION}_${GOOS}_${GOARCH}"

    if [ "$GOOS" = "windows" ]; then
        OUTPUT_NAME+=".exe"
    fi

    info "Building $GOOS/$GOARCH..."

    # Build with CGO disabled for static binaries
    env GOOS="$GOOS" GOARCH="$GOARCH" CGO_ENABLED=0 \
        go build \
        -ldflags="-s -w -X main.version=$VERSION" \
        -o "$BUILD_DIR/$OUTPUT_NAME" \
        "./$CLI_DIR"

    # Create tarball (or zip for Windows)
    cd "$BUILD_DIR"
    if [ "$GOOS" = "windows" ]; then
        zip -q "${OUTPUT_NAME%.exe}.zip" "$OUTPUT_NAME"
        rm "$OUTPUT_NAME"
    else
        tar -czf "${OUTPUT_NAME}.tar.gz" "$OUTPUT_NAME"
        rm "$OUTPUT_NAME"
    fi
    cd ..

    success "  -> $OUTPUT_NAME"
done

echo ""

# Generate checksums
info "Generating checksums..."
cd "$BUILD_DIR"
sha256sum * > checksums.txt
cd ..

success "Checksums generated"

echo ""
info "Build artifacts:"
ls -la "$BUILD_DIR"

echo ""
success "Release build complete!"
