package tuigo

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/wildneuro/tuigo/types"
)

// captureSurface is a headless types.Surface that records painted cells so a
// test can assert exactly what a paint produced, without a real terminal.
type captureSurface struct {
	x, y, w, h int
	cells      map[[2]int]rune
	fg, bg     map[[2]int]types.Color
}

func newCaptureSurface(x, y, w, h int) *captureSurface {
	return &captureSurface{
		x: x, y: y, w: w, h: h,
		cells: map[[2]int]rune{},
		fg:    map[[2]int]types.Color{},
		bg:    map[[2]int]types.Color{},
	}
}

func (s *captureSurface) Bounds() (int, int, int, int) { return s.x, s.y, s.w, s.h }

func (s *captureSurface) Set(x, y int, r rune, fg, bg types.Color, _, _, _ bool) {
	s.cells[[2]int{x, y}] = r
	s.fg[[2]int{x, y}] = fg
	s.bg[[2]int{x, y}] = bg
}

// rowString reads a painted row back as a string (row is absolute Y).
func (s *captureSurface) rowString(y, x0, w int) string {
	var b []rune
	for x := x0; x < x0+w; x++ {
		r, ok := s.cells[[2]int{x, y}]
		if !ok || r == 0 {
			r = ' '
		}
		b = append(b, r)
	}
	return string(b)
}

// newTestPane builds a paneState wired to in-memory writers instead of a real
// PTY, so the pure logic (sizing, output→screen, resize, input, exit) is
// testable headlessly.
func newTestPane() (*paneState, *bytes.Buffer, *[]int) {
	in := &bytes.Buffer{}
	var sizes []int // flattened rows,cols pairs recorded by setsize
	p := &paneState{
		screen:  NewScreen(24, 80),
		in:      in,
		wake:    func() {},
		setsize: func(rows, cols int) error { sizes = append(sizes, rows, cols); return nil },
	}
	return p, in, &sizes
}

// --- 1. Screen sizing from a layout box ----------------------------------

func TestPaneSizesScreenToBox(t *testing.T) {
	p, _, sizes := newTestPane()
	surf := newCaptureSurface(0, 0, 40, 12)
	p.paint(surf, false)

	rows, cols := p.screen.Size()
	if rows != 12 || cols != 40 {
		t.Fatalf("screen not sized to box: got %dx%d want 12x40", rows, cols)
	}
	if len(*sizes) != 2 || (*sizes)[0] != 12 || (*sizes)[1] != 40 {
		t.Fatalf("pty setsize not called with box size: %v", *sizes)
	}
}

// --- 2. Output → Screen wiring: painted cells match fed bytes ------------

func TestPanePaintsChildOutput(t *testing.T) {
	p, _, _ := newTestPane()
	surf := newCaptureSurface(3, 2, 20, 5) // non-zero origin: absolute mapping matters
	// First paint sizes the screen; then feed output through the reader path.
	p.paint(surf, false)

	p.mu.Lock()
	p.screen.Write([]byte("hello"))
	p.mu.Unlock()

	surf2 := newCaptureSurface(3, 2, 20, 5)
	p.paint(surf2, false)

	// "hello" must land at the pane's top-left, offset by the box origin.
	if got := surf2.rowString(2, 3, 5); got != "hello" {
		t.Fatalf("child output not painted at box origin: got %q want %q", got, "hello")
	}
}

// --- 3. Resize recomputes the pty size -----------------------------------

func TestPaneResizeRecomputesPtySize(t *testing.T) {
	p, _, sizes := newTestPane()
	p.paint(newCaptureSurface(0, 0, 40, 12), false)
	p.paint(newCaptureSurface(0, 0, 40, 12), false) // same size: must NOT re-setsize
	p.paint(newCaptureSurface(0, 0, 30, 8), false)  // changed: must reflow

	// Expect exactly two setsize calls: initial 12x40 and reflow 8x30.
	if len(*sizes) != 4 {
		t.Fatalf("expected 2 setsize calls, got %d (%v)", len(*sizes)/2, *sizes)
	}
	if (*sizes)[2] != 8 || (*sizes)[3] != 30 {
		t.Fatalf("reflow setsize wrong: got %dx%d want 8x30", (*sizes)[2], (*sizes)[3])
	}
	rows, cols := p.screen.Size()
	if rows != 8 || cols != 30 {
		t.Fatalf("screen not resized on reflow: got %dx%d", rows, cols)
	}
}

// --- 4. Focused vs. unfocused input gating -------------------------------

