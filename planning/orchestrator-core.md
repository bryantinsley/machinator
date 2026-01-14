# Machinator v3: Orchestrator Core Design

## Philosophy

The orchestrator is a **reliable, resilient, observable set of cooperating watchers**.

Everything else (APIs, UIs, configuration) exists to:

1. Feed work into the system
2. Observe the system's state
3. Configure the system's behavior

---

## Agent States and Ownership

Each agent state is owned by exactly one watcher. Only the owner can transition OUT of that state.

```
┌─────────────────────────────────────────────────────────┐
│  State       │  Owner        │  Responsibility          │
├─────────────────────────────────────────────────────────┤
│  pending     │  SetupWatcher │  Worktree creation, env  │
│  ready       │  Assigner     │  Find work, assign task  │
│  assigned    │  AgentWatcher │  Run to completion/death │
└─────────────────────────────────────────────────────────┘
```

State transitions:

```
                    User requests new agent
                           │
                           ▼
                      ┌─────────┐
                      │ pending │ ◄──── SetupWatcher owns
                      └────┬────┘
                           │ setup complete
                           ▼
     ┌────────────────┌─────────┐◄───────────────┐
     │                │  ready  │ ◄──── Assigner │
     │                └────┬────┘        owns    │
     │                     │ task assigned       │
     │                     ▼                     │
     │               ┌──────────┐                │
     └───────────────│ assigned │ ◄──── Agent   │
       task done,    └──────────┘        Watcher │
       killed, or                        owns    │
       timed out     ────────────────────────────┘
```

No mutex needed - ownership rules prevent races:

- Assigner: only writes `ready` → `assigned`
- AgentWatcher: only writes `assigned` → `ready`
- SetupWatcher: only writes `pending` → `ready`

---

## Initialization

```go
func main() {
    state := loadState()

    go assigner(&state)
    go quotaWatcher(&state)
    go agentWatcher(&state)
    go setupWatcher(&state)

    // HTTP API server (for web UI)
    go serveAPI(&state)

    // Block forever (or until signal)
    select {}
}
```

Each watcher loops and handles agents in its owned state. On startup, watchers pick up where they left off based on persisted state.

---

## Quota Management

### Data Structure

```go
type Quota struct {
    Accounts  []AccountQuota
    UpdatedAt time.Time
}

type AccountQuota struct {
    Name   string
    Models map[string]float64  // model -> remaining fraction (0.0 to 1.0)
}

// Example data:
// Accounts: [
//   { Name: "primary", Models: { "gemini-3-flash-preview": 0.85, "gemini-3-pro-preview": 0.42 } },
//   { Name: "secondary", Models: { "gemini-3-flash-preview": 0.95, "gemini-3-pro-preview": 0.78 } },
// ]
```

### Methods

```go
// For Assigner: aggregate capacity for task weighting
func (q *Quota) TotalFor(model string) float64 {
    total := 0.0
    for _, acc := range q.Accounts {
        total += acc.Models[model]
    }
    return total
}

// For AgentWatcher: pick best account when starting gemini
func (q *Quota) BestAccountFor(model string) (string, error) {
    best := ""
    bestVal := 0.0
    for _, acc := range q.Accounts {
        if v := acc.Models[model]; v > bestVal {
            best = acc.Name
            bestVal = v
        }
    }
    if best == "" {
        return "", fmt.Errorf("no account with quota for %s", model)
    }
    return best, nil
}
```

### QuotaWatcher

Simple refresh loop. Data can be slightly stale (up to 60s).

```go
func quotaWatcher(state *State) {
    for {
        err := state.Quota.Refresh()
        if err != nil {
            log.Error("quota refresh: %v", err)
        }
        time.Sleep(60 * time.Second)
    }
}

func (q *Quota) Refresh() error {
    for i, acc := range q.Accounts {
        models, err := fetchQuotaForAccount(acc.Name)
        if err != nil {
            log.Warn("quota fetch failed for %s: %v", acc.Name, err)
            continue
        }
        q.Accounts[i].Models = models
    }
    q.UpdatedAt = time.Now()
    return nil
}
```

