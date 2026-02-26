#!/usr/bin/env bash
# build-pkg.sh — Build macOS .pkg installer for Argus Compound
#
# Usage: ./scripts/package/build-pkg.sh [--sign "Developer ID Installer: ..."]
#
# Prerequisites: run `make app` first to create build/Argus.app

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
BUILD_DIR="$PROJECT_ROOT/build"
APP_PATH="$BUILD_DIR/Argus.app"
PKG_ID="com.argus.compound"
PKG_VERSION="1.0.0"

# Parse optional --sign argument
SIGN_IDENTITY=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --sign) SIGN_IDENTITY="$2"; shift 2 ;;
        *) echo "Unknown argument: $1"; exit 1 ;;
    esac
done

# Verify .app exists
if [ ! -d "$APP_PATH" ]; then
    echo "❌ $APP_PATH not found. Run 'make app' first."
    exit 1
fi

echo "📦 Building component package..."
pkgbuild \
    --root "$APP_PATH" \
    --identifier "$PKG_ID" \
    --version "$PKG_VERSION" \
    --install-location "/Applications/Argus.app" \
    "$BUILD_DIR/Argus-component.pkg"

echo "📦 Building product installer..."
SIGN_ARGS=()
if [ -n "$SIGN_IDENTITY" ]; then
    SIGN_ARGS=(--sign "$SIGN_IDENTITY")
fi

productbuild \
    --distribution "$SCRIPT_DIR/distribution.xml" \
    --package-path "$BUILD_DIR" \
    "${SIGN_ARGS[@]}" \
    "$BUILD_DIR/Argus-Installer.pkg"

# Clean up intermediate component package
rm -f "$BUILD_DIR/Argus-component.pkg"

echo "✅ Installer created: $BUILD_DIR/Argus-Installer.pkg"
ls -lh "$BUILD_DIR/Argus-Installer.pkg"
