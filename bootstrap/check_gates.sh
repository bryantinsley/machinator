#!/bin/bash
# Check for gate beads that may have resolved
# Gates are beads with type=gate that block other work

# Check if bd is available
if ! command -v bd &> /dev/null; then
    exit 0
fi

# List gate-type beads and check their status
GATES=$(bd list --type=gate --status=open --json 2>/dev/null)
if [ -z "$GATES" ] || [ "$GATES" = "[]" ] || [ "$GATES" = "null" ]; then
    # No open gates, nothing to do
    exit 0
fi

GATE_COUNT=$(echo "$GATES" | jq 'length' 2>/dev/null || echo "0")
if [ "$GATE_COUNT" -gt 0 ]; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] 🚧 $GATE_COUNT open gate(s) found"
fi

exit 0
