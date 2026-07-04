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

	closeOnce sync.Once

	havePos      bool
	lastX, lastY int
	haveStyle    bool
	lastStyle    cellStyle
}

type cellStyle struct {
	Fg, Bg                  types.Color
	Bold, Italic, Underline bool
}

// NewTerminal puts stdin into raw mode, enters the alternate screen with the
// cursor hidden, enables SGR mouse reporting, and starts the single
// goroutine that decodes stdin into key/mouse events. Callers must defer
// Close to restore the terminal, even on panic.
func NewTerminal() (*Terminal, error) {
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
	}
	// Alt screen + hide cursor + SGR extended mouse reporting (modes 1000
	// and 1006), which reports every press/release/drag with unambiguous
	// coordinates past column/row 223.
	t.out.WriteString("\x1b[?1049h\x1b[?25l\x1b[?1000h\x1b[?1006h")
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
		t.out.WriteString("\x1b[?1006l\x1b[?1000l\x1b[?25h\x1b[?1049l")
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
		switch kind {
		case eventKey:
			t.keyCh <- key
		case eventMouse:
			t.mouseCh <- mouse
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

// readEscapeSequence handles a lone Esc, the small set of CSI sequences
// terminals send for arrows/Home/End/Delete, and SGR mouse reports
// ("ESC [ < Cb;Cx;Cy M/m"). Escape sequences arrive as one burst, so if
// nothing is already buffered after the ESC byte, it's a bare Esc rather
// than the start of a sequence.
func (t *Terminal) readEscapeSequence() (types.Key, types.MouseEvent, eventKind, bool) {
	esc := func() (types.Key, types.MouseEvent, eventKind, bool) {
		return types.Key{Special: types.KeyEsc}, types.MouseEvent{}, eventKey, true
	}
	if t.in.Buffered() == 0 {
		return esc()
	}
	b2, err := t.in.ReadByte()
	if err != nil || b2 != '[' {
		return esc()
	}
	b3, err := t.in.ReadByte()
	if err != nil {
		return esc()
	}
	if b3 == '<' {
		return t.readMouseReport()
	}
	switch b3 {
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
	case '3':
		t.in.ReadByte() // consume the trailing '~' of "ESC [ 3 ~"
		return types.Key{Special: types.KeyDelete}, types.MouseEvent{}, eventKey, true
	default:
		return esc()
	}
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
