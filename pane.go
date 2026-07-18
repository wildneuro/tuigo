package tuigo

// pane.go — TerminalPane: a tuigo element that EMBEDS a child process running
// in a PTY and composites its live screen as a rectangular region of the
// layout. This is what turns tuigo into a terminal compositor
// (tmux/zellij-as-a-library): the pane owns a child, drives it with the pane's
// laid-out size, paints its cells every frame, and forwards keystrokes to it
// while focused.
//
// How the pieces connect to tuigo's existing machinery:
//   - RENDER: the pane is an ElementTypeCanvas whose Paint callback copies the
//     child's Screen cell grid onto the Surface the renderer hands it. The
//     Surface's Bounds() is the pane's laid-out box, so the pane always paints
//     to (and sizes the child to) exactly its flex-allocated rect.
//   - OUTPUT → SCREEN: pty.Start gives a master fd; a background goroutine
//     copies child output into a tuigo.Screen (vt10x-backed) and calls
//     Ctx.Wake() to make the render loop draw one more frame with the new
//     content.
//   - RESIZE: Paint compares the box size to the last size and, on a change,
//     Screen.Resize + pty.Setsize so the child reflows to the pane.
//   - INPUT: an Any key handler (NON-global) forwards translated keystrokes to
//     the child's PTY stdin. tuigo's focus dispatch (dispatchFocusedKey) only
//     fires it on the FOCUSED element, so an unfocused pane receives nothing —
//     the focus gating is free.
//   - LIFECYCLE: Ctx.OnCleanup kills+reaps the child and closes the PTY when
//     the pane is unmounted; the reader goroutine exits on the resulting EOF.
//     An OnPaneExit option surfaces the child's own exit to a parent.

import (
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
	"github.com/wildneuro/tuigo/types"
)

// A focused TerminalPane now shows the REAL hardware cursor at the child's
// cursor cell (see cursor.go / Terminal.SetCursor) rather than drawing a
// reverse-video block: the TerminalPane's Paint publishes the child cursor's
// absolute cell into the frame's cursor slot, and the render loop un-hides and
// positions the terminal cursor there when the pane is focused (and no overlay
// covers it), hiding it otherwise.

// paneState is a TerminalPane's per-instance runtime, persisted across renders
// via UseState. The Screen is shared between the render goroutine (Resize +
// CellAt in paint) and the reader goroutine (Write), so every access is guarded
// by mu.
type paneState struct {
	mu     sync.Mutex
	screen *Screen

	in      io.Writer // child PTY master: pane input → child stdin
	setsize func(rows, cols int) error
	closeFn func()

	rows, cols int // last size the child/screen were sized to
	cmd        *exec.Cmd
	ptmx       *os.File

	wake   func()      // ask the render loop to draw a frame
	onExit func(error) // parent callback fired once when the child exits

	closed  bool
	exited  bool
	exitErr error
}

// OnPaneExit registers a callback fired once when a TerminalPane's child
// process exits (or fails to start). The error is the child's exit error, or
// nil for a clean exit. A parent typically reacts by exiting the app or
// replacing the pane.
func OnPaneExit(fn func(error)) Option {
	return func(e *Element) {
		if e.Props == nil {
			e.Props = types.Props{}
		}
		e.Props["tuigo.paneExit"] = fn
	}
}

// TerminalPane spawns argv[0] (with argv[1:] as arguments) in a PTY and returns
// a Canvas element that composites the child's live screen into the tuigo
// layout. It must be called from inside a component (it uses ctx hooks). Give
// it a stable WithKey so focus (and thus input routing) targets it; it is
// Focusable by default. Standard layout options (Width, Height, flex) apply as
// they do to a Box; OnPaneExit surfaces the child's exit.
//
//	tuigo.TerminalPane(ctx, []string{"bash"}, tuigo.WithKey("term"))
func TerminalPane(ctx *Ctx, argv []string, opts ...Option) Element {
	// GrabKeys by default: while focused, the pane receives Tab/Shift-Tab (an
	// embedded agent uses them) instead of the render loop cycling focus.
	// Focus is still switchable with the global chord (tuigo.FocusCycleKey,
	// default Ctrl-O).
	e := Element{Type: types.ElementTypeCanvas, Focusable: true, GrabKeys: true}
	for _, opt := range opts {
		opt(&e)
	}
	key := e.Key

	st, setSt := UseState[*paneState](ctx, (*paneState)(nil))
	if st == nil {
		st = &paneState{screen: NewScreen(24, 80), wake: ctx.Wake}
		if e.Props != nil {
			if fn, ok := e.Props["tuigo.paneExit"].(func(error)); ok {
				st.onExit = fn
			}
		}
		st.start(argv)
		setSt(st)
		ctx.OnCleanup(st.close)
	}

	e.Paint = func(s types.Surface) {
		st.paint(s)
		// When focused, publish the child's cursor cell so the render loop
		// shows the real hardware cursor there (cursor.go). Unfocused panes
		// publish nothing, so the cursor stays hidden.
		if ctx.IsFocused(key) {
			bx, by, bw, bh := s.Bounds()
			st.mu.Lock()
			cx, cy, vis := st.screen.Cursor()
			st.mu.Unlock()
			publishPaneCursor(ctx.app, true, bx, by, bw, bh, cx, cy, vis)
		}
	}
	e.Handlers = append(e.Handlers, types.KeyHandler{
		Any:    true,
		Handle: func(k types.Key) { st.writeKey(k) },
	})
	return e
}

