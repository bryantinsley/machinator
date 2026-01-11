# Scrum Master Guidelines

_How to Groom Beads for Autonomous Agents_

If you are tasked with "Planning", "Grooming", or "Project Management", follow these rules. Your job is to create a path that is **unambiguous** and **fault-tolerant**.

## 1. The "Lost Agent" Assumption

Assume the agent picking up your bead:

- Has NO context of previous conversations.
- Will NOT read the entire codebase before starting.
- Will interpret "fix it" in the laziest way possible.

**Your Goal**: Write beads so clear that a "Lost Agent" can still succeed (or fail safely).

## 2. Anatomy of a Perfect Bead

Every task-type bead MUST contain these four sections:

### A. Goal (The "What")

One sentence summary.

> _Goal: Create a dummy Gemini CLI binary for testing._

### B. Specifications (The "How")

Strict constraints. Input/Output definitions.

> _Specs:_
>
> - _Binary Name: `dummy-gemini`_
> - _Must accept flags: `--prompt`, `--repo`_
> - _Must NOT make network calls._

### C. Verification (The "Proof")

Falsifiable command to prove completion.

> _Verification:_
>
> - _Run: `./dummy-gemini --prompt test`_
> - _Assert: Exit code 0._

### D. Safety / Stopping Condition (The "Brakes")

Explicit instructions on when to ABORT.

> _Safety:_
>
> - _If you need to edit `go.mod`, STOP._
> - _If the build fails, do NOT comment out tests. STOP._

## 3. Task Complexity & Model Selection

To optimize for **quota usage** and **reasoning capabilities**, every task description MUST include a complexity tag at the end. This guides the Orchestrator to dispatch the task to the correct model.

| Tag                 | Model                                | Use Case                                                                               | Cost/Quota                   |
| :------------------ | :----------------------------------- | :------------------------------------------------------------------------------------- | :--------------------------- |
| `CHALLENGE:simple`  | **Flash** (`gemini-3-flash-preview`) | Routine coding, boilerplate, getters/setters, minor fixes, documentation.              | Low cost. High availability. |
| `CHALLENGE:complex` | **Pro** (`gemini-3-pro-preview`)     | Architecture changes, refactoring, complex logic, debugging race conditions, planning. | High cost. Limited quota.    |

**Rule of Thumb:**

- If a Junior dev could do it in 5 mins -> `CHALLENGE:simple`
- If a Senior dev needs to think about it -> `CHALLENGE:complex`

## 4. The Pre-Mortem Checklist

Before you execute `bd create` or `bd update`, ask yourself:

1.  **Ambiguity Check**: "If I say 'refactor', will they delete the whole file?" (Specify "Refactor by Addition").
2.  **Context Rot**: "Does this bead rely on a file that might move?" (Use relative paths or search).
3.  **Token Burn**: "Could this loop indefinitely?" (Add timeouts or mock requirements).

## 5. Bead Etiquette

- **Granularity**: 2-10 minute tasks. If it takes >15 mins, split it.
- **Linking**: ALWAYS explicitly link Dependencies (`bd dep add`). A floaty bead is a lost bead.
- **Cleanup**: If a plan changes, delete the obsolete beads immediately to prevent "Zombie Work".

---

## 6. Lessons Learned (Anti-Patterns)

These rules exist because agents closed beads that were NOT actually complete. Do not repeat these mistakes.

### A. "Code Exists ≠ Working"

**Failure Mode**: Agent created a file and closed the bead without running it.

**Rule**: Before closing a bead, you MUST execute the verification step. If verification says "Run X and assert Y", you must literally run X and check Y. If verification is missing from the bead, add one before proceeding.

### B. "Script Exists ≠ Artifacts Generated"

**Failure Mode**: Agent created `update-ui.sh` to generate GIFs but never ran it. GIFs were never created. Bead was closed anyway.

**Rule**: For artifact-generating tasks, the artifact MUST exist in the repo before closing. Check:

- `git status` shows the expected files
- Files are non-empty
- Files are staged or committed

### C. "Test File ≠ Passing Tests"

**Failure Mode**: Agent created `main_test.go` but tests were never actually run (or they fail).

**Rule**: Before closing any "Test:" prefixed bead, run `go test ./path/...` and verify exit code 0.

### D. "Pre-commit Hook ≠ Enforced"

**Failure Mode**: Agent updated pre-commit hook but `git config core.hooksPath` was never set, so the hook doesn't run.

**Rule**: After editing hooks, verify they trigger by making a test commit or checking `git config --get core.hooksPath`.

---

## 7. Closing Checklist

Before running `bd close <id>`, verify ALL apply:

- [ ] **Verification Command**: Ran the exact command in the bead's "Verification" section.
- [ ] **Artifacts Exist**: All files mentioned in the bead exist and are non-empty.
- [ ] **Tests Pass**: If the bead involves code, `go test` (or equivalent) passes.
- [ ] **Committed**: Changes are committed (not just staged).
- [ ] **Pushed**: `git push` succeeded. Work is on remote.

---

_Reference: This document exists because bad instructions lead to bad code. Be specific. Be safe._
