#!/bin/bash

# Tmux-based 2x2 grid setup for autonomous agent development
# Layout: 2x2 grid with 75% top, 25% bottom

SESSION_NAME="${BD_SESSION_NAME:-bd-agents}"
AGENT_NAME="${BD_AGENT_NAME:-Gemini-01}"

# Check if tmux is installed
if ! command -v tmux &> /dev/null; then
    echo "❌ tmux is not installed. Install with: brew install tmux"
    exit 1
fi

# Check if session already exists
if tmux has-session -t "$SESSION_NAME" 2>/dev/null; then
    echo "⚠️  Session '$SESSION_NAME' already exists."
    read -p "Attach to existing session? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        tmux attach-session -t "$SESSION_NAME"
        exit 0
    else
        echo "Exiting. Use 'tmux kill-session -t $SESSION_NAME' to remove it."
        exit 1
    fi
fi

echo "🚀 Creating tmux session: $SESSION_NAME with 2x2 grid layout"

# Create new session with first window
tmux new-session -d -s "$SESSION_NAME" -n "agent-$AGENT_NAME"

# Enable mouse support for easier navigation
tmux set-option -t "$SESSION_NAME" mouse on

# Create the 2x2 grid layout
# Strategy: Split horizontally first (left/right), then split each side vertically (top/bottom)

# Pane 0: Top-left (Gemini)
tmux send-keys -t "$SESSION_NAME:0.0" "cd $(pwd)" C-m
tmux send-keys -t "$SESSION_NAME:0.0" "clear" C-m
tmux send-keys -t "$SESSION_NAME:0.0" "echo '🤖 Gemini Agent: $AGENT_NAME'" C-m
tmux send-keys -t "$SESSION_NAME:0.0" "echo 'This pane will show Gemini working on tasks.'" C-m
tmux send-keys -t "$SESSION_NAME:0.0" "echo ''" C-m

# Split horizontally to create right column
# Pane 1: Top-right (Orchestrator)
tmux split-window -h -t "$SESSION_NAME:0.0"
tmux send-keys -t "$SESSION_NAME:0.1" "cd $(pwd)" C-m
tmux send-keys -t "$SESSION_NAME:0.1" "clear" C-m
tmux send-keys -t "$SESSION_NAME:0.1" "echo '🔄 Orchestrator Event Loop'" C-m
tmux send-keys -t "$SESSION_NAME:0.1" "echo 'Starting in 5 seconds...'" C-m
tmux send-keys -t "$SESSION_NAME:0.1" "sleep 5" C-m
tmux send-keys -t "$SESSION_NAME:0.1" "export BD_GEMINI_PANE='$SESSION_NAME:0.0'" C-m
tmux send-keys -t "$SESSION_NAME:0.1" "export BD_AGENT_NAME='$AGENT_NAME'" C-m
tmux send-keys -t "$SESSION_NAME:0.1" "./bootstrap/orchestrator_with_quota.sh" C-m

# Now split the left column vertically (pane 0 -> pane 0 top, pane 2 bottom)
# Pane 2: Bottom-left (Git log)
tmux split-window -v -t "$SESSION_NAME:0.0" -p 25
tmux send-keys -t "$SESSION_NAME:0.2" "cd $(pwd)" C-m
tmux send-keys -t "$SESSION_NAME:0.2" "clear" C-m
tmux send-keys -t "$SESSION_NAME:0.2" "watch -n 30 'git log --oneline --decorate --graph -10 && echo && git branch -v'" C-m

# Split bottom-left pane horizontally to add bd list
# Pane 4: Bottom-left-right (BD list)
tmux split-window -h -t "$SESSION_NAME:0.2"
tmux send-keys -t "$SESSION_NAME:0.4" "cd $(pwd)" C-m
tmux send-keys -t "$SESSION_NAME:0.4" "clear" C-m
tmux send-keys -t "$SESSION_NAME:0.4" "watch -n 30 'echo "=== READY TASKS ===" && bd ready | head -10 && echo && echo "=== IN PROGRESS ===" && bd list --status=in_progress'" C-m

