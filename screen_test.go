package tuigo

import (
	"bytes"
	"crypto/sha256"
	"strings"
	"testing"
)

// cellAt is a test helper: the rune shown at (x,y) on the current screen.
func cellAt(s *Screen, x, y int) rune {
	return fromGlyph(s.term.Cell(x, y)).Rune
}

// rowString reads a whole row as a trimmed string.
func rowString(s *Screen, y int) string {
	var b strings.Builder
	for x := 0; x < s.cols; x++ {
		b.WriteRune(cellAt(s, x, y))
	}
	return strings.TrimRight(b.String(), " ")
}

func TestScreenPrintablePlacesRunesAtCursor(t *testing.T) {
	s := NewScreen(4, 20)
	s.Write([]byte("hello"))
	if got := rowString(s, 0); got != "hello" {
		t.Fatalf("row0 = %q, want %q", got, "hello")
	}
	cur := s.term.Cursor()
	if cur.X != 5 || cur.Y != 0 {
		t.Fatalf("cursor = (%d,%d), want (5,0)", cur.X, cur.Y)
	}
}

func TestScreenUTF8SplitAcrossWrites(t *testing.T) {
	s := NewScreen(2, 10)
	// "é" is 0xC3 0xA9 — feed it split across two Writes.
	s.Write([]byte{0xC3})
	s.Write([]byte{0xA9})
	s.Write([]byte("x"))
	if got := rowString(s, 0); got != "éx" {
		t.Fatalf("row0 = %q, want %q", got, "éx")
	}
}

func TestScreenCUPPositionsCursor(t *testing.T) {
	s := NewScreen(5, 20)
	s.Write([]byte("\x1b[3;5HZ")) // row 3, col 5 (1-based)
	if got := cellAt(s, 4, 2); got != 'Z' {
		t.Fatalf("cell(4,2) = %q, want Z", got)
	}
}

func TestScreenCarriageReturnAndLineFeed(t *testing.T) {
	s := NewScreen(4, 20)
	s.Write([]byte("abc\r\ndef"))
	if got := rowString(s, 0); got != "abc" {
		t.Fatalf("row0 = %q, want abc", got)
	}
	if got := rowString(s, 1); got != "def" {
		t.Fatalf("row1 = %q, want def", got)
	}
}

func TestScreenEraseLine(t *testing.T) {
	s := NewScreen(3, 20)
	s.Write([]byte("abcdef"))
	s.Write([]byte("\x1b[1;4H")) // back to col 4 (over 'd')
	s.Write([]byte("\x1b[0K"))   // erase to end of line
	if got := rowString(s, 0); got != "abc" {
		t.Fatalf("row0 = %q, want abc", got)
	}
}

func TestScreenEraseDisplayClearsAll(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("aaa\r\nbbb\r\nccc"))
	s.Write([]byte("\x1b[2J"))
	for y := 0; y < 3; y++ {
		if got := rowString(s, y); got != "" {
			t.Fatalf("row%d = %q, want empty after ED2", y, got)
		}
	}
}

func TestScreenScrollOnLineFeedAtBottom(t *testing.T) {
	s := NewScreen(3, 10)
	// Fill three rows then force one more line feed → top row scrolls off.
	s.Write([]byte("r0\r\nr1\r\nr2"))
	s.Write([]byte("\r\nr3"))
	if got := rowString(s, 0); got != "r1" {
		t.Fatalf("row0 = %q, want r1 (scrolled)", got)
	}
	if got := rowString(s, 2); got != "r3" {
		t.Fatalf("row2 = %q, want r3", got)
	}
}

func TestScreenSGRColorRoundTripsThroughRestore(t *testing.T) {
	s := NewScreen(2, 20)
	// bold red on default, then a truecolor fg.
	s.Write([]byte("\x1b[1;31mRED\x1b[0m \x1b[38;2;10;20;30mTC\x1b[0m"))
	assertRoundTrip(t, s)
}

func TestScreenAltScreenSnapshotsActiveBuffer(t *testing.T) {
	s := NewScreen(3, 10)
	s.Write([]byte("main"))
	s.Write([]byte("\x1b[?1049h\x1b[H")) // enter alt screen, home cursor
	s.Write([]byte("ALT"))
	if got := rowString(s, 0); got != "ALT" {
		t.Fatalf("alt row0 = %q, want ALT", got)
	}
	// Snapshot should capture the ALT buffer (the active one).
	snap := s.Snapshot()
	if snap.cells[0].Rune != 'A' {
		t.Fatalf("snapshot cell0 = %q, want A (alt active)", snap.cells[0].Rune)
	}
	s.Write([]byte("\x1b[?1049l")) // leave alt screen → main restored
	if got := rowString(s, 0); got != "main" {
		t.Fatalf("after leave, row0 = %q, want main", got)
	}
}

// assertRoundTrip snapshots s, restores it to a byte stream, feeds that stream
// into a fresh Screen of the same size, and asserts the two snapshots' grids +
// cursors are identical.
func assertRoundTrip(t *testing.T, s *Screen) {
	t.Helper()
	snap := s.Snapshot()
	var buf bytes.Buffer
	if err := snap.Restore(&buf); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	fresh := NewScreen(snap.rows, snap.cols)
	fresh.Write(buf.Bytes())
	got := fresh.Snapshot()
	if got.cx != snap.cx || got.cy != snap.cy {
		t.Fatalf("cursor after round-trip = (%d,%d), want (%d,%d)", got.cx, got.cy, snap.cx, snap.cy)
	}
	if len(got.cells) != len(snap.cells) {
		t.Fatalf("cell count = %d, want %d", len(got.cells), len(snap.cells))
	}
	for i := range snap.cells {
		if got.cells[i] != snap.cells[i] {
			t.Fatalf("cell %d = %+v, want %+v", i, got.cells[i], snap.cells[i])
		}
	}
}

