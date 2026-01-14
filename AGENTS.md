# Agent Instructions

This project uses **bd** (beads) for issue tracking. Run `bd onboard` to get started.

## Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --status in_progress  # Claim work
bd close <id>         # Complete work
bd sync               # Sync with git
```

## Core Principles

1.  **Trust But Verify**: Never assume an action had the predicted outcome. Always check.
    - _Example_: If you run `bd create --deps ...`, run `bd ready` immediately after to confirm dependencies were actually linked correctly.
    - _Example_: If you edit a file, run a build or test to confirm it didn't break.
    - _Anti-Pattern_: "I ran the command, so it must be done." -> **WRONG**.

## Visual Evidence

When modifying UI, you MUST update visual artifacts (Golden files, VHS tapes) to prove it looks correct.

**VHS Recordings (CI Gated)**
For VHS tapes (`.tape` files), do NOT run them locally (slow, sandbox issues). Instead:

1.  **Modify**: Make your changes to `.tape` files or the code they record.
2.  **Push**: Commit and push your changes.
3.  **Gate**: Create a **gate** bead to wait for CI generation:

    ```bash
    bd create --type gate --title "Wait for VHS CI" --desc "Waiting for GitHub Action to generate GIFs" --parent <current_task_id>
    ```

    _(Note: The CI will run VHS on Linux, generate GIFs, and commit them back to the repo. The next agent will pull these changes.)_

4.  **Follow the Vision**: Before major refactors, consult `planning/architecture-vision.md` to ensure alignment with the long-term plan (Unified Binary, Dummy Testing).
5.  **Preserve History**: Use `git mv` when renaming or moving files to maintain git history. Do not use `rm` or `mv` alone for versioned files.

## Testing & Building

This project uses **Bazel** as the primary build and test system. **Do NOT use raw Go commands** (`go build`, `go test`, `go mod tidy`) unless absolutely necessary.

### Bazel Commands (PREFERRED)

```bash
bazel build //backend/cmd/machinator:machinator   # Build the binary
bazel test //backend/...                          # Run all backend tests
bazel test //backend/internal/beads:beads_upstream_test  # Run a specific test
bazel query //...                                 # List all targets
```

### BUILD Files

- Build files are named `BUILD` (NOT `BUILD.bazel`)
- Each Go package needs a `BUILD` file with `go_library` and optionally `go_test`
- Use `go_library`, `go_test` from `@rules_go//go:def.bzl`

### Why Bazel?

1. **Hermetic builds** - Same result on any machine
2. **Caching** - Only rebuilds what changed
3. **No permission issues** - Avoids macOS sandbox conflicts with Go caches
4. **Dependency management** - Handled via `go.mod` + `gazelle`

### Go Environment Setup (Direct Go Usage)

**CRITICAL**: If you run raw Go commands (`go build`, `go test`, `go mod tidy`, etc.) instead of Bazel, you MUST configure Go to use local workspace directories for its caches. Failure to do this will result in "operation not permitted" errors on macOS due to sandbox restrictions.

**Bazel handles its own caches and does NOT require these exports.**

**Run this ONLY if using direct `go` commands:**

```bash
export GOPATH="$(pwd)/.go-cache"
export GOCACHE="$(pwd)/.go-build-cache"
export GOMODCACHE="$(pwd)/.go-cache/pkg/mod"
export PATH="$(pwd)/.go-cache/bin:$PATH"
mkdir -p "$GOPATH" "$(pwd)/.go-cache/bin" "$GOCACHE" "$GOMODCACHE"
```

**Why?** The default Go cache locations (`~/Library/Caches/go-build`, etc.) are blocked by macOS security restrictions in sandboxed environments. Using project-local directories avoids these permission errors.

## System-Wide Installations (FORBIDDEN)

**Agents are NOT permitted to install anything system-wide.** This includes:

- `brew install ...`
- `npm install -g ...`
- `go install ...` (without local GOPATH)
- `pip install ...` (without virtualenv)
- Any command requiring `sudo`

**Why?** The sandbox will block these operations and your session will fail.

**If a dependency can ONLY be installed system-wide:**

1. **STOP** - Do not attempt the installation
2. **Document** - Create a blocking issue explaining what's needed
3. **Escalate** - The human operator (Bryan) will install it manually
4. **Wait** - Move to a different task until the blocker is resolved

**Allowed local installations:**

- Go modules via `go get` (with local GOPATH configured)
- npm packages in `node_modules/` (project-local)
- Python packages in a virtualenv within the project

## Running Machinator (FORBIDDEN)

**Agents are NOT permitted to run machinator or any orchestrator commands.** This includes:

- `./machinator`
- `bazel run :machinator`
- Any variations with `--headless`, `--execute`, etc.

**Why?** You ARE running inside machinator. Running it again causes recursive fork bombs that spawn infinite processes.

**If you need to test orchestrator behavior:**

1. **Use Bazel** - Run `bazel test //...` to ensure everything is working correctly.
2. **Write unit tests** - Test individual functions within the `orchestrator/pkg/...` packages.
3. **Use mocks** - Mock external dependencies (e.g., using `tools/dummy-gemini`).
4. **Trust the orchestrator** - It handles execution, you handle code.