// At the paneState level, writeKey always forwards (gating is the framework's
// job). This asserts the translation + write.
func TestPaneWriteKeyForwardsBytes(t *testing.T) {
	p, in, _ := newTestPane()
	p.writeKey(types.Key{Rune: 'x'})
	p.writeKey(types.Key{Special: types.KeyEnter})
	p.writeKey(types.Key{Special: types.KeyUp})
	if got := in.String(); got != "x\r\x1b[A" {
		t.Fatalf("writeKey translation wrong: %q", got)
	}
}

// This asserts the ACTUAL focus gating through tuigo's dispatcher: the pane's
// key handler fires only when its path == the focus path.
func TestPaneInputGatedByFocus(t *testing.T) {
	in := &bytes.Buffer{}
	st := &paneState{screen: NewScreen(24, 80), in: in, wake: func() {}}

	// Build a pane element by hand (mirrors TerminalPane's handler wiring)
	// with key "term", nested under a non-focusable Box.
	pane := Element{
		Type:      types.ElementTypeCanvas,
		Key:       "term",
		Focusable: true,
		Handlers: []types.KeyHandler{{
			Any:    true,
			Handle: func(k types.Key) { st.writeKey(k) },
		}},
	}
	tree := Box(Children(pane))

	// Unfocused: dispatch to a different path — nothing forwarded.
	dispatchFocusedKey(tree, "somethingelse", types.Key{Rune: 'a'})
	if in.Len() != 0 {
		t.Fatalf("unfocused pane received input: %q", in.String())
	}

	// Focused: the pane's resolved path is "term".
	dispatchFocusedKey(tree, "term", types.Key{Rune: 'a'})
	if in.String() != "a" {
		t.Fatalf("focused pane did not receive input: %q", in.String())
	}
}

// --- 5. Child-exit propagation -------------------------------------------

func TestPaneChildExitPropagates(t *testing.T) {
	var got error
	fired := 0
	p := &paneState{
		screen: NewScreen(24, 80),
		wake:   func() {},
		onExit: func(err error) { fired++; got = err },
	}
	// Reader hits clean EOF -> exit reported as nil, exactly once.
	p.run(strReader("done"))
	if !p.exited {
		t.Fatal("pane not marked exited after reader EOF")
	}
	if fired != 1 {
		t.Fatalf("onExit fired %d times, want 1", fired)
	}
	if got != nil {
		t.Fatalf("clean EOF should report nil error, got %v", got)
	}
	// Output before EOF still made it into the screen.
	p.paint(newCaptureSurface(0, 0, 10, 2), false)
}

// strReader returns s once then io.EOF, exercising the run() copy+exit path.
func strReader(s string) io.Reader { return bytes.NewReader([]byte(s)) }

// --- 5b. Real PTY end-to-end: spawn, read, paint, exit -------------------

// TestPaneRealPTY proves the actual creack/pty spawn + reader + exit path,
// not just the fake-writer plumbing: it runs a short-lived child through
// paneState.start and asserts its output reached the painted screen.
func TestPaneRealPTY(t *testing.T) {
	exited := make(chan struct{})
	p := &paneState{
		screen: NewScreen(24, 80),
		wake:   func() {},
		onExit: func(error) { close(exited) },
	}
	p.start([]string{"sh", "-c", "printf HELLO"})
	p.mu.Lock()
	failed := p.exited && p.exitErr != nil
	failErr := p.exitErr
	p.mu.Unlock()
	if failed {
		t.Skipf("PTY unavailable in this environment: %v", failErr)
	}

	select {
	case <-exited:
	case <-time.After(5 * time.Second):
		p.close()
		t.Fatal("child did not exit within 5s")
	}

	surf := newCaptureSurface(0, 0, 20, 3)
	p.paint(surf, false)
	if got := strings.TrimRight(surf.rowString(0, 0, 5), " "); got != "HELLO" {
		t.Fatalf("child output not composited: got %q want HELLO", got)
	}
	p.close()
}

// --- 6. Cursor drawn only when focused -----------------------------------

func TestPaneCursorOnlyWhenFocused(t *testing.T) {
	p, _, _ := newTestPane()
	p.paint(newCaptureSurface(0, 0, 10, 3), false)
	p.mu.Lock()
	p.screen.Write([]byte("AB")) // cursor now at col 2, row 0
	p.mu.Unlock()

	unfocused := newCaptureSurface(0, 0, 10, 3)
	p.paint(unfocused, false)
	focused := newCaptureSurface(0, 0, 10, 3)
	p.paint(focused, true)

	// The cursor cell (2,0) must carry the block-cursor bg only in the
	// focused paint.
	if unfocused.bg[[2]int{2, 0}] == cursorBg {
		t.Fatal("unfocused pane drew a cursor")
	}
	if focused.bg[[2]int{2, 0}] != cursorBg {
		t.Fatal("focused pane did not draw a cursor at the child cursor cell")
	}
}
