#!/bin/bash
# Check and resolve GitHub workflow gates
# Run every 5 minutes from the orchestrator

# Check for open gates
OPEN_GATES=$(bd gate list --json 2>/dev/null | jq -r ".[] | select(.status == \"open\")")

if [ -z "$OPEN_GATES" ]; then
    exit 0
fi

# Resolve "named" gates to specific IDs
./scripts/resolve-vhs-gates.sh

# Use bd gate check to evaluate gates
bd gate check --type=gh:run 2>/dev/null || true
