package tuigo

import "testing"

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
