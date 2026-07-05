package tuigo

import "testing"

func TestClamp01(t *testing.T) {
	cases := []struct {
		in, want float64
	}{
		{-0.5, 0}, {0, 0}, {0.5, 0.5}, {1, 1}, {1.5, 1},
	}
	for _, c := range cases {
		if got := Clamp01(c.in); got != c.want {
			t.Errorf("Clamp01(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestLerpEndpointsAndMidpoint(t *testing.T) {
	if got := Lerp(10, 20, 0); got != 10 {
		t.Errorf("Lerp(10,20,0) = %v, want 10", got)
	}
	if got := Lerp(10, 20, 1); got != 20 {
		t.Errorf("Lerp(10,20,1) = %v, want 20", got)
	}
	if got := Lerp(10, 20, 0.5); got != 15 {
		t.Errorf("Lerp(10,20,0.5) = %v, want 15", got)
	}
}

func TestEaseOutCubicEndpoints(t *testing.T) {
	if got := EaseOutCubic(0); got != 0 {
		t.Errorf("EaseOutCubic(0) = %v, want 0", got)
	}
	if got := EaseOutCubic(1); got != 1 {
		t.Errorf("EaseOutCubic(1) = %v, want 1", got)
	}
}

func TestEaseOutCubicFastStart(t *testing.T) {
	// "Fast start, slow finish": at t=0.25 the eased value should already
	// be past the linear midpoint of that early stretch.
	if got := EaseOutCubic(0.25); got <= 0.25 {
		t.Errorf("EaseOutCubic(0.25) = %v, want > 0.25 (fast start)", got)
	}
}

func TestEaseInCubicEndpoints(t *testing.T) {
	if got := EaseInCubic(0); got != 0 {
		t.Errorf("EaseInCubic(0) = %v, want 0", got)
	}
	if got := EaseInCubic(1); got != 1 {
		t.Errorf("EaseInCubic(1) = %v, want 1", got)
	}
}

func TestEaseInCubicSlowStart(t *testing.T) {
	if got := EaseInCubic(0.25); got >= 0.25 {
		t.Errorf("EaseInCubic(0.25) = %v, want < 0.25 (slow start)", got)
	}
}
