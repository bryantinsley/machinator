# Machinator Directory Structure

_Reference for ~/.machinator and project-local ./machinator layouts_

**Last Updated**: January 2026

---

## 1. Global Directory: `~/.machinator/`

User-wide configuration and resources, shared across all projects.

```
~/.machinator/
├── settings.json           # Global settings (default model, preferences)
├── gemini                   # Managed Gemini CLI binary
│
├── accounts/                # Account pool for multi-account rotation
│   ├── account1.json        # Account metadata (name, auth_type, api_key, gemini_dir)
│   ├── account2.json
│   ├── default/             # Legacy structure: Primary account (fake HOME)
│   │   ├── .gemini/         # Gemini auth for this account
│   │   └── account.json     # Account metadata (name, auth_type)
│   └── ...
│
├── projects/                # Project registry
│   ├── 1/                   # Project by numeric ID (names are mutable)
│   │   └── project.json     # Project config (name, repo_url, agent_count, timeouts)
│   ├── 2/
│   │   └── project.json
│   └── ...
│
└── resources/               # Shared resources (templates, etc.)
```

### Key Files

**settings.json**

```json
{
  "default_model": "gemini-3-flash-preview",
  "account_pooling": true,
  "theme": "dark"
}
```

**accounts/*.json**

```json
{
  "name": "default",
  "auth_type": "api_key", // or "google"
  "api_key": "AIza...",
  "gemini_dir": "/path/to/.gemini",
  "home_dir": "/path/to/fake/home"
}
```

**projects/\*/project.json**

```json
{
  "id": 1,
  "name": "My Project",
  "repo_url": "https://github.com/user/repo.git",
  "agent_count": 3,
  "idle_timeout": "5m",
  "max_runtime": "30m",
  "worktree_strategy": "per-invocation"
}
```

#### Worktree Strategies (`worktree_strategy`)

- `per-task`: Fresh worktree every task (clean but loses uncommitted work).
- `per-invocation`: Reset on new task, keep on retry (default).
- `persistent`: Never auto-reset.

---

## 2. Project-Local Directory: `./machinator/`

Project-specific orchestration data, lives in the repo root.

```
./machinator/
├── logs/                    # Orchestration logs
│   └── tui_debug.log
│
├── agents/                  # Per-agent working directories (multi-agent)
│   ├── 1/                   # Git worktree for agent 1
│   ├── 2/                   # Git worktree for agent 2
│   └── 3/                   # Git worktree for agent 3
│
└── tmp/                     # Temporary files (directive files, etc.)
```

### Agent Worktrees

For multi-agent orchestration, each agent gets its own git worktree:

```bash
git worktree add ./machinator/agents/1 main
git worktree add ./machinator/agents/2 main
git worktree add ./machinator/agents/3 main
```

This allows parallel work without file conflicts.

---

## 3. Bootstrap Directory: `./bootstrap/`

Legacy bootstrap orchestrator (shell-based). Being replaced by the Go TUI.

```
./bootstrap/
├── run.sh                   # Start bootstrap orchestrator
├── stop.sh                  # Stop all agents
├── orchestrator_with_quota.sh
├── directive_template.txt
├── unblocking_directive.txt
└── ...
```

---

## 4. Related Directories

**`.beads/`** - Beads issue tracker database (not Machinator-specific)

**`.gemini/`** - User's default Gemini CLI config (used if no account pooling)

---

## 5. Gitignore Patterns

These should be in `.gitignore`:

```
# Machinator
machinator/logs/
machinator/agents/
machinator/tmp/
.machinator/

# Temporary binaries
machinator-linux
```
