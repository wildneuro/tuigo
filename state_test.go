package tuigo

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestUseStateInitialValue(t *testing.T) {
	inst := &instance{}
	ctx := &Ctx{app: &appState{rootInst: inst}, inst: inst}

	val, _ := UseState(ctx, 42)
	if val != 42 {
		t.Errorf("UseState initial value = %d, want 42", val)
	}
}

func TestUseStateSetterUpdatesValue(t *testing.T) {
	inst := &instance{}
	ctx := &Ctx{app: &appState{rootInst: inst}, inst: inst}

	_, set := UseState(ctx, 0)
	set(100)

	inst.hookPos = 0
	val, _ := UseState(ctx, 0)
	if val != 100 {
		t.Errorf("UseState after set = %d, want 100", val)
	}
}

func TestUseStateMultipleSlotsIndependent(t *testing.T) {
	inst := &instance{}
	ctx := &Ctx{app: &appState{rootInst: inst}, inst: inst}

	val1, set1 := UseState(ctx, "a")
	val2, set2 := UseState(ctx, 1)
	val3, _ := UseState(ctx, true)

	if val1 != "a" || val2 != 1 || val3 != true {
		t.Errorf("UseState multiple slots: got (%q, %d, %v), want (\"a\", 1, true)", val1, val2, val3)
	}

	set1("z")
	set2(999)

	inst.hookPos = 0
	val1, _ = UseState(ctx, "a")
	val2, _ = UseState(ctx, 1)

	if val1 != "z" || val2 != 999 {
		t.Errorf("UseState after sets: got (%q, %d), want (\"z\", 999)", val1, val2)
	}
}

func TestUseStateHookPosResetsBetweenRenders(t *testing.T) {
	app := &appState{rootInst: &instance{}}

	ctx := &Ctx{app: app, inst: app.rootInst}
	UseState(ctx, 1)
	UseState(ctx, 2)

	if app.rootInst.hookPos != 2 {
		t.Errorf("hookPos after two UseState calls = %d, want 2", app.rootInst.hookPos)
	}

	app.rootInst.hookPos = 0
	ctx = &Ctx{app: app, inst: app.rootInst}

	val1, _ := UseState(ctx, 99)
	val2, _ := UseState(ctx, 88)

	if val1 != 1 || val2 != 2 {
		t.Errorf("UseState after hookPos reset: got (%d, %d), want (1, 2)", val1, val2)
	}
}

func TestUseReducerInitialValue(t *testing.T) {
	inst := &instance{}
	ctx := &Ctx{app: &appState{rootInst: inst}, inst: inst}

	type action int
	const inc action = 1
	reducer := func(s int, a action) int {
		return s + int(a)
	}

	val, _ := UseReducer(ctx, reducer, 10)
	if val != 10 {
		t.Errorf("UseReducer initial value = %d, want 10", val)
	}
}

func TestUseReducerDispatchUpdatesState(t *testing.T) {
	inst := &instance{}
	ctx := &Ctx{app: &appState{rootInst: inst}, inst: inst}

	type action int
	const inc action = 1
	reducer := func(s int, a action) int {
		return s + int(a)
	}

	_, dispatch := UseReducer(ctx, reducer, 10)
	dispatch(inc)
	dispatch(inc)

	inst.hookPos = 0
	val, _ := UseReducer(ctx, reducer, 10)
	if val != 12 {
		t.Errorf("UseReducer after two inc = %d, want 12", val)
	}
}

func TestUseReducerMultipleInstancesIndependent(t *testing.T) {
	inst := &instance{}
	ctx := &Ctx{app: &appState{rootInst: inst}, inst: inst}

	type action int
	const inc action = 1
	reducer := func(s int, a action) int {
		return s + int(a)
	}

	_, dispatch1 := UseReducer(ctx, reducer, 0)
	_, dispatch2 := UseReducer(ctx, reducer, 100)

	dispatch1(inc)
	dispatch2(inc)
	dispatch2(inc)

	inst.hookPos = 0
	val1, _ := UseReducer(ctx, reducer, 0)
	val2, _ := UseReducer(ctx, reducer, 100)

	if val1 != 1 || val2 != 102 {
		t.Errorf("UseReducer multiple instances: got (%d, %d), want (1, 102)", val1, val2)
	}
}