func TestScreenRoundTripFullPage(t *testing.T) {
	s := NewScreen(6, 30)
	s.Write([]byte("\x1b[2J\x1b[H"))
	s.Write([]byte("Title line\r\n"))
	s.Write([]byte("\x1b[7m reversed status \x1b[0m\r\n"))
	s.Write([]byte("\x1b[34mblue\x1b[0m and \x1b[4munderline\x1b[0m\r\n"))
	s.Write([]byte("\x1b[38;5;208m256color\x1b[0m\r\n"))
	s.Write([]byte("\x1b[10;3Hplaced")) // absolute-positioned text
	assertRoundTrip(t, s)
}

// TestScreenRestoreHashIsStable is the user's round-trip-by-hash idea: feed a
// known sequence, save the page, restore to a buffer, and compare a hash of the
// restored bytes against the same page restored twice — restore must be
// deterministic.
func TestScreenRestoreHashIsStable(t *testing.T) {
	s := NewScreen(5, 24)
	s.Write([]byte("\x1b[2J\x1b[H\x1b[1;36mhello \x1b[0mworld\r\nsecond line"))
	snap := s.Snapshot()

	var a, b bytes.Buffer
	_ = snap.Restore(&a)
	_ = snap.Restore(&b)
	if sha256.Sum256(a.Bytes()) != sha256.Sum256(b.Bytes()) {
		t.Fatalf("restore is not deterministic: hashes differ")
	}
	// And restoring a fresh screen built from those bytes yields the same page.
	assertRoundTrip(t, s)
}

// ---- pages stack --------------------------------------------------------

func TestPagesPushPopRestoresPage(t *testing.T) {
	s := NewScreen(3, 20)
	s.Write([]byte("\x1b[2J\x1b[Hchild screen"))

	s.PushPage() // dialog opens: save the child's page
	if s.PageDepth() != 1 {
		t.Fatalf("depth after push = %d, want 1", s.PageDepth())
	}

	// The dialog scribbles over the screen.
	s.Write([]byte("\x1b[2J\x1b[HDIALOG"))
	if got := rowString(s, 0); got != "DIALOG" {
		t.Fatalf("row0 mid-dialog = %q, want DIALOG", got)
	}

	// Dialog closes: pop restores the child's page as bytes...
	var buf bytes.Buffer
	if err := s.PopPage(&buf); err != nil {
		t.Fatalf("PopPage: %v", err)
	}
	if s.PageDepth() != 0 {
		t.Fatalf("depth after pop = %d, want 0", s.PageDepth())
	}
	// ...and replaying those bytes reconstructs the child's page.
	replay := NewScreen(3, 20)
	replay.Write(buf.Bytes())
	if got := rowString(replay, 0); got != "child screen" {
		t.Fatalf("restored row0 = %q, want %q", got, "child screen")
	}
}

func TestPagesNestingRestoresEachLayer(t *testing.T) {
	s := NewScreen(3, 20)
	s.Write([]byte("\x1b[2J\x1b[Hlayer A"))
	s.PushPage() // save A

	s.Write([]byte("\x1b[2J\x1b[Hlayer B"))
	s.PushPage() // save B (nested dialog)

	s.Write([]byte("\x1b[2J\x1b[Hlayer C"))
	if s.PageDepth() != 2 {
		t.Fatalf("depth = %d, want 2", s.PageDepth())
	}

	// Pop inner → restores B.
	var bufB bytes.Buffer
	if err := s.PopPage(&bufB); err != nil {
		t.Fatalf("PopPage B: %v", err)
	}
	rb := NewScreen(3, 20)
	rb.Write(bufB.Bytes())
	if got := rowString(rb, 0); got != "layer B" {
		t.Fatalf("inner pop row0 = %q, want layer B", got)
	}

	// Pop outer → restores A.
	var bufA bytes.Buffer
	if err := s.PopPage(&bufA); err != nil {
		t.Fatalf("PopPage A: %v", err)
	}
	ra := NewScreen(3, 20)
	ra.Write(bufA.Bytes())
	if got := rowString(ra, 0); got != "layer A" {
		t.Fatalf("outer pop row0 = %q, want layer A", got)
	}
}

func TestPagesPopEmptyErrors(t *testing.T) {
	s := NewScreen(2, 10)
	var buf bytes.Buffer
	if err := s.PopPage(&buf); err == nil {
		t.Fatalf("PopPage on empty stack: want error, got nil")
	}
}

func TestScreenResizePreservesTopLeft(t *testing.T) {
	s := NewScreen(4, 20)
	s.Write([]byte("keep"))
	s.Resize(6, 30)
	if got := rowString(s, 0); got != "keep" {
		t.Fatalf("after resize row0 = %q, want keep", got)
	}
	if c, r := s.term.Size(); c != 30 || r != 6 {
		t.Fatalf("size after resize = (%d,%d), want (30,6)", c, r)
	}
}
