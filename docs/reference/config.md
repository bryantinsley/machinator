# Configuration Reference

Machinator uses several JSON configuration files and environment variables to control its behavior.

## Global Settings

**File Path:** `~/.machinator/settings.json`

This file contains global configuration for the Machinator environment.

| Field | Type | Description |
| :--- | :--- | :--- |
| `gemini_cli_path` | string | Path to the `gemini` executable used by agents. |
| `gemini_cli_repo` | string | URL of the `gemini-cli-mods` repository. |
| `initialized_at` | string | ISO 8601 timestamp of when Machinator was first initialized. |
| `next_project_id` | integer | The ID to be assigned to the next created project. |

### Example

```json
{
  "gemini_cli_path": "/home/user/.machinator/gemini",
  "gemini_cli_repo": "https://github.com/bryantinsley/gemini-cli-mods.git",
  "initialized_at": "2026-01-12T10:00:00Z",
  "next_project_id": 5
}
```

---

## Project Configuration

**File Path:** `~/.machinator/projects/<id>/project.json`

Each project has its own `project.json` file defining its specific settings.

| Field | Type | Description |
| :--- | :--- | :--- |
| `id` | integer | Unique identifier for the project. |
| `name` | string | Display name of the project. |
| `repo_url` | string | Git URL of the repository. |
| `branch` | string | Git branch to use. |
| `agent_count` | integer | Number of parallel agents to run for this project. |
| `worktree_strategy` | string | Strategy for managing agent worktrees: `per-task`, `per-invocation`, or `persistent`. (Default: `per-invocation`) |
| `idle_timeout` | duration | How long to wait when no tasks are ready before checking again. (Stored as nanoseconds in JSON). |
| `max_task_runtime` | duration | Maximum time allowed for an agent to work on a single task. (Stored as nanoseconds in JSON). |
| `max_cycles` | integer | Maximum number of orchestration cycles before stopping. |
| `cooldown_period` | duration | Duration to wait between orchestration cycles. (Stored as nanoseconds in JSON). |
| `has_beads` | boolean | Whether the project is a Beads-enabled repository. |
| `created_at` | string | ISO 8601 timestamp of when the project was added. |

### Worktree Strategies

- `per-invocation`: A worktree is created when the orchestrator starts and reused for all tasks during that run. (Default)
- `per-task`: A fresh worktree is created for every single task and removed afterward.
- `persistent`: Worktrees are created once and never removed, allowing for faster task startup but consuming more disk space.

### Example

```json
{
  "id": 1,
  "name": "my-project",
  "repo_url": "https://github.com/user/my-project.git",
  "branch": "main",
  "agent_count": 2,
  "worktree_strategy": "per-invocation",
  "idle_timeout": 300000000000,
  "max_task_runtime": 1800000000000,
  "max_cycles": 10000,
  "cooldown_period": 5000000000,
  "has_beads": true,
  "created_at": "2026-01-12T12:00:00Z"
}
```
*Note: 300,000,000,000 nanoseconds = 5 minutes.*

---

## Account Configuration

**File Path:** `~/.machinator/accounts/<name>/account.json`

Machinator supports multiple accounts for API key rotation and quota management.

| Field | Type | Description |
| :--- | :--- | :--- |
| `name` | string | Unique name for the account. |
| `auth_type` | string | Authentication method: `api_key` or `google`. |

### Example

```json
{
  "name": "primary-account",
  "auth_type": "api_key"
}
```

---

## Environment Variables

These environment variables can be used to override default settings or provide necessary information to Machinator.

| Variable | Description | Default |
| :--- | :--- | :--- |
| `BD_AGENT_NAME` | The base name for agents (e.g., "Gemini-01"). Agents are numbered if multiple are running. | `Gemini-01` |
| `MACHINATOR_DIR` | Custom home directory for Machinator data and configuration. | `~/.machinator` |
| `MACHINATOR_POOLING_ENABLED` | Set to `false` to disable automatic account pooling/rotation. | `true` |
| `MACHINATOR_IDLE_TIMEOUT` | Global override for the idle timeout (uses Go duration string format like `10m`, `5s`). | Value from `project.json` |
| `MACHINATOR_MAX_TASK_RUNTIME` | Global override for the maximum task runtime (uses Go duration string format like `1h`). | Value from `project.json` |

### Example Usage

```bash
export BD_AGENT_NAME="Elite-Agent"
export MACHINATOR_POOLING_ENABLED="false"
machinator
```

---

## Directives & Templates

Machinator uses templates to generate instructions (directives) for AI agents.

### Directive Template

**File Path:** `templates/directive_template.txt`

This template defines the base instructions for all agents. It uses Go `text/template` syntax.

| Variable | Description |
| :--- | :--- |
| `{{.AgentName}}` | The name of the agent (from `BD_AGENT_NAME`). |
| `{{.TaskID}}` | The ID of the current Beads task. |
| `{{.TaskContext}}` | Full details of the task (from `bd show <id>`). |
| `{{.ProjectContext}}` | Contextual information from the first 100 lines of `AGENTS.md`. |

### Unblocking Template

**File Path:** `templates/unblocking_directive.txt`

Used when an agent is stuck or blocked. It instructs the agent to analyze the situation and report the blocker.