# Split the right column vertically (pane 1 -> pane 1 top, pane 3 bottom)
# Pane 3: Bottom-right (Gemini Watchdog - 20% of right column)
tmux split-window -v -t "$SESSION_NAME:0.1" -p 20
tmux send-keys -t "$SESSION_NAME:0.3" "cd $(pwd)" C-m
tmux send-keys -t "$SESSION_NAME:0.3" "clear" C-m
tmux send-keys -t "$SESSION_NAME:0.3" "echo '🐕 Gemini Watchdog'" C-m
tmux send-keys -t "$SESSION_NAME:0.3" "./bootstrap/gemini_watchdog.sh" C-m

# Create second window for manual interaction
tmux new-window -t "$SESSION_NAME:1" -n "manual"
tmux send-keys -t "$SESSION_NAME:1" "cd $(pwd)" C-m
tmux send-keys -t "$SESSION_NAME:1" "clear" C-m
tmux send-keys -t "$SESSION_NAME:1" "echo '🛠️  Manual Control Panel'" C-m
tmux send-keys -t "$SESSION_NAME:1" "echo ''" C-m
tmux send-keys -t "$SESSION_NAME:1" "echo 'Quick commands:'" C-m
tmux send-keys -t "$SESSION_NAME:1" "echo '  bd ready              # Show available tasks'" C-m
tmux send-keys -t "$SESSION_NAME:1" "echo '  bd list --status=...  # List tasks by status'" C-m
tmux send-keys -t "$SESSION_NAME:1" "echo '  bd show <id>          # View task details'" C-m
tmux send-keys -t "$SESSION_NAME:1" "echo '  bd stats              # Project statistics'" C-m
tmux send-keys -t "$SESSION_NAME:1" "echo '  touch /tmp/bd_task_<id>_complete  # Signal task completion'" C-m
tmux send-keys -t "$SESSION_NAME:1" "echo ''" C-m
tmux send-keys -t "$SESSION_NAME:1" "bd stats" C-m

# Select the Gemini pane (top-left)
tmux select-window -t "$SESSION_NAME:0"
tmux select-pane -t "$SESSION_NAME:0.0"

# Attach to the session
clear
echo "✅ Session created with layout:\n"
echo ""
echo "   ┌─────────────────────┬─────────────────────┐"
echo "   │  🤖 Gemini          │  🔄 Orchestrator    │"
echo "   │  (Interactive)      │  (Logs - 80%)       │  75%"
echo "   │                     ├─────────────────────┤"
echo "   │                     │  🐕 Watchdog (20%)  │"
echo "   ├──────────┬──────────┴─────────────────────┤"
echo "   │ 📦 Git   │ 📋 Tasks │                     │  25%"
echo "   └──────────┴──────────┴─────────────────────┘"
echo "       25%        25%            50%"
echo ""
echo "   Window 1: �️  Manual control panel"
echo ""
echo "📌 Tmux cheat sheet:"
echo "   Mouse enabled     - Click to select panes, scroll to navigate history"
echo "   Ctrl+b d         - Detach from session"
echo "   Ctrl+b [0-1]     - Switch between windows"
echo "   Ctrl+b arrow     - Navigate between panes"
echo "   Ctrl+b z         - Zoom/unzoom current pane"
echo "   Ctrl+b x         - Kill current pane"
echo ""
echo "🎯 How it works:"
echo "   1. Orchestrator (top right) finds tasks and sends commands to Gemini"
echo "   2. Gemini (top left) executes tasks - you see everything"
echo "   3. Git log (bottom left) shows commit history"
echo "   4. Gemini watchdog (kills stuck processes) (bottom right) shows available and in-progress tasks"
echo "   5. When Gemini finishes a task: touch /tmp/bd_task_<id>_complete"
echo ""
echo "To reattach later: tmux attach-session -t $SESSION_NAME"
echo ""
sleep 2

tmux attach-session -t "$SESSION_NAME"
