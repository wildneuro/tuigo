package renderer

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/term"

	"github.com/wildneuro/tuigo/types"
)

// Size is a terminal dimension in cells.
type Size struct{ W, H int }

// Terminal is the impure shell around a real TTY: raw mode, alt screen,
// stdin key/mouse decoding, resize notification, and diffed ANSI flushing.
// It is the only piece of tuigo that touches the OS terminal directly.
type Terminal struct {
	fd       int
	oldState *term.State
	in       *bufio.Reader
	out      *os.File

	keyCh   chan types.Key
	mouseCh chan types.MouseEvent

	// done is closed by Close so a readLoop blocked on delivering an event
	// exits instead of leaking on a channel nobody reads after Render returns.
	done chan struct{}

	// inline is set for a NO-ALT-SCREEN terminal (see NewInlineTerminal): it
	// never enters/leaves the alternate screen buffer, so a host that has its
	// own alt-screen content (e.g. a PTY-wrapped child) keeps it untouched.
	inline bool

	closeOnce sync.Once

	havePos      bool
	lastX, lastY int
	haveStyle    bool
	lastStyle    cellStyle

	// cursorShown tracks the real hardware cursor's visibility so SetCursor
	// toggles it only on change (no flicker). The cursor starts hidden
	// (enterSeq emits ?25l).
	cursorShown bool
}

// enterSeq is the escape burst written on terminal setup. The alt-screen
// (mode 1049) enter is emitted ONLY in the default (non-inline) mode; inline
// mode renders on the MAIN screen and must not touch the alt buffer, so a
// caller that switched away from a child's alt screen keeps that content
// intact. Cursor-hide (25l) and SGR mouse reporting (1000+1006) apply to
// both. Pure — unit-tested.
func enterSeq(inline bool) string {
	if inline {
		return "\x1b[?25l\x1b[?1000h\x1b[?1006h"
	}
	return "\x1b[?1049h\x1b[?25l\x1b[?1000h\x1b[?1006h"
}

// leaveSeq is the escape burst written on Close, undoing enterSeq. In the
// default mode it leaves the alternate screen (1049l), restoring the host
// screen the terminal saved on enter. In inline mode there is no alt buffer
// to leave; instead it resets SGR and clears the main-screen region the modal
// painted so it cleans up its own lines. Mouse-off + cursor-show apply to
// both. Pure — unit-tested.
func leaveSeq(inline bool) string {
	if inline {
		return "\x1b[?1006l\x1b[?1000l\x1b[?25h\x1b[0m\x1b[2J\x1b[H"
	}
	return "\x1b[?1006l\x1b[?1000l\x1b[?25h\x1b[?1049l"
}

type cellStyle struct {
	Fg, Bg                  types.Color
	Bold, Italic, Underline bool
}

// NewTerminal puts stdin into raw mode, enters the alternate screen with the
// cursor hidden, enables SGR mouse reporting, and starts the single
// goroutine that decodes stdin into key/mouse events. Callers must defer
// Close to restore the terminal, even on panic.
func NewTerminal() (*Terminal, error) { return newTerminal(false) }

// NewInlineTerminal is NewTerminal WITHOUT the alternate-screen takeover: it
// renders on the MAIN screen at the current cursor/screen and never emits the
// alt-screen enter/leave (mode 1049). Use it when the host already owns an
// alternate screen it must preserve — e.g. a PTY wrapper that switched away
// from a full-screen child's alt buffer and needs the terminal to keep that
// buffer intact while a modal is shown. Raw mode, cursor-hide, and SGR mouse
// reporting still apply for the render's duration; Close cleans up its own
// lines. Callers must defer Close, even on panic.
func NewInlineTerminal() (*Terminal, error) { return newTerminal(true) }

// newTerminal puts stdin into raw mode, enters (unless inline) the alternate
// screen with the cursor hidden, enables SGR mouse reporting, and starts the
// single goroutine that decodes stdin into key/mouse events.
func newTerminal(inline bool) (*Terminal, error) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return nil, fmt.Errorf("tuigo: enter raw mode: %w", err)
	}
	t := &Terminal{
		fd:       fd,
		oldState: oldState,
		in:       bufio.NewReaderSize(os.Stdin, 64),
		out:      os.Stdout,
		keyCh:    make(chan types.Key),
		mouseCh:  make(chan types.MouseEvent),
		done:     make(chan struct{}),
		inline:   inline,
	}
	// (Alt screen) + hide cursor + SGR extended mouse reporting (modes 1000
	// and 1006), which reports every press/release/drag with unambiguous
	// coordinates past column/row 223. Inline mode omits the alt-screen enter.
	t.out.WriteString(enterSeq(inline))
	go t.readLoop()
	return t, nil
}

