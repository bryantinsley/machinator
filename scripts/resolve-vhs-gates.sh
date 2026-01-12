#!/bin/bash
# Find and resolve "named" VHS gates to actual GitHub Run IDs

# Find all open gates with external-ref "gh:run:vhs"
GATES=$(bd gate list --json 2>/dev/null | jq -r '.[] | select(.status == "open" and .external_ref == "gh:run:vhs") | .id')

if [ -z "$GATES" ]; then
    exit 0
fi

# Get the most recent VHS run ID
RUN_ID=$(gh run list --workflow vhs.yml --limit 1 --json databaseId --jq '.[0].databaseId' 2>/dev/null)

if [ -n "$RUN_ID" ]; then
    for GATE_ID in $GATES; do
        echo "Resolving gate $GATE_ID to GitHub Run $RUN_ID"
        # Update the gate to use the specific run ID
        bd update "$GATE_ID" --external-ref="gh:run" --await-id="$RUN_ID"
    done
fi
