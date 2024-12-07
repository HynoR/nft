#!/bin/bash

# Default architecture
ARCH="amd64"

# Check if architecture parameter is provided
if [ $# -eq 1 ]; then
    ARCH="$1"
fi

# Binary name
BINARY="nat-go"

# Build with optimizations
echo "Building for architecture: $ARCH"
CGO_ENABLED=0 GOOS=linux GOARCH=$ARCH go build -ldflags="-s -w" -o $BINARY 

# Check if build was successful
if [ $? -ne 0 ]; then
    echo "Build failed!"
    exit 1
fi

# Compress with UPX if available
if command -v upx >/dev/null 2>&1; then
    echo "Compressing with UPX..."
    upx --best --lzma $BINARY
else
    echo "UPX not found, skipping compression"
fi

echo "Build completed: $BINARY"