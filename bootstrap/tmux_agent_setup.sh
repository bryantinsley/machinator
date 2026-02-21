#!/bin/bash

# Tmux-based layout for autonomous agent development
# Layout:
#   ┌────────────────────┬────────────────────┐
#   │  Gemini Agent      │  Orchestrator      │  75%
#   │  (top-left)        │  (top-right)       │
#   ├──────────┬─────────┼────────────────────┤
#   │  Git Log │  Tasks  │  Watchdog          │  25%
#   └──────────┴─────────┴────────────────────┘

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
# This creates window 0, pane 0. Will be: Gemini (top-left)
tmux new-session -d -s "$SESSION_NAME" -n "agent" -c "$PROJECT_DIR"

# Enable mouse
tmux set-option -t "$SESSION_NAME" mouse on

# ─── Build layout step by step ───────────────────────────────────────────────
# We split in a specific order to get predictable pane indices.
#
# Step 1: Split pane 0 horizontally → left=0, right=1
tmux split-window -h -t "$SESSION_NAME:0.0" -c "$PROJECT_DIR"

# Step 2: Split pane 1 (right) vertically, bottom 25% → top-right=1, bottom-right=2
tmux split-window -v -t "$SESSION_NAME:0.1" -p 25 -c "$PROJECT_DIR"

# Step 3: Split pane 0 (left) vertically, bottom 25% → top-left=0, bottom-left=1
# NOTE: new pane inserts at index 1, pushing top-right to 2, bottom-right to 3
tmux split-window -v -t "$SESSION_NAME:0.0" -p 25 -c "$PROJECT_DIR"

# Step 4: Split pane 1 (bottom-left) horizontally for tasks pane
# → bottom-left-left=1, bottom-left-right=2, top-right=3, bottom-right=4
tmux split-window -h -t "$SESSION_NAME:0.1" -p 50 -c "$PROJECT_DIR"

# Final pane map:
#   Pane 0: top-left      → Gemini Agent
#   Pane 1: bottom-left   → Git Log
#   Pane 2: bottom-center → Task Monitor
#   Pane 3: top-right     → Orchestrator
#   Pane 4: bottom-right  → Watchdog

# ─── Pane 0: Gemini Agent (top-left) ────────────────────────────────────────
tmux send-keys -t "$SESSION_NAME:0.0" "clear" C-m
tmux send-keys -t "$SESSION_NAME:0.0" "echo '🤖 Gemini Agent: $AGENT_NAME'" C-m
tmux send-keys -t "$SESSION_NAME:0.0" "echo 'Waiting for orchestrator to send tasks...'" C-m

# ─── Pane 3: Orchestrator (top-right) ───────────────────────────────────────
tmux send-keys -t "$SESSION_NAME:0.3" "clear" C-m
tmux send-keys -t "$SESSION_NAME:0.3" "echo '🔄 Orchestrator starting in 5 seconds...'" C-m
tmux send-keys -t "$SESSION_NAME:0.3" "sleep 5" C-m
tmux send-keys -t "$SESSION_NAME:0.3" "export BD_GEMINI_PANE='$SESSION_NAME:0.0'" C-m
tmux send-keys -t "$SESSION_NAME:0.3" "export BD_AGENT_NAME='$AGENT_NAME'" C-m
tmux send-keys -t "$SESSION_NAME:0.3" "./bootstrap/orchestrator_with_quota.sh" C-m

# ─── Pane 1: Git Log (bottom-left) ──────────────────────────────────────────
tmux send-keys -t "$SESSION_NAME:0.1" "clear" C-m
tmux send-keys -t "$SESSION_NAME:0.1" "watch -n 30 'echo \"═══ RECENT COMMITS ═══\" && git log --oneline --decorate -8'" C-m

# ─── Pane 2: Task + Quota Monitor (bottom-center) ───────────────────────────
tmux send-keys -t "$SESSION_NAME:0.2" "clear" C-m
tmux send-keys -t "$SESSION_NAME:0.2" "watch -n 30 'echo \"═══ QUOTA ═══\" && ./bootstrap/check_quota.sh 2>/dev/null && echo && echo \"═══ READY ═══\" && bd ready 2>/dev/null | head -6 && echo && echo \"═══ IN PROGRESS ═══\" && bd list --status=in_progress 2>/dev/null | head -5'" C-m

# ─── Pane 4: Watchdog (bottom-right) ────────────────────────────────────────
tmux send-keys -t "$SESSION_NAME:0.4" "clear" C-m
tmux send-keys -t "$SESSION_NAME:0.4" "export BD_WATCHDOG_TIMEOUT=300" C-m
tmux send-keys -t "$SESSION_NAME:0.4" "./bootstrap/gemini_watchdog.sh" C-m

# ─── Create Window 1: Manual control ─────────────────────────────────────────
tmux new-window -t "$SESSION_NAME:1" -n "manual" -c "$PROJECT_DIR"
tmux send-keys -t "$SESSION_NAME:1" "clear" C-m
tmux send-keys -t "$SESSION_NAME:1" "echo '🛠️  Manual Control Panel'" C-m
tmux send-keys -t "$SESSION_NAME:1" "echo ''" C-m
tmux send-keys -t "$SESSION_NAME:1" "echo 'Quick commands:'" C-m
tmux send-keys -t "$SESSION_NAME:1" "echo '  bd ready              # Show available tasks'" C-m
tmux send-keys -t "$SESSION_NAME:1" "echo '  bd list               # List all tasks'" C-m
tmux send-keys -t "$SESSION_NAME:1" "echo '  bd show <id>          # View task details'" C-m
tmux send-keys -t "$SESSION_NAME:1" "echo ''" C-m
tmux send-keys -t "$SESSION_NAME:1" "echo 'Stop everything:'" C-m
tmux send-keys -t "$SESSION_NAME:1" "echo '  ./bootstrap/stop.sh'" C-m

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
echo "  │  Pane 0            │  Pane 3            │  75%"
echo "  ├──────────┬─────────┼────────────────────┤"
echo "  │ 📦 Git   │ 📋 Tasks│  🐕 Watchdog       │"
echo "  │ Pane 1   │ Pane 2  │  Pane 4            │  25%"
echo "  └──────────┴─────────┴────────────────────┘"
echo ""
echo "  Window 1: 🛠️  Manual control panel"
echo ""
echo "  📌 Tmux: Ctrl+b d = detach | Ctrl+b [0-1] = switch windows"
echo "     Mouse enabled — click panes, scroll history"
echo ""
echo "  Reattach: tmux attach -t $SESSION_NAME"
echo ""
sleep 2

tmux attach-session -t "$SESSION_NAME"
