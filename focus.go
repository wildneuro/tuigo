package tuigo

// focus.go — the focus-switch model that lets a focused grab-all element (a
// TerminalPane embedding e.g. Claude Code) keep Tab and Shift-Tab for itself.
//
// The problem: tuigo's render loop historically consumed Tab to cycle focus,
// so a focused pane never saw Tab/Shift-Tab — but embedded agents NEED them
// (Claude uses Tab + Shift-Tab to switch modes). The fix splits the two jobs
// onto different keys:
//
//   - Tab / Shift-Tab cycle focus ONLY when the focused element does not grab
//     input; when it does (Element.GrabKeys, set via GrabInput), they are
//     dispatched to it like any other key.
//   - A distinct global chord — FocusCycleKey, default Ctrl-O — ALWAYS cycles
//     focus, whatever is focused, so you can leave a grab-all pane. It's the
//     tmux-prefix idea reduced to a single reserved chord.
//
// FocusCycleKey is a package var so an app can override the default (e.g. set
// it to F6, or a rune). Keep it off keys the embedded child needs.

import "github.com/wildneuro/tuigo/types"

// FocusCycleKey is the global chord that cycles focus between focusable
// elements, regardless of what is focused (so you can always leave a grab-all
// TerminalPane). Default: Ctrl-O. Override it before Render, e.g.
//
//	tuigo.FocusCycleKey = tuigo.Key{Special: tuigo.KeyBackTab}
//
// Pick something the embedded child doesn't rely on.
var FocusCycleKey = types.Key{Special: types.KeyCtrlO}

// keyRoute is the render loop's decision for one key press.
type keyRoute int

const (
	// routeDispatch: deliver the key normally (global handlers + the focused
	// element). This is where a grab-all pane's Tab/Shift-Tab end up.
	routeDispatch keyRoute = iota
	// routeFocusNext: cycle focus forward instead of delivering the key.
	routeFocusNext
	// routeFocusPrev: cycle focus backward instead of delivering the key.
	routeFocusPrev
)

// keyMatches reports whether k equals the want chord (by SpecialKey when want
// names one, else by rune).
func keyMatches(k, want types.Key) bool {
	if want.Special != types.KeyNone {
		return k.Special == want.Special
	}
	return k.Special == types.KeyNone && want.Rune != 0 && k.Rune == want.Rune
}

// routeKey decides what the render loop does with a key given whether the
// focused element grabs input. The focus chord always wins (so a grab-all pane
// can still be left); Tab/Shift-Tab cycle focus only when the focused element
// does NOT grab input — otherwise they fall through to normal dispatch and
// reach the pane.
func routeKey(k types.Key, focusedGrabs bool) keyRoute {
	if keyMatches(k, FocusCycleKey) {
		return routeFocusNext
	}
	if !focusedGrabs {
		switch k.Special {
		case types.KeyTab:
			return routeFocusNext
		case types.KeyBackTab:
			return routeFocusPrev
		}
	}
	return routeDispatch
}
