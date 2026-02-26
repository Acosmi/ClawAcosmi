#!/usr/bin/env bash
# sign-console.sh — Sign the Wails argus-console.app with local "Argus Dev" certificate
#
# Why: Without code signing, each rebuild produces a different binary hash,
#      causing macOS to revoke previously granted permissions (Accessibility,
#      Screen Recording, etc.). Signing with a persistent Keychain certificate
#      keeps the identity stable across rebuilds so permissions survive.
#
# Prerequisites:
#   - "Argus Dev" self-signed certificate in Keychain
#     (create via: Keychain Access → Certificate Assistant → Create a Certificate
#      → Name: "Argus Dev", Type: Code Signing)
#   - Wails app built: wails-console/build/bin/argus-console.app
#
# Usage: ./scripts/package/sign-console.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
APP_PATH="$PROJECT_ROOT/wails-console/build/bin/argus-console.app"
ENTITLEMENTS="$PROJECT_ROOT/wails-console/build/darwin/entitlements.plist"
SIGN_IDENTITY="Argus Dev"

# ---------- Preflight checks ----------

if [ ! -d "$APP_PATH" ]; then
    echo "❌ $APP_PATH not found. Run 'make console' first."
    exit 1
fi

if [ ! -f "$ENTITLEMENTS" ]; then
    echo "❌ Entitlements file not found: $ENTITLEMENTS"
    exit 1
fi

# Verify signing identity exists in Keychain
if ! security find-identity -v -p codesigning | grep -q "$SIGN_IDENTITY"; then
    echo "❌ Signing identity '$SIGN_IDENTITY' not found in Keychain."
    echo "   Create it via: Keychain Access → Certificate Assistant → Create a Certificate"
    echo "   Name: $SIGN_IDENTITY  |  Type: Code Signing"
    exit 1
fi

# ---------- Strip extended attributes (prevent codesign errors) ----------

echo "🧹 Removing extended attributes..."
xattr -cr "$APP_PATH"

# ---------- Sign ----------

echo "🔏 Signing $APP_PATH with identity '$SIGN_IDENTITY'..."
codesign --force --deep --options runtime \
    -s "$SIGN_IDENTITY" \
    --entitlements "$ENTITLEMENTS" \
    "$APP_PATH"

# ---------- Verify ----------

echo "✅ Verifying signature..."
codesign --verify --deep --verbose=2 "$APP_PATH" 2>&1

echo ""
echo "🔑 Signature details:"
codesign -dvv "$APP_PATH" 2>&1 | grep -E "^(Authority|Identifier|TeamIdentifier|Signature)"

echo ""
echo "✅ argus-console.app signed successfully with '$SIGN_IDENTITY'"
echo "   System permissions (Accessibility, Screen Recording, etc.) will persist across rebuilds."
