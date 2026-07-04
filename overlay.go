package tuigo

// Overlay is one floating panel — a Picture-in-Picture widget, a toast, a
// popup — composited on top of the main frame at a fixed screen position.
// Element must carry explicit Width/Height (e.g. via the Width/Height
// options) since it's laid out and drawn independently of the main tree,
// with no parent to size it.
//
// tuigo has no z-order/absolute-positioning support in the layout tree
// itself (see ARCHITECTURE.md) — Overlay is the escape hatch: it renders
// its own small Element tree into its own Buffer, then stamps that buffer
// onto the main frame after the main Draw call and before the cell diff,
// so only the overlay's actually-changed cells get re-flushed like
// anything else. It does not participate in flow layout, hit-testing, or
// focus; call OnClick/OnScroll on its own Element if it needs input.
type Overlay struct {
	Element Element
	X, Y    int
}

// Overlay registers ov to be drawn on top of the main frame for this
// render. Call it from within Component — like UseState, call it
// unconditionally on every render you want the overlay visible; omit the
// call on renders where it should disappear.
func (c *Ctx) Overlay(el Element, x, y int) {
	c.app.pendingOverlays = append(c.app.pendingOverlays, Overlay{Element: el, X: x, Y: y})
}
