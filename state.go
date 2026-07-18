package tuigo

import (
	"strings"
	"time"
)

type instance struct {
	hooks    []any
	hookPos  int
	timers   []func()
	children map[string]*instance
	visited  bool
}

func (inst *instance) getOrCreateChild(key string) *instance {
	if inst.children == nil {
		inst.children = make(map[string]*instance)
	}
	if inst.children[key] == nil {
		inst.children[key] = &instance{}
	}
	return inst.children[key]
}

func (inst *instance) prune() {
	for k, child := range inst.children {
		if !child.visited {
			for _, cancel := range child.timers {
				cancel()
			}
			delete(inst.children, k)
		} else {
			child.visited = false
			child.prune()
		}
	}
}

func (inst *instance) resetVisited() {
	inst.visited = false
	for _, child := range inst.children {
		child.resetVisited()
	}
}

type appState struct {
	rootInst *instance

	width, height int
	exited        bool

	focusPath  string
	focusOrder []string

	timers          chan func()
	pendingOverlays []Overlay
}

type Ctx struct {
	app  *appState
	inst *instance
}

func (c *Ctx) Size() (width, height int) {
	return c.app.width, c.app.height
}

func (c *Ctx) Exit() {
	c.app.exited = true
}

func (c *Ctx) Focus(key string) {
	c.app.focusPath = key
}

func (c *Ctx) FocusNext() {
	if len(c.app.focusOrder) == 0 {
		return
	}
	if c.app.focusPath == "" {
		c.app.focusPath = c.app.focusOrder[0]
		return
	}
	for i, k := range c.app.focusOrder {
		if k == c.app.focusPath {
			next := (i + 1) % len(c.app.focusOrder)
			c.app.focusPath = c.app.focusOrder[next]
			return
		}
	}
	c.app.focusPath = c.app.focusOrder[0]
}

func (c *Ctx) FocusPrev() {
	if len(c.app.focusOrder) == 0 {
		return
	}
	if c.app.focusPath == "" {
		c.app.focusPath = c.app.focusOrder[len(c.app.focusOrder)-1]
		return
	}
	for i, k := range c.app.focusOrder {
		if k == c.app.focusPath {
			prev := (i - 1 + len(c.app.focusOrder)) % len(c.app.focusOrder)
			c.app.focusPath = c.app.focusOrder[prev]
			return
		}
	}
	c.app.focusPath = c.app.focusOrder[len(c.app.focusOrder)-1]
}

// Wake asks the render loop to run one more frame as soon as it can. It is
// the redraw trigger a BACKGROUND goroutine uses after mutating state the UI
// reads (e.g. a TerminalPane's reader goroutine after writing child output
// into its Screen): the loop selects on the same timers channel Ctx.After
// feeds, so a wake is just an empty job pushed onto it. Safe to call from any
// goroutine; non-blocking (a wake is dropped only when one is already queued,
// which is fine — the queued frame will observe the latest state anyway).
func (c *Ctx) Wake() {
	select {
	case c.app.timers <- func() {}:
	default:
	}
}

// IsFocused reports whether the element identified by key currently holds
// focus. It matches the app's focus path exactly or by trailing segment, so a
// pane keyed "term" is focused whether its resolved path is "term" or
// "panes/term". Used by a Canvas painter to decide whether to draw the child's
// cursor.
func (c *Ctx) IsFocused(key string) bool {
	if key == "" {
		return false
	}
	fp := c.app.focusPath
	return fp == key || strings.HasSuffix(fp, "/"+key)
}

// OnCleanup registers fn to run when this component instance is unmounted
// (removed from the tree on a later render). It piggybacks the same
// per-instance teardown list Ctx.After cancels use, so a long-lived resource —
// e.g. a TerminalPane's child process and PTY — is released when the pane
// leaves the tree. Call it once per mount (guard with UseState), not every
// render.
func (c *Ctx) OnCleanup(fn func()) {
	c.inst.timers = append(c.inst.timers, fn)
}

func (c *Ctx) After(d time.Duration, fn func()) func() {
	t := time.AfterFunc(d, func() {
		select {
		case c.app.timers <- fn:
		default:
		}
	})
	stop := t.Stop
	c.inst.timers = append(c.inst.timers, func() { stop() })
	return func() { stop() }
}

func UseState[T any](ctx *Ctx, initial T) (T, func(T)) {
	inst := ctx.inst
	idx := inst.hookPos
	inst.hookPos++
	if idx == len(inst.hooks) {
		inst.hooks = append(inst.hooks, initial)
	}
	val := inst.hooks[idx].(T)
	setter := func(v T) {
		inst.hooks[idx] = v
	}
	return val, setter
}

func UseReducer[S, A any](ctx *Ctx, reducer func(S, A) S, initial S) (S, func(A)) {
	inst := ctx.inst
	idx := inst.hookPos
	inst.hookPos++
	if idx == len(inst.hooks) {
		inst.hooks = append(inst.hooks, initial)
	}
	val := inst.hooks[idx].(S)
	dispatch := func(action A) {
		val = reducer(val, action)
		inst.hooks[idx] = val
	}
	return val, dispatch
}
