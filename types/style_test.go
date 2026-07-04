package types

import "testing"

func TestRGBColorIsRecognizedAsRGB(t *testing.T) {
	c := RGB(10, 20, 30)
	if !c.IsRGB() {
		t.Errorf("RGB(10,20,30).IsRGB() = false, want true")
	}
	r, g, b := c.RGB()
	if r != 10 || g != 20 || b != 30 {
		t.Errorf("RGB round-trip = (%d,%d,%d), want (10,20,30)", r, g, b)
	}
}

func TestPaletteColorIsNotRGB(t *testing.T) {
	c := Color(5)
	if c.IsRGB() {
		t.Errorf("Color(5).IsRGB() = true, want false (it's a palette index)")
	}
}

func TestRGBBlackIsDistinguishableFromUnset(t *testing.T) {
	// RGB(0,0,0) must not be confused with the zero-value "unset" sentinel.
	c := RGB(0, 0, 0)
	if c == 0 {
		t.Errorf("RGB(0,0,0) == 0, collides with the unset sentinel")
	}
	if !c.IsRGB() {
		t.Errorf("RGB(0,0,0).IsRGB() = false, want true")
	}
}
