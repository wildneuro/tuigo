# Changelog

## Unreleased - v0.1.19

- **Hotkeys.** New `OnHotkey(k SpecialKey, handler func())` option — sugar
  over `Global()(OnSpecialKey(...))` — plus the free C0 control bytes are now
  parsed as distinct `SpecialKey`s: Ctrl-Space, Ctrl-G, Ctrl-\, Ctrl-],
  Ctrl-^, Ctrl-_.

## v0.1.18 - 2026-07-17

(v0.1.17 was tagged as a placeholder maintenance release with no changes;
this version carries the actual work.)

Closes the three known input gaps from v0.1.16: mouse never reached the
embedded child, paste wasn't handled specially, and F-keys/Alt-letter chords
were lost.

- **Mouse passthrough to the focused pane's child.** When the FOCUSED
  `TerminalPane`'s child has itself enabled mouse reporting (new
  `Screen.MouseMode() (enabled, sgr bool)`, backed by vt10x's mode bits),
  host mouse events are forwarded to the child's PTY as SGR reports
  (`ESC [ < Cb;Cx;Cy M/m`, wheel included). Coordinates are translated from
  absolute screen cells to pane-relative, 1-indexed cells using the pane's
  last-painted origin (tracked in `paint()`); an event whose translated
  coordinate falls outside the pane's last box size is dropped. If the child
  hasn't opted into mouse reporting, forwarding is a silent no-op — same as a
  real terminal toward an app that never asked for mouse events. New pure
  helper `encodeSGRMouse` (mouse.go) is the inverse of
  `renderer.readMouseReport`.
- **Bracketed paste, delivered atomically.** `renderer.Terminal` now decodes
  a full `ESC [ 200~ ... ESC [ 201~` burst as ONE `types.KeyPaste` event
  carrying the inner text (markers stripped) in the new `Key.Paste` field —
  never split across events/frames. `TerminalPane.keyToBytes` re-wraps it in
  the bracketing markers when forwarding to the child, so the wire format
  round-trips byte-for-byte.
- **F1-F12, PageUp/PageDown, and Alt-letter chords.** The terminal's escape
  reader now generalizes to a full numeric-CSI parser (`ESC [ <n> ~`) instead
  of the old hardcoded single-digit Delete case, adding PageUp/PageDown and
  F5-F12; F1-F4 arrive via SS3 (`ESC O P/Q/R/S`). New `types.Key.Alt` field
  fixes a bug where `ESC` followed by a non-`[`/`O` byte (a real keyboard's
  Alt+<key> chord) was silently dropped as a bare Esc — it now decodes as
  `Key{Rune: <byte>, Alt: true}`. `keyToBytes` gained matching cases for
  every new `SpecialKey` plus the Alt-rune re-prefix. Ctrl-letter chords
  (other than the pre-existing Ctrl-C/Ctrl-O/Enter/Tab/Backspace
  special-cases) needed no new code — they already round-trip via the
  generic raw-control-byte path.
- New `SpecialKey` constants (appended after `KeyCtrlO`, so existing values
  are unchanged): `KeyPageUp`, `KeyPageDown`, `KeyF1`..`KeyF12`, `KeyPaste`.
  New `Key` fields: `Alt bool`, `Paste string`. New `Screen` method:
  `MouseMode() (enabled, sgr bool)`.
- **Known remaining gap (unchanged by this release):** DECSCUSR / cursor
  SHAPE passthrough is still NOT addressed — vt10x doesn't model cursor
  shape, so a focused pane's real hardware cursor always renders at the
  terminal's default shape regardless of what the child requested.

## v0.1.16 - 2026-07-17

Compositor polish for embedding a DEMANDING full-screen child (e.g. Claude
Code) in a `TerminalPane` and overlaying a dialog on it without corruption.
New example `_examples/compositor` is the end-to-end proof.

- **FIX 1 — Tab reaches the focused pane.** A focused element can now GRAB
  input (`Element.GrabKeys`, set via the new `tuigo.GrabInput()` option;
  `TerminalPane` sets it by default): while it holds focus, Tab and Shift-Tab
  are dispatched TO it instead of being consumed for focus cycling, so an
  embedded agent that uses Tab/Shift-Tab for its own modes receives them.
  Focus is instead switched by a distinct global chord `tuigo.FocusCycleKey`
  (**default Ctrl-O**, overridable) which always cycles focus, even over a
  grab-all pane. New special keys `KeyBackTab` (Shift-Tab, CSI `ESC [ Z`) and
  `KeyCtrlO` (byte `0x0f`) are decoded by the terminal and mapped back to
  their VT bytes for the child (`\x1b[Z`).
- **FIX 2 — real hardware cursor for the focused pane.** Instead of drawing a
  reverse-video block, a focused `TerminalPane` publishes its child cursor's
  absolute cell and the render loop UN-hides and positions the real terminal
  cursor there (via the new `renderer.(*Terminal).SetCursor`), hiding it again
  when the pane is unfocused, a non-pane element is focused, or an overlay
  covers the cell. No flicker: the cursor is hidden while cells are flushed and
  shown+positioned once per frame. (vt10x exposes no cursor SHAPE, so shape is
  left as the terminal default.)
- **FIX 3 — dialog OVER a pane with clean restore.** The compositor owns the
  composite and repaints the pane from its `Screen` every frame, so a floating
  `Ctx.Overlay` (e.g. a `Menu`) stamped on top of a pane wins where it overlaps
  and, when it closes, the pane's region repaints with NO artifacts — the diff
  restores exactly the covered cells. Verified by test.
- new example `_examples/compositor`: a `bash` (or any argv) pane filling the
  screen above a focusable status bar, with `Ctrl-O` to switch focus and a
  `Menu` dialog opened over the live shell that closes back to an intact pane.
- tests: focus routing (grab pane keeps Tab; chord cycles), cursor-visibility
  decision (focused pane shows at computed absolute cell; unfocused/covered
  hidden), the `SetCursor` escape state machine, and dialog-over-pane
  compositing + restore. Full suite green under `-race`.

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