// start spawns the child in a PTY and launches the output reader. On failure it
// records the exit so the pane paints an (empty) screen and the parent is
// notified rather than the app crashing.
func (p *paneState) start(argv []string) {
	if len(argv) == 0 {
		argv = []string{"/bin/sh"}
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	ptmx, err := pty.Start(cmd)
	if err != nil {
		p.markExit(err)
		return
	}
	p.cmd = cmd
	p.ptmx = ptmx
	p.in = ptmx
	p.setsize = func(rows, cols int) error {
		return pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	}
	p.closeFn = func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = ptmx.Close()
		_ = cmd.Wait()
	}
	// Give the child a sane initial size before the first paint reflows it.
	_ = p.setsize(24, 80)
	go p.run(ptmx)
}

// run copies child output into the Screen, waking the render loop after each
// chunk, and records the child's exit when the stream ends. It owns all Writes
// to the Screen; paint owns Resize/CellAt — both under mu.
func (p *paneState) run(r io.Reader) {
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			p.mu.Lock()
			p.screen.Write(buf[:n])
			p.mu.Unlock()
			if p.wake != nil {
				p.wake()
			}
		}
		if err != nil {
			p.markExit(err)
			return
		}
	}
}

// paint copies the Screen's cell grid onto the pane's Surface, reflowing the
// child to the box size when it changes. The child's cursor is drawn by the
// render loop as the real hardware cursor (see cursor.go), not painted here.
func (p *paneState) paint(s types.Surface) {
	bx, by, bw, bh := s.Bounds()
	if bw < 1 || bh < 1 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if bh != p.rows || bw != p.cols {
		p.rows, p.cols = bh, bw
		p.screen.Resize(bh, bw)
		if p.setsize != nil {
			_ = p.setsize(bh, bw)
		}
	}

	for row := 0; row < bh; row++ {
		for col := 0; col < bw; col++ {
			r, fg, bg, bold, italic, underline := p.screen.CellAt(col, row)
			s.Set(bx+col, by+row, r, fg, bg, bold, italic, underline)
		}
	}
}

// writeKey forwards a translated keystroke to the child's PTY stdin. It is
// only reached when the pane is focused (tuigo dispatches non-global key
// handlers to the focused element only), so unfocused panes receive nothing.
func (p *paneState) writeKey(k types.Key) {
	p.mu.Lock()
	w := p.in
	p.mu.Unlock()
	if w == nil {
		return
	}
	if b := keyToBytes(k); len(b) > 0 {
		_, _ = w.Write(b)
	}
}

// close kills and reaps the child and closes the PTY. Idempotent; registered
// with Ctx.OnCleanup so it runs when the pane is unmounted.
func (p *paneState) close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	fn := p.closeFn
	p.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// markExit records the child's exit exactly once, firing OnPaneExit and waking
// the loop so the parent's reaction renders. An io.EOF (clean stream end) is
// reported as a nil error.
func (p *paneState) markExit(err error) {
	p.mu.Lock()
	if p.exited {
		p.mu.Unlock()
		return
	}
	p.exited = true
	if err == io.EOF {
		err = nil
	}
	p.exitErr = err
	onExit := p.onExit
	p.mu.Unlock()

	if onExit != nil {
		onExit(err)
	}
	if p.wake != nil {
		p.wake()
	}
}

// keyToBytes translates a tuigo Key back into the bytes a terminal child
// expects on stdin. Special keys map to their canonical VT sequences; a plain
// rune is sent as its UTF-8 encoding. Tab ("\t") and Shift-Tab (CBT "\x1b[Z")
// DO reach a focused pane now — it grabs input by default (GrabKeys), so the
// render loop no longer steals them for focus cycling (that moved to the
// FocusCycleKey chord).
func keyToBytes(k types.Key) []byte {
	switch k.Special {
	case types.KeyEnter:
		return []byte{'\r'}
	case types.KeyEsc:
		return []byte{0x1b}
	case types.KeyTab:
		return []byte{'\t'}
	case types.KeyBackTab:
		return []byte("\x1b[Z")
	case types.KeyBackspace:
		return []byte{0x7f}
	case types.KeyDelete:
		return []byte("\x1b[3~")
	case types.KeyUp:
		return []byte("\x1b[A")
	case types.KeyDown:
		return []byte("\x1b[B")
	case types.KeyRight:
		return []byte("\x1b[C")
	case types.KeyLeft:
		return []byte("\x1b[D")
	case types.KeyHome:
		return []byte("\x1b[H")
	case types.KeyEnd:
		return []byte("\x1b[F")
	case types.KeyCtrlC:
		return []byte{0x03}
	}
	if k.Rune != 0 {
		return []byte(string(k.Rune))
	}
	return nil
}
