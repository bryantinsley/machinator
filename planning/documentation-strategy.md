# Documentation Strategy

_Making Machinator self-explanatory_

**Status**: 🔴 Planning  
**Last Updated**: January 2026

---

## 1. The North Star

**The ultimate quickstart guide should be one command:**

```bash
bazel run //:machinator
```

That's it. No setup guides. No prerequisites lists. No configuration walkthroughs.

When you run Machinator for the first time, it should:

1. Detect you're new (no ~/.machinator)
2. Walk you through authentication (Gemini API key or Google auth)
3. Help you pick/create a project
4. Start orchestrating

**If the user needs to read documentation to get started, the UX has failed.**

The experience should be:

- **Intuitive** — obvious what to do next
- **Discoverable** — help is available in-context, not in docs
- **Progressive** — basic usage requires nothing, power features are discoverable
- **No demos** — the real product IS the demo

---

## 2. Documentation Structure

### README.md — The Front Door

The README should be:

1. **Hook** — What is this? Why should I care? (2-3 sentences)
2. **Story** — How it was built (the bootstrapping journey)
3. **Quick Start** — Single command: `bazel run //:machinator`
4. **Visual proof** — GIFs showing it in action
5. **What's Next** — Links to deeper docs if curious

**What the README is NOT:**

- Not a complete user guide
- Not a reference manual
- Not a troubleshooting guide

### docs/ — The Documentation Site

```
docs/
├── concepts/                 # Understanding the system
│   ├── how-it-works.md       # Architecture overview
│   ├── beads-integration.md  # How Machinator uses Beads
│   └── multi-agent.md        # Multi-agent coordination
│
├── guides/                   # Task-oriented walkthroughs
│   ├── first-project.md      # Your first orchestrated project
│   ├── adding-accounts.md    # Setting up multiple Gemini accounts
│   └── customizing.md        # Tweaking behavior
│
├── reference/                # Lookup-oriented content
│   ├── config.md             # Configuration options
│   ├── keyboard.md           # Keyboard shortcuts
│   └── templates.md          # Directive template syntax
│
├── troubleshooting/          # Problem-oriented content
│   ├── quota-issues.md       # When quota runs out
│   └── agent-stuck.md        # When agents aren't progressing
│
└── ui-history/               # Visual artifacts
    ├── crud.gif
    └── navigation.gif
```

### AGENTS.md — For the AI

This stays focused on rules AI agents need to follow. It's not for humans learning the system.

---

## 3. Content Principles

### Write for Scanners

- Bold important terms
- Use bullet points over paragraphs
- Put the answer first, explanation second
- One idea per section

### Show, Don't Tell

- Every concept needs an example
- Prefer GIFs/screenshots over descriptions
- Code blocks over prose

### Progressive Disclosure

- Start with the simplest case
- Reveal complexity only when needed
- "Want more? See X" links

### Use the Product's Language

- Use terms consistently
- Name things once, name things well
- If you need to explain a name, rename it

---

## 4. Naming Improvements (TBD)

Current names worth revisiting:

- "Unblocking mode" — What do users call this?
- "Directive template" — "Agent instructions"?
- "Beads" — Always explain as "task/issue tracker" on first use
- "Account pooling" — "Multi-account rotation"?

---

## 5. UX Improvements for Self-Explanation

### First-Run Experience

- [ ] Detect first run (no ~/.machinator)
- [ ] Friendly welcome screen in TUI
- [ ] Guided setup flow (API key → project → go)
- [ ] No reading required

### In-App Help

- [ ] `?` key shows contextual help
- [ ] Status bar shows current state
- [ ] Error messages include "what to do next"

### Onboarding Flow

- [ ] Explain what's happening as it happens
- [ ] "Setting up your first project..."
- [ ] "Looking for ready tasks..."
- [ ] "Starting agent with Flash model..."

---

## 6. Migration Plan

### Phase 1: Capture North Star

- [x] Create this document
- [ ] Add north star to architecture-vision.md
- [ ] Create bazel //:machinator alias

### Phase 2: Restructure README

- [ ] Slim down to hook + story + quick start
- [ ] Move technical details to docs/
- [ ] Add compelling GIFs

### Phase 3: Create docs/ Structure

- [ ] Create directory structure
- [ ] Move content from README to appropriate places
- [ ] Write missing concept docs

### Phase 4: UX Improvements

- [ ] Implement first-run detection
- [ ] Add in-app help
- [ ] Improve error messages

### Phase 5: Naming Audit

- [ ] Review all user-facing terms
- [ ] Pick better names where needed
- [ ] Update consistently everywhere

---

## 7. Success Metrics

How we know we've succeeded:

1. **Zero questions before running** — New user can start with `bazel run //:machinator`
2. **10-second value** — Within 10 seconds of running, user understands what it does
3. **Self-service troubleshooting** — Error messages tell you what to do
4. **No "see documentation"** — The TUI itself is the documentation
