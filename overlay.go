package tuigo

import "github.com/wildneuro/tuigo/renderer"

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

// DimBackdrop darkens every cell already painted into buf, in place, so a
// floating Ctx.Overlay panel reads as a window over dimmed content rather
// than blending into whatever the main frame drew underneath it. Call it on
// the frame Buffer BEFORE stamping the opaque panel on top (it has no
// knowledge of panel bounds — dim the whole buffer, or a sub-region you've
// already isolated, then Stamp your panel over the dimmed result).
//
// It repaints every cell's Fg/Bg to the given near-black colors and clears
// Bold/Italic/Underline, while leaving each cell's Rune untouched — the
// shape of the backdrop still shows through, just dimmed, the same
// shadow-over-content look tldrq's chat/panel overlays hand-rolled before
// Overlay existed. Pick fg/bg once per app (e.g. via RGB(0x33,0x33,0x33) and
// RGB(0x03,0x03,0x03)) and reuse them for every dimmed overlay.
func DimBackdrop(buf *renderer.Buffer, fg, bg Color) {
	if buf == nil {
		return
	}
	for i := range buf.Cells {
		c := &buf.Cells[i]
		c.Fg = fg
		c.Bg = bg
		c.Bold = false
		c.Italic = false
		c.Underline = false
	}
}
