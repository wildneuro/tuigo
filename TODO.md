# tuigo TODO

Tracks the gap between the current MVP (see the `tuigo-mvp-v1-architecture`
memory) and the target architecture (`tuigo-design-philosophy-vcl-2026`:
better than React, VCL/TObject-style events, best-in-class 2026 TUI).

Each item has clear instructions and acceptance criteria so it can be picked
up independently, by any model, without re-deriving context. Follow
STYLEGUIDE.md's rules while implementing (pure→impure layering, no
mutation of shared Elements, table-driven tests for every pure package).

Items 1-3 are architecturally significant — **read ARCHITECTURE.md first**,
it has concrete API sketches and an implementation order (1, then 2, then 3)
for exactly these three items, so they don't need to be designed from
scratch. Items 4-9 are mechanical and well-bounded — safe to hand to a
lighter-weight model or implement quickly.

---

## 1. Per-component stable state (architectural)

**Problem:** `UseState` (state.go) stores every hook in one flat
`appState.hooks` slice indexed by call order (`hookPos`), shared across the
*entire* component tree for the whole render. This only works if every
`UseState` call happens unconditionally in the same order every render —
it breaks the moment two sibling components each call `UseState`
conditionally, or a list of components (each needing independent state) is
rendered with a variable count.

**Instructions:**
- Give each component *invocation* its own persistent hook storage, keyed
  by a stable identity across renders — reuse `Element.Key` (already in
  `types.Element`, already settable via `WithKey` in node.go) as that
  identity.
- This requires matching each new-render Element against the
  correspondingly-keyed Element from the previous render, i.e. some form of
  identity-based tree matching. This does **not** mean building full
  patch-based reconciliation (STYLEGUIDE.md / the design philosophy memory
  are explicit that tuigo should keep skipping React-style diffing for
  cell-level redraws) — it means enough bookkeeping to answer "does this
  hook-owning node correspond to one from last frame, and if so, which hook
  storage does it own."
- A reasonable approach: maintain a `map[string]*hookSlots` keyed by the
  concatenation of key-path from root (to disambiguate siblings without
  explicit keys colliding), populated during `renderOnce`, with entries not
  touched during a render pruned afterward so removed components' state is
  freed.
- Update the doc comment on `UseState` in state.go — it currently documents
  the flat-sequence limitation explicitly; that documentation must change
  to describe the new per-instance guarantee.

**Acceptance criteria:**
- A test with two sibling components, each calling `UseState` a different
  number of times based on their own props/state (i.e. conditional hook
  calls within one component, unconditional across siblings), does not
  corrupt state between them.
- A test rendering a variable-length list of keyed sub-components (e.g. a
  todo list where items can be added/removed) preserves each item's
  independent state across renders, matched by key, and frees state for
  removed items (verify via a counter/map size check, not just behavior).
- All existing tests still pass; `node_test.go`'s
  `TestOptionsDoNotMutateSharedElement`-style non-mutation guarantees still
  hold.
- `go test -race ./...` stays clean.

---

## 2. Focus-based event dispatch (architectural)

**Problem:** `dispatchKey` (tuigo.go) does a full depth-first walk of the
*entire* Element tree on every keystroke, invoking every matching handler
it finds. There's no concept of "the currently active input" — every
`OnAnyKey`/`OnKey`/`OnSpecialKey` handler anywhere in the tree fires for
every keystroke that matches it. This is the opposite of VCL/TObject's
model, where an event goes to the one focused component (plus explicit
global bindings).

**Instructions:**
- Introduce a focus concept: one Element (or its key/path) is "focused" at
  a time. Add a way to mark an Element as focusable (e.g. a `Focusable()`
  option) and a way to move focus (e.g. `Ctx.Focus(key string)` /
  `Ctx.FocusNext()` for Tab-cycling).
- Split handler semantics: `OnAnyKey`/`OnKey` attached to the focused
  element receive the event; handlers explicitly marked "global" (decide on
  an API — e.g. a `Global()` modifier on the option, or keep
  `OnSpecialKey` tree-wide by convention since it's typically used for
  app-level shortcuts like Ctrl+C) still fire regardless of focus.
- Update `_examples/chat/main.go` to use the new focus API for its input
  instead of relying on tree-wide `OnAnyKey`.

**Acceptance criteria:**
- A test with two focusable elements each holding an `OnAnyKey` handler:
  only the focused one's handler fires for a given key event; switching
  focus and re-dispatching proves the other one now fires instead.
- A global handler (e.g. Ctrl+C to quit) still fires regardless of which
  element has focus.
- Tab (or the chosen focus-cycling key) moves focus between focusable
  elements in tree order; test this cycles correctly including wraparound.
