package renderer

import (
	"strings"
	"testing"
)

// TestEnterLeaveSeqInline pins the NO-ALT-SCREEN contract: inline mode must
// NEVER emit the alternate-screen enter (1049h) or leave (1049l), so a host
// that owns its own alt buffer keeps it intact. The default mode must emit
// both. Both modes still toggle cursor visibility and SGR mouse reporting.
func TestEnterLeaveSeqInline(t *testing.T) {
	// Default (alt-screen) mode brackets the render with 1049h/1049l.
	enter := enterSeq(false)
	leave := leaveSeq(false)
	if !strings.Contains(enter, "\x1b[?1049h") {
		t.Fatalf("default enter must switch to the alt screen: %q", enter)
	}
	if !strings.Contains(leave, "\x1b[?1049l") {
		t.Fatalf("default leave must return from the alt screen: %q", leave)
	}

	// Inline mode must touch NEITHER 1049h nor 1049l — the whole point is to
	// leave the host's alt buffer untouched.
	ienter := enterSeq(true)
	ileave := leaveSeq(true)
	if strings.Contains(ienter, "1049") {
		t.Fatalf("inline enter must not touch the alt screen (no 1049): %q", ienter)
	}
	if strings.Contains(ileave, "1049") {
		t.Fatalf("inline leave must not touch the alt screen (no 1049): %q", ileave)
	}

	// Both modes still manage the cursor + SGR mouse reporting.
	for _, s := range []string{enter, ienter} {
		if !strings.Contains(s, "\x1b[?25l") || !strings.Contains(s, "\x1b[?1000h") || !strings.Contains(s, "\x1b[?1006h") {
			t.Fatalf("enter must hide cursor + enable mouse: %q", s)
		}
	}
	for _, s := range []string{leave, ileave} {
		if !strings.Contains(s, "\x1b[?25h") || !strings.Contains(s, "\x1b[?1000l") || !strings.Contains(s, "\x1b[?1006l") {
			t.Fatalf("leave must show cursor + disable mouse: %q", s)
		}
	}

	// Inline leave cleans up its own lines (reset SGR + clear the main-screen
	// region it painted) since there is no alt buffer to discard.
	if !strings.Contains(ileave, "\x1b[2J") || !strings.Contains(ileave, "\x1b[0m") {
		t.Fatalf("inline leave must reset SGR and clear its main-screen lines: %q", ileave)
	}
}
