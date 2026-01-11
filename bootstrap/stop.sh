#!/bin/bash
# Stop the autonomous agent system

SESSION_NAME="bd-agents"

echo "🛑 Stopping autonomous agent system..."

# Kill the tmux session
if tmux has-session -t "$SESSION_NAME" 2>/dev/null; then
    tmux kill-session -t "$SESSION_NAME"
    echo "✅ Killed tmux session '$SESSION_NAME'"
else
    echo "ℹ️  Session '$SESSION_NAME' is not running"
fi

# Kill any stray orchestrator processes
ORCHESTRATOR_PIDS=$(pgrep -f "orchestrator_with_quota.sh" 2>/dev/null)
if [ -n "$ORCHESTRATOR_PIDS" ]; then
    echo "🔪 Killing stray orchestrator processes: $ORCHESTRATOR_PIDS"
    kill -9 $ORCHESTRATOR_PIDS 2>/dev/null
fi

# Kill any stray gemini processes
GEMINI_PIDS=$(pgrep -f "gemini --yolo" 2>/dev/null)
if [ -n "$GEMINI_PIDS" ]; then
    echo "🔪 Killing stray gemini processes: $GEMINI_PIDS"
    kill -9 $GEMINI_PIDS 2>/dev/null
fi

# Kill watchdog
WATCHDOG_PIDS=$(pgrep -f "gemini_watchdog.sh" 2>/dev/null)
if [ -n "$WATCHDOG_PIDS" ]; then
    echo "🔪 Killing watchdog processes: $WATCHDOG_PIDS"
    kill -9 $WATCHDOG_PIDS 2>/dev/null
fi

echo "✅ All autonomous agent processes stopped"