---

## Assigner

Finds ready agents, assigns tasks with quota-aware weighted selection.

```go
func assigner(state *State) {
    for {
        for _, agent := range state.Agents {
            if agent.State == "ready" {
                task, err := state.SelectTask()
                if err != nil {
                    log.Error("SelectTask: %v", err)
                    continue
                }
                if task != nil {
                    agent.TaskID = task.ID
                    agent.State = "assigned"
                    persistState(state)
                    // AgentWatcher will pick it up
                }
            }
        }
        time.Sleep(1 * time.Second)
    }
}
```

### Task Selection with Weighted Random

Prefer complex (pro) tasks 5:1 when pro quota is available.

```go
func (s *State) SelectTask() (*Task, error) {
    proCapacity := s.Quota.TotalFor("gemini-3-pro-preview")
    flashCapacity := s.Quota.TotalFor("gemini-3-flash-preview")

    ready := s.ReadyTasks()  // Parse from beads JSONL
    if len(ready) == 0 {
        return nil, nil
    }

    // Build weights
    var weights []float64
    for _, task := range ready {
        if task.IsComplex() && proCapacity > 0 {
            weights = append(weights, 5.0)  // Prefer complex
        } else if !task.IsComplex() && flashCapacity > 0 {
            weights = append(weights, 1.0)
        } else {
            weights = append(weights, 0.0)  // Can't run this one
        }
    }

    return weightedRandom(ready, weights)
}
```

---

## AgentWatcher

Monitors all assigned agents. Spawns per-agent watch goroutines.

```go
func agentWatcher(state *State) {
    watching := make(map[int]bool)  // Track which agents have watchers

    for {
        for _, agent := range state.Agents {
            if agent.State == "assigned" && !watching[agent.ID] {
                watching[agent.ID] = true
                go watchAgent(state, agent, func() {
                    watching[agent.ID] = false
                })
            }
        }
        time.Sleep(100 * time.Millisecond)
    }
}
```

### Per-Agent Watch Loop

```go
func watchAgent(state *State, agent *Agent, onDone func()) {
    defer onDone()
    defer cleanup(state, agent)

    if err := ensureGeminiStarted(state, agent); err != nil {
        log.Error("Agent %d: start failed: %v", agent.ID, err)
        return
    }

    for {
        if shouldKill, reason := checkHealth(state, agent); shouldKill {
            log.Warn("Agent %d: %s, killing", agent.ID, reason)
            killProcess(agent.PID)
            break
        }

        if !processAlive(agent.PID) {
            log.Info("Agent %d: process exited", agent.ID)
            break
        }

        time.Sleep(100 * time.Millisecond)
    }
}
```

### ensureGeminiStarted

Start gemini if not already running.

```go
func ensureGeminiStarted(state *State, agent *Agent) error {
    if agent.PID != 0 {
        return nil  // Already running
    }

    pid, err := startGemini(state, agent)
    if err != nil {
        agent.State = "ready"  // Return to pool
        persistState(state)
        return err
    }

    agent.PID = pid
    agent.StartedAt = time.Now()
    agent.LastActivity = time.Now()
    agent.LogOffset = 0
    persistState(state)
    return nil
}
```

### checkHealth

Check log file for new events, run diagnostics, check timeouts.
Returns (shouldKill, reason) if agent needs to be killed.

