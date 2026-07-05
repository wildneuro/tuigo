package tuigo

// This file is small on purpose: tuigo has no animation/transition system
// built into the render loop (see ARCHITECTURE.md — Render only re-renders
// on discrete events plus a 1s ticker). Smooth motion is an
// application-level pattern, not a framework feature: track a start time
// in UseState, compute progress each render, and use Ctx.After to request
// extra fast re-renders only while something is actually animating. Lerp
// and the easing functions below are the only genuinely reusable pieces of
// that pattern — see _examples/chat's sliding info drawer for the pattern
// itself.

// Clamp01 clamps t to [0, 1] — animation progress is almost always a
// t := elapsed/duration computation that can overshoot past 1 once the
// animation has finished but before the caller notices and stops ticking.
func Clamp01(t float64) float64 {
	return max(min(t, 1), 0)
}

// Lerp linearly interpolates between a and b by t. t is not clamped —
// pass Clamp01(t) yourself if you need that, since some callers
// deliberately extrapolate (e.g. an overshoot bounce).
func Lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}

// EaseOutCubic is a common UI motion curve: fast start, slow finish. Pass
// a Clamp01'd t in [0, 1].
func EaseOutCubic(t float64) float64 {
	u := 1 - t
	return 1 - u*u*u
}

// EaseInCubic is the mirror of EaseOutCubic: slow start, fast finish.
func EaseInCubic(t float64) float64 {
	return t * t * t
}
