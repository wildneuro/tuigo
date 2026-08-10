package tuigo

import (
	"sync"
	"time"

	"github.com/wildneuro/tuigo/layout"
	"github.com/wildneuro/tuigo/renderer"
	"github.com/wildneuro/tuigo/types"
)

type Component func(ctx *Ctx) Element

func Render(component Component) (err error) {
	return render(component, renderer.NewTerminal)
}

// RenderInline is Render on the MAIN screen — it never takes over the
// alternate screen buffer (see renderer.NewInlineTerminal). Use it when the
// host already owns an alt screen that must survive the render, e.g. a PTY
// wrapper showing a modal over a full-screen child: the wrapper switches away
// from the child's alt buffer, calls RenderInline, then switches back and the
// terminal restores the child's screen untouched. Same component/event model
// as Render; still raw mode + mouse + its own stdin for the render's duration.
func RenderInline(component Component) (err error) {
	return render(component, renderer.NewInlineTerminal)
}

// render is the shared event loop for Render/RenderInline, differing only in
// how the Terminal is constructed (alt-screen vs main-screen).
func render(component Component, newTerm func() (*renderer.Terminal, error)) (err error) {
	term, err := newTerm()
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := term.Close(); err == nil {
			err = closeErr
		}
	}()
	defer func() {
		if r := recover(); r != nil {
			term.Close()
			panic(r)
		}
	}()

	app := &appState{
		rootInst: &instance{},
		timers:   make(chan func(), 32),
		done:     make(chan struct{}),
	}
	var closeDoneOnce sync.Once
	app.closeDone = func() { closeDoneOnce.Do(func() { close(app.done) }) }
	// Every return path out of render — the normal loop exit below, and a
	// recovered panic via the defer above — must close done exactly once so
	// no Ctx.After goroutine outlives this call. defer (not an explicit
	// close before each return) is what makes the panic path safe too.
	defer app.closeDone()
	app.width, app.height = term.Size()

	keyCh := term.Keys()
	mouseCh := term.Mouse()
	resizeCh := term.Resizes()

	// Drives periodic re-renders for continuously-updating content (e.g. a
	// live clock) that no key/resize/mouse event would otherwise trigger.
	// The buffer diff in renderOnce means a tick that changes nothing
	// visible costs CPU only, not terminal writes. TODO.md item 3's
	// Ctx.After scheduler should eventually subsume this for
	// component-driven timers; this ticker only covers "redraw roughly
	// once a second so wall-clock-derived UI stays live."
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	type overlayHit struct {
		Node layout.Node
		X, Y int
	}

	var tree Element
	var node layout.Node
	var overlayNodes []overlayHit
	var prevBuf *renderer.Buffer

	renderOnce := func() {
		app.rootInst.resetVisited()

		app.rootInst.hookPos = 0
		tempCtx := &Ctx{app: app, inst: app.rootInst}
		tempTree := component(tempCtx)

		buildInstTree(app.rootInst, tree, tempTree, "")

		app.rootInst.prune()

		app.rootInst.hookPos = 0
		app.pendingOverlays = nil // only the real pass's Ctx.Overlay calls should stick
		ctx := &Ctx{app: app, inst: app.rootInst}
		tree = component(ctx)

		markVisitedInst(app.rootInst, tree, "")

		app.focusOrder = nil
		app.focusGrab = map[string]bool{}
		collectFocusOrder(tree, "", app)

		// Preserve an existing focus target (set by Tab/FocusNext/FocusPrev,
		// or by the component itself via Ctx.Focus) across renders. Only
		// fall back to the first focusable when there's no valid target —
		// e.g. the first render, or the previously focused element's key
		// disappeared from the tree. Resetting unconditionally here would
		// silently undo every focus change every single frame.
		stillValid := false
		for _, k := range app.focusOrder {
			if k == app.focusPath {
				stillValid = true
				break
			}
		}
		if !stillValid {
			app.focusPath = ""
			if len(app.focusOrder) > 0 {
				app.focusPath = app.focusOrder[0]
			}
		}

		node = layout.Layout(tree, app.width, app.height)
		// Clear the frame's cursor slot before Draw; a focused TerminalPane's
		// Paint republishes it (cursor.go). This is what makes the real
		// hardware cursor follow the focused pane and vanish otherwise.
		app.cursorVisible = false
		buf := renderer.Draw(node, app.width, app.height)
		overlayNodes = overlayNodes[:0]
		var overlayRects []layout.Rect
		for _, ov := range app.pendingOverlays {
			ow, oh := ov.Element.Style.Width, ov.Element.Style.Height
			if ow <= 0 || oh <= 0 {
				continue // Overlay's doc comment requires explicit Width/Height; skip silently rather than panic
			}
			ovNode := layout.Layout(ov.Element, ow, oh)
			buf2 := renderer.Draw(ovNode, ow, oh)
			// Overlay wins where it overlaps the pane (Stamp overwrites); when
			// it's gone next frame the pane repaints from its Screen and Diff
			// restores those cells — the dialog-over-pane proof (FIX 3).
			renderer.Stamp(buf, buf2, ov.X, ov.Y)
			overlayNodes = append(overlayNodes, overlayHit{Node: ovNode, X: ov.X, Y: ov.Y})
			overlayRects = append(overlayRects, layout.Rect{X: ov.X, Y: ov.Y, W: ow, H: oh})
		}
		term.Flush(renderer.Diff(prevBuf, buf))
		// Park the real cursor after the cells are flushed: the focused pane's
		// child cursor, unless an overlay covers it (then hide, so a dialog's
		// cells aren't pierced by the child's cursor).
		cx, cy, cvis := computeCursor(app.cursorX, app.cursorY, app.cursorVisible, overlayRects)
		term.SetCursor(cx, cy, cvis)
		prevBuf = buf
	}

	renderOnce()
	for !app.exited {
		select {
		case k, ok := <-keyCh:
			if !ok {
				return nil
			}
			// The focus chord (default Ctrl-O) always cycles focus; Tab/
			// Shift-Tab cycle focus only when the focused element does NOT grab
			// input (a grab-all TerminalPane keeps them). See focus.go.
			switch routeKey(k, app.focusGrab[app.focusPath]) {
			case routeFocusNext:
				(&Ctx{app: app, inst: app.rootInst}).FocusNext()
			case routeFocusPrev:
				(&Ctx{app: app, inst: app.rootInst}).FocusPrev()
			default:
				dispatchGlobalKey(tree, k)
				dispatchFocusedKey(tree, app.focusPath, k)
			}
		case m, ok := <-mouseCh:
			if !ok {
				return nil
			}
			// Overlays are drawn last (on top), so hit-test them first, in
			// reverse registration order, translating into each overlay's
			// local coordinate space before falling back to the main tree.
			handled := false
			for i := len(overlayNodes) - 1; i >= 0; i-- {
				ov := overlayNodes[i]
				local := types.MouseEvent{X: m.X - ov.X, Y: m.Y - ov.Y, Button: m.Button, Action: m.Action}
				if dispatchMouse(ov.Node, local) {
					handled = true
					break
				}
			}
			if !handled {
				dispatchMouse(node, m)
			}
		case sz, ok := <-resizeCh:
			if !ok {
				return nil
			}
			app.width, app.height = sz.W, sz.H
		case <-ticker.C:
			// no dispatch needed; renderOnce below re-reads wall-clock time
		case fn, ok := <-app.timers:
			if !ok {
				return nil
			}
			fn()
		}
		if app.exited {
			break
		}
		renderOnce()
	}
	return nil
}

