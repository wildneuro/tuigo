package renderer

import (
	"bufio"
	"strings"
	"testing"

	"github.com/wildneuro/tuigo/types"
)

// newTestTerminal builds a bare Terminal wrapping an in-memory reader, so
// readEvent/readEscapeSequence can be exercised headlessly without touching a
// real TTY or raw mode.
func newTestTerminal(input string) *Terminal {
	return &Terminal{in: bufio.NewReaderSize(strings.NewReader(input), 64)}
}

// TestReadEventCSINumeric pins the general multi-digit CSI parameter parser:
// PageUp/PageDown/F5-F12, and that Delete ("ESC[3~") still works through the
// generalized path.
func TestReadEventCSINumeric(t *testing.T) {
	cases := []struct {
		name string
		seq  string
		want types.SpecialKey
	}{
		{"Delete", "\x1b[3~", types.KeyDelete},
		{"PageUp", "\x1b[5~", types.KeyPageUp},
		{"PageDown", "\x1b[6~", types.KeyPageDown},
		{"F5", "\x1b[15~", types.KeyF5},
		{"F6", "\x1b[17~", types.KeyF6},
		{"F7", "\x1b[18~", types.KeyF7},
		{"F8", "\x1b[19~", types.KeyF8},
		{"F9", "\x1b[20~", types.KeyF9},
		{"F10", "\x1b[21~", types.KeyF10},
		{"F11", "\x1b[23~", types.KeyF11},
		{"F12", "\x1b[24~", types.KeyF12},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			term := newTestTerminal(c.seq)
			k, _, kind, ok := term.readEvent()
			if !ok || kind != eventKey {
				t.Fatalf("%s: readEvent failed ok=%v kind=%v", c.name, ok, kind)
			}
			if k.Special != c.want {
				t.Fatalf("%s: got Special=%v, want %v", c.name, k.Special, c.want)
			}
		})
	}
}

// TestReadEventSS3FKeys pins F1-F4 via SS3 ("ESC O P/Q/R/S").
func TestReadEventSS3FKeys(t *testing.T) {
	cases := []struct {
		seq  string
		want types.SpecialKey
	}{
		{"\x1bOP", types.KeyF1},
		{"\x1bOQ", types.KeyF2},
		{"\x1bOR", types.KeyF3},
		{"\x1bOS", types.KeyF4},
	}
	for _, c := range cases {
		term := newTestTerminal(c.seq)
		k, _, kind, ok := term.readEvent()
		if !ok || kind != eventKey || k.Special != c.want {
			t.Fatalf("seq %q: got %+v kind=%v ok=%v, want Special=%v", c.seq, k, kind, ok, c.want)
		}
	}
}

// TestReadEventAltLetter pins the Alt+letter fix: ESC immediately followed by
// a non-'['/'O' byte is Alt+<that byte>, NOT a dropped/bare Esc.
func TestReadEventAltLetter(t *testing.T) {
	term := newTestTerminal("\x1ba")
	k, _, kind, ok := term.readEvent()
	if !ok || kind != eventKey {
		t.Fatalf("readEvent failed ok=%v kind=%v", ok, kind)
	}
	if !k.Alt || k.Rune != 'a' || k.Special != types.KeyNone {
		t.Fatalf("Alt+a: got %+v, want {Rune:'a' Alt:true}", k)
	}
}

// TestReadEventBareEsc pins that a lone Esc (nothing buffered after it) is
// still a bare KeyEsc, not misrouted.
func TestReadEventBareEsc(t *testing.T) {
	term := newTestTerminal("\x1b")
	k, _, kind, ok := term.readEvent()
	if !ok || kind != eventKey || k.Special != types.KeyEsc {
		t.Fatalf("bare Esc: got %+v kind=%v ok=%v", k, kind, ok)
	}
}

// TestReadEventBracketedPaste pins that a full "ESC[200~...ESC[201~" burst is
// decoded as ONE KeyPaste event carrying the inner text with markers
// stripped, atomically (not split across events).
func TestReadEventBracketedPaste(t *testing.T) {
	term := newTestTerminal("\x1b[200~hello\nworld\x1b[201~")
	k, _, kind, ok := term.readEvent()
	if !ok || kind != eventKey {
		t.Fatalf("readEvent failed ok=%v kind=%v", ok, kind)
	}
	if k.Special != types.KeyPaste {
		t.Fatalf("got Special=%v, want KeyPaste", k.Special)
	}
	if k.Paste != "hello\nworld" {
		t.Fatalf("Paste = %q, want %q", k.Paste, "hello\nworld")
	}
}

// TestReadEventBracketedPasteThenMore pins that a paste event doesn't
// consume bytes past its own terminator — the next event is read correctly.
func TestReadEventBracketedPasteThenMore(t *testing.T) {
	term := newTestTerminal("\x1b[200~abc\x1b[201~x")
	k, _, kind, ok := term.readEvent()
	if !ok || kind != eventKey || k.Special != types.KeyPaste || k.Paste != "abc" {
		t.Fatalf("first event: got %+v kind=%v ok=%v", k, kind, ok)
	}
	k2, _, kind2, ok2 := term.readEvent()
	if !ok2 || kind2 != eventKey || k2.Rune != 'x' {
		t.Fatalf("second event: got %+v kind=%v ok=%v, want rune 'x'", k2, kind2, ok2)
	}
}

// TestReadEventUnmappedCSIDropped pins that an unmapped numeric CSI code
// (e.g. Insert=2) is dropped rather than misrouted, falling through to the
// next real event.
func TestReadEventUnmappedCSIDropped(t *testing.T) {
	term := newTestTerminal("\x1b[2~x")
	k, _, kind, ok := term.readEvent()
	if !ok || kind != eventKey || k.Rune != 'x' {
		t.Fatalf("got %+v kind=%v ok=%v, want the fallback-through rune 'x'", k, kind, ok)
	}
}

// TestReadEventArrowsStillWork guards against a regression in the
// single-letter CSI cases (arrows/Home/End/Shift-Tab) while generalizing the
// digit path.
func TestReadEventArrowsStillWork(t *testing.T) {
	cases := []struct {
		seq  string
		want types.SpecialKey
	}{
		{"\x1b[A", types.KeyUp},
		{"\x1b[B", types.KeyDown},
		{"\x1b[C", types.KeyRight},
		{"\x1b[D", types.KeyLeft},
		{"\x1b[H", types.KeyHome},
		{"\x1b[F", types.KeyEnd},
		{"\x1b[Z", types.KeyBackTab},
	}
	for _, c := range cases {
		term := newTestTerminal(c.seq)
		k, _, kind, ok := term.readEvent()
		if !ok || kind != eventKey || k.Special != c.want {
			t.Fatalf("seq %q: got %+v kind=%v ok=%v, want Special=%v", c.seq, k, kind, ok, c.want)
		}
	}
}
