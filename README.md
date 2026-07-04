# tuigo

A declarative, component-based terminal UI framework for Go — modeled after
[Ink](https://github.com/vadimdemedes/ink) (React for CLIs), but built from
plain Go function calls. No JSX, no code generation, no virtual-DOM
reconciliation you don't need. Just `Box`, `Text`, hooks, and events.

![demo](demo.gif)

## Why tuigo

Most terminal UI is either imperative cursor math, or a port of a browser
idea (virtual DOM diffing) into a place that never needed it — a terminal
screen is a few thousand cells, not a million-node DOM tree. tuigo skips
that step on purpose: every frame is a full rebuild of a small element
tree, and only the **cells that actually changed** get written to the
terminal. Simpler pipeline, faster in practice, at the scale a TUI actually
runs at.

The design goal, stated plainly: **best-in-class for 2026** — event-driven
and property-based like Borland's VCL/TObject era, not a React clone in Go.
See [ARCHITECTURE.md](ARCHITECTURE.md) for the reasoning behind every
architectural choice below.

- **TUIML** — components are just Go functions returning an `Element` tree
  built from `Box`/`Text`/`Fragment` and functional options. If you can
  write Go, you already know the syntax.
- **Hooks, not callbacks-on-callbacks** — `UseState`/`UseReducer`, one hook
  slot per component instance (keyed by tree position or an explicit key),
  the same mental model as React/Ink without needing a JS runtime.
- **Real events** — keyboard (focus-scoped, with a `Global()` escape hatch
  for app-wide shortcuts like Ctrl+C) and mouse (click/scroll, hit-tested
  against the actual rendered layout — including floating overlays).
- **Fast by construction** — flexbox layout → cell buffer → diff → ANSI.
  SGR state and cursor position are tracked across frames so unchanged
  style/position never gets re-emitted.
- **Batteries included** — a widget layer (`Panel`, `Dialog`, `Menu`,
  `ProgressBar`, `Spinner`, `Clock`, `Badge`), a Picture-in-Picture overlay
  system, and standalone `audio` and `asciiart` libraries.

## Install

```sh
go get github.com/wildneuro/tuigo
```

Requires Go 1.22+.

## Quick start

```go
package main

import tuigo "github.com/wildneuro/tuigo"

func Counter(ctx *tuigo.Ctx) tuigo.Element {
	n, setN := tuigo.UseState(ctx, 0)

	return tuigo.Box(
		tuigo.Padding(1), tuigo.FlexColumn(), tuigo.Border(tuigo.BorderSingle),
		tuigo.OnKey('+', func() { setN(n + 1) }),
		tuigo.Global()(tuigo.OnSpecialKey(tuigo.KeyCtrlC, ctx.Exit)),
		tuigo.Children(
			tuigo.Text("count: %d", n),
			tuigo.With(tuigo.Text("press + to increment, ctrl+c to quit"), tuigo.ColorFg(tuigo.ColorGray)),
		),
	)
}

func main() {
	if err := tuigo.Render(Counter); err != nil {
		panic(err)
	}
}
```

`Render` puts the terminal into raw/alt-screen mode, runs the loop, and
always restores it on exit — including on panic. See
`_examples/static-counter`, `_examples/style-showcase`, and the full
`_examples/chat` app (bubbles, a `/` command menu, mouse, focus/Tab
cycling, and 3+ simultaneous PiP windows) for more.

## Core concepts

### TUIML: composition, not markup

Every node is a plain `types.Element` built by a constructor plus
functional options. Options apply left-to-right; later options win when
they touch the same field.

```go
tuigo.Box(
    tuigo.Padding(1), tuigo.FlexColumn(), tuigo.Gap(1), tuigo.Border(tuigo.BorderSingle),
    tuigo.Children(
        tuigo.Text("hello"),
        tuigo.With(tuigo.Text("dim caption"), tuigo.ColorFg(tuigo.ColorGray), tuigo.Italic()),
    ),
)
```

`Box` is a flex container (row or column, your call via `FlexRow()` /
`FlexColumn()`, the latter being the common one). `Text` renders a
formatted string. `Fragment` groups children without its own box, exactly
like React's `<>...</>`. `With` applies extra options to an
already-constructed element — mainly for styling `Text`, whose own
variadic slot is reserved for `fmt.Sprintf` args.

### Layout: pure Go flexbox

`layout/` computes every node's position and size with no terminal
involved — testable in isolation, deterministic, side-effect free. Padding,
margin, gap, explicit `Width`/`Height`, and intrinsic sizing (a `Text`
node's height is its word-wrapped line count) all compose the way you'd
expect from CSS flexbox, minus the parts a terminal doesn't need.

### Style: cascade where it makes sense, not everywhere

Text-styling fields (`ColorFg`, `ColorBg`, `Bold`, `Italic`, `Underline`)
cascade from an ancestor `Box` down to `Text` children that don't set their
own value — set a theme color once on a container, every child inherits
it. Layout fields (`Padding`, `Margin`, `Gap`, `Border`, size, direction)
never inherit; they only ever apply to the node they're set on. Colors
support both the 256-color xterm palette (`ColorRed`, `ColorBlue`, …) and
24-bit truecolor (`RGB(r, g, b)`) for terminals that support it.

### State: hooks, per component instance

```go
count, setCount := tuigo.UseState(ctx, 0)
state, dispatch := tuigo.UseReducer(ctx, reducer, initialState)
```

Call hooks unconditionally at the top of your component, same rule as
React. Each component instance (identified by its position in the tree, or
an explicit key via `WithKey` for lists whose order/count can change) owns
its own hook storage — not one flat sequence for the whole app, so
independent components don't stomp on each other's state.

### Events: focus-scoped by default, global when it matters

```go
tuigo.OnKey('q', handler)              // matches a specific rune
tuigo.OnSpecialKey(tuigo.KeyEnter, h)  // matches a named key
tuigo.OnAnyKey(func(k tuigo.Key) {})   // every key — build text inputs with this
tuigo.OnClick(func(m tuigo.MouseEvent) {})
tuigo.OnScroll(func(delta int) {})     // -1 wheel up, +1 wheel down

tuigo.Focusable()          // mark an element as a Tab target
ctx.Focus("key")           // move focus programmatically
ctx.FocusNext(), ctx.FocusPrev()  // Tab / Shift+Tab

tuigo.Global()(tuigo.OnSpecialKey(tuigo.KeyCtrlC, ctx.Exit)) // fires regardless of focus
```

Keyboard events go to whichever element currently has focus — the VCL
model, not "every handler in the tree fires for every keystroke."
`Global()` wraps a handler so it fires app-wide instead (use it sparingly:
quit shortcuts, not a way to avoid thinking about focus). Mouse events are
hit-tested against the actual last-rendered layout, including floating
overlays, bubbling from the innermost matching element outward.

### Widgets

`Panel`, `Divider`, `Badge`, `ProgressBar`, `Spinner`, `Clock`, `Dialog`,
`Menu` — all pure composition over `Box`/`Text`, no new primitives. Read
their doc comments for the honest caveats (e.g. `Dialog` is a
full-viewport takeover, not a floating window — see Overlays below for
that).

### Picture-in-Picture overlays

tuigo's layout is flow-only — no z-order, no absolute positioning. For
floating content (PiP panels, popups, toasts) there's a small, explicit
escape hatch instead of bolting z-order onto the whole layout model:

```go
ctx.Overlay(myPanel, x, y) // call every render you want it visible
```

Each overlay gets its own independent layout+draw pass, sized by its own
explicit `Width`/`Height`, then is composited onto the main frame *before*
the cell diff — so it participates in the same minimal-redraw diffing as
everything else. `_examples/chat` runs three-plus of these simultaneously:
a live telemetry log, a CPU/mem meter, and (via the `/music` command) a
music player + an ASCII-art image gallery, all on top of the same chat
view at once.

### Timers

```go
cancel := ctx.After(2*time.Second, func() { /* runs on the render loop, safely */ })
```

Routed through a channel into the single render-loop goroutine — nothing
outside that goroutine ever touches component state directly, so there's
no locking and no data races to reason about.

## Standalone libraries

Two more packages, usable with or without tuigo:

- **`audio`** — `Play`/`PlayAsync`/`Record` by shelling out to
  platform-native tools (afplay/paplay/aplay/ffplay/PowerShell), plus a
  pure-Go WAV chirp synthesizer (`Beep`, `GenerateChirpWAV`) for UI
  feedback sounds with zero external dependencies.
- **`asciiart`** — decode an image and render it as a grayscale ASCII ramp
  or a truecolor RGB grid (for solid-block "pixel art" in a 24-bit
  terminal). Pure stdlib, no CGo, no third-party deps.

## Project layout

```
tuigo/
├── tuigo.go, node.go, style.go, event.go, state.go, overlay.go, widgets.go
├── types/       Element, Style, Key/MouseEvent — shared, avoids import cycles
├── layout/      pure flexbox geometry, zero terminal dependency
├── renderer/    Cell/Buffer/Diff + the terminal shell (raw mode, ANSI, input decode)
├── reconciler/  reserved — deliberately unimplemented, see ARCHITECTURE.md
├── audio/       standalone play/record library
├── asciiart/    standalone image → terminal-art library
└── _examples/   static-counter, style-showcase, chat
```

## Development

```sh
go build ./...
go test ./...
go test -race ./...
go vet ./...
gofmt -l .
./example-chat.sh     # run the full chat demo in a real terminal
./make-demo.sh        # regenerate demo.gif via vhs
./release.sh          # bump VERSION, run checks, regenerate the demo, commit
```

`example-chat.sh` needs a real TTY on stdin (raw mode) — run it in an
actual terminal, not piped through another tool.

## Read more

- [PLAN.md](PLAN.md) — original design doc and v1 scope
- [STYLEGUIDE.md](STYLEGUIDE.md) — the TUIML ruleset: what's enforced and why
- [ARCHITECTURE.md](ARCHITECTURE.md) — the case for skipping React-style
  reconciliation, the focus/instance/timer design
- [TODO.md](TODO.md) — what's left, with acceptance criteria per item

## Status

Under active development. The core pipeline (layout → render → diff →
flush), hooks, focus-scoped events, mouse, timers, truecolor, and the PiP
overlay system all work today and are covered by tests — see TODO.md for
the remaining gaps (row-direction text wrapping, Windows resize, more
`tuigo.go` test coverage). Not yet at a tagged v1.