// dispatchMouse hit-tests m against the last computed layout. It bubbles
// from the innermost node containing the point outward, firing every
// matching MouseHandler on the first (deepest) node that has one, then
// stopping — an ancestor whose own rect also contains the point but whose
// deeper child had a handler does not additionally fire.
func dispatchMouse(n layout.Node, m types.MouseEvent) bool {
	for _, c := range n.Children {
		if dispatchMouse(c, m) {
			return true
		}
	}
	if m.X < n.Rect.X || m.X >= n.Rect.X+n.Rect.W || m.Y < n.Rect.Y || m.Y >= n.Rect.Y+n.Rect.H {
		return false
	}
	hit := false
	for _, h := range n.Element.MouseHandlers {
		if h.Matches(m) {
			h.Handle(m)
			hit = true
		}
	}
	return hit
}

func dispatchFocusedKey(e Element, focusPath string, k types.Key) {
	findAndDispatch(e, focusPath, k)
}

func findAndDispatch(e Element, focusPath string, k types.Key) {
	cp := currentPath(e, "")
	if cp == focusPath {
		for _, h := range e.Handlers {
			if !h.Global && h.Matches(k) {
				h.Handle(k)
			}
		}
	}
	for i, child := range e.Children {
		childPath := buildChildPath(e, child, i)
		findAndDispatchAt(child, focusPath, childPath, k)
	}
}