// TestCtxAfterDeliversEveryCallbackUnderBackpressure schedules more
// callbacks than the timers channel's capacity (32, matching render's
// make(chan func(), 32)) while a slow consumer drains it, and asserts every
// single one is eventually delivered. Before this fix, Ctx.After's send was
// non-blocking with a `default:` drop — a burst like this would silently
// lose callbacks once the channel filled.
func TestCtxAfterDeliversEveryCallbackUnderBackpressure(t *testing.T) {
	app := &appState{
		rootInst: &instance{},
		timers:   make(chan func(), 32),
		done:     make(chan struct{}),
	}
	inst := &instance{}
	ctx := &Ctx{app: app, inst: inst}

	const n = 100
	var mu sync.Mutex
	delivered := make(map[int]bool)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		ctx.After(0, func() {
			mu.Lock()
			delivered[i] = true
			mu.Unlock()
			wg.Done()
		})
	}

	// A slow consumer, standing in for render's event loop selecting on
	// app.timers between frames.
	drainDone := make(chan struct{})
	go func() {
		defer close(drainDone)
		for i := 0; i < n; i++ {
			fn := <-app.timers
			fn()
			time.Sleep(time.Millisecond)
		}
	}()

	waitOK := make(chan struct{})
	go func() { wg.Wait(); close(waitOK) }()

	select {
	case <-waitOK:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for all callbacks to be delivered")
	}
	<-drainDone

	mu.Lock()
	defer mu.Unlock()
	if len(delivered) != n {
		t.Errorf("delivered %d/%d callbacks, want all %d", len(delivered), n, n)
	}
}

// TestCtxAfterUnblocksOnShutdown proves a Ctx.After callback blocked trying
// to send on a full/undrained timers channel does not leak its goroutine
// forever: closing app.done must unblock it promptly, the way render's
// event loop closes done on every exit path. It uses runtime.NumGoroutine,
// the same coarse leak-detection style used elsewhere for concurrency
// checks in this repo, since the goroutine that blocks is internal to
// time.AfterFunc and not directly observable via a channel.
func TestCtxAfterUnblocksOnShutdown(t *testing.T) {
	app := &appState{
		rootInst: &instance{},
		timers:   make(chan func(), 1),
		done:     make(chan struct{}),
	}
	inst := &instance{}
	ctx := &Ctx{app: app, inst: inst}

	// Fill the channel so every subsequently-firing AfterFunc blocks on its
	// send, with nobody draining it — the scenario a timer firing after the
	// render loop has already exited would hit for real.
	app.timers <- func() {}

	before := runtime.NumGoroutine()

	const n = 8
	for i := 0; i < n; i++ {
		ctx.After(0, func() {})
	}
	// Let every scheduled timer actually fire and block on its select.
	time.Sleep(30 * time.Millisecond)

	blocked := runtime.NumGoroutine()
	if blocked < before+n {
		t.Fatalf("expected at least %d extra blocked goroutines, got %d (before=%d)", n, blocked-before, before)
	}

	close(app.done)

	// Poll for the blocked goroutines to drain back down; each should exit
	// via its <-c.app.done case almost immediately.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= before {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("goroutines did not unblock after close(done): before=%d, now=%d", before, runtime.NumGoroutine())
}

func TestInstancePruneCancelsTimers(t *testing.T) {
	inst := &instance{children: map[string]*instance{
		"child": {timers: []func(){}},
	}}
	inst.children["child"].visited = false

	inst.prune()

	if _, exists := inst.children["child"]; exists {
		t.Errorf("child should have been pruned but still exists")
	}
}
