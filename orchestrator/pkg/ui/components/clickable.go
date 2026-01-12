package components

import tea "github.com/charmbracelet/bubbletea"

// Clickable is implemented by all interactive components
type Clickable interface {
	// Contains returns true if the point is within the component's bounds
	Contains(x, y int) bool

	// HandleClick processes a click at the given position
	HandleClick(x, y int) tea.Cmd

	// Bounds returns the component's screen position
	Bounds() (x, y, width, height int)

	// SetBounds sets the component's screen position
	SetBounds(x, y, width, height int)
}

// ClickDispatcher routes mouse events to the right component
type ClickDispatcher struct {
	components []Clickable
}

// NewClickDispatcher creates a new dispatcher with the given components
func NewClickDispatcher(components []Clickable) *ClickDispatcher {
	return &ClickDispatcher{
		components: components,
	}
}

// Register adds a component to the dispatcher
func (d *ClickDispatcher) Register(c Clickable) {
	d.components = append(d.components, c)
}

// Clear removes all registered components
func (d *ClickDispatcher) Clear() {
	d.components = []Clickable{}
}

// HandleMouse processes a mouse message and delegates to the appropriate component
func (d *ClickDispatcher) HandleMouse(msg tea.MouseMsg) tea.Cmd {
	if msg.Action != tea.MouseActionRelease {
		return nil
	}
	// Iterate in reverse order to handle z-index (topmost first) effectively
	// though for TUI usually last rendered is top, so reverse might be better if they overlap.
	// But let's stick to simple iteration for now or reverse if we assume render order.
	// Typically in TUI, later draw calls overlap earlier ones.
	for i := len(d.components) - 1; i >= 0; i-- {
		c := d.components[i]
		if c.Contains(msg.X, msg.Y) {
			return c.HandleClick(msg.X, msg.Y)
		}
	}
	return nil
}