func findAndDispatchAt(e Element, focusPath, ePath string, k types.Key) {
	if ePath == focusPath {
		for _, h := range e.Handlers {
			if !h.Global && h.Matches(k) {
				h.Handle(k)
			}
		}
	}
	for i, child := range e.Children {
		childPath := buildChildPath(e, child, i)
		findAndDispatchAt(child, focusPath, childPath, k)
	}
}

func dispatchGlobalKey(e Element, k types.Key) {
	for _, h := range e.Handlers {
		if h.Global && h.Matches(k) {
			h.Handle(k)
		}
	}
	for _, c := range e.Children {
		dispatchGlobalKey(c, k)
	}
}

func collectFocusOrder(el Element, path string, app *appState) {
	cp := path
	if el.Key != "" {
		if cp == "" {
			cp = el.Key
		} else {
			cp = cp + "/" + el.Key
		}
	}
	if el.Focusable && cp != "" {
		app.focusOrder = append(app.focusOrder, cp)
		if app.focusGrab == nil {
			app.focusGrab = map[string]bool{}
		}
		app.focusGrab[cp] = el.GrabKeys
	}
	for i, child := range el.Children {
		if el.Focusable {
			childPath := buildChildPath(el, child, i)
			collectFocusOrder(child, childPath, app)
		} else {
			collectFocusOrder(child, "", app)
		}
	}
}

func buildChildPath(parent Element, child Element, idx int) string {
	p := ""
	if parent.Key != "" {
		p = parent.Key
	}
	if child.Key != "" {
		if p == "" {
			return child.Key
		}
		return p + "/" + child.Key
	}
	if p == "" {
		return childPathNumeric(idx)
	}
	return p + childPathNumeric(idx)
}

func childPathNumeric(idx int) string {
	return "/" + itoa(idx)
}

func currentPath(e Element, parentPath string) string {
	if e.Key != "" {
		if parentPath == "" {
			return e.Key
		}
		return parentPath + "/" + e.Key
	}
	return parentPath
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	result := ""
	for i > 0 {
		result = string(rune('0'+i%10)) + result
		i /= 10
	}
	return result
}

func buildInstTree(inst *instance, prev, next Element, path string) {
	if inst.children == nil {
		inst.children = make(map[string]*instance)
	}

	for i, child := range next.Children {
		cp := buildChildPathFromParent(next, child, i, path)
		if inst.children[cp] == nil {
			inst.children[cp] = &instance{}
		}
		var prevChild Element
		if prev.Type != 0 && i < len(prev.Children) {
			prevChild = prev.Children[i]
		}
		buildInstTree(inst.children[cp], prevChild, child, cp)
	}
}

func buildChildPathFromParent(parent, child Element, idx int, parentPath string) string {
	cp := parentPath
	if child.Key != "" {
		if cp == "" {
			cp = child.Key
		} else {
			cp = cp + "/" + child.Key
		}
	} else {
		if cp == "" {
			cp = itoa(idx)
		} else {
			cp = cp + "/" + itoa(idx)
		}
	}
	return cp
}

func markVisitedInst(inst *instance, el Element, path string) {
	inst.visited = true
	for i, child := range el.Children {
		cp := buildChildPathFromParent(el, child, i, path)
		if childInst, ok := inst.children[cp]; ok {
			markVisitedInst(childInst, child, cp)
		}
	}
}
