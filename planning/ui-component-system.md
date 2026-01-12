# UI Component System Design

_Ensuring everything is mouse-clickable and consistently styled_

**Status**: 🔴 Not Started  
**Last Updated**: January 2026

---

## 1. The Problem

The TUI has inconsistent interactivity:

- Some elements respond to mouse clicks, others don't
- No standard component library
- Developers must manually implement click handling each time
- Easy to forget or do it wrong

## 2. The Solution: Reusable Component Library

Create `orchestrator/pkg/ui/components/` with standardized components that:

- **Always** handle mouse events
- **Always** use consistent styling
- Are **easy to use** - if you use the component, it just works

### Core Components Needed

| Component  | Behavior                  | Mouse Events                  |
| ---------- | ------------------------- | ----------------------------- |
| `Button`   | Clickable action trigger  | Click → OnClick callback      |
| `ListItem` | Selectable item in a list | Click → Select item           |
| `Table`    | Rows/cells with selection | Click row → Select row        |
| `Panel`    | Container with title      | Click title → Collapse/expand |
| `Tab`      | Tab in a tab bar          | Click → Switch tab            |
| `Input`    | Text input field          | Click → Focus                 |
| `Checkbox` | Toggle boolean            | Click → Toggle                |
| `Link`     | Actionable text           | Click → OnClick callback      |

### Component Interface

```go
// Clickable is implemented by all interactive components
type Clickable interface {
    // Contains returns true if the point is within the component's bounds
    Contains(x, y int) bool

    // HandleClick processes a click at the given position
    HandleClick(x, y int) tea.Cmd

    // Bounds returns the component's screen position
    Bounds() (x, y, width, height int)
}

// Example: Button
type Button struct {
    Label    string
    OnClick  func() tea.Cmd
    x, y     int  // Position (set during render)
    width    int
    focused  bool
    style    lipgloss.Style
}

func (b *Button) Contains(x, y int) bool {
    return x >= b.x && x < b.x+b.width && y == b.y
}

func (b *Button) HandleClick(x, y int) tea.Cmd {
    if b.OnClick != nil {
        return b.OnClick()
    }
    return nil
}
```

### Click Dispatcher

A central dispatcher routes mouse events to the right component:

```go
type ClickDispatcher struct {
    components []Clickable
}

func (d *ClickDispatcher) HandleMouse(msg tea.MouseMsg) tea.Cmd {
    if msg.Action != tea.MouseActionRelease {
        return nil
    }
    for _, c := range d.components {
        if c.Contains(msg.X, msg.Y) {
            return c.HandleClick(msg.X, msg.Y)
        }
    }
    return nil
}
```

## 3. Migration Path

### Phase 1: Create Component Library

- [ ] Create `orchestrator/pkg/ui/components/` package
- [ ] Implement Button component with tests
- [ ] Implement ListItem component with tests
- [ ] Create ClickDispatcher

### Phase 2: Migrate Setup TUI

- [ ] Identify all clickable elements in Setup
- [ ] Replace with component library
- [ ] Add tests verifying click behavior
- [ ] Update golden files

### Phase 3: Migrate Orchestrator TUI

- [ ] Identify all clickable elements in Orchestrator
- [ ] Replace with component library
- [ ] Add tests verifying click behavior
- [ ] Update golden files

### Phase 4: Verification & Governance

- [ ] Create E2E test that clicks every visible element
- [ ] Add linting rule: "All interactive elements must use component library"
- [ ] Update AGENTS.md with component usage guidelines

## 4. Styling Consistency

All components use a shared style system:

```go
// pkg/ui/styles/styles.go
var (
    ButtonStyle = lipgloss.NewStyle().
        Bold(true).
        Padding(0, 2).
        Background(lipgloss.Color("62")).
        Foreground(lipgloss.Color("230"))

    ButtonFocusedStyle = ButtonStyle.Copy().
        Background(lipgloss.Color("205"))

    // ... etc
)
```

## 5. Success Criteria

- [ ] Every interactive element uses a component from the library
- [ ] Every component responds to mouse clicks
- [ ] Consistent visual styling across all screens
- [ ] E2E test verifies all elements are clickable
- [ ] New developers can easily use components correctly

## 6. Governance

After implementation, these rules apply:

1. **Never** create ad-hoc clickable elements - always use components
2. **All** new screens must use the component library
3. **VHS tests** must demonstrate mouse interaction
4. **Code review** checks for component usage

---

## Tracking

This document tracks the theme. Implementation tasks should be created as beads once Phase 1 is started.

| Phase                           | Status         | Notes |
| ------------------------------- | -------------- | ----- |
| Phase 1: Component Library      | 🔴 Not Started |       |
| Phase 2: Setup Migration        | 🔴 Not Started |       |
| Phase 3: Orchestrator Migration | 🔴 Not Started |       |
| Phase 4: Verification           | 🔴 Not Started |       |