// Size returns the current terminal width and height in cells.
func (t *Terminal) Size() (int, int) {
	w, h, err := term.GetSize(t.fd)
	if err != nil {
		return 80, 24
	}
	return w, h
}

// Close restores cursor visibility, disables mouse reporting, leaves the
// alternate screen, and restores the terminal's original mode. It is safe
// to call more than once.
func (t *Terminal) Close() error {
	var err error
	t.closeOnce.Do(func() {
		// Signal the reader to stop so a readLoop parked on delivering an
		// event exits rather than leaking on channels nobody reads once
		// Render has returned. (A read already blocked in the kernel still
		// consumes one more byte before it unblocks and sees done — an
		// inherent property of a blocking raw-stdin read.)
		close(t.done)
		t.out.WriteString(leaveSeq(t.inline))
		err = term.Restore(t.fd, t.oldState)
	})
	return err
}

// Keys returns the channel key events are sent on. It closes when stdin
// hits EOF or an error.
func (t *Terminal) Keys() <-chan types.Key { return t.keyCh }

// Mouse returns the channel mouse events are sent on. It closes together
// with the Keys channel.
func (t *Terminal) Mouse() <-chan types.MouseEvent { return t.mouseCh }

// readLoop is the single goroutine reading stdin. Both keys and mouse
// reports arrive interleaved on the same fd, so one reader demultiplexes
// into two channels rather than racing two goroutines on the same
// bufio.Reader.
func (t *Terminal) readLoop() {
	defer close(t.keyCh)
	defer close(t.mouseCh)
	for {
		key, mouse, kind, ok := t.readEvent()
		if !ok {
			return
		}
		// Deliver, but abandon a send that no one will receive once Close has
		// fired (done closed) so this goroutine never leaks blocked on an
		// unread channel.
		switch kind {
		case eventKey:
			select {
			case t.keyCh <- key:
			case <-t.done:
				return
			}
		case eventMouse:
			select {
			case t.mouseCh <- mouse:
			case <-t.done:
				return
			}
		}
	}
}

type eventKind int

const (
	eventKey eventKind = iota
	eventMouse
)

func (t *Terminal) readEvent() (types.Key, types.MouseEvent, eventKind, bool) {
	b, err := t.in.ReadByte()
	if err != nil {
		return types.Key{}, types.MouseEvent{}, eventKey, false
	}
	switch b {
	case 3:
		return types.Key{Special: types.KeyCtrlC}, types.MouseEvent{}, eventKey, true
	case 13, 10:
		return types.Key{Special: types.KeyEnter}, types.MouseEvent{}, eventKey, true
	case 9:
		return types.Key{Special: types.KeyTab}, types.MouseEvent{}, eventKey, true
	case 15:
		// Ctrl-O — the default global focus-cycle chord (tuigo.FocusCycleKey),
		// kept distinct from Tab so Tab stays free for a focused pane.
		return types.Key{Special: types.KeyCtrlO}, types.MouseEvent{}, eventKey, true
	case 127, 8:
		return types.Key{Special: types.KeyBackspace}, types.MouseEvent{}, eventKey, true
	case 27:
		return t.readEscapeSequence()
	}
	if b < 0x80 {
		return types.Key{Rune: rune(b)}, types.MouseEvent{}, eventKey, true
	}
	// Multi-byte UTF-8: put the lead byte back and decode it as a rune.
	if err := t.in.UnreadByte(); err != nil {
		return types.Key{Rune: rune(b)}, types.MouseEvent{}, eventKey, true
	}
	r, _, err := t.in.ReadRune()
	if err != nil {
		return types.Key{Rune: rune(b)}, types.MouseEvent{}, eventKey, true
	}
	return types.Key{Rune: r}, types.MouseEvent{}, eventKey, true
}

