package tuigo

import "time"

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
