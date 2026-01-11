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
GEMINI_ARGS="--yolo"

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

# Function to check quota status using check_quota.sh
check_quota() {
    ./bootstrap/check_quota.sh >/dev/null 2>&1
    return $?
}

# Function to get quota reset time
get_reset_time() {
    node /Users/bryantinsley/Code/gemini-cli/packages/cli/dist/index.js --dump-quota 2>/dev/null | \
        jq -r '.buckets[] | select(.modelId == "gemini-2.5-pro") | .resetTime'
}

# Function to calculate seconds until reset
seconds_until_reset() {
    local reset_time=$1
    local reset_epoch=$(date -j -f "%Y-%m-%dT%H:%M:%SZ" "$reset_time" +%s 2>/dev/null)
    local now_epoch=$(date +%s)
    echo $((reset_epoch - now_epoch))
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
    
    # Check quota before proceeding
    if ! check_quota; then
        RESET_TIME=$(get_reset_time)
        
        log "⏸️  QUOTA EXHAUSTED - Entering sleep mode"
        log "⏰ Quota resets at: $RESET_TIME"
        log "💤 Checking every 5 minutes for early refresh..."
        
        # Check quota every 5 minutes until it's available
        # Don't log each check - could get surprise early refresh
        while ! check_quota; do
            sleep 300
        done
        
        log "✅ Quota refreshed! Resuming operations..."
    fi
    
    # 1. Check for tasks assigned to this agent (in_progress status)
    log "🔍 Checking for in-progress tasks..."
    CURRENT_TASK=$(bd list --status=in_progress --json 2>/dev/null | jq -r --arg agent "$AGENT_NAME" '.[] | select(.assignee == $agent) | .id' | head -1)
    
    if [ -n "$CURRENT_TASK" ]; then
        log "🔄 Found in-progress task: $CURRENT_TASK"
        TASK_ID="$CURRENT_TASK"
    else
        # 2. If no active task, fetch the next READY task
        log "🔍 Checking for ready tasks..."
        TASK_ID=$(bd ready --json 2>/dev/null | jq -r '.[].id' | head -1)
    fi

    # If still empty, check if we should run unblocking agent
    if [ -z "$TASK_ID" ]; then
        # Check if there are ANY tasks at all
        TOTAL_TASKS=$(bd list --json 2>/dev/null | jq '. | length' 2>/dev/null || echo "0")
        
        if [ "$TOTAL_TASKS" -gt 0 ]; then
            log "🔍 No ready tasks, but $TOTAL_TASKS tasks exist - entering unblocking mode"
            
            # Get blocked tasks details
            BLOCKED_TASK_DETAILS=$(bd blocked 2>/dev/null || echo "Unable to fetch blocked tasks")
            
            # Build unblocking directive
            DIRECTIVE_FILE=$(mktemp)
            cat bootstrap/unblocking_directive.txt | \
                sed -e "s/\$AGENT_NAME/$AGENT_NAME/g" \
                    -e "s/\$TOTAL_TASKS/$TOTAL_TASKS/g" \
                    -e "s|\$BLOCKED_TASK_DETAILS|$BLOCKED_TASK_DETAILS|g" \
                > "$DIRECTIVE_FILE"
            
            # Launch Gemini in unblocking mode
            log "🚀 Launching Gemini in UNBLOCKING mode..."
            if [ -n "$GEMINI_PANE" ]; then
                send_to_gemini "clear"
                sleep 1
                GEMINI_CMD="cat $DIRECTIVE_FILE | $GEMINI_CMD_BASE $GEMINI_ARGS"
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

    # 3. Update task to in_progress and assign to this agent
    if [ "$TASK_STATUS" != "in_progress" ]; then
        log "📌 Locking task to $AGENT_NAME..."
        bd update "$TASK_ID" --status=in_progress --assignee="$AGENT_NAME" 2>/dev/null
    fi

    # 4. Construct the system directive
    log "📝 Building system directive..."
    DIRECTIVE_FILE=$(mktemp)
    
    # Get context
    TASK_CONTEXT=$(echo "$TASK_INFO" | jq -r '.[0].description // "No description"' | tr '\n' ' ' | sed 's/"/\\"/g')
    PROJECT_CONTEXT="See AGENTS.md for full project context."

    cat bootstrap/directive_template.txt | \
        sed -e "s/\$AGENT_NAME/$AGENT_NAME/g" \
            -e "s/\$TASK_ID/$TASK_ID/g" \
            -e "s|\$TASK_CONTEXT|$TASK_CONTEXT|g" \
            -e "s|\$PROJECT_CONTEXT|$PROJECT_CONTEXT|g" \
        > "$DIRECTIVE_FILE"
    
    if [ -n "$GEMINI_PANE" ]; then
        # Clear the Gemini pane
        send_to_gemini "clear"
        sleep 1
        
        # Send the Gemini command with directive via stdin
        # Pipe the directive file to gemini with --yolo for auto-approval
        GEMINI_CMD="cat $DIRECTIVE_FILE | $GEMINI_CMD_BASE $GEMINI_ARGS"
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