- `_examples/chat` still works end-to-end when manually tested in a real
  terminal (raw mode needs a TTY — can't be verified headlessly, note this
  in the PR/commit).

---

## 3. Timers / async state updates (architectural, concurrency-sensitive)

**Problem:** The render loop only reacts to `keyCh`/`resizeCh` (tuigo.go).
There is no way for a component to schedule a future state update (e.g. a
"Bot is typing…" indicator before an auto-reply appears). The current
single-goroutine design is deliberately race-free because nothing besides
the loop goroutine ever touches `appState` — any timer mechanism must
preserve that invariant or introduce real correctness risk.

**Instructions:**
- Add a way to schedule a delayed callback, e.g. `Ctx.After(d
  time.Duration, fn func())`, where `fn` is meant to call a state setter.
- Implementation must NOT let the timer's goroutine mutate `appState`
  directly. Route it through a new channel (e.g. `appState.timers chan
  func()`) that only the loop goroutine ever reads from and executes;
  `Render`'s `select` gains a case for it, followed by a `renderOnce()`
  exactly like the key/resize cases.
- Cancel pending timers tied to a component when the component's state is
  torn down (interacts with item 1 — per-instance state cleanup should also
  cancel that instance's pending timers, to avoid a stale closure firing
  into hook storage that no longer exists).

**Acceptance criteria:**
- A test that schedules a timer callback and verifies (via a test-only
  clock/channel injection, not a real `time.Sleep` race) that it runs on
  the loop goroutine and triggers exactly one `renderOnce`.
- `go test -race ./...` stays clean — this is the primary bar for this
  item, since the entire point is not reintroducing a data race.
- `_examples/chat` gains a real "Bot is typing…" delay before the canned
  reply appears, using this mechanism, verified manually in a real
  terminal.

---

## 4. 24-bit truecolor + a real theme

