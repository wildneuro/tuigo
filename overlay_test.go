package tuigo

import (
	"testing"

	"github.com/wildneuro/tuigo/renderer"
)

func TestDimBackdropOverwritesFgBg(t *testing.T) {
	buf := renderer.NewBuffer(3, 2)
	buf.Set(0, 0, renderer.Cell{Rune: 'A', Fg: ColorBrightWhite, Bg: ColorBlue, Bold: true})
	buf.Set(1, 0, renderer.Cell{Rune: 'B', Fg: ColorRed, Bg: ColorGreen, Underline: true})

	dimFg, dimBg := RGB(0x33, 0x33, 0x33), RGB(0x03, 0x03, 0x03)
	DimBackdrop(buf, dimFg, dimBg)

	for y := 0; y < buf.Height; y++ {
		for x := 0; x < buf.Width; x++ {
			c := buf.At(x, y)
			if c.Fg != dimFg || c.Bg != dimBg {
				t.Errorf("cell (%d,%d) Fg/Bg = %v/%v, want dim colors %v/%v", x, y, c.Fg, c.Bg, dimFg, dimBg)
			}
			if c.Bold || c.Italic || c.Underline {
				t.Errorf("cell (%d,%d) still carries an SGR attribute after dimming: %+v", x, y, c)
			}
		}
	}
}

func TestDimBackdropPreservesRunes(t *testing.T) {
	buf := renderer.NewBuffer(2, 1)
	buf.Set(0, 0, renderer.Cell{Rune: 'X'})
	buf.Set(1, 0, renderer.Cell{Rune: 'Y'})

	DimBackdrop(buf, ColorGray, ColorBlack)

	if got := buf.At(0, 0).Rune; got != 'X' {
		t.Errorf("rune at (0,0) = %q, want 'X' (dimming must not touch the rune)", got)
	}
	if got := buf.At(1, 0).Rune; got != 'Y' {
		t.Errorf("rune at (1,0) = %q, want 'Y' (dimming must not touch the rune)", got)
	}
}

func TestDimBackdropNilBufferNoPanic(t *testing.T) {
	DimBackdrop(nil, ColorGray, ColorBlack) // must not panic
}
