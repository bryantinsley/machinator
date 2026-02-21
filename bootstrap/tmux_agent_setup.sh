#!/bin/bash

# Tmux-based layout for autonomous agent development
# Layout:
#   ┌────────────────────┬────────────────────┐
#   │  Gemini Agent      │  Orchestrator      │  75%
#   │  (top-left)        │  (top-right)       │
#   ├────────────────────┼────────────────────┤
#   │  Git Log           │  Task Monitor      │  25%
#   │  (bottom-left)     │  (bottom-right)    │
#   └────────────────────┴────────────────────┘

SESSION_NAME="${BD_SESSION_NAME:-bd-agents}"
AGENT_NAME="${BD_AGENT_NAME:-Gemini-01}"
PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

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
        echo "Kill it with: tmux kill-session -t $SESSION_NAME"
        exit 1
    fi
fi

echo "🚀 Creating tmux session: $SESSION_NAME"
echo "📂 Project dir: $PROJECT_DIR"

# ─── Create session with initial pane ─────────────────────────────────────────
# This creates window 0, pane 0 (will be: Gemini / top-left)
tmux new-session -d -s "$SESSION_NAME" -n "agent" -c "$PROJECT_DIR"

# Enable mouse
tmux set-option -t "$SESSION_NAME" mouse on

# ─── Step 1: Split into LEFT and RIGHT columns (50/50) ───────────────────────
# After: pane 0 = left, pane 1 = right
tmux split-window -h -t "$SESSION_NAME:0.0" -c "$PROJECT_DIR"

# ─── Step 2: Split RIGHT column into TOP-RIGHT and BOTTOM-RIGHT ──────────────
# Split pane 1 (right) vertically. -p 25 = bottom gets 25%.
# After: pane 0 = left, pane 1 = top-right, pane 2 = bottom-right
tmux split-window -v -t "$SESSION_NAME:0.1" -p 25 -c "$PROJECT_DIR"

# ─── Step 3: Split LEFT column into TOP-LEFT and BOTTOM-LEFT ─────────────────
# Split pane 0 (left) vertically. -p 25 = bottom gets 25%.
# After: pane 0 = top-left, pane 1 = bottom-left, pane 2 = top-right, pane 3 = bottom-right
# NOTE: When we split pane 0, the NEW pane gets inserted as pane 1, pushing others up.
tmux split-window -v -t "$SESSION_NAME:0.0" -p 25 -c "$PROJECT_DIR"

# At this point, pane layout is:
#   Pane 0: top-left     (Gemini)
#   Pane 1: bottom-left  (Git log)
#   Pane 2: top-right    (Orchestrator)
#   Pane 3: bottom-right (Task monitor)

# ─── Set up Pane 0: Gemini Agent (top-left) ──────────────────────────────────
tmux send-keys -t "$SESSION_NAME:0.0" "clear" C-m
tmux send-keys -t "$SESSION_NAME:0.0" "echo '🤖 Gemini Agent: $AGENT_NAME'" C-m
tmux send-keys -t "$SESSION_NAME:0.0" "echo 'Waiting for orchestrator to send tasks...'" C-m

# ─── Set up Pane 2: Orchestrator (top-right) ─────────────────────────────────
tmux send-keys -t "$SESSION_NAME:0.2" "clear" C-m
tmux send-keys -t "$SESSION_NAME:0.2" "echo '🔄 Orchestrator starting in 5 seconds...'" C-m
tmux send-keys -t "$SESSION_NAME:0.2" "sleep 5" C-m
tmux send-keys -t "$SESSION_NAME:0.2" "export BD_GEMINI_PANE='$SESSION_NAME:0.0'" C-m
tmux send-keys -t "$SESSION_NAME:0.2" "export BD_AGENT_NAME='$AGENT_NAME'" C-m
tmux send-keys -t "$SESSION_NAME:0.2" "./bootstrap/orchestrator_with_quota.sh" C-m

# ─── Set up Pane 1: Git Log (bottom-left) ────────────────────────────────────
tmux send-keys -t "$SESSION_NAME:0.1" "clear" C-m
tmux send-keys -t "$SESSION_NAME:0.1" "watch -n 30 'echo \"═══ RECENT COMMITS ═══\" && git log --oneline --decorate -10 && echo && echo \"═══ BRANCHES ═══\" && git branch -v'" C-m

# ─── Set up Pane 3: Task Monitor (bottom-right) ──────────────────────────────
tmux send-keys -t "$SESSION_NAME:0.3" "clear" C-m
tmux send-keys -t "$SESSION_NAME:0.3" "watch -n 30 'echo \"═══ READY TASKS ═══\" && bd ready 2>/dev/null | head -10 && echo && echo \"═══ IN PROGRESS ═══\" && bd list --status=in_progress 2>/dev/null | head -10'" C-m

# ─── Create Window 1: Manual control ─────────────────────────────────────────
tmux new-window -t "$SESSION_NAME:1" -n "manual" -c "$PROJECT_DIR"
tmux send-keys -t "$SESSION_NAME:1" "clear" C-m
tmux send-keys -t "$SESSION_NAME:1" "echo '🛠️  Manual Control Panel'" C-m
tmux send-keys -t "$SESSION_NAME:1" "echo ''" C-m
tmux send-keys -t "$SESSION_NAME:1" "echo 'Quick commands:'" C-m
tmux send-keys -t "$SESSION_NAME:1" "echo '  bd ready              # Show available tasks'" C-m
tmux send-keys -t "$SESSION_NAME:1" "echo '  bd list               # List all tasks'" C-m
tmux send-keys -t "$SESSION_NAME:1" "echo '  bd show <id>          # View task details'" C-m
tmux send-keys -t "$SESSION_NAME:1" "echo '  bd stats              # Stats'" C-m
tmux send-keys -t "$SESSION_NAME:1" "echo ''" C-m
tmux send-keys -t "$SESSION_NAME:1" "echo 'Stop everything:'" C-m
tmux send-keys -t "$SESSION_NAME:1" "echo '  ./bootstrap/stop.sh'" C-m
tmux send-keys -t "$SESSION_NAME:1" "echo ''" C-m

# ─── Focus on agent window, Gemini pane ──────────────────────────────────────
tmux select-window -t "$SESSION_NAME:0"
tmux select-pane -t "$SESSION_NAME:0.0"

# ─── Print layout guide ─────────────────────────────────────────────────────
clear
echo ""
echo "  ✅ Session '$SESSION_NAME' created!"
echo ""
echo "  ┌────────────────────┬────────────────────┐"
echo "  │  🤖 Gemini Agent   │  🔄 Orchestrator   │"
echo "  │  Pane 0 (top-left) │  Pane 2 (top-right)│  75%"
echo "  ├────────────────────┼────────────────────┤"
echo "  │  📦 Git Log        │  📋 Task Monitor   │"
echo "  │  Pane 1 (bot-left) │  Pane 3 (bot-right)│  25%"
echo "  └────────────────────┴────────────────────┘"
echo ""
echo "  Window 1: 🛠️  Manual control panel"
echo ""
echo "  📌 Tmux cheat sheet:"
echo "     Ctrl+b d       - Detach (session keeps running)"
echo "     Ctrl+b [0-1]   - Switch windows"
echo "     Ctrl+b arrow   - Navigate panes"
echo "     Ctrl+b z       - Zoom/unzoom pane"
echo "     Mouse enabled  - Click to select, scroll to navigate"
echo ""
echo "  Reattach later: tmux attach -t $SESSION_NAME"
echo ""
sleep 2

tmux attach-session -t "$SESSION_NAME"
