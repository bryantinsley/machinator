---
description: UI consistency checklist for TUI screens
---

# UI Review Checklist

Use this checklist when adding or modifying TUI screens to ensure consistency.

## Buttons

- [ ] Buttons are styled with `Background()` and `Foreground()` colors
- [ ] Buttons are **gray** when not selected
- [ ] Buttons change to their action color when selected (green=add, red=delete, blue=back/cancel)
- [ ] Button text stays **exactly the same** regardless of selection state
- [ ] No cursor icons (▸) on buttons - color is the only indicator
- [ ] No bold/unbold changes on buttons
- [ ] Buttons are **mouse clickable** via `handleMouse()`

## Navigation

- [ ] No navigation hints (no "↑↓ navigate • Enter select")
- [ ] Up/down (or k/j) moves cursor
- [ ] Enter activates selected item
- [ ] Esc/q goes back
- [ ] Layout stays **stable** when cursor moves (no jitter)

## Destructive Actions

- [ ] Delete operations show a **confirmation dialog**
- [ ] Confirmation uses Y/N keys
- [ ] Confirmation clearly states what will be deleted

## Text Input

- [ ] Simple underline style (no box borders)
- [ ] Blinking cursor block (█) at end
- [ ] No "Enter to save • Esc to cancel" hints
- [ ] Save and Cancel buttons below input

## Layout

- [ ] Consistent indentation (2 spaces for cursor column)
- [ ] Labels and values aligned properly
- [ ] Long values (like URLs) on their own indented line
- [ ] Beads/stats in grid format

## Screen-Specific Checks

### Main Screen

- [ ] Gemini CLI status at top
- [ ] Projects listed with `#ID) Name` format
- [ ] Add Project and Exit as styled buttons at bottom
- [ ] Both buttons mouse clickable

### Project Detail Screen

- [ ] Project name as title
- [ ] Name, Repository, Agents as selectable items
- [ ] Beads stats in grid (not selectable)
- [ ] Delete and Back as styled buttons
- [ ] Delete goes to confirmation, not immediate delete

### Edit Screens (Name, Repo, Agents)

- [ ] Clean text input or number display
- [ ] Save/Cancel buttons
- [ ] Warning text for destructive changes (repo change)
