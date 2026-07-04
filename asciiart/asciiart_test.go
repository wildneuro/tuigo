package asciiart

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// solidPNG returns an encoded w x h PNG filled with c, for tests that don't
// care about actual image content, just dimensions/decoding.
func solidPNG(t *testing.T, w, h int, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encoding test PNG: %v", err)
	}
	return buf.Bytes()
}

func TestDecodeRejectsGarbage(t *testing.T) {
	if _, err := Decode([]byte("not an image")); err == nil {
		t.Error("Decode(garbage) = nil error, want an error")
	}
}

func TestGridSamplesExactDimensions(t *testing.T) {
	data := solidPNG(t, 100, 100, color.RGBA{200, 50, 10, 255})
	img, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	grid, err := Grid(img, 8, 4)
	if err != nil {
		t.Fatalf("Grid: %v", err)
	}
	if len(grid) != 4 {
		t.Fatalf("got %d rows, want 4", len(grid))
	}
	for _, row := range grid {
		if len(row) != 8 {
			t.Fatalf("got %d cols, want 8", len(row))
		}
	}
}

func TestGridAveragesSolidColorExactly(t *testing.T) {
	data := solidPNG(t, 50, 50, color.RGBA{200, 50, 10, 255})
	img, _ := Decode(data)
	grid, err := Grid(img, 3, 3)
	if err != nil {
		t.Fatalf("Grid: %v", err)
	}
	want := Pixel{R: 200, G: 50, B: 10}
	if got := grid[1][1]; got != want {
		t.Errorf("center pixel = %+v, want %+v", got, want)
	}
}

func TestGridRejectsNonPositiveDimensions(t *testing.T) {
	data := solidPNG(t, 10, 10, color.Black)
	img, _ := Decode(data)
	if _, err := Grid(img, 0, 5); err == nil {
		t.Error("Grid(img, 0, 5) = nil error, want an error")
	}
	if _, err := Grid(img, 5, -1); err == nil {
		t.Error("Grid(img, 5, -1) = nil error, want an error")
	}
}

func TestGridFitCapsToMaxDimensionsAndPreservesAspect(t *testing.T) {
	// 200x100 (2:1 landscape) source.
	data := solidPNG(t, 200, 100, color.RGBA{100, 100, 100, 255})
	img, _ := Decode(data)
	grid, err := GridFit(img, 40, 40)
	if err != nil {
		t.Fatalf("GridFit: %v", err)
	}
	w := len(grid[0])
	h := len(grid)
	if w > 40 || h > 40 {
		t.Fatalf("GridFit exceeded bounds: %dx%d, want within 40x40", w, h)
	}
	if w == 0 || h == 0 {
		t.Fatalf("GridFit produced an empty grid")
	}
}

func TestRenderProducesOneLinePerRow(t *testing.T) {
	data := solidPNG(t, 20, 20, color.RGBA{128, 128, 128, 255})
	out, err := Render(data, 10, 10)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	lines := 0
	for _, r := range out {
		if r == '\n' {
			lines++
		}
	}
	if lines == 0 {
		t.Error("Render output has no newlines")
	}
}

func TestRenderBlackVsWhiteUseDifferentGlyphs(t *testing.T) {
	black, err := Render(solidPNG(t, 10, 10, color.RGBA{0, 0, 0, 255}), 4, 4)
	if err != nil {
		t.Fatalf("Render(black): %v", err)
	}
	white, err := Render(solidPNG(t, 10, 10, color.RGBA{255, 255, 255, 255}), 4, 4)
	if err != nil {
		t.Fatalf("Render(white): %v", err)
	}
	if black == white {
		t.Error("Render(black) == Render(white), want different glyph ramps")
	}
	if rune(black[0]) != '@' {
		t.Errorf("Render(black)[0] = %q, want '@' (darkest ramp char)", black[0])
	}
	if rune(white[0]) != ' ' {
		t.Errorf("Render(white)[0] = %q, want ' ' (lightest ramp char)", white[0])
	}
}
