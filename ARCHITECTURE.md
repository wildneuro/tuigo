# tuigo architecture direction: instance identity, focus, timers

This is the design doc for TODO.md items 1–3 — the three architecturally
significant gaps between the current MVP and the target described in the
`tuigo-design-philosophy-vcl-2026` memory (VCL/TObject-style event-driven,
property-based components; better than React by *not* copying its
tree-reconciliation model where terminal scale doesn't need it).

Read this before implementing items 1–3 in TODO.md — it gives concrete API
sketches so the "design decision, not mechanical port" points called out
there have an actual answer instead of being invented ad hoc.

## Why these three are one design, not three

Focus (item 2) needs to know "which node is this, across renders" to keep
focus pointing at the same logical input as the tree reshapes. Timer
cancellation (item 3) needs to know "which component registered this timer,
so I can cancel it when that component goes away." Both of those are the
same problem `UseState` already has (item 1): **stable identity across
renders**. Solve identity once, and focus + timer cleanup fall out of it.

**Implementation order: 1, then 2, then 3.** Don't build focus or timers on
top of the flat hook sequence — build the instance tree first.

## 1. Instance identity & per-component state

### The core idea

Introduce an `instance` tree that mirrors the Element tree but persists
across renders, matched by identity instead of rebuilt from scratch:

```go
type instance struct {
    hooks    []any
    timers   []cancelFunc          // registered via Ctx.After, see §3
    children map[string]*instance  // keyed by child identity
    visited  bool                  // reset to false at the start of each render
}
```

**Identity key** for a child at position `i` in its parent: `Element.Key` if
set (via `WithKey`), otherwise a positional fallback like
`fmt.Sprintf("#%d", i)`. This is exactly React's key/index rule — explicit
keys for anything whose order or count can change (lists), positional
fallback for everything else (static layout).

### Render walk

Replace the flat `app.hookPos` counter with a walk that carries the current
`*instance` alongside the current `Element`:

```go
func renderComponent(inst *instance, el types.Element) types.Element {
    inst.visited = true
    inst.hookPos = 0          // hooks reset per node, not per whole tree
    // ... call into the component-shaped closures that build el's children,
    // threading `inst` through Ctx so UseState/UseReducer/After resolve
    // against THIS node's instance, not a global sequence.
}
```

`Ctx` needs to carry the *current* `*instance`, not just the root
`*appState`. Practically: `Ctx.app` stays as the shared root state (focus,
exit, size), and a separate `Ctx.inst *instance` field tracks "whose hooks
are we resolving right now," swapped as the walk descends. `UseState` reads
`ctx.inst.hooks[idx]` instead of `ctx.app.hooks[idx]`.

### Pruning

After a full render, walk the previous instance tree: any `*instance` with
`visited == false` was for an Element that disappeared this frame. Cancel
its `timers` (§3) and drop it from its parent's `children` map. This is the
only "diffing" tuigo does — identity bookkeeping, not patch computation.
Layout/Draw/Buffer-diff remain a full rebuild + cell-diff every frame,
unchanged from today. **Do not build a general Element patch/reconciler
here** — that's explicitly the thing tuigo is choosing not to do.

### Where this plugs into the existing code

- `state.go`: `UseState`/`UseReducer` resolve against `ctx.inst`, not
  `ctx.app` directly.
- `tuigo.go`: `renderOnce` becomes a tree walk building/reusing `*instance`
  nodes as it goes, instead of a single `component(ctx)` call with a global
  `hookPos` reset.
- This only matters once components compose other components as functions
  returning sub-trees keyed by position/key — if `Component` stays "one
  function, called once per render" (as it is today), the flat sequence and
  the instance-tree version behave identically for a single top-level
  call. The instance tree earns its keep once composition + lists of
  keyed children are common (see TODO.md item 1's acceptance test: a
  variable-length list of keyed sub-components).

## 2. Focus model

### API

```go
func Focusable() Option                 // marks an Element as a focus target
func (c *Ctx) Focus(key string)         // move focus to the element with this Key
func (c *Ctx) FocusNext()               // Tab-style cycle, in tree order
func (c *Ctx) FocusPrev()               // Shift+Tab
func Global() func(Option) Option       // modifier: wraps OnKey/OnSpecialKey/OnAnyKey
                                         // so the handler fires regardless of focus
```

Usage: `tuigo.Global()(tuigo.OnSpecialKey(tuigo.KeyCtrlC, ctx.Exit))` — a
touch awkward as a function-wrapping-a-function, but keeps `Global` from
needing to know which handler kind it's wrapping. If that reads badly once
implemented, an acceptable alternative is a `Global bool` field set by a
variant constructor per handler kind (`OnGlobalKey`, etc.) — implementer's
call, but pick one and use it consistently across all three handler kinds.

### Dispatch changes

`appState` gains `focusPath string` (the focused node's key path, e.g.
`"root/input"`) and `focusOrder []string` (all focusable key paths in tree
order, rebuilt each render alongside the instance walk).

`dispatchKey` (tuigo.go) changes from "walk everything, fire every match" to:

```go
func dispatchKey(root Element, focusPath string, k types.Key) {
    // 1. Global handlers: walk the whole tree as today, but only fire
    //    handlers marked Global.
    // 2. Focused handlers: find the node at focusPath, fire its
    //    non-global handlers that match k.
}
```

`KeyTab` (when not otherwise consumed) triggers `FocusNext` by default —
decide during implementation whether Tab is always reserved for focus
cycling or only when no focused element's own handler claims it.

### Why key paths, not `*Element` pointers

Elements are values, rebuilt every render — there's no stable pointer to
hold onto. The instance tree from §1 already computes stable key paths;
reuse them for `focusPath` instead of inventing a second identity scheme.

## 3. Timer scheduling

### API

```go
func (c *Ctx) After(d time.Duration, fn func()) (cancel func())
```

`fn` is expected to call a state setter (`UseState`'s or `UseReducer`'s).

### Concurrency contract

**Only the render-loop goroutine may touch `appState` or any `instance`.**
`After`'s `fn` must not run on the timer's own goroutine. Implementation:

```go
// appState gains: timers chan func()   // buffered, e.g. cap 32

func (c *Ctx) After(d time.Duration, fn func()) func() {
    t := time.AfterFunc(d, func() {
        select {
        case c.app.timers <- fn:
        default: // loop is backed up; drop rather than block the timer goroutine
        }
    })
    registered := t.Stop // cancel func returned to the caller
    c.inst.timers = append(c.inst.timers, registered)
    return registered
}
```

`Render`'s `select` (tuigo.go) gains a case:

```go
case fn := <-app.timers:
    fn()
    // fall through to renderOnce(), same as the key/resize cases
```

### Cancellation on instance removal

When §1's pruning drops an `*instance` because its Element disappeared,
call every `cancelFunc` in `inst.timers` first. Otherwise a timer fired
after its owning component is gone would call a setter closing over
`inst.hooks[idx]` for an index that may now belong to a different,
newly-created instance — exactly the kind of stale-closure bug hooks are
prone to if identity isn't tracked carefully.

## STYLEGUIDE.md additions once this lands

Add to STYLEGUIDE.md's rule list after implementation:

7. **Give lists of components explicit keys.** Positional fallback identity
   only works while sibling order and count are stable; anything that can
   reorder or resize needs `WithKey`.
8. **Events are focus-scoped by default.** Use `Global()` deliberately and
   sparingly — for app-wide shortcuts (quit, help), not as a way to avoid
   thinking about focus.
9. **Only the render loop goroutine touches state.** Anything triggered
   from another goroutine (timers, future async I/O) must go through a
   channel into the loop, never call a setter directly.