// readEscapeSequence handles a lone Esc, the CSI sequences terminals send for
// arrows/Home/End/Delete/PageUp/PageDown/F5-F12/bracketed-paste, SS3 F1-F4,
// SGR mouse reports ("ESC [ < Cb;Cx;Cy M/m"), and Alt+<key> chords (a real
// keyboard sends ESC immediately followed by the key byte, with nothing else
// buffered in between). Escape sequences arrive as one burst, so if nothing
// is already buffered after the ESC byte, it's a bare Esc rather than the
// start of a sequence.
func (t *Terminal) readEscapeSequence() (types.Key, types.MouseEvent, eventKind, bool) {
	esc := func() (types.Key, types.MouseEvent, eventKind, bool) {
		return types.Key{Special: types.KeyEsc}, types.MouseEvent{}, eventKey, true
	}
	if t.in.Buffered() == 0 {
		return esc()
	}
	b2, err := t.in.ReadByte()
	if err != nil {
		return esc()
	}
	if b2 == 'O' {
		// SS3: F1-F4.
		b3, err := t.in.ReadByte()
		if err != nil {
			return esc()
		}
		switch b3 {
		case 'P':
			return types.Key{Special: types.KeyF1}, types.MouseEvent{}, eventKey, true
		case 'Q':
			return types.Key{Special: types.KeyF2}, types.MouseEvent{}, eventKey, true
		case 'R':
			return types.Key{Special: types.KeyF3}, types.MouseEvent{}, eventKey, true
		case 'S':
			return types.Key{Special: types.KeyF4}, types.MouseEvent{}, eventKey, true
		default:
			return esc()
		}
	}
	if b2 != '[' {
		// Not a CSI/SS3 sequence: this is Alt+<key> — a real keyboard sends
		// ESC immediately followed by the key's own byte. A single-byte read
		// is an acceptable simplification (a literal Alt+<multi-byte-UTF8>
		// chord from a real keyboard is exceedingly rare).
		return types.Key{Rune: rune(b2), Alt: true}, types.MouseEvent{}, eventKey, true
	}
	b3, err := t.in.ReadByte()
	if err != nil {
		return esc()
	}
	if b3 == '<' {
		return t.readMouseReport()
	}
	switch b3 {
	case 'Z':
		// Shift-Tab (CBT). A distinct key so a focused grab-all pane can
		// receive it; otherwise the render loop uses it to cycle focus back.
		return types.Key{Special: types.KeyBackTab}, types.MouseEvent{}, eventKey, true
	case 'A':
		return types.Key{Special: types.KeyUp}, types.MouseEvent{}, eventKey, true
	case 'B':
		return types.Key{Special: types.KeyDown}, types.MouseEvent{}, eventKey, true
	case 'C':
		return types.Key{Special: types.KeyRight}, types.MouseEvent{}, eventKey, true
	case 'D':
		return types.Key{Special: types.KeyLeft}, types.MouseEvent{}, eventKey, true
	case 'H':
		return types.Key{Special: types.KeyHome}, types.MouseEvent{}, eventKey, true
	case 'F':
		return types.Key{Special: types.KeyEnd}, types.MouseEvent{}, eventKey, true
	}
	if b3 >= '0' && b3 <= '9' {
		return t.readCSINumeric(b3)
	}
	return esc()
}