```go
func checkHealth(state *State, agent *Agent) (bool, string) {
    // 1. Process new log data
    fileSize, err := statLogFile(agent)
    if err == nil && fileSize > agent.LogOffset {
        bytes := readFrom(agent.LogFile, agent.LogOffset, fileSize)

        diagnosis, newOffset, events, err := processEvents(bytes, agent.LogOffset)
        if err != nil {
            log.Error("Agent %d: processEvents: %v", agent.ID, err)
        }

        // Update state
        agent.LogOffset = newOffset
        if len(events) > 0 {
            agent.LastActivity = time.Now()
            for _, e := range events {
                broadcast(e)
            }
        }
        persistState(state)

        // Check diagnosis
        if diagnosis == Sick {
            return true, "pathology detected"
        }
    }

    // 2. Check idle timeout
    if time.Since(agent.LastActivity) > state.Config.IdleTimeout {
        return true, fmt.Sprintf("idle timeout (%v)", state.Config.IdleTimeout)
    }

    // 3. Check max runtime
    if time.Since(agent.StartedAt) > state.Config.MaxRuntime {
        return true, fmt.Sprintf("max runtime exceeded (%v)", state.Config.MaxRuntime)
    }

    return false, ""
}
```

### cleanup

Triage worktree and return agent to ready state.

```go
func cleanup(state *State, agent *Agent) {
    if err := triageWorktree(agent); err != nil {
        log.Error("Agent %d: triage failed: %v", agent.ID, err)
    }

    agent.PID = 0
    agent.TaskID = ""
    agent.State = "ready"
    persistState(state)

    log.Info("Agent %d: returned to ready state", agent.ID)
}
```

---

## Processing Events

Pure function - no side effects. Returns results for watcher to handle.

```go
type Diagnosis int
const (
    Healthy Diagnosis = iota
    Sick              // Kill the agent
)

func processEvents(bytes []byte, startOffset int) (Diagnosis, newOffset int, events []Event, err error) {
    offset := startOffset

    for _, line := range splitLines(bytes) {
        event, err := parseJSON(line)
        if err != nil {
            return Healthy, offset, events, fmt.Errorf("parse event: %w", err)
        }

        offset += len(line) + 1  // +1 for newline
        events = append(events, event)

        // Check for pathologies
        diagnosis, err := diagnose(event)
        if err != nil {
            return Healthy, offset, events, fmt.Errorf("diagnose: %w", err)
        }
        if diagnosis == Sick {
            return Sick, offset, events, nil
        }
    }

    return Healthy, offset, events, nil
}

func diagnose(event Event) (Diagnosis, error) {
    // Known pathologies:

    // 1. Repeated identical errors (infinite loop)
    if isRepeatedError(event) {
        return Sick, nil
    }

    // 2. Fatal errors
    if strings.Contains(event.Message, "FATAL") {
        return Sick, nil
    }

    // 3. Seatbelt sandbox poisoning
    // When macOS sandbox blocks shell exec, session is permanently broken
    if event.Type == "tool_result" && event.Tool == "run_shell_command" {
        if strings.Contains(event.Error, "operation not permitted") ||
           strings.Contains(event.Error, "sandbox") {
            return Sick, nil
        }
    }

    // 4. TODO: Add more patterns as discovered

    return Healthy, nil
}
```

---

## State Persistence

State file written on every mutation:

```json
{
  "agents": [
    {
      "id": 1,
      "state": "assigned",
      "pid": 12345,
      "task_id": "machinator-8rn",
      "started_at": "2026-01-13T20:00:00Z",
      "log_offset": 45678,
      "last_activity": "2026-01-13T20:05:00Z"
    },
    {
      "id": 2,
      "state": "ready",
      "pid": 0,
      "task_id": ""
    }
  ],
  "quota": {
    "accounts": [...],
    "updated_at": "2026-01-13T20:04:00Z"
  }
}
```

On startup, each watcher reads state and handles its owned agents:

- AgentWatcher: reattaches to `assigned` agents with valid PIDs
- SetupWatcher: resumes `pending` agents
- Assigner: assigns tasks to `ready` agents

---

## Worktree Triage

After agent exits (success, failure, timeout, or kill):

