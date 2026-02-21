#!/bin/bash

# Gemini watchdog - kills stuck Gemini processes
# Monitors for Gemini processes that haven't produced output in a while
# Two detection methods:
#   1. Check if the gemini process itself is still running
#   2. Track how long it's been since process started (via /proc or ps)

TIMEOUT_SECONDS="${BD_WATCHDOG_TIMEOUT:-300}"  # 5 minutes default
CHECK_INTERVAL=30

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"
}

log "🐕 Gemini watchdog starting..."
log "⏱️  Timeout: ${TIMEOUT_SECONDS}s | Check every: ${CHECK_INTERVAL}s"

while true; do
    # Find any running gemini process (match broadly)
    GEMINI_PIDS=$(pgrep -f "gemini.*--yolo" 2>/dev/null)
    
    if [ -z "$GEMINI_PIDS" ]; then
        log "💤 No gemini process running"
        sleep "$CHECK_INTERVAL"
        continue
    fi
    
    for PID in $GEMINI_PIDS; do
        # Get process elapsed time (macOS ps format: elapsed time in seconds)
        ELAPSED=$(ps -o etime= -p "$PID" 2>/dev/null | tr -d ' ')
        
        if [ -z "$ELAPSED" ]; then
            continue  # Process already exited
        fi
        
        # Parse elapsed time (format: [[DD-]HH:]MM:SS)
        SECONDS_RUNNING=0
        if echo "$ELAPSED" | grep -q '-'; then
            # Has days: DD-HH:MM:SS
            DAYS=$(echo "$ELAPSED" | cut -d'-' -f1)
            REST=$(echo "$ELAPSED" | cut -d'-' -f2)
            SECONDS_RUNNING=$((DAYS * 86400))
            ELAPSED="$REST"
        fi
        
        # Count colons to determine format
        COLONS=$(echo "$ELAPSED" | tr -cd ':' | wc -c | tr -d ' ')
        if [ "$COLONS" -eq 2 ]; then
            # HH:MM:SS
            H=$(echo "$ELAPSED" | cut -d':' -f1)
            M=$(echo "$ELAPSED" | cut -d':' -f2)
            S=$(echo "$ELAPSED" | cut -d':' -f3)
            SECONDS_RUNNING=$((SECONDS_RUNNING + H * 3600 + M * 60 + S))
        elif [ "$COLONS" -eq 1 ]; then
            # MM:SS
            M=$(echo "$ELAPSED" | cut -d':' -f1)
            S=$(echo "$ELAPSED" | cut -d':' -f2)
            SECONDS_RUNNING=$((SECONDS_RUNNING + M * 60 + S))
        fi
        
        # Get the command for logging
        CMD_SHORT=$(ps -o command= -p "$PID" 2>/dev/null | head -c 80)
        
        if [ "$SECONDS_RUNNING" -gt "$TIMEOUT_SECONDS" ]; then
            log "⚠️  KILLING STUCK GEMINI (PID: $PID, running: ${SECONDS_RUNNING}s > ${TIMEOUT_SECONDS}s timeout)"
            log "    Command: $CMD_SHORT"
            kill -9 "$PID" 2>/dev/null
            log "✅ Killed PID $PID"
        else
            REMAINING=$((TIMEOUT_SECONDS - SECONDS_RUNNING))
            log "✓ Gemini active (PID: $PID, running: ${SECONDS_RUNNING}s, kill in: ${REMAINING}s)"
        fi
    done
    
    sleep "$CHECK_INTERVAL"
done
