package types

import "slices"

// SpecialKey identifies a non-rune key such as Enter or an arrow key.
type SpecialKey int

const (
	KeyNone SpecialKey = iota
	KeyEnter
	KeyEsc
	KeyTab
	KeyBackspace
	KeyDelete
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyHome
	KeyEnd
	KeyCtrlC
	// KeyBackTab is Shift-Tab (the terminal CBT sequence "ESC [ Z"). It is a
	// distinct key so a focused grab-all element (TerminalPane) can receive
	// it — embedded agents use Tab/Shift-Tab to move between modes.
	KeyBackTab
	// KeyCtrlO is Ctrl-O (byte 0x0f). It is the default global focus-cycle
	// chord (tuigo.FocusCycleKey): a key distinct from Tab so Tab stays free
	// for a focused pane while focus can still be switched between panes.
	KeyCtrlO
)

// Key is a single keyboard event: either a plain rune (Special == KeyNone)
// or a named SpecialKey.
type Key struct {
	Rune    rune
	Special SpecialKey
}

// KeyHandler matches an incoming Key against a rune, a SpecialKey, or every
// key (Any), and invokes Handle when it matches. Global handlers fire
// regardless of which element has focus.
type KeyHandler struct {
	Rune    rune
	Special SpecialKey
	Any     bool
	Global  bool
	Handle  func(Key)
}

// Matches reports whether k satisfies this handler's match criteria.
func (h KeyHandler) Matches(k Key) bool {
	if h.Any {
		return true
	}
	if h.Special != KeyNone {
		return k.Special == h.Special
	}
	return k.Special == KeyNone && k.Rune == h.Rune
}

// MouseButton identifies which button (or wheel direction) a MouseEvent
// reports.
type MouseButton int

const (
	MouseNone MouseButton = iota
	MouseLeft
	MouseMiddle
	MouseRight
	MouseWheelUp
	MouseWheelDown
)

// MouseAction identifies what the button did.
type MouseAction int

const (
	MousePress MouseAction = iota
	MouseRelease
	MouseMove
)

// MouseEvent is a single mouse report: a button/wheel action at a cell
// coordinate. X and Y are 0-indexed to match layout.Rect, unlike the
// 1-indexed wire format terminals send.
type MouseEvent struct {
	X, Y   int
	Button MouseButton
	Action MouseAction
}

// MouseHandler fires Handle for every MouseEvent that reaches the Element
// it's attached to. Unlike KeyHandler, matching is by hit-testing the
// Element's rendered rect (done by the dispatcher), not by a field on the
// handler itself.
type MouseHandler struct {
	// Buttons restricts which buttons this handler cares about; empty means
	// "any button/wheel action".
	Buttons []MouseButton
	Handle  func(MouseEvent)
}

// Matches reports whether m satisfies this handler's button filter.
func (h MouseHandler) Matches(m MouseEvent) bool {
	if len(h.Buttons) == 0 {
		return true
	}
	return slices.Contains(h.Buttons, m.Button)
}
