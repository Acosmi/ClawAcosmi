#!/bin/bash
# build-app.sh — Build go-sensory into a macOS .app bundle
#
# Usage:
#   bash scripts/build-app.sh
#
# Output:
#   ./Argus Sensory.app
#
# The .app bundle supports two launch modes:
#   1. Double-click (or `open`)  → HTTP dashboard mode (launch.sh)
#   2. Claude Desktop MCP        → stdio MCP mode (mcp-launcher.sh)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
APP_NAME="Argus Sensory"
BUNDLE_ID="cc.fgfn.argus.sensory"
BINARY_NAME="sensory-server"
VERSION="1.0.0"

APP_DIR="${PROJECT_DIR}/${APP_NAME}.app"
CONTENTS_DIR="${APP_DIR}/Contents"
MACOS_DIR="${CONTENTS_DIR}/MacOS"
RESOURCES_DIR="${CONTENTS_DIR}/Resources"

echo "╔═══════════════════════════════════════════════╗"
echo "║   Building Argus Sensory.app                  ║"
echo "╚═══════════════════════════════════════════════╝"

# --- Step 1: Compile Go binary ---
echo "→ Compiling Go binary..."
cd "$PROJECT_DIR"
CGO_ENABLED=1 go build -o "$BINARY_NAME" ./cmd/server/
echo "  ✓ Built: ${BINARY_NAME}"

# --- Step 2: Create .app bundle structure ---
echo "→ Creating .app bundle..."
rm -rf "$APP_DIR"
mkdir -p "$MACOS_DIR"
mkdir -p "$RESOURCES_DIR"

# Move binary into bundle
mv "$BINARY_NAME" "$MACOS_DIR/"

# Copy VLM config if exists
if [ -f "$PROJECT_DIR/vlm-config.json" ]; then
    cp "$PROJECT_DIR/vlm-config.json" "$RESOURCES_DIR/"
    echo "  ✓ Copied vlm-config.json to Resources"
fi

# --- Step 3: Create HTTP launcher (double-click mode) ---
cat > "$MACOS_DIR/launch.sh" << 'LAUNCHER'
#!/bin/bash
# Launch the sensory server in HTTP dashboard mode
DIR="$(cd "$(dirname "$0")" && pwd)"
RESOURCES_DIR="$(cd "$DIR/../Resources" && pwd)"

# Use VLM config from Resources if available
VLM_CONFIG="$RESOURCES_DIR/vlm-config.json"
if [ -f "$VLM_CONFIG" ]; then
    exec "$DIR/sensory-server" --fps 2 --open-browser=true --vlm-config "$VLM_CONFIG" "$@"
else
    exec "$DIR/sensory-server" --fps 2 --open-browser=true "$@"
fi
LAUNCHER
chmod +x "$MACOS_DIR/launch.sh"

# --- Step 4: Create MCP launcher (Claude Desktop / AI Agent mode) ---
cat > "$MACOS_DIR/mcp-launcher.sh" << 'MCP_LAUNCHER'
#!/bin/bash
# Launch the sensory server in MCP stdio mode
# This script is intended to be called by AI agents (Claude Desktop, etc.)
# via their MCP server configuration.
#
# Usage in claude_desktop_config.json:
#   {
#     "mcpServers": {
#       "argus-sensory": {
#         "command": "/path/to/Argus Sensory.app/Contents/MacOS/mcp-launcher.sh",
#         "args": []
#       }
#     }
#   }
DIR="$(cd "$(dirname "$0")" && pwd)"
RESOURCES_DIR="$(cd "$DIR/../Resources" && pwd)"

# Use VLM config from Resources if available
VLM_CONFIG="$RESOURCES_DIR/vlm-config.json"
if [ -f "$VLM_CONFIG" ]; then
    exec "$DIR/sensory-server" --mcp --vlm-config "$VLM_CONFIG" "$@"
else
    exec "$DIR/sensory-server" --mcp "$@"
fi
MCP_LAUNCHER
chmod +x "$MACOS_DIR/mcp-launcher.sh"

# --- Step 5: Create Info.plist ---
cat > "$CONTENTS_DIR/Info.plist" << PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleName</key>
    <string>${APP_NAME}</string>
    <key>CFBundleDisplayName</key>
    <string>${APP_NAME}</string>
    <key>CFBundleIdentifier</key>
    <string>${BUNDLE_ID}</string>
    <key>CFBundleVersion</key>
    <string>${VERSION}</string>
    <key>CFBundleShortVersionString</key>
    <string>${VERSION}</string>
    <key>CFBundleExecutable</key>
    <string>${BINARY_NAME}</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>LSMinimumSystemVersion</key>
    <string>12.3</string>
    <key>NSHighResolutionCapable</key>
    <true/>
    <key>LSUIElement</key>
    <true/>
    <key>NSScreenCaptureUsageDescription</key>
    <string>Argus Sensory needs screen capture access to monitor your display for AI-powered visual understanding.</string>
    <key>NSAccessibilityUsageDescription</key>
    <string>Argus Sensory needs accessibility access to perform mouse clicks and keyboard input on your behalf.</string>
</dict>
</plist>
PLIST

# --- Step 6: Code sign the bundle ---
# macOS TCC requires a consistent code signature identifier for Screen Recording
# permission. Without this, Go binaries default to identifier "a.out".
echo "→ Code signing .app bundle..."
codesign -s - --identifier "${BUNDLE_ID}" --force --deep "$APP_DIR" 2>&1
echo "  ✓ Ad-hoc signed with identifier ${BUNDLE_ID}"

# --- Step 7: Summary ---
APP_SIZE=$(du -sh "$APP_DIR" | cut -f1)
MCP_LAUNCHER_PATH="${MACOS_DIR}/mcp-launcher.sh"

echo ""
echo "═══════════════════════════════════════════════"
echo "  ✅ Build complete!"
echo ""
echo "  App:  ${APP_DIR}"
echo "  Size: ${APP_SIZE}"
echo ""
echo "  📺 HTTP Dashboard mode:"
echo "    open '${APP_DIR}'"
echo ""
echo "  🤖 MCP Server mode (Claude Desktop):"
echo "    Add to claude_desktop_config.json:"
echo ""
echo "    {"
echo "      \"mcpServers\": {"
echo "        \"argus-sensory\": {"
echo "          \"command\": \"${MCP_LAUNCHER_PATH}\","
echo "          \"args\": []"
echo "        }"
echo "      }"
echo "    }"
echo ""
echo "  🔐 Screen Recording Permission:"
echo "    System Settings → Privacy → Screen Recording"
echo "    Look for '${APP_NAME}' and enable it."
echo "═══════════════════════════════════════════════"
