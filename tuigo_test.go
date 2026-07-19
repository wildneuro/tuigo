package tuigo

import (
	"testing"

	"github.com/wildneuro/tuigo/types"
)

func TestDispatchFocusedKeyFiresOnFocusedElement(t *testing.T) {
	fired := false
	e := Box(
		WithKey("input"),
		Focusable(),
		OnKey('a', func() { fired = true }),
	)

	dispatchFocusedKey(e, "input", types.Key{Rune: 'a'})

	if !fired {
		t.Errorf("OnKey handler for 'a' on focused element was not fired")
	}
}

func TestDispatchFocusedKeyDoesNotFireOnUnfocusedElement(t *testing.T) {
	fired := false
	e := Box(
		WithKey("input"),
		Focusable(),
		OnKey('a', func() { fired = true }),
	)

	dispatchFocusedKey(e, "other", types.Key{Rune: 'a'})

	if fired {
		t.Errorf("OnKey handler was fired for unfocused element")
	}
}

func TestDispatchFocusedKeyOnAnyKeyFiresOnFocusedElement(t *testing.T) {
	keys := []types.Key{}
	e := Box(
		WithKey("input"),
		Focusable(),
		OnAnyKey(func(k types.Key) {
			keys = append(keys, k)
		}),
	)

	dispatchFocusedKey(e, "input", types.Key{Rune: 'a'})
	dispatchFocusedKey(e, "input", types.Key{Rune: 'b'})

	if len(keys) != 2 {
		t.Errorf("OnAnyKey received %d keys, want 2", len(keys))
	}
}

func TestDispatchFocusedKeyNestedFocusedChildren(t *testing.T) {
	fired := 0
	inner := Box(
		WithKey("inner"),
		Focusable(),
		OnKey('x', func() { fired++ }),
	)
	e := Box(
		WithKey("parent"),
		Focusable(),
		Children(inner),
	)

	dispatchFocusedKey(e, "parent/inner", types.Key{Rune: 'x'})

	if fired != 1 {
		t.Errorf("Nested OnKey fired %d times, want 1", fired)
	}
}

func TestDispatchFocusedKeySpecialKey(t *testing.T) {
	fired := false
	e := Box(
		WithKey("input"),
		Focusable(),
		OnSpecialKey(KeyEnter, func() { fired = true }),
	)

	dispatchFocusedKey(e, "input", types.Key{Special: types.KeyEnter})

	if !fired {
		t.Errorf("OnSpecialKey handler for KeyEnter was not fired")
	}
}

func TestDispatchGlobalKeyFiresAllGlobalHandlers(t *testing.T) {
	fired := 0
	e := Box(
		Global()(OnKey('q', func() { fired++ })),
		Children(
			Box(Global()(OnKey('q', func() { fired++ }))),
		),
	)

	dispatchGlobalKey(e, types.Key{Rune: 'q'})

	if fired != 2 {
		t.Errorf("Global handlers fired %d times, want 2", fired)
	}
}

func TestDispatchGlobalKeyNotFiredForNonGlobal(t *testing.T) {
	fired := false
	e := Box(
		OnKey('q', func() { fired = true }),
	)

	dispatchGlobalKey(e, types.Key{Rune: 'q'})

	if fired {
		t.Errorf("Non-global handler was fired by dispatchGlobalKey")
	}
}

// TestOnHotkeyFiresRegardlessOfFocus proves OnHotkey is sugar over
// Global()(OnSpecialKey(...)): it fires via dispatchGlobalKey even when a
// DIFFERENT focusable child holds focus.
func TestOnHotkeyFiresRegardlessOfFocus(t *testing.T) {
	fired := 0
	e := Box(
		OnHotkey(KeyCtrlRBracket, func() { fired++ }),
		Children(
			Box(WithKey("input"), Focusable()),
		),
	)

	dispatchGlobalKey(e, types.Key{Special: types.KeyCtrlRBracket})
	dispatchFocusedKey(e, "input", types.Key{Special: types.KeyCtrlRBracket})

	if fired != 1 {
		t.Errorf("OnHotkey fired %d times, want 1", fired)
	}
}

// TestNonGlobalSpecialKeyNotFiredWhenUnfocused extends the existing
// non-global coverage to a SpecialKey handler on an unfocused element.
func TestNonGlobalSpecialKeyNotFiredWhenUnfocused(t *testing.T) {
	fired := false
	e := Box(
		WithKey("input"),
		Focusable(),
		OnSpecialKey(KeyCtrlRBracket, func() { fired = true }),
	)

	dispatchFocusedKey(e, "other", types.Key{Special: types.KeyCtrlRBracket})

	if fired {
		t.Errorf("non-global OnSpecialKey handler fired while unfocused")
	}
}

func TestFocusableElementCanBeFocused(t *testing.T) {
	e := Box(
		WithKey("input"),
		Focusable(),
	)

	cp := currentPath(e, "")
	if cp != "input" {
		t.Errorf("Focusable element path = %q, want \"input\"", cp)
	}
}