// readCSINumeric accumulates a numeric CSI parameter (digits already started
// by first) up to its terminating '~', then maps the code to a Key. Codes
// with no tuigo equivalent are dropped silently (return ok=true with a
// zero-value key/no dispatch upstream would be wrong, so instead we recurse
// into reading the NEXT event rather than emit a bogus key) — simplest safe
// behavior for a rare unmapped CSI code.
func (t *Terminal) readCSINumeric(first byte) (types.Key, types.MouseEvent, eventKind, bool) {
	n := int(first - '0')
	for {
		b, err := t.in.ReadByte()
		if err != nil {
			return types.Key{Special: types.KeyEsc}, types.MouseEvent{}, eventKey, true
		}
		if b >= '0' && b <= '9' {
			n = n*10 + int(b-'0')
			continue
		}
		if b == '~' {
			break
		}
		// Unexpected terminator (e.g. a ';'-separated param this table
		// doesn't need) — bail out to a bare Esc rather than misroute.
		return types.Key{Special: types.KeyEsc}, types.MouseEvent{}, eventKey, true
	}
	switch n {
	case 3:
		return types.Key{Special: types.KeyDelete}, types.MouseEvent{}, eventKey, true
	case 5:
		return types.Key{Special: types.KeyPageUp}, types.MouseEvent{}, eventKey, true
	case 6:
		return types.Key{Special: types.KeyPageDown}, types.MouseEvent{}, eventKey, true
	case 15:
		return types.Key{Special: types.KeyF5}, types.MouseEvent{}, eventKey, true
	case 17:
		return types.Key{Special: types.KeyF6}, types.MouseEvent{}, eventKey, true
	case 18:
		return types.Key{Special: types.KeyF7}, types.MouseEvent{}, eventKey, true
	case 19:
		return types.Key{Special: types.KeyF8}, types.MouseEvent{}, eventKey, true
	case 20:
		return types.Key{Special: types.KeyF9}, types.MouseEvent{}, eventKey, true
	case 21:
		return types.Key{Special: types.KeyF10}, types.MouseEvent{}, eventKey, true
	case 23:
		return types.Key{Special: types.KeyF11}, types.MouseEvent{}, eventKey, true
	case 24:
		return types.Key{Special: types.KeyF12}, types.MouseEvent{}, eventKey, true
	case 200:
		return t.readBracketedPaste()
	default:
		// Unmapped digit-CSI code (e.g. Insert=2): drop silently by reading
		// the next real event rather than emitting a bogus key.
		return t.readEvent()
	}
}

// bracketedPasteEnd is the literal terminator byte sequence "ESC [ 201 ~".
var bracketedPasteEnd = []byte{0x1b, '[', '2', '0', '1', '~'}

// readBracketedPaste reads raw bytes (no key decoding) until it sees the
// literal bracketed-paste end marker, then emits ONE KeyPaste event carrying
// everything before the marker — so a paste is delivered atomically, never
// split across events/frames.
func (t *Terminal) readBracketedPaste() (types.Key, types.MouseEvent, eventKind, bool) {
	var buf []byte
	for {
		b, err := t.in.ReadByte()
		if err != nil {
			return types.Key{Special: types.KeyPaste, Paste: string(buf)}, types.MouseEvent{}, eventKey, true
		}
		buf = append(buf, b)
		if len(buf) >= len(bracketedPasteEnd) && bytesHaveSuffix(buf, bracketedPasteEnd) {
			buf = buf[:len(buf)-len(bracketedPasteEnd)]
			return types.Key{Special: types.KeyPaste, Paste: string(buf)}, types.MouseEvent{}, eventKey, true
		}
	}
}

func bytesHaveSuffix(b, suffix []byte) bool {
	if len(b) < len(suffix) {
		return false
	}
	tail := b[len(b)-len(suffix):]
	for i := range suffix {
		if tail[i] != suffix[i] {
			return false
		}
	}
	return true
}

// readMouseReport parses the body of an SGR mouse sequence after "ESC [ <":
// digits and ';' up to a terminating 'M' (press/motion) or 'm' (release).
func (t *Terminal) readMouseReport() (types.Key, types.MouseEvent, eventKind, bool) {
	var body strings.Builder
	var final byte
	for {
		b, err := t.in.ReadByte()
		if err != nil {
			return types.Key{}, types.MouseEvent{}, eventMouse, false
		}
		if b == 'M' || b == 'm' {
			final = b
			break
		}
		body.WriteByte(b)
	}
	parts := strings.SplitN(body.String(), ";", 3)
	if len(parts) != 3 {
		return types.Key{}, types.MouseEvent{}, eventMouse, true // malformed; drop silently, keep reading
	}
	cb, _ := strconv.Atoi(parts[0])
	cx, _ := strconv.Atoi(parts[1])
	cy, _ := strconv.Atoi(parts[2])

	m := types.MouseEvent{X: cx - 1, Y: cy - 1} // wire format is 1-indexed
	switch {
	case cb&64 != 0:
		if cb&1 == 0 {
			m.Button = types.MouseWheelUp
		} else {
			m.Button = types.MouseWheelDown
		}
		m.Action = types.MousePress
	default:
		switch cb & 3 {
		case 0:
			m.Button = types.MouseLeft
		case 1:
			m.Button = types.MouseMiddle
		case 2:
			m.Button = types.MouseRight
		}
		switch {
		case final == 'm':
			m.Action = types.MouseRelease
		case cb&32 != 0:
			m.Action = types.MouseMove
		default:
			m.Action = types.MousePress
		}
	}
	return types.Key{}, m, eventMouse, true
}

