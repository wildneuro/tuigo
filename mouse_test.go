package tuigo

import (
	"testing"

	"github.com/wildneuro/tuigo/types"
)

// TestEncodeSGRMouse pins encodeSGRMouse against known SGR wire strings for a
// table of {button, action, col, row} inputs — the inverse of what
// renderer.readMouseReport decodes.
func TestEncodeSGRMouse(t *testing.T) {
	cases := []struct {
		name   string
		btn    types.MouseButton
		action types.MouseAction
		col    int
		row    int
		want   string
	}{
		{"left press", types.MouseLeft, types.MousePress, 1, 1, "\x1b[<0;1;1M"},
		{"left release", types.MouseLeft, types.MouseRelease, 5, 3, "\x1b[<0;5;3m"},
		{"middle press", types.MouseMiddle, types.MousePress, 2, 2, "\x1b[<1;2;2M"},
		{"right press", types.MouseRight, types.MousePress, 4, 4, "\x1b[<2;4;4M"},
		{"left drag/move", types.MouseLeft, types.MouseMove, 10, 10, "\x1b[<32;10;10M"},
		{"wheel up", types.MouseWheelUp, types.MousePress, 7, 8, "\x1b[<64;7;8M"},
		{"wheel down", types.MouseWheelDown, types.MousePress, 7, 8, "\x1b[<65;7;8M"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := encodeSGRMouse(c.btn, c.action, c.col, c.row)
			if got != c.want {
				t.Fatalf("encodeSGRMouse(%v,%v,%d,%d) = %q, want %q", c.btn, c.action, c.col, c.row, got, c.want)
			}
		})
	}
}

// TestForwardMousePaneRelativeTranslation pins the abs->pane-relative
// coordinate translation and the drop-if-outside-pane and
// drop-if-child-hasn't-opted-in behaviors.
func TestForwardMousePaneRelativeTranslation(t *testing.T) {
	p, in, _ := newTestPane()
	p.bx, p.by = 5, 2
	p.rows, p.cols = 10, 20

	// Child has NOT enabled mouse mode (fresh Screen): forwarding is a
	// silent no-op.
	p.forwardMouse(types.MouseEvent{X: 7, Y: 4, Button: types.MouseLeft, Action: types.MousePress})
	if in.Len() != 0 {
		t.Fatalf("mouse mode disabled: expected no bytes written, got %q", in.Bytes())
	}

	// Enable mouse mode + SGR the way a real child would: feed the SGR mouse
	// enable escape sequence through the Screen's vt10x emulator.
	p.screen.Write([]byte("\x1b[?1000h\x1b[?1006h"))

	// In-bounds: (7,4) absolute minus origin (5,2) = (2,2) relative, wire is
	// 1-indexed => (3,3).
	p.forwardMouse(types.MouseEvent{X: 7, Y: 4, Button: types.MouseLeft, Action: types.MousePress})
	want := "\x1b[<0;3;3M"
	if in.String() != want {
		t.Fatalf("in-bounds forward = %q, want %q", in.String(), want)
	}
	in.Reset()

	// Out-of-bounds (past cols/rows): dropped.
	p.forwardMouse(types.MouseEvent{X: 5 + 20, Y: 2, Button: types.MouseLeft, Action: types.MousePress})
	if in.Len() != 0 {
		t.Fatalf("out-of-bounds (x): expected drop, got %q", in.Bytes())
	}
	p.forwardMouse(types.MouseEvent{X: 5, Y: 2 + 10, Button: types.MouseLeft, Action: types.MousePress})
	if in.Len() != 0 {
		t.Fatalf("out-of-bounds (y): expected drop, got %q", in.Bytes())
	}

	// Before the pane's origin: dropped (negative relative coordinate).
	p.forwardMouse(types.MouseEvent{X: 0, Y: 0, Button: types.MouseLeft, Action: types.MousePress})
	if in.Len() != 0 {
		t.Fatalf("before origin: expected drop, got %q", in.Bytes())
	}
}
