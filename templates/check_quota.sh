#!/bin/bash

# Proper quota checker using gemini CLI's --dump-quota flag
# Returns 0 if quota available, 1 if exhausted, 2 if error

# Check if gemini CLI with --dump-quota is available
GEMINI_CLI="/Users/bryantinsley/Code/gemini-cli/packages/cli/dist/index.js"

if [ ! -f "$GEMINI_CLI" ]; then
    echo "⚠️  Custom gemini CLI not found at $GEMINI_CLI"
    exit 2
fi

# Models to check
MODELS=("gemini-3-flash-preview" "gemini-3-pro-preview")
ANY_LOW=0

for MODEL in "${MODELS[@]}"; do
    # Get quota for the specific model
    # Note: select argument needs to match exactly in the JSON output
    QUOTA_DATA=$(node "$GEMINI_CLI" --dump-quota 2>/dev/null | jq --arg model "$MODEL" '.buckets[] | select(.modelId == $model)')

    if [ -z "$QUOTA_DATA" ]; then
        echo "⚠️  Could not retrieve quota data for $MODEL"
        # Don't fail the whole script if just one data point is missing, but maybe warn
        continue 
    fi

    # Extract remaining fraction
    REMAINING=$(echo "$QUOTA_DATA" | jq -r '.remainingFraction')
    RESET_TIME=$(echo "$QUOTA_DATA" | jq -r '.resetTime')
    PERCENT=$(echo "$REMAINING * 100" | bc -l | cut -d. -f1)

    # Check if quota is low (less than 2%)
    if (( $(echo "$REMAINING < 0.02" | bc -l) )); then
        echo "⚠️  Quota exhausted for $MODEL (${REMAINING}% remaining, resets at $RESET_TIME)"
        ANY_LOW=1
    else
        echo "✅ $MODEL: ${PERCENT}% remaining"
    fi
done

if [ "$ANY_LOW" -eq 1 ]; then
    exit 1
fi

exit 0
