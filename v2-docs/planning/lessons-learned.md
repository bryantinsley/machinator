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

## Post-Shutdown Worktree Triage

When an agent shuts down (success, failure, timeout, or kill), the orchestrator needs to decide what to do with the worktree. This is a heuristic decision based on worktree state:

### Decision Matrix

| Worktree State   | Uncommitted Changes               | Action                        |
| ---------------- | --------------------------------- | ----------------------------- |
| Clean            | None                              | Reset and reuse for next task |
| Minor changes    | <20 lines, 1 file                 | Discard changes, reset        |
| Significant work | Many files, substantial diff      | Evaluate for salvage          |
| Partial commit   | Some work committed, more pending | May need manual review        |

### Salvage Evaluation

When significant uncommitted work exists:

1. Check if tests pass with the changes
2. Check if the changes address the task (parse commit messages, file paths)
3. If salvageable: commit to a `salvage/<task-id>-<timestamp>` branch
4. If not: discard and retry task with fresh context

### Implementation Pattern (v2)

```go
type WorktreeState struct {
    Clean           bool
    UncommittedFiles int
    UncommittedLines int
    CommittedSince  int  // Commits since branch creation
}

func (e *Executor) TriageWorktree(state WorktreeState) Action {
    if state.Clean {
        return ActionReset
    }
    if state.UncommittedFiles <= 1 && state.UncommittedLines < 20 {
        return ActionDiscard
    }
    if state.CommittedSince > 0 {
        return ActionReviewPartial
    }
    return ActionSalvage
}
```

---

## Agent Execution Tracing

Every agent execution should be traceable. Use a structured identifier:

### Execution ID Format

```
${BEAD_ID}-${YYYYMMDD}-${HHMMSS}
```

Example: `machinator-8rn-20260113-150423`

### Where to Use It

**Commit Messages:**

```bash
git commit -m "[machinator-8rn-20260113-150423] Fix quota display alignment"
```

**Log Directory Structure:**

```
~/.machinator/logs/
├── executions/
│   ├── machinator-8rn-20260113-150423/
│   │   ├── orchestrator.log    # Orchestrator's view of this execution
│   │   ├── gemini.log          # Raw gemini output
│   │   ├── events.jsonl        # Parsed ACP events
│   │   └── directive.md        # What the agent was told
│   └── machinator-4c7-20260113-151022/
│       └── ...
└── latest -> executions/machinator-8rn-20260113-150423
```

**Branch Names:**

```bash
# Current: machinator/<task-id>
# Better: machinator/<task-id>-<execution-id>
git checkout -B machinator/machinator-8rn-20260113-150423 origin/main
```

### Benefits

1. **Traceability**: From any commit, you can find the logs
2. **Debugging**: From any log, you can find the commits
3. **Retries**: Multiple executions of the same task are distinct
4. **Audit**: Full history of what each agent did when

### Implementation

```go
func NewExecutionID(taskID string) string {
    return fmt.Sprintf("%s-%s", taskID, time.Now().Format("20060102-150405"))
}

// Use throughout the execution
exec := NewExecutionID("machinator-8rn")
logDir := filepath.Join(machinatorDir, "logs", "executions", exec)
os.MkdirAll(logDir, 0755)
```

### Git Author Identity

Set git config per-worktree so commits are automatically attributed to the agent:

```go
// In executeTask, after worktree setup
exec.Command("git", "config", "user.name",
    fmt.Sprintf("Machinator Agent: %d", agentID)).Run()
exec.Command("git", "config", "user.email",
    fmt.Sprintf("agent-%d@machinator.local", agentID)).Run()
```

This is transparent to the agent - it just runs `git commit -m "..."` and the authorship is handled automatically. The git log becomes self-documenting:

```
$ git log --oneline --format="%h %an: %s"
a1b2c3d Machinator Agent: 1: Fix quota display
d4e5f6g Machinator Agent: 2: Add branch protection
h7i8j9k Bryan Tinsley: docs: Add lessons learned
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
