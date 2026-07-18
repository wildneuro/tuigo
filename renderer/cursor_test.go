package renderer

import (
	"strings"
	"testing"
)

// TestCursorSeq pins the hardware-cursor escape state machine (cursorSeq, the
// pure core of Terminal.SetCursor): first visible frame moves + un-hides; a
// later visible frame at a new cell repositions with no extra show; hiding
// emits exactly one ?25l; and an idle hidden frame emits nothing (no flicker).
func TestCursorSeq(t *testing.T) {
	// First visible frame: move to (col 5, row 2) -> "\x1b[3;6H" + show.
	seq, shown := cursorSeq(5, 2, true, false)
	if !shown || !strings.Contains(seq, "\x1b[3;6H") || !strings.Contains(seq, "\x1b[?25h") {
		t.Fatalf("first visible frame must move+show: seq=%q shown=%v", seq, shown)
	}

	// Second visible frame at a new cell: reposition only, no extra ?25h.
	seq, shown = cursorSeq(7, 2, true, true)
	if !shown || !strings.Contains(seq, "\x1b[3;8H") || strings.Contains(seq, "\x1b[?25h") {
		t.Fatalf("second visible frame must move only: seq=%q shown=%v", seq, shown)
	}

	// Visible -> hidden: exactly one ?25l.
	seq, shown = cursorSeq(0, 0, false, true)
	if shown || seq != "\x1b[?25l" {
		t.Fatalf("hide frame must emit exactly one ?25l: seq=%q shown=%v", seq, shown)
	}

	// Idle hidden frame: nothing.
	seq, shown = cursorSeq(0, 0, false, false)
	if shown || seq != "" {
		t.Fatalf("idle hidden frame must emit nothing: seq=%q shown=%v", seq, shown)
	}
}