```go
func triageWorktree(agent *Agent) error {
    status, err := git(agent.WorktreeDir, "status", "--porcelain")
    if err != nil {
        return fmt.Errorf("git status: %w", err)
    }

    if status == "" {
        // Clean - nothing to do
        return nil
    }

    diff, err := git(agent.WorktreeDir, "diff", "--stat")
    if err != nil {
        return fmt.Errorf("git diff: %w", err)
    }

    lines := countChangedLines(diff)
    files := countChangedFiles(diff)

    if files <= 1 && lines < 20 {
        // Minor changes - discard
        _, err := git(agent.WorktreeDir, "checkout", "--", ".")
        return err
    }

    // Significant work - mark for retry
    agent.NeedsRetry = true
    return nil
}
```

---

## Gemini Invocation

Hard-won knowledge from v1/v2:

```go
func startGemini(state *State, agent *Agent) (int, error) {
    task := state.GetTask(agent.TaskID)

    // Select model with quota-aware fallback
    model, accountName, err := selectModelAndAccount(state, task)
    if err != nil {
        return 0, err
    }
    account := state.GetAccount(accountName)

    // Build command
    cmd := exec.Command(geminiPath,
        "--yolo",
        "--sandbox",
        "--model", model,
        "--output-format", "stream-json",
        directive,
    )

    // Set working directory to agent's worktree
    cmd.Dir = agent.WorktreeDir

    // Redirect output to log file
    logFile, err := os.Create(agent.LogFile)
    if err != nil {
        return 0, fmt.Errorf("create log file: %w", err)
    }
    cmd.Stdout = logFile
    cmd.Stderr = logFile

    // Account isolation
    cmd.Env = append(os.Environ(),
        "HOME="+account.HomeDir,
        "GEMINI_CLI_HOME="+account.HomeDir,
        "GEMINI_FORCE_FILE_STORAGE=true",
    )

    // Set git author for this agent
    cmd.Env = append(cmd.Env,
        fmt.Sprintf("GIT_AUTHOR_NAME=Machinator Agent: %d", agent.ID),
        fmt.Sprintf("GIT_AUTHOR_EMAIL=agent-%d@machinator.local", agent.ID),
        fmt.Sprintf("GIT_COMMITTER_NAME=Machinator Agent: %d", agent.ID),
        fmt.Sprintf("GIT_COMMITTER_EMAIL=agent-%d@machinator.local", agent.ID),
    )

    if err := cmd.Start(); err != nil {
        return 0, fmt.Errorf("start gemini: %w", err)
    }

    return cmd.Process.Pid, nil
}
```

### selectModelAndAccount

Quota-aware model and account selection with fallback.

```go
func selectModelAndAccount(state *State, task *Task) (model, account string, err error) {
    complexModel := state.Config.Models.ComplexModelName
    simpleModel := state.Config.Models.SimpleModelName

    if task.IsComplex() {
        // Complex tasks require complex model - no fallback
        acc, err := state.Quota.BestAccountFor(complexModel)
        if err != nil {
            return "", "", fmt.Errorf("complex task but no quota for %s: %w", complexModel, err)
        }
        return complexModel, acc, nil
    }

    // Simple task: prefer simple model, upgrade to complex if no quota
    if acc, err := state.Quota.BestAccountFor(simpleModel); err == nil {
        return simpleModel, acc, nil
    }

    // Upgrade: use complex model for simple task
    if acc, err := state.Quota.BestAccountFor(complexModel); err == nil {
        log.Info("Task %s: upgrading to %s (no %s quota)", task.ID, complexModel, simpleModel)
        return complexModel, acc, nil
    }

    return "", "", fmt.Errorf("no quota available for any model")
}
```

## Beads Integration

The orchestrator treats beads as **read-only** on the main repo:

| Operation     | Where          | How                                             |
| ------------- | -------------- | ----------------------------------------------- |
| List tasks    | Main repo      | Parse `.beads/issues.jsonl` directly            |
| Claim task    | Agent worktree | `bd --sandbox update TASK --status=in_progress` |
| Complete task | Agent worktree | Agent does `bd update TASK --status=closed`     |
| Sync changes  | Agent          | Agent does `git push`                           |

The orchestrator **never** writes to main repo beads. This prevents dirty worktree issues.

---

## Web API

