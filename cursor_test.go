package tuigo

import (
	"testing"

	"github.com/wildneuro/tuigo/layout"
)

// TestComputeCursorVisibility pins FIX 2's cursor-visibility decision: a
// focused pane's published cursor shows at its computed absolute cell; an
// unpublished cursor (unfocused / non-pane focused) is hidden; and an overlay
// covering the cell hides it (a dialog must not be pierced by the child cursor).
func TestComputeCursorVisibility(t *testing.T) {
	// Focused pane published a cursor at (5, 2), no overlays: shown there.
	if x, y, vis := computeCursor(5, 2, true, nil); !vis || x != 5 || y != 2 {
		t.Fatalf("focused pane cursor: got (%d,%d) vis=%v want (5,2) vis=true", x, y, vis)
	}

	// Nothing published (unfocused / non-pane focused): hidden.
	if _, _, vis := computeCursor(0, 0, false, nil); vis {
		t.Fatal("no published cursor should be hidden")
	}

	// Cursor cell covered by an overlay (dialog over the pane): hidden.
	overlay := []layout.Rect{{X: 3, Y: 1, W: 6, H: 4}} // covers (5,2)
	if _, _, vis := computeCursor(5, 2, true, overlay); vis {
		t.Fatal("cursor under an overlay should be hidden")
	}

	// Published cursor OUTSIDE the overlay: still shown.
	if x, y, vis := computeCursor(1, 0, true, overlay); !vis || x != 1 || y != 0 {
		t.Fatalf("cursor outside overlay: got (%d,%d) vis=%v want (1,0) vis=true", x, y, vis)
	}
}
