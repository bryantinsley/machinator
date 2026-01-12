#!/bin/bash
set -e

# Script to wait for the VHS GitHub Action to complete for the current commit.
# Used by agents to gate progress until VHS tapes are generated.

COMMIT_HASH=$(git rev-parse HEAD)
echo "Looking for VHS workflow run for commit: $COMMIT_HASH"

# Timeout loop to find the run (wait up to 2 minutes for it to be scheduled)
TIMEOUT=120
START_TIME=$(date +%s)

RUN_ID=""

while [[ -z "$RUN_ID" ]]; do
  CURRENT_TIME=$(date +%s)
  ELAPSED=$((CURRENT_TIME - START_TIME))
  
  if [[ $ELAPSED -gt $TIMEOUT ]]; then
    echo "Timeout waiting for workflow to start."
    exit 1
  fi

  # Filter by workflow filename 'vhs.yml' and commit.
  # We select the most recent one (index 0) if multiple exist for some reason.
  RUN_ID=$(gh run list --commit "$COMMIT_HASH" --workflow vhs.yml --json databaseId --jq '.[0].databaseId')
  
  if [[ -n "$RUN_ID" ]]; then
    echo "Found Run ID: $RUN_ID"
    break
  fi
  
  echo "Waiting for workflow to start... ($ELAPSED/${TIMEOUT}s)"
  sleep 5
done

echo "Watching workflow run $RUN_ID..."
gh run watch "$RUN_ID" --exit-status
echo "VHS Workflow completed successfully."
