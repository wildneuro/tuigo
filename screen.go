package tuigo

// screen.go — "pages": save and restore the VISIBLE SCREEN behind a dialog,
// the way tmux popups (capture-pane) do. A host that already proxies a
// full-screen child's output (e.g. tldrq's PTY shell wrapper) feeds that byte
// stream into a Screen; before showing a dialog it PushPage()s the current
// visible screen, and after the dialog closes it PopPage(w)s to repaint the
// child's exact page — no reliance on the child redrawing, no alt-screen
// guesswork.
//
// A "page" is one snapshot of the visible viewport (its cell grid + cursor).
// PushPage / PopPage form a STACK, so a dialog-over-a-dialog nests cleanly:
// push, push, pop, pop restores each layer in turn. The lower-level
// Snapshot() / Restore(w) pair is kept for callers that want a page value
// they manage themselves.
//
// Rather than hand-roll a VT parser, Screen WRAPS github.com/hinshun/vt10x —
// a pure-Go (CGO-free) VT10x/xterm emulator with a cell grid + cursor,
// purpose-built for programmatic terminal capture (the go-to for expect/pty
// tools). vt10x handles the full escape-sequence long tail; Screen adds the
// pages concept and a faithful grid → SGR repaint. Restoring the fixed
// rows×cols viewport is BOUNDED work (O(cells)) and never pollutes scrollback,
// unlike replaying the child's raw output stream.

import (
	"fmt"
	"io"
	"strings"

	"github.com/hinshun/vt10x"
)

// scolor is a screen-cell color: unset (terminal default) unless set, then
// either a 256-palette index or 24-bit RGB. Kept independent of vt10x.Color so
// the ambiguous zero (palette index 0 vs. "unset") never bites us.
type scolor struct {
	set     bool
	rgb     bool
	idx     int  // palette index 0..255 when !rgb
	r, g, b byte // RGB channels when rgb
}

// screenCell is one grid position: its rune plus the SGR attributes we can
// faithfully repaint. (vt10x models bold/italic/underline/reverse; it has no
// separate "dim", so Screen doesn't carry one.) Comparable, so grid equality
// is a plain ==.
type screenCell struct {
	Rune                             rune
	Fg, Bg                           scolor
	Bold, Italic, Underline, Reverse bool
}

// vt10x Glyph.Mode attribute bits (mirrors vt10x/state.go's private consts;
// re-declared here because they are unexported in the library).
const (
	vtAttrReverse   int16 = 1 << 0
	vtAttrUnderline int16 = 1 << 1
	vtAttrBold      int16 = 1 << 2
	// 1<<3 attrGfx, 1<<5 attrBlink, 1<<6 attrWrap — not repainted.
	vtAttrItalic int16 = 1 << 4
)

// Screen is a cell-grid terminal emulator (backed by vt10x) plus a page stack.
// It ingests a terminal output byte stream via Write (io.Writer) and can save
// the visible screen onto a page stack for later restore. It is not safe for
// concurrent use; the host must serialise Write against the page/snapshot
// calls.
type Screen struct {
	rows, cols int
	term       vt10x.Terminal
	pages      []*ScreenSnapshot
	utail      []byte // held-back trailing bytes of an incomplete UTF-8 rune
}

// NewScreen creates a rows×cols emulator with a blank screen and an empty page
// stack.
func NewScreen(rows, cols int) *Screen {
	if rows < 1 {
		rows = 1
	}
	if cols < 1 {
		cols = 1
	}
	return &Screen{
		rows: rows,
		cols: cols,
		// vt10x takes (cols, rows).
		term: vt10x.New(vt10x.WithSize(cols, rows)),
	}
}

// Write ingests a terminal output byte stream, updating the emulated screen.
// A multibyte UTF-8 rune split across two Writes (e.g. at a PTY read-buffer
// boundary) would otherwise be dropped by vt10x, so an incomplete trailing
// rune is held back and prepended to the next Write. All input bytes are
// accepted, so the reported count is always len(p).
func (s *Screen) Write(p []byte) (int, error) {
	n := len(p)
	if len(s.utail) > 0 {
		combined := make([]byte, 0, len(s.utail)+len(p))
		combined = append(combined, s.utail...)
		combined = append(combined, p...)
		p = combined
		s.utail = s.utail[:0]
	}
	if tail := incompleteUTF8Tail(p); tail > 0 {
		s.utail = append(s.utail[:0], p[len(p)-tail:]...)
		p = p[:len(p)-tail]
	}
	_, err := s.term.Write(p)
	return n, err
}

