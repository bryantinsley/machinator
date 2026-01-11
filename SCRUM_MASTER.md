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

## 3. The Pre-Mortem Checklist

Before you execute `bd create` or `bd update`, ask yourself:

1.  **Ambiguity Check**: "If I say 'refactor', will they delete the whole file?" (Specify "Refactor by Addition").
2.  **Context Rot**: "Does this bead rely on a file that might move?" (Use relative paths or search).
3.  **Token Burn**: "Could this loop indefinitely?" (Add timeouts or mock requirements).

## 4. Bead Etiquette

- **Granularity**: 2-10 minute tasks. If it takes >15 mins, split it.
- **Linking**: ALWAYS explicitly link Dependencies (`bd dep add`). A floaty bead is a lost bead.
- **Cleanup**: If a plan changes, delete the obsolete beads immediately to prevent "Zombie Work".

---

_Reference: This document exists because bad instructions lead to bad code. Be specific. Be safe._
