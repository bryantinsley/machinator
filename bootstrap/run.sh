#!/bin/bash
# Start or attach to the autonomous agent tmux session

SESSION_NAME="bd-agents"

# Check if session already exists
if tmux has-session -t "$SESSION_NAME" 2>/dev/null; then
    echo "✅ Session '$SESSION_NAME' is already running"
    echo "📎 Attaching to session..."
    tmux attach-session -t "$SESSION_NAME"
else
    echo "🚀 Starting new autonomous agent session..."
    cd "$(dirname "$0")/.." || exit 1
    ./bootstrap/tmux_agent_setup.sh
    echo "✅ Session started!"
    echo "📎 Attaching to session..."
    tmux attach-session -t "$SESSION_NAME"
fi