// incompleteUTF8Tail returns the number of trailing bytes that form an
// incomplete UTF-8 sequence (a lead byte without all its continuation bytes),
// or 0 when the input ends on a rune boundary.
func incompleteUTF8Tail(b []byte) int {
	for i := len(b) - 1; i >= 0 && i >= len(b)-3; i-- {
		c := b[i]
		if c < 0x80 {
			return 0 // ASCII — complete boundary
		}
		if c&0xC0 == 0x80 {
			continue // continuation byte — keep scanning back for the lead
		}
		var need int
		switch {
		case c&0xE0 == 0xC0:
			need = 2
		case c&0xF0 == 0xE0:
			need = 3
		case c&0xF8 == 0xF0:
			need = 4
		default:
			return 0 // invalid lead — let vt10x deal with it
		}
		if avail := len(b) - i; avail < need {
			return avail
		}
		return 0
	}
	return 0
}

// Resize changes the emulated screen to rows×cols.
func (s *Screen) Resize(rows, cols int) {
	if rows < 1 {
		rows = 1
	}
	if cols < 1 {
		cols = 1
	}
	if rows == s.rows && cols == s.cols {
		return
	}
	s.rows, s.cols = rows, cols
	s.term.Resize(cols, rows) // vt10x takes (cols, rows)
}

// ---- pages: the primary API ---------------------------------------------

// PushPage saves the current visible screen onto the page stack. Pair it with
// PopPage to repaint that exact page after a dialog closes; nested dialogs push
// again and pop in reverse order.
func (s *Screen) PushPage() {
	s.pages = append(s.pages, s.Snapshot())
}

// PopPage repaints the top saved page to w and pops it off the stack. It
// returns an error if the stack is empty (nothing to restore) or the write
// fails. The saved page — not the current live screen — is what gets
// repainted, so a dialog drawn over the child leaves no trace.
func (s *Screen) PopPage(w io.Writer) error {
	n := len(s.pages)
	if n == 0 {
		return fmt.Errorf("tuigo: PopPage on empty page stack")
	}
	top := s.pages[n-1]
	s.pages = s.pages[:n-1]
	return top.Restore(w)
}

// PageDepth reports how many pages are currently stacked (unclosed PushPage
// calls). Useful for asserting balanced push/pop.
func (s *Screen) PageDepth() int { return len(s.pages) }

// ---- snapshot / restore: the lower-level pair ---------------------------

// ScreenSnapshot is an immutable copy of a Screen's visible viewport (cell grid
// + cursor), produced by Snapshot / PushPage and repainted by Restore /
// PopPage.
type ScreenSnapshot struct {
	rows, cols   int
	cells        []screenCell
	cx, cy       int
	cursorHidden bool
}

// Snapshot copies the current visible screen — every cell, the cursor
// position, and cursor visibility — into a standalone page value that Restore
// can repaint later.
func (s *Screen) Snapshot() *ScreenSnapshot {
	cols, rows := s.term.Size()
	cells := make([]screenCell, rows*cols)
	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			cells[y*cols+x] = fromGlyph(s.term.Cell(x, y))
		}
	}
	cur := s.term.Cursor()
	return &ScreenSnapshot{
		rows:         rows,
		cols:         cols,
		cells:        cells,
		cx:           clamp(cur.X, 0, cols-1),
		cy:           clamp(cur.Y, 0, rows-1),
		cursorHidden: !s.term.CursorVisible(),
	}
}

// Restore is a convenience that snapshots the current screen and repaints it to
// w.
func (s *Screen) Restore(w io.Writer) error { return s.Snapshot().Restore(w) }

