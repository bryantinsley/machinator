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

## Go Environment Setup

**CRITICAL**: Before running ANY Go commands (`go build`, `go test`, `go mod tidy`, etc.), you MUST configure Go to use local workspace directories for its caches. Failure to do this will result in "operation not permitted" errors on macOS due to sandbox restrictions.

**Run this at the start of every session:**

```bash
export GOPATH="$(pwd)/.go-cache"
export GOCACHE="$(pwd)/.go-build-cache"
export GOMODCACHE="$(pwd)/.go-cache/pkg/mod"
export PATH="$(pwd)/.go-cache/bin:$PATH"
mkdir -p "$GOPATH" "$(pwd)/.go-cache/bin" "$GOCACHE" "$GOMODCACHE"
```

**Why?** The default Go cache locations (`~/Library/Caches/go-build`, etc.) are blocked by macOS security restrictions in sandboxed environments. Using project-local directories avoids these permission errors.

**Directories created:**

- `.go-cache/` - Go module downloads and workspace data
- `.go-build-cache/` - Compiled build artifacts

These directories are already in `.gitignore`.

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
   # Ensure Go env is set first!
   export GOPATH="$(pwd)/.go-cache"
   export GOCACHE="$(pwd)/.go-build-cache"
   export GOMODCACHE="$(pwd)/.go-cache/pkg/mod"
   GOOS=linux GOARCH=amd64 go build -o machinator-linux ./orchestrator/cmd/machinator
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
2. **Run quality gates** (if code changed) - Tests, linters, builds
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
