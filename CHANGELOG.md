# Changelog

## v0.1.15 - 2026-07-17

- add `tuigo.TerminalPane(ctx, argv, opts...)`: a tuigo element that EMBEDS a
  child process running in a PTY and composites its live screen as a
  rectangular region of the layout — the core of turning tuigo into a terminal
  compositor (tmux/zellij-as-a-library). It spawns argv in a PTY
  (`github.com/creack/pty`, pure-Go/CGO-free), models the child's output with a
  `tuigo.Screen` (vt10x), and paints its cell grid every frame.
  - OUTPUT: a background goroutine copies child PTY output into the Screen and
    calls the new `Ctx.Wake()` to make the render loop draw one more frame with
    the fresh content (the redraw-from-goroutine trigger — an empty job pushed
    onto the same timers channel `Ctx.After` feeds).
  - RESIZE: the pane reflows the child to its flex-allocated box —
    `Screen.Resize` + `pty.Setsize` — whenever the laid-out size changes.
  - INPUT: a non-global Any key handler forwards translated keystrokes to the
    child's PTY stdin; tuigo's focus dispatch fires it ONLY on the focused
    element, so an unfocused pane receives nothing (focus gating for free).
    `Ctx.IsFocused(key)` gates the block cursor drawn at the child's cursor.
  - LIFECYCLE: `Ctx.OnCleanup` kills+reaps the child and closes the PTY on
    unmount; `OnPaneExit(fn)` surfaces the child's own exit to a parent.
- add the `ElementTypeCanvas` element type + `Element.Paint(types.Surface)`
  hook: a leaf that paints its own cells into its laid-out rect (flex like a
  Box, no children). The renderer supplies a clipped `Surface`; TerminalPane is
  built on it. New `Ctx` helpers: `Wake`, `IsFocused`, `OnCleanup`. New
  `Screen` accessors: `Size`, `Cursor`, `CellAt`.
- new example `_examples/termpane`: a minimal compositor embedding a live
  `bash`/`vim` beside a status column.

## v0.1.14 - 2026-07-17

- add `tuigo.Screen` + the **pages** concept: save/restore the VISIBLE SCREEN
  behind a dialog, the way tmux popups (capture-pane) do. `Screen` is a
  cell-grid terminal emulator that ingests a child's output byte stream via
  `Write` (io.Writer); `PushPage()` saves the current visible screen onto a
  page STACK and `PopPage(w)` repaints the top saved page — so a dialog over a
  dialog nests cleanly (push, push, pop, pop). The lower-level `Snapshot()` /
  `(*ScreenSnapshot).Restore(w)` pair is kept for callers that manage a page
  value themselves. `Resize(rows, cols)` tracks SIGWINCH.
- `Screen` WRAPS the pure-Go (CGO-free) `github.com/hinshun/vt10x` emulator for
  the full escape-sequence long tail (SGR 16/256/truecolor + bold/italic/
  underline/reverse, cursor motion, erase, scroll region, alt-screen, …) and
  adds a faithful grid → SGR repaint plus UTF-8-boundary buffering (a multibyte
  rune split across two `Write`s — e.g. at a PTY read-buffer edge — is held
  back and completed on the next `Write`, which vt10x alone drops).
- restoring the fixed rows×cols viewport is BOUNDED work and never pollutes
  scrollback, unlike replaying the child's raw output stream.

## v0.1.13 - 2026-07-17

- add `RenderInline` / `renderer.NewInlineTerminal`: a NO-ALT-SCREEN render
  mode that renders on the MAIN screen and never emits the alt-screen
  enter/leave (mode 1049), so a host that already owns an alternate screen
  (e.g. a PTY wrapper around a full-screen child) keeps it untouched. Cleans
  up its own main-screen lines on exit.
- readLoop no longer leaks: `Close` signals the stdin reader (via a `done`
  channel) so a goroutine parked on delivering an event exits instead of
  blocking forever on a channel nobody reads after Render returns.

## v0.1.12 - 2026-07-04

- maintenance release (no notable changes)

## v0.1.11 - 2026-07-04

- demo.tape

## v0.1.10 - 2026-07-04

- maintenance release (no notable changes)

## v0.1.9 - 2026-07-04

- release.sh

## v0.1.8 - 2026-07-04

- maintenance release (no notable changes)

## v0.1.7 - 2026-07-04

- release v0.1.6
- release v0.1.6
- release v0.1.5
- release v0.1.4
- release v0.1.3
- docs: add README
- release v0.1.2
- release v0.1.1
- 4th of July
- 4th of July
- Initial commit
