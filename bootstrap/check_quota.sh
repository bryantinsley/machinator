#!/bin/bash

# Proper quota checker using gemini CLI's --dump-quota flag
# Returns 0 if quota available, 1 if exhausted, 2 if error

# Check if gemini CLI with --dump-quota is available
GEMINI_CLI="/Users/bryantinsley/Code/gemini-cli/packages/cli/dist/index.js"

if [ ! -f "$GEMINI_CLI" ]; then
    echo "⚠️  Custom gemini CLI not found at $GEMINI_CLI"
    exit 2
fi

# Get quota for gemini-2.5-pro (the model we're using)
QUOTA_DATA=$(node "$GEMINI_CLI" --dump-quota 2>/dev/null | jq '.buckets[] | select(.modelId == "gemini-2.5-pro")')

if [ -z "$QUOTA_DATA" ]; then
    echo "⚠️  Could not retrieve quota data"
    exit 2
fi

# Extract remaining fraction
REMAINING=$(echo "$QUOTA_DATA" | jq -r '.remainingFraction')
RESET_TIME=$(echo "$QUOTA_DATA" | jq -r '.resetTime')

# Check if quota is low (less than 2%)
if (( $(echo "$REMAINING < 0.02" | bc -l) )); then
    echo "⚠️  Quota exhausted (${REMAINING}% remaining, resets at $RESET_TIME)"
    exit 1
fi

# Quota available
PERCENT=$(echo "$REMAINING * 100" | bc -l | cut -d. -f1)
echo "✅ Quota available (${PERCENT}% remaining, resets at $RESET_TIME)"
exit 0
