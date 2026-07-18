package tuigo

import (
	"testing"

	"github.com/wildneuro/tuigo/layout"
	"github.com/wildneuro/tuigo/renderer"
	"github.com/wildneuro/tuigo/types"
)

// fillPane is a Canvas that paints every cell of its rect with r — a stand-in
// for a TerminalPane repainting its Screen every frame.
func fillPane(r rune) Element {
	return Element{Type: types.ElementTypeCanvas, Paint: func(s types.Surface) {
		x, y, w, h := s.Bounds()
		for j := 0; j < h; j++ {
			for i := 0; i < w; i++ {
				s.Set(x+i, y+j, r, 0, 0, false, false, false)
			}
		}
	}}
}

// TestDialogOverPaneCompositesAndRestores is FIX 3's proof: a dialog stamped on
// top of a pane wins where they overlap and the pane shows elsewhere; the next
// frame WITHOUT the dialog repaints the pane from scratch and the diff restores
// every previously-covered cell — no corruption. This mirrors exactly what the
// render loop does (Draw the tree, Stamp overlays, Diff against the prev frame).
func TestDialogOverPaneCompositesAndRestores(t *testing.T) {
	const W, H = 20, 8
	pane := fillPane('P')
	node := layout.Layout(pane, W, H)

	// --- Frame 1: dialog shown over the pane -----------------------------
	frame1 := renderer.Draw(node, W, H)

	// A 6x3 dialog full of 'D', stamped at (4, 2).
	const ow, oh, ox, oy = 6, 3, 4, 2
	dialog := renderer.NewBuffer(ow, oh)
	for j := 0; j < oh; j++ {
		for i := 0; i < ow; i++ {
			dialog.Set(i, j, renderer.Cell{Rune: 'D'})
		}
	}
	renderer.Stamp(frame1, dialog, ox, oy)

	inDialog := func(x, y int) bool {
		return x >= ox && x < ox+ow && y >= oy && y < oy+oh
	}

	// The dialog cells win where they overlap; the pane shows everywhere else.
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			want := 'P'
			if inDialog(x, y) {
				want = 'D'
			}
			if got := frame1.At(x, y).Rune; got != want {
				t.Fatalf("frame1 (%d,%d): got %q want %q", x, y, got, want)
			}
		}
	}

	// --- Frame 2: dialog closed — pane repaints, diff restores -----------
	frame2 := renderer.Draw(node, W, H) // no Stamp this frame

	// Every cell is 'P' again (the pane repainted its whole region).
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			if got := frame2.At(x, y).Rune; got != 'P' {
				t.Fatalf("frame2 (%d,%d): pane not restored, got %q", x, y, got)
			}
		}
	}

	// The diff that the terminal would flush restores EXACTLY the dialog
	// region (the only cells that changed) back to 'P' — no stray artifacts.
	patches := renderer.Diff(frame1, frame2)
	if len(patches) != ow*oh {
		t.Fatalf("expected %d restore patches (the dialog region), got %d", ow*oh, len(patches))
	}
	for _, p := range patches {
		if !inDialog(p.X, p.Y) {
			t.Fatalf("restore patch outside the dialog region at (%d,%d) — artifact", p.X, p.Y)
		}
		if p.Cell.Rune != 'P' {
			t.Fatalf("restore patch at (%d,%d) is %q, want the pane cell 'P'", p.X, p.Y, p.Cell.Rune)
		}
	}
}
