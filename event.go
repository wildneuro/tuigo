package tuigo

import "github.com/wildneuro/tuigo/types"

type SpecialKey = types.SpecialKey

const (
	KeyNone      = types.KeyNone
	KeyEnter     = types.KeyEnter
	KeyEsc       = types.KeyEsc
	KeyTab       = types.KeyTab
	KeyBackspace = types.KeyBackspace
	KeyDelete    = types.KeyDelete
	KeyUp        = types.KeyUp
	KeyDown      = types.KeyDown
	KeyLeft      = types.KeyLeft
	KeyRight     = types.KeyRight
	KeyHome      = types.KeyHome
	KeyEnd       = types.KeyEnd
	KeyCtrlC     = types.KeyCtrlC
	KeyBackTab   = types.KeyBackTab
	KeyCtrlO     = types.KeyCtrlO
)

type Key = types.Key

func Focusable() Option {
	return func(e *Element) {
		e.Focusable = true
	}
}

// GrabInput marks an element as focusable AND a grab-all key sink: while it
// holds focus, Tab and Shift-Tab are delivered to it (via its key handlers)
// instead of cycling focus, so an embedded full-screen child gets those keys.
// Focus is still switchable with the global focus chord (FocusCycleKey,
// default Ctrl-O). TerminalPane sets this by default.
func GrabInput() Option {
	return func(e *Element) {
		e.Focusable = true
		e.GrabKeys = true
	}
}

func Global() func(Option) Option {
	return func(opt Option) Option {
		return func(e *Element) {
			opt(e)
			for i := range e.Handlers {
				e.Handlers[i].Global = true
			}
		}
	}
}

func OnKey(r rune, handler func()) Option {
	return func(e *Element) {
		e.Handlers = append(e.Handlers, types.KeyHandler{Rune: r, Handle: func(types.Key) { handler() }})
	}
}

func OnSpecialKey(k SpecialKey, handler func()) Option {
	return func(e *Element) {
		e.Handlers = append(e.Handlers, types.KeyHandler{Special: k, Handle: func(types.Key) { handler() }})
	}
}

func OnAnyKey(handler func(Key)) Option {
	return func(e *Element) {
		e.Handlers = append(e.Handlers, types.KeyHandler{Any: true, Handle: handler})
	}
}

// MouseButton identifies which button (or wheel direction) a MouseEvent
// reports.
type MouseButton = types.MouseButton

const (
	MouseLeft      = types.MouseLeft
	MouseMiddle    = types.MouseMiddle
	MouseRight     = types.MouseRight
	MouseWheelUp   = types.MouseWheelUp
	MouseWheelDown = types.MouseWheelDown
)

// MouseEvent is a single mouse report: a button/wheel action at a 0-indexed
// cell coordinate, delivered by OnClick/OnMouse/OnScroll.
type MouseEvent = types.MouseEvent

// OnClick calls handler when the left mouse button is pressed anywhere
// within this Element's rendered rect. The dispatcher hit-tests using the
// last computed layout, so it works for Box and Text alike.
func OnClick(handler func(MouseEvent)) Option {
	return func(e *Element) {
		e.MouseHandlers = append(e.MouseHandlers, types.MouseHandler{
			Buttons: []MouseButton{MouseLeft},
			Handle:  handler,
		})
	}
}

// OnMouse calls handler for every mouse event that hits this Element's
// rendered rect, regardless of button.
func OnMouse(handler func(MouseEvent)) Option {
	return func(e *Element) {
		e.MouseHandlers = append(e.MouseHandlers, types.MouseHandler{Handle: handler})
	}
}

// OnScroll calls handler with -1 for wheel-up or +1 for wheel-down when the
// scroll wheel is used over this Element's rendered rect.
func OnScroll(handler func(delta int)) Option {
	return func(e *Element) {
		e.MouseHandlers = append(e.MouseHandlers, types.MouseHandler{
			Buttons: []MouseButton{MouseWheelUp, MouseWheelDown},
			Handle: func(m MouseEvent) {
				if m.Button == MouseWheelUp {
					handler(-1)
				} else {
					handler(1)
				}
			},
		})
	}
}
