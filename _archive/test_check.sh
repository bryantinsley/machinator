#!/bin/bash

check_ttyd() {
    if command -v ttyd &> /dev/null; then
        echo "  ✅ ttyd found: $(ttyd --version | head -n 1)"
    else
        echo "  ⚠️  ttyd not found. Install with: brew install ttyd"
        echo "     ttyd is required for VHS terminal recording."
    fi
}

echo "Testing with ttyd present:"
check_ttyd

echo ""
echo "Testing with ttyd missing:"
(
    PATH="/usr/bin:/bin"
    check_ttyd
)