## Multi-Agent Orchestration

This project supports running multiple agents in parallel.

- **Isolation**: Each agent runs in its own **Git Worktree** under `./machinator/agents/<n>/`.
- **Coordination**: Agents do not talk to each other; they coordinate via **Beads** status and shared state.
- **Quota**: The orchestrator manages a pool of Gemini accounts to aggregate quota and avoid rate limits.

**If you are an agent:** You may be running in a worktree. Always use relative paths when possible.

## macOS Sandbox & Security

Agents run under a strict macOS Seatbelt profile.

- **Filesystem**: Access is restricted to the project directory and temporary directories.
- **Network**: Restricted to necessary API endpoints (Gemini, GitHub).
- **Home Dir**: The `$HOME` directory is often spoofed to a project-local directory to isolate configurations.
- **Keychain**: Git operations use the macOS Keychain for persistent HTTPS authentication.

If you encounter "Operation not permitted" errors, it is likely a sandbox violation. Document the blocked path and escalate.

## Git Authentication

Agents must be able to push changes to the remote repository. This project uses **HTTPS** for git operations (not SSH).

### Why HTTPS?

- **Sandbox Isolation**: SSH requires access to a `.ssh/` directory in the user's `$HOME`. The macOS sandbox restricts access to these directories, and spoofing a full SSH environment is complex.
- **Credential Persistence**: Git's HTTPS authentication uses the **macOS Keychain** (`osxkeychain` helper). The Keychain is accessible to all processes regardless of their `$HOME` environment variable, making it ideal for sandboxed agents.

### How it Works

1.  **Helper Configuration**: The repository is configured to use the `osxkeychain` credential helper.
2.  **Auth Flow**: When an agent runs `git push`, git calls the helper. The helper retrieves the Personal Access Token (PAT) from the system keychain.
    _Note: A global rate limit of 1 push every 30 seconds is enforced via a git pre-push hook. If another push has occurred recently, your push will wait automatically._
3.  **No Explicit Config**: Agents do **not** need to provide passwords, tokens, or SSH keys. As long as the host machine is authenticated, the agents "just work."

### Alternative: Moving to SSH

If the project ever migrates to SSH, agents would need:

- `GIT_SSH_COMMAND` override to point to a specific key.
- A symlink to the real `.ssh` directory within the agent's spoofed `$HOME`.
- **Note**: This is currently avoided to keep the agent environment simple and robust.

### Risks and Mitigations

- **Risk**: Any process (including a malicious or buggy agent) could potentially use the stored credentials.
- **Mitigation**: Agents are restricted by the macOS sandbox to only access the project directory and temporary directories. They cannot access arbitrary system resources.
- **Risk**: Credentials expire.
- **Mitigation**: The human operator (Bryan) must ensure the Keychain remains populated with a valid token.

## Documentation Sync

When changing code in these areas, **also update the corresponding docs**:

| Code Area                  | Update This Doc                                  |
| -------------------------- | ------------------------------------------------ |
| `~/.machinator/` structure | `planning/directory-structure.md`                |
| `./machinator/` local dirs | `planning/directory-structure.md`                |
| Config file formats        | `planning/directory-structure.md`, `README.md`   |
| UI components              | `planning/ui-component-system.md`                |
| Directive templates        | `templates/`, `bootstrap/directive_template.txt` |
| Architecture changes       | `planning/architecture-vision.md`                |

**Docs are part of the change.** Don't merge code that makes docs stale.

## CI-Gated Operations (BETA)

For slow operations (like VHS terminal recording) or operations incompatible with the macOS sandbox, use the CI-gated workflow:

1.  **Make your changes** and commit:

    ```bash
    git add -A && git commit -m "feat: update TUI layout" && git push
    ```

2.  **Create a gate** that waits for the CI workflow:

    ```bash
    bd create --type=gate --title="Wait for VHS CI" --external-ref="gh:run:vhs"
    ```

3.  **Link your follow-up task** to the gate:

    ```bash
    bd dep add <follow-up-task-id> <gate-id>
    ```

4.  **Exit immediately** - the orchestrator will check gates periodically.

5.  **When CI completes**, the gate resolves and the follow-up task unblocks.

This keeps your laptop cool and avoids macOS sandbox restrictions!

### Local Fallback (Use if CI is down)

If GitHub Actions are unavailable, use the local Docker wrapper (slow but functional):

1. **Build Linux Binary**:

   ```bash
   bazel build //orchestrator/cmd/machinator:machinator-linux
   cp bazel-bin/orchestrator/cmd/machinator/machinator-linux .
   ```

2. **Run VHS with Docker**:

   ```bash
   ./scripts/vhs-docker.sh orchestrator/e2e/navigation.tape
   ./scripts/vhs-docker.sh orchestrator/e2e/crud.tape
   ```

3. **Verify & Commit**:
   Check `docs/ui-history/*.gif` and commit them.

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - `bazel test //...`
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd sync
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**

- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
