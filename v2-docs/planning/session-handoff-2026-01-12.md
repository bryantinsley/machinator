# Session Handoff Notes

_Lessons learned and context for the next co-architect/scrum master_

**Session Date**: January 11-12, 2026  
**Participants**: Bryan (human), Claude (AI pair programmer), Gemini agents (task execution)

---

## 1. Project State Summary

**Machinator** is a TUI-based AI agent orchestrator. It:

- Fetches tasks from Beads (issue tracker)
- Builds directives and runs Gemini CLI
- Monitors agent progress, handles timeouts
- Supports quota management across multiple models

**Current architecture:**

- **Unified binary**: `bazel run //:machinator` handles both setup and orchestration
- **Context-aware**: Launches setup if no `.beads/`, otherwise orchestrator
- **Mode switching**: `--setup` flag forces setup mode (but UI switching is broken - P1 bug)

---

## 2. Key Documents to Read

| Document                             | Purpose                                                   |
| ------------------------------------ | --------------------------------------------------------- |
| `planning/architecture-vision.md`    | The north star - unified binary, multi-agent, testability |
| `planning/documentation-strategy.md` | UX philosophy: self-evident UI, no docs needed            |
| `planning/ui-component-system.md`    | Clickable component library design                        |
| `planning/directory-structure.md`    | ~/.machinator and ./machinator layouts                    |
| `AGENTS.md`                          | Rules for AI agents working on this codebase              |

---

## 3. Lessons Learned This Session

### Task Design

- **Tasks should be 2-5 minutes of focused work** - if larger, break into subtasks
- **Be directive, not suggestive** - tell agents what to do, don't say "consider" or "recommend"
- **Include CHALLENGE:simple or CHALLENGE:complex** - controls which model is used
- **Epics should be CHALLENGE:simple** - they're containers, not deep work

### Agent Behavior

- **Agents read tasks once** - mid-task comments won't be seen
- **ONE TASK PER SESSION** - added explicit rule to prevent agents doing multiple tasks
- **Agents share working directory** - can accidentally commit each other's work

### Quota Management

- **Flash vs Pro** - simple tasks use Flash (cheap), complex use Pro (expensive)
- **Gemini CLI has internal classifier** - even with `--model` flag, it may make extra API calls
- **Account pooling** - work in progress to aggregate quota across multiple accounts

### Testing

- **VHS for terminal recordings** - runs in Docker (Linux) not locally
- **Use real `bd` not mocks** - install bd in Docker container, use fixture .beads/ database
- **Golden files** for UI testing - compare rendered output to known-good snapshots

---

## 4. Current Work Streams

### In Progress

- **Account Pooling** (`machinator-dgj.*`) - fake HOME dirs per account
- **VHS Re-recording** (`machinator-ydu`) - fix broken GIFs
- **Multi-Agent UI** (`machinator-7jw.*`) - dropdowns, agent panels, controls

### Blocked

- **CI/GitHub Actions** (`machinator-awl.*`) - billing issue, waiting for Bryan to fix

### Design Decisions Made

- **Worktree strategy**: Configurable per-project (per-task, per-invocation, persistent)
- **Branch enforcement**: Agents must work on project's selected branch
- **CLI flags**: `--project`, `--agents`, `--run` for scripting

---

## 5. Known Issues / Tech Debt

1. **Mode switching broken** - P1 bug `machinator-nxw`, no way to go Setup → Orchestrator in UI
2. **Separate setup binary archived** - `orchestrator/cmd/setup/` has been moved to `_archive/cmd-setup-legacy/`
3. **VHS tapes use echo for fixtures** - should use pre-made fixture directory
4. **Bootstrap orchestrator still in use** - Go TUI is the successor but not fully deployed
5. **bd fixture is stub** - `testdata/vhs-fixture/bin/bd` just echoes `[]`

---

## 6. North Star Reminder

> **The ultimate quickstart is one command: `bazel run //:machinator`**

Two levels of failure:

1. If users need documentation → UX failed
2. If the UI needs to explain itself → UI design failed

Every feature should be obvious without explanation.

---

## 7. How to Continue

1. **Check ready tasks**: `bd ready`
2. **Review this session's new tasks**: Most are P2/P3, will chain-unblock
3. **Fix P1 bug first**: Mode switching (nxw) is blocking unified experience
4. **Run overnight**: 34 open tasks, 10 ready, should burn ~5-8 hours of quota

---

## 8. Bryan's Preferences (Observed)

- **No `rm` command** - move to `_archive/` instead
- **No `find` command** - use alternatives
- **Agents work unattended** - sandbox is the safety net
- **Prefer real tools over mocks** - use actual `bd` in Docker, not mock scripts
- **Docs are part of the change** - update planning docs when changing code
- **Be direct** - tell agents what to do, don't suggest

---

_Good luck to the next collaborator! 🚀_
