package tuigo

import (
	"testing"

	"github.com/wildneuro/tuigo/types"
)

// TestRouteKeyGrabPaneGetsTab pins FIX 1: when the focused element grabs input
// (a TerminalPane), Tab and Shift-Tab are dispatched to it, not consumed for
// focus cycling — while a focused NON-grab element uses them to cycle.
func TestRouteKeyGrabPaneGetsTab(t *testing.T) {
	tab := types.Key{Special: types.KeyTab}
	backtab := types.Key{Special: types.KeyBackTab}

	// Grab-all focused: Tab / Shift-Tab fall through to normal dispatch (reach
	// the pane), NOT focus cycling.
	if got := routeKey(tab, true); got != routeDispatch {
		t.Fatalf("grab pane: Tab should dispatch to the pane, got %v", got)
	}
	if got := routeKey(backtab, true); got != routeDispatch {
		t.Fatalf("grab pane: Shift-Tab should dispatch to the pane, got %v", got)
	}

	// Non-grab focused: Tab cycles forward, Shift-Tab backward.
	if got := routeKey(tab, false); got != routeFocusNext {
		t.Fatalf("non-grab: Tab should cycle focus forward, got %v", got)
	}
	if got := routeKey(backtab, false); got != routeFocusPrev {
		t.Fatalf("non-grab: Shift-Tab should cycle focus backward, got %v", got)
	}
}

// TestRouteKeyNewKeysAlwaysDispatch pins the invariant that neither a
// bracketed-paste event nor an Alt-prefixed rune is ever mistaken for the
// focus-cycle chord or Tab/Shift-Tab — they always reach normal dispatch,
// whether or not the focused element grabs input.
func TestRouteKeyNewKeysAlwaysDispatch(t *testing.T) {
	paste := types.Key{Special: types.KeyPaste, Paste: "hello"}
	altA := types.Key{Rune: 'a', Alt: true}

	for _, grabs := range []bool{true, false} {
		if got := routeKey(paste, grabs); got != routeDispatch {
			t.Fatalf("KeyPaste (grabs=%v) should always dispatch, got %v", grabs, got)
		}
		if got := routeKey(altA, grabs); got != routeDispatch {
			t.Fatalf("Alt+a (grabs=%v) should always dispatch, got %v", grabs, got)
		}
	}
}

// TestRouteKeyFocusChord pins the distinct focus-switch chord: it ALWAYS cycles
// focus, even over a grab-all pane (that's how you leave the pane). The default
// is Ctrl-O; an override is honoured.
func TestRouteKeyFocusChord(t *testing.T) {
	ctrlO := types.Key{Special: types.KeyCtrlO}

	// Default chord cycles focus regardless of grab.
	if got := routeKey(ctrlO, true); got != routeFocusNext {
		t.Fatalf("focus chord over grab pane should cycle focus, got %v", got)
	}
	if got := routeKey(ctrlO, false); got != routeFocusNext {
		t.Fatalf("focus chord should cycle focus, got %v", got)
	}

	// A plain rune is NOT the chord: it dispatches (reaches the pane).
	if got := routeKey(types.Key{Rune: 'a'}, true); got != routeDispatch {
		t.Fatalf("plain rune should dispatch, got %v", got)
	}

	// Override the chord to a rune and confirm routing follows.
	saved := FocusCycleKey
	defer func() { FocusCycleKey = saved }()
	FocusCycleKey = types.Key{Rune: 6} // Ctrl-F
	if got := routeKey(types.Key{Rune: 6}, true); got != routeFocusNext {
		t.Fatalf("overridden chord should cycle focus, got %v", got)
	}
	if got := routeKey(ctrlO, true); got != routeDispatch {
		t.Fatalf("old chord should no longer cycle after override, got %v", got)
	}
}

// TestFocusGrabLookup pins that collectFocusOrder records, per focusable path,
// whether that element grabs input — the map the render loop reads to route
// Tab. A TerminalPane grabs; a plain focusable Box does not.
func TestFocusGrabLookup(t *testing.T) {
	tree := Box(Children(
		With(Box(Children(Text("pane"))), WithKey("term"), GrabInput()),
		With(Box(Children(Text("bar"))), WithKey("bar"), Focusable()),
	))
	app := &appState{}
	collectFocusOrder(tree, "", app)

	if len(app.focusOrder) != 2 {
		t.Fatalf("expected 2 focusables, got %v", app.focusOrder)
	}
	if !app.focusGrab["term"] {
		t.Fatal("term (GrabInput) should be recorded as grabbing input")
	}
	if app.focusGrab["bar"] {
		t.Fatal("bar (plain Focusable) should NOT grab input")
	}
}