// Restore repaints the whole page to w: hide cursor, reset SGR, clear screen,
// then for every cell emit its rune with the right SGR (re-emitting a style
// only when it changes), and finally leave the cursor exactly where the
// snapshot had it. Feeding Restore's output into a fresh NewScreen of the same
// size reproduces an identical grid (round-trip).
func (snap *ScreenSnapshot) Restore(w io.Writer) error {
	var b strings.Builder
	b.WriteString("\x1b[?25l") // hide cursor while repainting
	b.WriteString("\x1b[0m")   // reset pen
	b.WriteString("\x1b[H")    // home
	b.WriteString("\x1b[2J")   // clear screen

	var cur screenCell
	haveStyle := false
	for y := 0; y < snap.rows; y++ {
		fmt.Fprintf(&b, "\x1b[%d;1H", y+1)
		for x := 0; x < snap.cols; x++ {
			c := snap.cells[y*snap.cols+x]
			st := c
			st.Rune = 0
			if !haveStyle || st != cur {
				writeScreenSGR(&b, c)
				cur = st
				haveStyle = true
			}
			r := c.Rune
			if r == 0 {
				r = ' '
			}
			b.WriteRune(r)
		}
	}

	// Park the cursor where the snapshot had it, pen reset.
	b.WriteString("\x1b[0m")
	fmt.Fprintf(&b, "\x1b[%d;%dH", snap.cy+1, snap.cx+1)
	if !snap.cursorHidden {
		b.WriteString("\x1b[?25h")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// ---- vt10x → screenCell mapping -----------------------------------------

// fromGlyph maps a vt10x Glyph into a screenCell. vt10x has already resolved
// reverse (it swaps FG/BG in the stored glyph) and bold-brightening, so we take
// the stored colors as-is and only re-emit reverse in the one case a plain
// color swap can't express: reverse over the default fg/bg (a common status-bar
// idiom).
func fromGlyph(g vt10x.Glyph) screenCell {
	fg := fromVTColor(g.FG)
	bg := fromVTColor(g.BG)
	r := g.Char
	if r == 0 {
		r = ' '
	}
	c := screenCell{
		Rune:      r,
		Fg:        fg,
		Bg:        bg,
		Bold:      g.Mode&vtAttrBold != 0,
		Italic:    g.Mode&vtAttrItalic != 0,
		Underline: g.Mode&vtAttrUnderline != 0,
	}
	if g.Mode&vtAttrReverse != 0 && !fg.set && !bg.set {
		c.Reverse = true
	}
	return c
}

// fromVTColor maps a vt10x.Color to a scolor. vt10x encodes defaults at
// >=1<<24 (DefaultFG/BG/Cursor), the 256-palette in [0,256), and 24-bit
// truecolor packed as r<<16|g<<8|b in [256,1<<24) — the standard reading used
// by vt10x's own renderers.
func fromVTColor(c vt10x.Color) scolor {
	switch {
	case uint32(c) >= 1<<24: // DefaultFG / DefaultBG / DefaultCursor
		return scolor{}
	case uint32(c) < 256:
		return scolor{set: true, idx: int(c)}
	default:
		v := uint32(c)
		return scolor{set: true, rgb: true, r: byte(v >> 16), g: byte(v >> 8), b: byte(v)}
	}
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// writeScreenSGR emits a full SGR reset followed by the cell's attributes and
// colors — the same reset-then-apply shape the tuigo renderer uses.
func writeScreenSGR(b *strings.Builder, c screenCell) {
	b.WriteString("\x1b[0m")
	if c.Bold {
		b.WriteString("\x1b[1m")
	}
	if c.Italic {
		b.WriteString("\x1b[3m")
	}
	if c.Underline {
		b.WriteString("\x1b[4m")
	}
	if c.Reverse {
		b.WriteString("\x1b[7m")
	}
	if c.Fg.set {
		if c.Fg.rgb {
			fmt.Fprintf(b, "\x1b[38;2;%d;%d;%dm", c.Fg.r, c.Fg.g, c.Fg.b)
		} else {
			fmt.Fprintf(b, "\x1b[38;5;%dm", c.Fg.idx)
		}
	}
	if c.Bg.set {
		if c.Bg.rgb {
			fmt.Fprintf(b, "\x1b[48;2;%d;%d;%dm", c.Bg.r, c.Bg.g, c.Bg.b)
		} else {
			fmt.Fprintf(b, "\x1b[48;5;%dm", c.Bg.idx)
		}
	}
}
