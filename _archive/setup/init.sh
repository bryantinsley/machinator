#!/bin/bash
# Machinator initialization script
# Creates ~/.machinator directory and builds the custom Gemini CLI

set -e

MACHINATOR_DIR="$HOME/.machinator"
RESOURCES_DIR="$MACHINATOR_DIR/resources"
GEMINI_CLI_REPO="https://github.com/bryantinsley/gemini-cli-mods.git"
GEMINI_CLI_DIR="$RESOURCES_DIR/gemini-cli-mods"
SETTINGS_FILE="$MACHINATOR_DIR/settings.json"

echo "🔧 Initializing Machinator..."
echo ""

# ═══════════════════════════════════════════════════════════════════════════════
# 1. Create directory structure
# ═══════════════════════════════════════════════════════════════════════════════
echo "📁 Creating directory structure..."
mkdir -p "$RESOURCES_DIR"
echo "  ✅ Created $MACHINATOR_DIR"
echo "  ✅ Created $RESOURCES_DIR"

# ═══════════════════════════════════════════════════════════════════════════════
# 2. Clone or update the Gemini CLI repo
# ═══════════════════════════════════════════════════════════════════════════════
echo ""
echo "📥 Setting up Gemini CLI..."

if [ -d "$GEMINI_CLI_DIR" ]; then
    echo "  → Repository exists, pulling latest..."
    cd "$GEMINI_CLI_DIR"
    git pull --ff-only || {
        echo "  ⚠️  Pull failed, trying reset..."
        git fetch origin
        git reset --hard origin/main
    }
else
    echo "  → Cloning repository..."
    git clone "$GEMINI_CLI_REPO" "$GEMINI_CLI_DIR"
fi

cd "$GEMINI_CLI_DIR"
echo "  ✅ Repository ready at $GEMINI_CLI_DIR"

# ═══════════════════════════════════════════════════════════════════════════════
# 3. Build the Gemini CLI
# ═══════════════════════════════════════════════════════════════════════════════
echo ""
echo "🔨 Building Gemini CLI..."

# Check for Go
if ! command -v go &> /dev/null; then
    echo "  ❌ Go not found. Please install Go first."
    exit 1
fi

# Try to build - first check what's in the repo
if [ -f "go.mod" ]; then
    echo "  → Go module detected, building..."
    go build -o gemini ./...
    GEMINI_BIN="$GEMINI_CLI_DIR/gemini"
elif [ -f "Makefile" ]; then
    echo "  → Makefile detected, running make..."
    make
    # Find the binary
    GEMINI_BIN=$(find . -type f -name "gemini" -perm +111 | head -1)
else
    echo "  → Attempting direct go build..."
    go build -o gemini .
    GEMINI_BIN="$GEMINI_CLI_DIR/gemini"
fi

# Verify binary exists
if [ ! -f "$GEMINI_BIN" ]; then
    # Try to find it
    GEMINI_BIN=$(find "$GEMINI_CLI_DIR" -type f -name "gemini" -perm +111 2>/dev/null | head -1)
fi

if [ -z "$GEMINI_BIN" ] || [ ! -f "$GEMINI_BIN" ]; then
    echo "  ❌ Build failed - could not find gemini binary"
    echo "  Please check the build output and try manually:"
    echo "    cd $GEMINI_CLI_DIR"
    echo "    go build -o gemini ."
    exit 1
fi

# Make absolute path
GEMINI_BIN=$(cd "$(dirname "$GEMINI_BIN")" && pwd)/$(basename "$GEMINI_BIN")
echo "  ✅ Built: $GEMINI_BIN"

# Verify it runs
if ! "$GEMINI_BIN" --version &> /dev/null; then
    echo "  ⚠️  Warning: gemini --version failed, binary may not work correctly"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# 4. Write settings.json
# ═══════════════════════════════════════════════════════════════════════════════
echo ""
echo "📝 Writing settings..."

cat > "$SETTINGS_FILE" << EOF
{
  "gemini_cli_path": "$GEMINI_BIN",
  "initialized_at": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "gemini_cli_repo": "$GEMINI_CLI_REPO"
}
EOF

echo "  ✅ Settings saved to $SETTINGS_FILE"

# ═══════════════════════════════════════════════════════════════════════════════
# Done!
# ═══════════════════════════════════════════════════════════════════════════════
echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "✅ Machinator initialized!"
echo ""
echo "Gemini CLI: $GEMINI_BIN"
echo "Settings:   $SETTINGS_FILE"
echo ""
echo "You can now run: bazel run //:tui"
echo "═══════════════════════════════════════════════════════════════"
