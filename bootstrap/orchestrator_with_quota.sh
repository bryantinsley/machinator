#!/bin/bash

# Quota-aware autonomous agent orchestrator
# Sleeps when quota is exhausted and resumes when refreshed

AGENT_NAME="${BD_AGENT_NAME:-Gemini-01}"
MAX_CYCLES="${BD_MAX_CYCLES:-10000}"
SLEEP_DURATION="${BD_SLEEP_DURATION:-60}"
QUOTA_CHECK_INTERVAL="${BD_QUOTA_CHECK_INTERVAL:-300}"  # Check quota every 5 minutes when sleeping
LOG_FILE="${BD_LOG_FILE:-/tmp/bd_orchestrator_${AGENT_NAME}.log}"
GEMINI_PANE="${BD_GEMINI_PANE}"


# Valid local CLI path was removed by user, sticking to system gemini but with args
GEMINI_CMD_BASE="gemini"
GEMINI_ARGS="--yolo --output-format=text"

# Sandbox Configuration
SANDBOX_POLICY="/Users/bryantinsley/Code/machinator/.gemini/sandbox-macos-custom.sb"
if [ -f "$SANDBOX_POLICY" ]; then
    log() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG_FILE"; } # hoist log for early use
    log "🔒 Enabling custom sandbox profile (SEATBELT_PROFILE=custom)"
    # Prepend env var to the command
    GEMINI_CMD_BASE="SEATBELT_PROFILE=custom gemini"
fi

# Logging function
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG_FILE"
}

# Function to send commands to the Gemini pane
send_to_gemini() {
    if [ -n "$GEMINI_PANE" ]; then
        tmux send-keys -t "$GEMINI_PANE" "$1" C-m
    else
        log "⚠️  GEMINI_PANE not set, cannot send command to Gemini"
    fi
}

# Gemini CLI Path
GEMINI_CLI_PATH="/Users/bryantinsley/Code/gemini-cli/packages/cli/dist/index.js"

# Function to check quota for a specific model
# Returns 0 if available (>2%), 1 if exhausted/error
check_model_quota() {
    local model="$1"
    
    if [ ! -f "$GEMINI_CLI_PATH" ]; then
        log "⚠️  Gemini CLI not found at $GEMINI_CLI_PATH"
        return 1
    fi

    # Get quota data
    local quota_data
    quota_data=$(node "$GEMINI_CLI_PATH" --dump-quota 2>/dev/null | jq --arg m "$model" '.buckets[] | select(.modelId == $m)')
    
    if [ -z "$quota_data" ]; then
        log "⚠️  Could not retrieve quota for $model"
        return 1
    fi
    
    local remaining
    remaining=$(echo "$quota_data" | jq -r '.remainingFraction')
    
    # Check if > 2%
    if (( $(echo "$remaining > 0.02" | bc -l) )); then
        return 0
    else
        return 1
    fi
}

log "🤖 $AGENT_NAME orchestrator starting (quota-aware mode)..."
log "📊 Max cycles: $MAX_CYCLES, Sleep: ${SLEEP_DURATION}s"
log "🎯 Gemini pane: ${GEMINI_PANE:-not set}"

