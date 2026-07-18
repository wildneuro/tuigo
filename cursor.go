package tuigo

// cursor.go — the REAL hardware cursor for the focused TerminalPane.
//
// tuigo hides the terminal cursor globally (renderer enterSeq emits ?25l) and
// draws the whole frame into a cell buffer. For an embedded agent that's not
// enough: its cursor should be the real terminal cursor, blinking at the
// child's cursor cell, so line-editing and shape feel native.
//
// The mechanism is a per-frame slot on appState. Each frame the render loop
// clears it, then the focused pane's Paint publishes its child cursor
// (absolute cell = pane origin + the child's Screen cursor) via
// publishPaneCursor. After compositing, the loop resolves the final cursor
// with computeCursor (which hides it when an overlay/dialog covers the cell)
// and hands it to Terminal.SetCursor, which un-hides + positions the real
// cursor when visible and hides it otherwise. An unfocused pane, or a focused
// non-pane element, publishes nothing, so the cursor stays hidden.
//
// vt10x models the cursor position + visibility but not its SHAPE (block/bar/
// underline — vt10x.Cursor has no style field), so the shape is left as the
// terminal default; positioning + visibility is what matters for an embedded
// agent.

import "github.com/wildneuro/tuigo/layout"

// publishPaneCursor records a focused TerminalPane's hardware-cursor request
// into the frame's cursor slot: the absolute cell is the pane's origin plus the
// child's local cursor. It no-ops when the pane is unfocused, the child's
// cursor is hidden, or the local cursor falls outside the pane box — so the
// only way the cursor shows is a focused pane with a live, in-bounds cursor.
func publishPaneCursor(app *appState, focused bool, originX, originY, w, h, localX, localY int, childCursorVisible bool) {
	if app == nil || !focused || !childCursorVisible {
		return
	}
	if localX < 0 || localX >= w || localY < 0 || localY >= h {
		return
	}
	app.cursorVisible = true
	app.cursorX = originX + localX
	app.cursorY = originY + localY
}

// computeCursor resolves the hardware cursor for a frame: the focused pane's
// requested cell, unless an overlay rect covers it. A dialog drawn on top of a
// pane must not let the child's cursor poke through it, so a covered cursor is
// hidden; when the overlay closes next frame the pane republishes and the
// cursor returns.
func computeCursor(x, y int, visible bool, overlays []layout.Rect) (int, int, bool) {
	if !visible {
		return 0, 0, false
	}
	for _, r := range overlays {
		if x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H {
			return 0, 0, false
		}
	}
	return x, y, true
}
