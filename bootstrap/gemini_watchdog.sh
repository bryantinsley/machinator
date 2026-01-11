#!/bin/bash

# Gemini watchdog - kills stuck Gemini processes
# Monitors log files for activity and kills Gemini if no progress for 10 minutes

TIMEOUT_SECONDS=120  # 2 minutes
LOG_PATTERN="/Users/bryantinsley/.gemini/tmp/**/*.json"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"
}

log "🐕 Gemini watchdog starting..."
log "📊 Monitoring: $LOG_PATTERN"
log "⏱️  Timeout: ${TIMEOUT_SECONDS} seconds"

while true; do
    # Get the most recent modification time of any log file
    LATEST_LOG=$(find /Users/bryantinsley/.gemini/tmp -name "*.json" -type f -exec stat -f "%m %N" {} \; 2>/dev/null | sort -rn | head -1)
    
    if [ -z "$LATEST_LOG" ]; then
        log "⚠️  No log files found, waiting..."
        sleep 60
        continue
    fi
    
    LATEST_TIME=$(echo "$LATEST_LOG" | awk '{print $1}')
    LATEST_FILE=$(echo "$LATEST_LOG" | cut -d' ' -f2-)
    CURRENT_TIME=$(date +%s)
    TIME_DIFF=$((CURRENT_TIME - LATEST_TIME))
    
    log "📝 Latest activity: $(date -r $LATEST_TIME '+%H:%M:%S') (${TIME_DIFF}s ago)"
    
    # Check if Gemini is running
    GEMINI_PID=$(pgrep -f "gemini --yolo --output-format=text --model gemini-3" 2>/dev/null)
    
    if [ -n "$GEMINI_PID" ]; then
        if [ $TIME_DIFF -gt $TIMEOUT_SECONDS ]; then
            log "⚠️  No activity for ${TIME_DIFF}s - KILLING STUCK GEMINI (PID: $GEMINI_PID)"
            kill -9 $GEMINI_PID
            log "✅ Killed Gemini process $GEMINI_PID"
        else
            log "✓ Gemini active (PID: $GEMINI_PID, idle: ${TIME_DIFF}s)"
        fi
    else
        log "💤 Gemini not running"
    fi
    
    # Check every minute
    sleep 30
done