**Problem:** `types.Color` is currently a 256-index xterm palette int
(style.go's `ColorRed`, `ColorBlue`, etc.), and the chat example applies
colors ad hoc per element. Direct feedback after testing: "looks not very
pretty but works."

**Instructions:**
- Add true 24-bit color support alongside (or replacing) the 256-index
  model — e.g. `types.RGB(r, g, b byte) Color` producing a distinguishable
  sentinel-encoded value, and update `renderer/terminal.go`'s `writeSGR` to
  emit `38;2;r;g;b` / `48;2;r;g;b` SGR sequences for RGB colors vs.
  `38;5;n` / `48;5;n` for palette colors.
- Design one small, named theme (a Go value, e.g. `var Theme = struct{...}`
  or a handful of exported `Color` constants with intentional names like
  `AccentColor`, `SurfaceColor`, `MutedColor`) and use it consistently in
  `_examples/chat` instead of raw `ColorBlue`/`ColorGray` calls scattered
  through the file.
- Keep the existing 256-index constants working — this is additive, not a
  breaking change (terminals without truecolor support still need the
  palette path).

**Acceptance criteria:**
- A test asserting an RGB `Color` and a palette `Color` produce visibly
  different SGR sequences from `writeSGR` (string-match the emitted
  escape codes).
- `_examples/chat` visibly uses truecolor + the named theme when run in a
  truecolor-capable terminal (manual verification, real terminal).
- `STYLEGUIDE.md` gets a short section documenting when to use RGB vs.
  palette colors (e.g. "prefer the theme's named constants; use raw RGB
  only for one-off effects").

---

## 5. Ellipsis truncation instead of silent hard-clipping

**Problem:** `drawText` (renderer/draw.go) and the clipping logic silently
drop any character that falls outside its rect/clip bounds — text that
doesn't fit just disappears mid-word with no indication it was cut off.

**Instructions:**
- Add a style option (e.g. `NoWrap()` or `Truncate()`) that, when set on a
  Text element, disables `WrapText`'s word-wrapping and instead renders a
  single line, replacing the last visible character with `…` if the text
  is wider than the available width.
- Without the option, behavior is unchanged (word-wrap, as today).

**Acceptance criteria:**
- A table-driven test in `renderer/draw_test.go`: text wider than its rect
  renders with a trailing `…` and fits exactly within the rect width; text
  that fits renders unchanged (no `…` added).
- A test confirming the option has no effect on `WrapText`'s existing
  column-direction wrapping when not set (no regression).

---

## 6. `Ctx.UseReducer`

**Problem:** PLAN.md's original v1 scope includes `Ctx.UseReducer` (step 7)
alongside `UseState`; only `UseState` exists today (state.go).

**Instructions:**
- Add `func UseReducer[S, A any](ctx *Ctx, reducer func(S, A) S, initial S)
  (S, func(A))` following the exact same hook-slot pattern as `UseState` in
  state.go (and inheriting whatever per-instance storage item 1 produces,
  if implemented first — otherwise the same flat-sequence caveat applies
  and should be documented identically).

**Acceptance criteria:**
- A test with a counter-style reducer (`type action int; func reducer(s
  int, a action) int`) dispatching multiple actions across renders,
  verifying state accumulates correctly and each `dispatch` call schedules
  exactly one rerender (mirroring `TestOptionsDoNotMutateSharedElement`-style
  rigor from node_test.go).
- Doc comment cross-references `UseState`'s hook-ordering caveat rather than
  duplicating it verbatim.

---

## 7. Row-direction text wrapping

**Problem:** `intrinsicMainSize` (layout/flexbox.go) only wraps Text in
column-direction layouts; in a row, a Text child's intrinsic width is just
`len([]rune(c.Text))` — no wrapping, so a long single-line text in a Row
Box either overflows or gets silently clipped by the renderer.

**Instructions:**
- Decide and implement a policy for wrapping Text within a Row: since row
  width is the constrained axis and height is typically flexible there,
  wrapping means the Text's height grows beyond 1 — which affects the
  row's own height, not just the text's width. Update `layoutChildren` so
  a Row's own cross-axis height can grow to accommodate a wrapped Text
  child, OR make row Text intrinsic width capped to the remaining content
  width with wrapping computed against that cap (choose one and document
  why in a code comment, since both are defensible — this is a genuine
  design decision, not a mechanical port of the column logic).

**Acceptance criteria:**
- A test: a Row Box containing a Text child longer than the available row
  width wraps (rather than silently overflowing/clipping) and the
  parent's computed height reflects the wrapped line count.
- Existing `TestLayoutRowIntrinsicWidthIsRuneLength` either still passes
  unchanged (single-line case) or is deliberately updated with a comment
  explaining the new multi-line behavior it now also covers.

---

## 8. Windows resize support

**Problem:** `renderer/terminal.go`'s `Resizes()` uses `syscall.SIGWINCH`
directly with no build tag — this fails to compile on Windows (no
`SIGWINCH` there).

**Instructions:**
- Split `Resizes()` into `terminal_unix.go` (build tag `//go:build !windows`,
  today's SIGWINCH implementation) and `terminal_windows.go` (`//go:build
  windows`), with a Windows implementation that polls `term.GetSize` on a
  short interval (there's no native resize signal via the standard Windows
  console API without more invasive syscalls — polling is the pragmatic
  choice here, document that tradeoff in a comment).

**Acceptance criteria:**
- `GOOS=windows go build ./...` succeeds (run via `env GOOS=windows
  GOARCH=amd64 go build ./...` — this doesn't need an actual Windows
  machine, just cross-compilation to succeed).
- `go build ./...` / `go test ./...` on the current platform are unaffected
  (still pass).

---

## 9. Test coverage for tuigo.go / state.go

**Problem:** `layout`, `renderer`, `types`, and the root `node.go`/`style.go`
all have table-driven tests; `tuigo.go` (the `Render` app loop and
`dispatchKey`) and `state.go` (`UseState`, `Ctx.Size`/`Ctx.Exit`) currently
have none.

**Instructions:**
- `dispatchKey` is pure and directly testable without a terminal — write
  table-driven tests covering: a matching `OnKey` handler fires, a
  non-matching one doesn't, `OnAnyKey` fires for every key, nested children
  are reached (DFS), and multiple matching handlers across different nodes
  all fire.
- `UseState` is also testable without a terminal by constructing an
  `appState`/`Ctx` directly (both are unexported but same-package tests can
  reach them) — cover: initial value returned on first call, setter updates
  the value, multiple `UseState` calls in one render keep independent
  slots, `hookPos` resets to 0 between renders (simulate by calling twice
  with `app.hookPos = 0` in between, matching what `renderOnce` does).
- `Render` itself (the full app loop with a real terminal) is harder to
  unit test — at minimum, factor the pure parts (`renderOnce`'s
  layout→draw→diff pipeline, minus the actual `term.Flush`) so they're
  covered by the `layout`/`renderer` tests that already exist; don't force
  a fake-terminal integration test in this pass unless it's cheap to add.

**Acceptance criteria:**
- `go test ./...` coverage visibly includes `tuigo.go` and `state.go`
  (check via `go test -cover ./...` before/after).
- All new tests follow the existing table-driven style used elsewhere in
  the repo (see `node_test.go`, `style_test.go` for the pattern).