count=0
while [ $count -lt $MAX_CYCLES ]; do
    log ""
    log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    log "📊 Cycle $((count + 1))/$MAX_CYCLES"
    log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    
    # Reset selection
    TASK_ID=""
    MODEL_ARGS=""

    # 1. Check for tasks assigned to this agent (in_progress) first
    log "🔍 Checking for in-progress tasks..."
    # We grab the full JSON object to check description/complexity
    IN_PROGRESS_JSON=$(bd list --status=in_progress --json 2>/dev/null | jq --arg agent "$AGENT_NAME" '[.[] | select(.assignee == $agent)]')
    IN_PROGRESS_COUNT=$(echo "$IN_PROGRESS_JSON" | jq 'length')

    if [ "$IN_PROGRESS_COUNT" -gt 0 ]; then
        # Check if we can run the first in-progress task
        CANDIDATE_ID=$(echo "$IN_PROGRESS_JSON" | jq -r '.[0].id')
        CANDIDATE_DESC=$(echo "$IN_PROGRESS_JSON" | jq -r '.[0].description // ""')
        
        # Determine strict requirements
        if [[ "$CANDIDATE_DESC" == *"CHALLENGE:complex"* ]]; then
            REQ_MODEL="gemini-3-pro-preview"
            if check_model_quota "$REQ_MODEL"; then
                TASK_ID="$CANDIDATE_ID"
                MODEL_ARGS="--model $REQ_MODEL"
                log "� Resuming complex task: $TASK_ID (Pro)"
            else
                log "⏳ Paasing complex task $CANDIDATE_ID - Pro quota exhausted"
            fi
        else
            # Simple (or default)
            REQ_MODEL="gemini-3-flash-preview"
            if check_model_quota "$REQ_MODEL"; then
                TASK_ID="$CANDIDATE_ID"
                MODEL_ARGS="--model $REQ_MODEL"
                log "🔄 Resuming simple task: $TASK_ID (Flash)"
            elif check_model_quota "gemini-3-pro-preview"; then
                 TASK_ID="$CANDIDATE_ID"
                 MODEL_ARGS="--model gemini-3-pro-preview"
                 log "🔄 Resuming simple task: $TASK_ID (Upgraded to Pro)"
            else
                 log "⏳ Pausing simple task $CANDIDATE_ID - All quotas exhausted"
            fi
        fi
    fi

    # 2. If no valid in-progress task selected, check READY tasks
    if [ -z "$TASK_ID" ]; then
        log "🔍 Checking for ready tasks..."
        READY_TASKS_JSON=$(bd ready --json 2>/dev/null)
        READY_COUNT=$(echo "$READY_TASKS_JSON" | jq 'length')
        
        if [ "$READY_COUNT" -gt 0 ]; then
             for i in $(seq 0 $((READY_COUNT - 1))); do
                  CANDIDATE=$(echo "$READY_TASKS_JSON" | jq ".[$i]")
                  CID=$(echo "$CANDIDATE" | jq -r '.id')
                  CDESC=$(echo "$CANDIDATE" | jq -r '.description // ""')
                  
                  if [[ "$CDESC" == *"CHALLENGE:complex"* ]]; then
                      # Complex -> Needs Pro
                      if check_model_quota "gemini-3-pro-preview"; then
                          TASK_ID="$CID"
                          MODEL_ARGS="--model gemini-3-pro-preview"
                          log "⚡ Selected complex task: $TASK_ID (Pro)"
                          break
                      fi
                  else
                      # Simple -> Flash preferred, Pro ok
                      if check_model_quota "gemini-3-flash-preview"; then
                          TASK_ID="$CID"
                          MODEL_ARGS="--model gemini-3-flash-preview"
                          log "⚡ Selected simple task: $TASK_ID (Flash)"
                          break
                      elif check_model_quota "gemini-3-pro-preview"; then
                          TASK_ID="$CID"
                          MODEL_ARGS="--model gemini-3-pro-preview"
                          log "⚡ Selected simple task: $TASK_ID (Upgraded to Pro)"
                          break
                      fi
                  fi
             done
             
             if [ -z "$TASK_ID" ] && [ "$READY_COUNT" -gt 0 ]; then
                 log "⚠️  $READY_COUNT ready tasks exist but none match available quota."
             fi
        fi
    fi

    # If still empty, check if we should run unblocking agent
    if [ -z "$TASK_ID" ]; then
        # Check if there are ANY tasks at all
        TOTAL_TASKS=$(bd list --json 2>/dev/null | jq '. | length' 2>/dev/null || echo "0")
        
        if [ "$TOTAL_TASKS" -gt 0 ]; then

            log "🔍 No ready tasks selected, but $TOTAL_TASKS tasks exist - checking unblocking mode"
            
            # Check Pro quota for unblocking (reasoning required)
            if ! check_model_quota "gemini-3-pro-preview"; then
                 log "💤 Cannot run unblocking - Pro quota exhausted. Sleeping ${SLEEP_DURATION}s..."
                 sleep "$SLEEP_DURATION"
                 continue
            fi
            
            log "⚡ Entering UNBLOCKING mode (Pro quota available)"
            
            # Get blocked tasks details
            BLOCKED_TASK_DETAILS=$(bd blocked 2>/dev/null || echo "Unable to fetch blocked tasks")
            
            # Build unblocking directive
            DIRECTIVE_FILE=".gemini/tmp/unblocking_${TOTAL_TASKS}.txt"
            cat bootstrap/unblocking_directive.txt | \
                sed -e "s/AGENT_NAME_VAR/$AGENT_NAME/g" \
                    -e "s/TOTAL_TASKS_VAR/$TOTAL_TASKS/g" \
                    -e "s|BLOCKED_TASK_DETAILS_VAR|$BLOCKED_TASK_DETAILS|g" \
                > "$DIRECTIVE_FILE"
            
            # Launch Gemini in unblocking mode
            log "🚀 Launching Gemini in UNBLOCKING mode..."
            if [ -n "$GEMINI_PANE" ]; then
                send_to_gemini "clear"
                sleep 1
                GEMINI_CMD="cat $DIRECTIVE_FILE | $GEMINI_CMD_BASE $GEMINI_ARGS --model gemini-3-pro-preview"
                send_to_gemini "$GEMINI_CMD"
                
                log "⏳ Monitoring unblocking agent..."
                sleep 5
                
                # Monitor for completion
                while pgrep -f "gemini --yolo" >/dev/null 2>&1; do
                    sleep 10
                done
                
                log "✅ Unblocking agent completed"
            fi
            
            rm -f "$DIRECTIVE_FILE"
            
            # Short cooldown before checking for newly unblocked tasks
            sleep 5
            continue
        else
            log "💤 No tasks available. Sleeping ${SLEEP_DURATION}s..."
            sleep "$SLEEP_DURATION"
            continue
        fi
    fi

    # Get task details
    TASK_INFO=$(bd show "$TASK_ID" --json 2>/dev/null)
    TASK_TITLE=$(echo "$TASK_INFO" | jq -r '.[0].title // "Unknown"')
    TASK_STATUS=$(echo "$TASK_INFO" | jq -r '.[0].status // "unknown"')

    
    log "⚡ TASK FOUND: [$TASK_ID] $TASK_TITLE"
    log "📋 Status: $TASK_STATUS"

    # Determine Model based on Challenge Tag
    # Already handled in selection phase: TASK_ID and MODEL_ARGS are set properly.
    
    # 3. Update task to in_progress and assign to this agent
    if [ "$TASK_STATUS" != "in_progress" ]; then
        log "📌 Locking task to $AGENT_NAME..."
        bd update "$TASK_ID" --status=in_progress --assignee="$AGENT_NAME" 2>/dev/null
    fi

    # 4. Construct the system directive
    log "📝 Building system directive..."
    # Use deterministic path in project structure to avoid sandbox issues
    DIRECTIVE_FILE=".gemini/tmp/directive_${TASK_ID}.txt"
    
    # Get context - escape special characters for sed
    TASK_CONTEXT=$(echo "$TASK_INFO" | jq -r '.[0].description // "No description"' | tr '\n' ' ' | sed 's/[&/\|]/\\&/g' | sed 's/"/\\"/g')
    PROJECT_CONTEXT="See AGENTS.md for full project context."

    # Generate directive using envsubst-style replacement to avoid sed issues
    cp bootstrap/directive_template.txt "$DIRECTIVE_FILE"
    sed -i '' -e "s/AGENT_NAME_VAR/$AGENT_NAME/g" "$DIRECTIVE_FILE"
    sed -i '' -e "s/TASK_ID_VAR/$TASK_ID/g" "$DIRECTIVE_FILE"
    # Use perl for complex substitutions to handle special chars better
    perl -i -pe "s|TASK_CONTEXT_VAR|$TASK_CONTEXT|g" "$DIRECTIVE_FILE" 2>/dev/null || \
        sed -i '' -e "s|TASK_CONTEXT_VAR|Task context unavailable - see bd show $TASK_ID|g" "$DIRECTIVE_FILE"
    perl -i -pe "s|PROJECT_CONTEXT_VAR|$PROJECT_CONTEXT|g" "$DIRECTIVE_FILE" 2>/dev/null || \
        sed -i '' -e "s|PROJECT_CONTEXT_VAR|$PROJECT_CONTEXT|g" "$DIRECTIVE_FILE"
    
    if [ -n "$GEMINI_PANE" ]; then
        # Clear the Gemini pane
        send_to_gemini "clear"
        sleep 1
        
        # Send the Gemini command with directive via stdin pipe
        GEMINI_CMD="cat $DIRECTIVE_FILE | $GEMINI_CMD_BASE $GEMINI_ARGS $MODEL_ARGS"
        log "🚀 Launching Gemini with args: $MODEL_ARGS"
        send_to_gemini "$GEMINI_CMD"
        
        log "✅ Gemini command sent to pane $GEMINI_PANE"
        log "⏳ Monitoring Gemini process..."
        
        # Wait a moment for Gemini to start
        sleep 5
        
        # Monitor for Gemini process completion
        while true; do
            # Check if gemini is still running
            GEMINI_RUNNING=$(pgrep -f "gemini --yolo" 2>/dev/null)
            
            if [ -z "$GEMINI_RUNNING" ]; then
                log "✅ Gemini process completed"
                break
            fi
            
            # Check every 10 seconds
            sleep 10
        done
    else
        log "❌ ERROR: GEMINI_PANE not set. Cannot launch Gemini."
        log "💡 Run this script via tmux_agent_setup.sh"
        exit 1
    fi
    
    # Cleanup
    rm -f "$DIRECTIVE_FILE"
    
    # 6. Post-execution check
    TASK_STATUS_AFTER=$(bd show "$TASK_ID" --json 2>/dev/null | jq -r '.[0].status // "unknown"')
    log "📋 Task $TASK_ID status after execution: $TASK_STATUS_AFTER"
    
    # Short cooldown
    log "⏸️  Cooldown period (5s)..."
    sleep 5
    ((count++))
done

log "🛑 Max cycles ($MAX_CYCLES) reached. Orchestrator shutting down."
