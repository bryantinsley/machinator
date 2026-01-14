# Lessons Learned: Machinator Orchestrator v1

This document captures hard-won operational knowledge from building the first version of Machinator. These lessons should inform the architecture of v2.

## Gemini Invocation

### Environment Setup

Gemini CLI uses several environment variables to find its configuration:

```bash
# BOTH must be set to the account directory for proper isolation
HOME=/path/to/account/dir
GEMINI_CLI_HOME=/path/to/account/dir

# Bypass macOS Keychain (which is shared across all processes)
GEMINI_FORCE_FILE_STORAGE=true
```

**Why both HOME and GEMINI_CLI_HOME?**

- `HOME` is where Gemini looks for `.gemini/` by default
- `GEMINI_CLI_HOME` is an explicit override
- Some code paths check one, some check the other
- Setting both ensures consistent behavior

### Working Directory

```bash
# Run from the agent's worktree, NOT the main repo
geminiCmd.Dir = "/path/to/agents/1/"
```

The `--sandbox` flag restricts file access to the current directory tree. If you run from the wrong directory, gemini can't access the files it needs.

### Command Flags

```bash
gemini \
  --yolo \                    # Skip confirmation prompts
  --sandbox \                 # Restrict file access to cwd
  --model gemini-3-flash \    # Or gemini-3-pro for complex tasks
  --output-format stream-json \ # Get parseable ACP events
  "Your directive here"
```

### Directive Delivery (Future Improvement)

Instead of passing a massive inline directive, write to a file:

```bash
# Write directive to agent's worktree
cat > agents/1/.machinator/directive.md << 'EOF'
# Task Assignment
Agent: Agent-1
Task: TASK-123
...
EOF

# Simple prompt that points to the file
gemini --yolo --sandbox "Read .machinator/directive.md for your instructions."
```

Benefits:

- No escaping nightmares with quotes/newlines
- Debuggable (cat the file to see what agent was told)
- Agent can reread mid-task if it loses context
- Avoids OS argument length limits

---

## Beads Database Management

### The Daemon Problem

The `bd` command has a background daemon that:

1. Watches the SQLite database for changes
2. Auto-exports to JSONL files
3. Auto-syncs with git

This causes problems when multiple processes interact with beads:

- Daemon modifies `.beads/issues.jsonl`
- This dirties the git working tree
- `git pull --ff-only` fails because of local changes

### The Solution: --sandbox Flag

```bash
# ALWAYS use --sandbox for bd commands from orchestrator
bd --sandbox list --json
bd --sandbox update TASK-123 --status=in_progress
bd --sandbox ready --json
```

The `--sandbox` flag:

- Disables the daemon
- Disables auto-sync
- Does not traverse parent directories to find .beads
- Safe for automated/orchestrated environments

### Read/Write Separation

The orchestrator should follow this pattern:

| Location                   | Access         | Method                            |
| -------------------------- | -------------- | --------------------------------- |
| `repo/.beads/issues.jsonl` | **READ ONLY**  | `git pull` + parse JSONL directly |
| `agents/N/.beads/`         | **READ/WRITE** | `bd --sandbox` commands           |

The orchestrator NEVER writes to the main repo's beads. All writes go to agent worktrees.

---

## Git Worktree Management

### Why Worktrees?

Each agent needs its own working directory to:

- Make changes without conflicting with other agents
- Have its own branch for its task
- Keep a clean separation of concerns

### Setup Pattern

```bash
# Create worktree for agent 1
git -C /path/to/repo worktree add --detach /path/to/agents/1 main

# Each agent gets a unique branch based on task ID
git -C /path/to/agents/1 checkout -B machinator/TASK-123 origin/main
```

### Branch Naming

- Pattern: `machinator/<task-id>`
- Each task gets its own branch
- Branches are force-created (`-B`) to handle retries
- Agent pushes to main when done (or creates PR if branch protection enabled)

### Cleanup

```bash
# Reset worktree for next task
git -C /path/to/agents/1 reset --hard
git -C /path/to/agents/1 fetch origin main
git -C /path/to/agents/1 checkout -B machinator/TASK-456 origin/main
```

---

## Account Pooling

### Directory Structure

```
~/.machinator/accounts/
├── primary/
│   ├── account.json      # {"name": "primary", "auth_type": "google"}
│   └── .gemini/          # Gemini credentials
├── secondary/
│   └── .gemini/
└── work/
    └── .gemini/
```

### Rotation Logic

```go
// Simple round-robin through accounts
accounts := pool.GetAccounts()
if len(accounts) > 1 {
    acc, _ := pool.NextAvailable()
    // Set HOME to account dir before running gemini
}
```

### Quota Checking

```bash
# Each account has its own quota
gemini --dump-quota  # Returns JSON with per-model quotas
```

The orchestrator checks quotas periodically and can route complex tasks to accounts with more Pro quota.

---

## Error Handling Patterns

### Timeout Handling

Two types of timeouts:

1. **Idle timeout** - No ACP events received for N minutes
2. **Max runtime timeout** - Task running for too long total

```go
// Kill the process, don't just abandon it
if agent.Cmd != nil && agent.Cmd.Process != nil {
    agent.Cmd.Process.Kill()
}
```

### Uncommitted Changes Recovery

When a task completes but leaves uncommitted changes:

1. Check if changes are "minor" (<20 lines, 1 file)
2. If minor: discard and continue
3. If significant: retry the task (up to N times)
4. If retries exhausted: mark as failed, move on

### Git Conflict Recovery

```bash
# If git pull fails due to conflicts
git reset --hard origin/main
# Retry the task from clean state
```

---

## Architecture Smells (What Not to Do)

### God Object Model

The current `model` struct has 50+ fields mixing:

- UI state (scroll positions, focus)
- Business logic (tasks, agents)
- Infrastructure (commands, channels)

**Lesson:** Separate UI state from business logic from infrastructure.

### Giant Switch Statement

The `Update()` function handles 30+ message types in one switch.

**Lesson:** Use a command pattern or separate handlers per message type.

### Inline Command Execution

Commands are scattered throughout the codebase with repeated patterns.

**Lesson:** Create service wrappers (`GitService`, `BeadsService`, `GeminiService`).

### File Size

`tui.go` is 2900+ lines.

**Lesson:** 250 lines per file max. Split by responsibility.

---

## Recommended v2 Architecture

```
machinator/
├── backend/
│   ├── api/              # HTTP/WebSocket handlers
│   ├── services/
│   │   ├── scheduler.go  # Task selection, claiming
│   │   ├── executor.go   # Gemini execution
│   │   ├── worktree.go   # Git worktree management
│   │   ├── quota.go      # Account quota management
│   │   └── beads.go      # Beads client (read-only for orchestrator)
│   └── models/           # Domain objects
│
├── frontend/             # React + Vite
│   ├── components/
│   └── hooks/
│
└── docs/
    └── lessons-learned.md  # This file
```

The backend runs as a service, exposes a REST/WebSocket API, and the frontend is a beautiful React dashboard that LLMs can actually build well.