### REST Endpoints

| Endpoint                    | Description                    |
| --------------------------- | ------------------------------ |
| `GET /api/agents`           | List agents and their state    |
| `GET /api/tasks`            | List tasks from beads          |
| `GET /api/quota`            | Current quota across accounts  |
| `GET /api/events?cursor=N`  | Get events since byte offset N |
| `POST /api/agents/:id/kill` | Kill a running agent           |
| `POST /api/agents/add`      | Add a new agent (→ pending)    |

### WebSocket

```
Client → Server: { "subscribe": ["events", "state"] }
Server → Client: { "type": "event", "agent": 1, "data": {...} }
Server → Client: { "type": "state", "agents": [...], "quota": {...} }
```

Real-time push of:

- Agent state changes (ready → assigned → ready)
- Gemini output events
- Quota updates
- Diagnostic alerts

---

## Configuration

### MACHINATOR_DIR

All machinator data lives under a single directory:

```bash
export MACHINATOR_DIR=~/.machinator  # default
```

Structure:

```
$MACHINATOR_DIR/
├── config.yaml           # User config
├── state.json            # Persisted state (agents, quota cache)
├── accounts/             # Account directories (auto-discovered)
│   ├── primary/
│   │   └── .gemini/      # Gemini credentials
│   └── secondary/
│       └── .gemini/
└── logs/                 # Execution logs
    └── agent-1/
        └── gemini.log
```

### config.yaml (global)

```yaml
# $MACHINATOR_DIR/config.yaml
agents:
  count: 3 # Number of agent slots

timeouts:
  idle: 10m # Kill if no activity
  max_runtime: 30m # Kill if running too long

intervals:
  assigner: 1s # Task assignment check
  quota_refresh: 60s # Quota polling
  agent_watch: 100ms # Log file polling
```

### Project config.json

Each project has its own config at `$MACHINATOR_DIR/projects/<id>/config.json`:

```json
{
  "repo": "git@github.com:bryantinsley/machinator",
  "branch": "main",
  "simple_model_name": "gemini-3-flash-preview",
  "complex_model_name": "gemini-3-pro-preview"
}
```

The orchestrator clones the repo to `$MACHINATOR_DIR/projects/<id>/repo/`.

Directory structure with projects:

```
$MACHINATOR_DIR/
├── config.yaml              # Global config
├── state.json               # Persisted state
├── accounts/                # Gemini accounts
│   ├── primary/.gemini/
│   └── secondary/.gemini/
├── projects/
│   └── 1/
│       ├── config.json      # Project config
│       ├── repo/            # Cloned repository
│       └── agents/          # Per-agent worktrees
│           ├── 1/
│           └── 2/
└── logs/
```

### Account Discovery

Accounts are discovered dynamically from filesystem:

```go
func (q *Quota) discoverAccounts(machinatorDir string) ([]string, error) {
    pattern := filepath.Join(machinatorDir, "accounts", "*")
    dirs, err := filepath.Glob(pattern)
    if err != nil {
        return nil, err
    }

    var accounts []string
    for _, dir := range dirs {
        info, err := os.Stat(dir)
        if err == nil && info.IsDir() {
            accounts = append(accounts, filepath.Base(dir))
        }
    }
    return accounts, nil
}
```

To add an account: `mkdir $MACHINATOR_DIR/accounts/newaccount`
To remove an account: delete the directory

---

## Open Questions

1. **Unblocking mode**: How does it work? Separate agent mode or separate task type?
2. **Task priority**: Should P0 tasks preempt running work?
3. **Directive delivery**: Inline argument or file in worktree?

---

## Next Steps

1. [ ] Define Go types for Agent, Task, State, Quota
2. [ ] Implement Assigner
3. [ ] Implement AgentWatcher with log polling
4. [ ] Implement QuotaWatcher
5. [ ] Implement SetupWatcher
6. [ ] Implement state persistence
7. [ ] Add HTTP API
8. [ ] Add WebSocket events
9. [ ] Build React frontend