// Resizes starts a goroutine listening for terminal size changes and returns
// the channel it sends the new Size on. On Unix it uses SIGWINCH; on Windows
// it polls the console size every 500ms.
func (t *Terminal) Resizes() <-chan Size {
	return t.resizes()
}

// Flush writes only the changed cells to the terminal, tracking cursor
// position and SGR state across calls so it moves the cursor and re-emits
// style codes only when they actually change.
func (t *Terminal) Flush(patches []Patch) {
	if len(patches) == 0 {
		return
	}
	var b strings.Builder
	// Hide the real cursor while writing cells so a shown cursor doesn't
	// visibly skate across the screen as cells are drawn; SetCursor (called
	// right after Flush) re-shows it at the child's cell. No-flicker frame:
	// hide, draw, move+show.
	if t.cursorShown {
		b.WriteString("\x1b[?25l")
		t.cursorShown = false
	}
	for _, p := range patches {
		if !t.havePos || p.Y != t.lastY || p.X != t.lastX {
			fmt.Fprintf(&b, "\x1b[%d;%dH", p.Y+1, p.X+1)
		}
		style := cellStyle{p.Cell.Fg, p.Cell.Bg, p.Cell.Bold, p.Cell.Italic, p.Cell.Underline}
		if !t.haveStyle || style != t.lastStyle {
			writeSGR(&b, style)
			t.lastStyle = style
			t.haveStyle = true
		}
		b.WriteRune(p.Cell.Rune)
		t.lastX, t.lastY = p.X+1, p.Y
		t.havePos = true
	}
	t.out.WriteString(b.String())
}

// SetCursor positions the real hardware cursor and shows or hides it. The
// render loop calls it once per frame after Flush: when a focused TerminalPane
// reports a live cursor tuigo shows the terminal cursor at the child's absolute
// cell (so an embedded agent's cursor is the real, blinking one); otherwise it
// hides it. When visible it always repositions (cell-drawing moved the cursor)
// and un-hides only on a hidden→shown transition; when not visible it hides
// only on a shown→hidden transition — so idle frames emit nothing.
func (t *Terminal) SetCursor(x, y int, visible bool) {
	seq, nowShown := cursorSeq(x, y, visible, t.cursorShown)
	t.cursorShown = nowShown
	if visible {
		// We moved the cursor away from where the last cell landed; make the
		// next Flush reposition before it writes.
		t.havePos = false
	}
	if seq != "" {
		t.out.WriteString(seq)
	}
}

// cursorSeq is SetCursor's pure escape-builder: given the target cell, whether
// the cursor should be visible, and whether it's currently shown, it returns
// the escape burst to write and the new shown-state. A visible cursor always
// repositions (cell-drawing moved it) and un-hides only on hidden→shown; an
// invisible cursor hides only on shown→hidden, so idle frames emit nothing.
func cursorSeq(x, y int, visible, currentlyShown bool) (seq string, nowShown bool) {
	if visible {
		var b strings.Builder
		fmt.Fprintf(&b, "\x1b[%d;%dH", y+1, x+1)
		if !currentlyShown {
			b.WriteString("\x1b[?25h")
		}
		return b.String(), true
	}
	if currentlyShown {
		return "\x1b[?25l", false
	}
	return "", false
}

func writeSGR(b *strings.Builder, s cellStyle) {
	b.WriteString("\x1b[0m")
	if s.Bold {
		b.WriteString("\x1b[1m")
	}
	if s.Italic {
		b.WriteString("\x1b[3m")
	}
	if s.Underline {
		b.WriteString("\x1b[4m")
	}
	if s.Fg != 0 {
		if s.Fg.IsRGB() {
			r, g, col := s.Fg.RGB()
			fmt.Fprintf(b, "\x1b[38;2;%d;%d;%dm", r, g, col)
		} else {
			fmt.Fprintf(b, "\x1b[38;5;%dm", int(s.Fg))
		}
	}
	if s.Bg != 0 {
		if s.Bg.IsRGB() {
			r, g, col := s.Bg.RGB()
			fmt.Fprintf(b, "\x1b[48;2;%d;%d;%dm", r, g, col)
		} else {
			fmt.Fprintf(b, "\x1b[48;5;%dm", int(s.Bg))
		}
	}
}
