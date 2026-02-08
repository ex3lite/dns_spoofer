#!/usr/bin/env bash
# Build DnsSpoofer for Ubuntu (Linux amd64)
#
# Usage:
#   ./scripts/build-ubuntu.sh
#   make build-ubuntu
#
# Output: dnsspoofer-linux-amd64 in repo root
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

BINARY_NAME="dnsspoofer"
OUTPUT_FILE="${BINARY_NAME}-linux-amd64"
LDFLAGS="-s -w"

cd "$REPO_ROOT"

echo "Building ${OUTPUT_FILE} for Ubuntu (Linux amd64)..."
GOOS=linux GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o "${OUTPUT_FILE}" .

if [[ -f "${OUTPUT_FILE}" ]]; then
    echo "✓ Built ${OUTPUT_FILE}"
    ls -lh "${OUTPUT_FILE}"
else
    echo "✗ Build failed: ${OUTPUT_FILE} not found"
    exit 1
fi
