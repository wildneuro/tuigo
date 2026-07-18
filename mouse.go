package tuigo

// mouse.go — forwarding a host mouse event into a focused TerminalPane's
// child as an SGR mouse report, the inverse of what
// renderer/terminal.go's readMouseReport DECODES. Only reached when the
// child has itself enabled mouse reporting (Screen.MouseMode); otherwise the
// pane drops the event silently, matching how a real terminal behaves toward
// an app that never opted in.

import (
	"fmt"

	"github.com/wildneuro/tuigo/types"
)

// encodeSGRMouse builds the SGR mouse escape sequence ("ESC [ < Cb;Cx;Cy
// M/m") for one MouseEvent at PANE-RELATIVE, already 1-indexed (col, row).
// It is the pure inverse of readMouseReport's decode: button/wheel encode
// into Cb the same way readMouseReport interprets it, and Action selects the
// M (press/motion) vs m (release) terminator.
func encodeSGRMouse(btn types.MouseButton, action types.MouseAction, col, row int) string {
	var cb int
	switch btn {
	case types.MouseWheelUp:
		cb = 64
	case types.MouseWheelDown:
		cb = 65
	case types.MouseMiddle:
		cb = 1
	case types.MouseRight:
		cb = 2
	default: // types.MouseLeft, types.MouseNone
		cb = 0
	}
	if action == types.MouseMove && cb < 64 {
		cb |= 32
	}
	final := byte('M')
	if action == types.MouseRelease {
		final = 'm'
	}
	return fmt.Sprintf("\x1b[<%d;%d;%d%c", cb, col, row, final)
}

// forwardMouse translates an absolute host MouseEvent into pane-relative,
// 1-indexed coordinates using the pane's last-painted origin (bx,by) and last
// box size (cols,rows), dropping (not forwarding) anything that lands
// outside [0,cols)x[0,rows) of the pane. It writes the encoded SGR sequence
// to the child's PTY.
func (p *paneState) forwardMouse(m types.MouseEvent) {
	p.mu.Lock()
	w := p.in
	bx, by := p.bx, p.by
	cols, rows := p.cols, p.rows
	screen := p.screen
	p.mu.Unlock()
	if w == nil || screen == nil {
		return
	}
	enabled, _ := screen.MouseMode()
	if !enabled {
		return
	}
	rx, ry := m.X-bx, m.Y-by
	if rx < 0 || ry < 0 || rx >= cols || ry >= rows {
		return
	}
	seq := encodeSGRMouse(m.Button, m.Action, rx+1, ry+1)
	_, _ = w.Write([]byte(seq))
}
