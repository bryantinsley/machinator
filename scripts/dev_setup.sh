#!/bin/bash
# Development environment setup for machinator
# Run this after cloning the repo

set -e

echo "🔧 Setting up development environment..."

# Check for required tools
echo ""
echo "Checking dependencies..."

# Check for Go
if command -v go &> /dev/null; then
    echo "  ✅ go found: $(go version | awk '{print $3}')"
else
    echo "  ❌ go not found"
    echo ""
    echo "  Install Go from: https://go.dev/dl/"
    echo "  Or with brew: brew install go"
    echo ""
    exit 1
fi

# Check for bd (beads)
if command -v bd &> /dev/null; then
    echo "  ✅ bd (beads) found: $(bd --version 2>/dev/null || echo 'installed')"
else
    echo "  ❌ bd (beads) not found"
    echo ""
    echo "  Install beads with:"
    echo "    curl -sSL https://raw.githubusercontent.com/steveyegge/beads/main/scripts/install.sh | bash"
    echo ""
    exit 1
fi

# Check for buildifier (optional)
if command -v buildifier &> /dev/null; then
    echo "  ✅ buildifier found"
else
    echo "  ⚠️  buildifier not found (optional, for Bazel formatting)"
    echo "     Install with: brew install buildifier"
fi

# Check for docker
if command -v docker &> /dev/null; then
    echo "  ✅ docker found: $(docker --version | head -n 1)"
else
    echo "  ⚠️  docker not found. Install Docker Desktop from: https://docker.io"
    echo "     docker is required for running VHS terminal recording in a container."
fi

# Configure git to use project hooks directory
echo ""
echo "Configuring git hooks..."
git config core.hooksPath scripts/hooks
echo "  ✅ Git hooks configured (scripts/hooks)"

# Verify beads can load the issues
echo ""
echo "Verifying beads setup..."
if bd list --json &> /dev/null; then
    ISSUE_COUNT=$(bd list --json 2>/dev/null | wc -l | tr -d ' ')
    echo "  ✅ Beads initialized ($ISSUE_COUNT issues loaded)"
else
    echo "  ⚠️  Beads database needs initialization"
    echo "     Run: bd sync"
fi

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "✅ Development environment ready!"
echo ""
echo "Next steps:"
echo "  1. Build the TUI: bazel build //:tui"
echo "  2. Run the TUI: bazel run //:tui"
echo "═══════════════════════════════════════════════════════════════"
