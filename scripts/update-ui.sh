#!/bin/bash
set -e

# Ensure we're in the project root
if [ ! -f "go.mod" ]; then
    echo "❌ Error: This script must be run from the project root."
    exit 1
fi

# Set Go environment to use local caches within the project
if [ -f "bootstrap/setup_go_env.sh" ]; then
    source bootstrap/setup_go_env.sh
else
    export GOPATH="$(pwd)/.go-cache"
    export GOCACHE="$(pwd)/.go-build-cache"
    export GOMODCACHE="$(pwd)/.go-cache/pkg/mod"
    mkdir -p "$GOPATH" "$GOCACHE" "$GOMODCACHE"
fi

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🔍 Updating TUI Golden Files..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if go test ./... -update; then
    echo "✅ Golden files updated successfully."
else
    echo "❌ Error: Failed to update golden files."
    exit 1
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🎥 Regenerating VHS Tapes..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Check if vhs is installed
if ! command -v vhs &> /dev/null; then
    echo "❌ Error: vhs is not installed. Please install it (e.g., brew install vhs)."
    exit 1
fi

# Find and run all .tape files in orchestrator/e2e
# We use shell globbing
for tape in orchestrator/e2e/*.tape; do
    if [ -f "$tape" ]; then
        echo "▶️  Running $tape..."
        vhs "$tape"
    fi
done

echo ""
echo "✅ UI update complete!"