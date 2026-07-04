# tuigo — A Go TUI framework inspired by Ink

A declarative, component-based terminal UI framework for Go, modeled after
[Ink](https://github.com/vadimdemedes/ink) (React for CLIs).

## Philosophy

tuigo should make terminal interfaces feel like small, durable applications,
not piles of cursor math. The user writes a pure-ish component tree, tuigo owns
the mechanical work: state, layout, event delivery, cell diffs, terminal mode,
and cleanup.

The framework should stay Go-shaped. No JSX, no code generation, no hidden
runtime magic beyond the minimum needed for hooks. Composition should look like
ordinary Go function calls, and the public API should be easy to inspect in an
editor without learning a second language.

The implementation should prefer boring, testable layers:

- Element construction is just data.
- Reconciliation is deterministic tree comparison.
- Layout is pure geometry.
- Rendering turns styled boxes and text into cells.
- Terminal I/O is the thin, impure shell around the system.

When in doubt, choose the smaller primitive that can be explained precisely.
Ink is the inspiration, but Go constraints are allowed to shape the result.

## Module

- Module path: `github.com/wildneuro/tuigo`.
- Minimum Go version: 1.22 (needed for comfortable generic hook helpers).
- `reconciler/`, `layout/`, and `renderer/` stay top-level, importable packages —
  they are useful in isolation (e.g. testing layout without a terminal, or
  building alternate renderers), so nothing moves under `internal/`.
- A `.github/workflows/ci.yml` should run the CI Expectations commands
  (`go test ./...`, `go test -race ./...`, `go vet ./...`, `gofmt -l .`) on
  every push/PR, added alongside step 1 below.

## Build Order

1. Scaffold the module and package layout.
2. Implement static elements: `Box`, `Text`, `Fragment`, options, and styles.
3. Implement an in-memory layout/render path that produces a `renderer.Buffer`.
4. Add buffer diffing and ANSI/tcell flushing.
5. Add the app loop: mount, render, wait for input, rerender, exit cleanly.
6. Add keyboard events and resize events.
7. Add `Ctx.UseState` and `Ctx.UseReducer`.
8. Expand examples and harden behavior with tests.

The first useful milestone is not a full framework. It is a static component
tree rendered into a cell buffer with deterministic tests. Interactivity comes
after the render path is solid.

## Core Loop

```
App(Component) → mount → render loop:
  1. Call Component() → returns Element tree (the "VDOM")
  2. Reconciler: diff new tree vs previous tree → patch list
  3. Layout: pure Go flexbox → (x, y, w, h) per node
  4. Render: apply patches to cell buffer → flush ANSI diffs
  5. Wait for event (keypress, resize, timer) → goto 1
```

## Package Layout

```
tuigo/
├── tuigo.go         # App(), Render(), public API
├── component.go     # Component interface (Render() Element)
├── node.go          # Element types: Box, Text, Fragment
├── style.go         # Flexbox style props (Padding, Gap, etc.)
├── state.go         # useState / useReducer hooks
├── event.go         # Key, Mouse, Resize event types
├── reconciler/
│   └── reconciler.go  # O(n) tree diff → insert/remove/update/move
├── layout/
│   └── flexbox.go     # Pure Go flexbox → position each node
├── renderer/
│   ├── cell.go        # Cell{Char, Fg, Bg, Bold, Italic…}
│   ├── buffer.go      # 2D cell grid + diff against previous
│   └── terminal.go    # tcell stdin/stdout + ANSI flush
└── _examples/
    └── counter/main.go
```

## Key Design Decisions

| Decision | Choice | Why |
|----------|--------|-----|
| JSX?         | **No** — function composition     | Go has no JSX; a precompiler adds complexity for marginal gain |
| Hooks?       | **Yes** — ctx.UseState(initial)   | Needed for ergonomic state, same trick Ink/React use |
| Layout engine| **Pure Go flexbox**                | ~500 lines, avoids CGo and cross-compile headaches |
| Terminal     | **tcell**                          | Mature, cross-platform, handles raw mode, resize, mouse |
| Diff         | **Per-cell** (not per-ANSI-op)     | Simpler, fast enough for terminal frame rates |

## API Shape

Prefer a concrete `Element` data structure over a deep interface hierarchy.
Constructors and options should keep user code pleasant while leaving internal
code easy to test.

Possible internal shape:

```go
type Element struct {
    Type     ElementType
    Key      string
    Text     string
    Style    Style
    Props    Props
    Children []Element
}
```

Event handlers can live in props/options during construction, then be collected
by the app loop after each render. That keeps the element tree declarative while
letting the runtime maintain the active keymap.

### Render Scheduling

Calling a state setter does not rerender synchronously. Like React, tuigo
batches every `set()` call that happens within one event tick (one keypress,
one resize, one timer fire) into a single rerender scheduled after the current
handler returns. This keeps `n+1` `+` handlers safe to call multiple times
per tick without redundant renders, and matches the mental model users bring
from React/Ink.

### Panic and Error Policy

`Component()` panics and render-loop errors must never leave the terminal in
raw mode or with the cursor hidden. The app loop recovers from panics at the
top of each render, restores the terminal (raw mode off, cursor shown, alt
screen exited), and then re-panics/returns the error to the caller of
`Render()`. Cleanup is a `defer` registered once at terminal setup, not
duplicated per panic site.

### Key Event Vocabulary

`OnKey` accepts a rune for plain character keys. Special keys (arrows, Enter,
Esc, Tab, Backspace, Delete, Home/End, F-keys) and modifiers (Ctrl, Alt,
Shift where the terminal reports them) are represented as a distinct `Key`
type with named constants (`KeyEnter`, `KeyCtrlC`, `KeyUp`, ...), so handlers
can match either a rune or a named key without string parsing.

### Style Inheritance

Text styling (color, bold, italic, underline) cascades from an ancestor `Box`
down to `Text` children that don't set their own value, mirroring Ink's
cascading text props. Layout-affecting style (padding, margin, gap, border,
width/height, flex direction) never inherits — it only ever applies to the
node it's set on.

### Text Wrapping

`Text` wraps at word boundaries to fit its computed width by default; a style
option can switch to hard-clip-with-ellipsis for single-line use cases (status
bars, table cells). Wrapping height then feeds back into flex layout like any
other intrinsic content size.

## API Sketch

```go
func Counter(ctx *tuigo.Ctx) tuigo.Element {
    n, set := ctx.UseState(0)
    return Box(
        Padding(1), FlexColumn(),
        Text("count: %d", n),
        Text("press + to increment, q to quit"),
        OnKey('+', func() { set(n + 1) }),
        OnKey('q', func() { ctx.Exit() }),
    )
}

func main() {
    tuigo.Render(Counter)
}
```

## Reconciler Algorithm

```
diff(old, new):
  if old == nil          → INSERT new
  if type changed        → REPLACE
  if same key+type       → UPDATE props + diff children pairwise
  if no key              → diff by position
```

Synchronous full-rebuild every frame, diffed into patches. No fiber /
concurrent mode — Ink doesn't use it either.

## Renderer Path

```
Element tree → layout → []Row{[]Cell} → diff vs prev → emit ANSI
```

Each `Cell`: `{Rune, FgColor, BgColor, Bold, Italic, Underline}`.
The buffer is `([height][width]Cell)`. Diff computes per-cell changes,
renderer writes only changed cells as ANSI escape codes.

## v1 Scope (what we build)

- Box (flex container with padding/margin/gap/align/border)
- Text (styled text string)
- Fragment (invisible wrapper)
- useState / useReducer
- Keyboard events + stdin listener
- Resize handling
- Style system (colors, bold, underline, border)
- Per-cell diff renderer via tcell

## Not v1

- Mouse events
- Virtualized lists
- SSR / inline mode
- Context / portals
- Custom hooks (can be plain Go functions)

## Testing Plan

The test strategy follows the architecture: pure layers get dense unit tests,
the terminal shell gets focused integration tests, and examples act as small
end-to-end contracts.

### Unit Tests

- Element constructors:
  - `Box`, `Text`, and `Fragment` produce the expected tree shape.
  - Options apply in order and do not mutate shared state.
  - Keys and event handlers survive construction.
- Style:
  - Defaults are stable and zero-value friendly.
  - Padding, margin, gap, border, color, and text attributes compose correctly.
  - Invalid or unsupported values fail predictably if validation exists.
- Reconciler:
  - Insert, remove, replace, update, and move cases.
  - Positional child diffing when keys are absent.
  - Keyed child diffing when keys are present.
  - Type changes force replacement instead of unsafe reuse.
- Layout:
  - Row and column flex direction.
  - Padding, margin, gap, explicit width/height, and terminal bounds.
  - Alignment and overflow behavior.
  - Borders reserve space correctly.
  - Layout functions are deterministic and do not mutate input elements.
- Renderer buffer:
  - Text writes the expected runes and styles into cells.
  - Wide, combining, and clipped runes have explicit tests once supported.
  - Empty cells preserve default style.
  - Borders render exact characters.
- Diff:
  - Identical buffers produce no changes.
  - Single-cell changes produce one patch.
  - Style-only changes are detected.
  - Resize/full-clear cases are handled intentionally.
- Hooks:
  - `UseState` returns the initial value on first render.
  - Setters schedule a rerender and preserve state by hook index.
  - Multiple hooks keep independent slots.
  - `UseReducer` applies actions in order.
  - Hook misuse has a clear failure mode.

### Integration Tests

- App loop with a fake terminal:
  - Initial render writes the expected cells.
  - Key events call the active handler and trigger rerender.
  - `ctx.Exit()` stops the loop and restores terminal state.
  - Resize events update layout dimensions.
  - Cleanup runs when render returns an error.
- Reconciler + layout + renderer:
  - Changing state updates only the affected cells where possible.
  - Removing nodes clears their old screen area.
  - Reordering keyed children does not corrupt state.
- Terminal backend:
  - Use an interface around `tcell.Screen` so tests can run without a real TTY.
  - Keep raw terminal/manual smoke tests separate from normal `go test`.

### Golden Tests

Golden tests should snapshot rendered buffers, not raw ANSI, for most cases.
Cell snapshots are easier to read and less brittle than escape sequences.

Golden format: a plain text grid (one rune per cell, row per line) plus a
compact legend mapping distinct styles to a marker (e.g. `B` = bold, `1` =
color index 1), so a snapshot is readable directly in a diff without decoding
escape sequences.

Use golden files for:

- Basic counter layout.
- Nested boxes with padding/gap/border.
- Styled text.
- Resize from wide to narrow.
- Removing or replacing a subtree.

ANSI output can have a small number of golden tests at the renderer boundary to
verify cursor movement, style reset, and minimal diff emission.

### Property and Fuzz Tests

- Buffer diff round trip:
  - Applying `Diff(a, b)` to `a` should produce `b`.
- Layout bounds:
  - Generated element trees should never produce negative widths/heights.
  - Children should not render outside their computed parent bounds unless an
    overflow mode explicitly allows it.
- Reconciler stability:
  - Diffing any generated trees should not panic.
  - Applying patches should produce the same logical tree as the target tree.

### Example Tests

Every example under `_examples/` should compile in CI. Small examples should
also have a non-interactive test mode where they render one frame into a fake
screen.

Initial examples:

- `_examples/counter`: state and key handling.
- `_examples/layout`: row/column, padding, gap, border.
- `_examples/style`: colors and text attributes.
- `_examples/resize`: layout reacting to terminal dimensions.

### Manual Smoke Tests

Some behavior needs a real terminal:

- Raw mode enters and exits cleanly.
- Ctrl-C restores terminal state.
- Resize events arrive on common terminals.
- Colors and text attributes reset after exit.
- The cursor is hidden during render and restored on exit.

Keep these as documented commands until there is a reliable pseudo-terminal
harness.

### CI Expectations

CI should run:

```sh
go test ./...
go test -race ./...
go vet ./...
gofmt -w .
```

Before v1, add a small benchmark suite for buffer diffing and layout so changes
to the core loop have visible performance impact.
