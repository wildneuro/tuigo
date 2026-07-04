// Package asciiart decodes images (anything image.Decode supports —
// png/jpeg/gif via blank imports below) into terminal-friendly grids,
// either a grayscale character ramp or per-cell averaged RGB for
// truecolor block rendering. Pure stdlib, no external dependencies —
// consistent with tuigo's "boring, testable layers" philosophy.
package asciiart

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"strings"
)

// rampChars goes from darkest to lightest; index chosen by sampled
// brightness.
const rampChars = "@%#*+=-:. "

// Pixel is one cell's averaged color, 0-255 per channel.
type Pixel struct {
	R, G, B uint8
}

// Decode reads and decodes an image from data (png/jpeg/gif).
func Decode(data []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("asciiart: decode: %w", err)
	}
	return img, nil
}

// Grid downsamples img into exactly w x h cells via nearest-neighbor
// sampling, each cell averaged-color from its source pixel. It does not
// aspect-correct — terminal cells are typically about twice as tall as
// wide, so callers usually want h roughly (w * srcHeight/srcWidth * 0.5);
// GridFit below does that automatically.
func Grid(img image.Image, w, h int) ([][]Pixel, error) {
	bounds := img.Bounds()
	dx, dy := bounds.Dx(), bounds.Dy()
	if dx == 0 || dy == 0 {
		return nil, fmt.Errorf("asciiart: empty image")
	}
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("asciiart: grid dimensions must be positive, got %dx%d", w, h)
	}

	grid := make([][]Pixel, h)
	for y := range h {
		row := make([]Pixel, w)
		sy := bounds.Min.Y + (y * dy / h)
		for x := range w {
			sx := bounds.Min.X + (x * dx / w)
			r, g, b, _ := img.At(sx, sy).RGBA()
			row[x] = Pixel{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8)}
		}
		grid[y] = row
	}
	return grid, nil
}

// GridFit is Grid with h derived from img's aspect ratio to look
// proportional in a terminal (cells ~2x taller than wide), capped to
// maxWidth x maxHeight.
func GridFit(img image.Image, maxWidth, maxHeight int) ([][]Pixel, error) {
	bounds := img.Bounds()
	dx, dy := bounds.Dx(), bounds.Dy()
	if dx == 0 || dy == 0 {
		return nil, fmt.Errorf("asciiart: empty image")
	}
	w := min(maxWidth, dx)
	aspect := float64(dy) / float64(dx)
	h := max(int(float64(w)*aspect*0.5), 1)
	if h > maxHeight {
		h = maxHeight
		w = min(int(float64(h)/(aspect*0.5)), maxWidth)
	}
	return Grid(img, w, h)
}

// Render renders data as a plain grayscale ASCII string — no color, works
// anywhere a monospace font does. maxWidth/maxHeight bound the output;
// the actual size is aspect-fit within them (see GridFit).
func Render(data []byte, maxWidth, maxHeight int) (string, error) {
	img, err := Decode(data)
	if err != nil {
		return "", err
	}
	grid, err := GridFit(img, maxWidth, maxHeight)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, row := range grid {
		for _, p := range row {
			gray := 0.299*float64(p.R) + 0.587*float64(p.G) + 0.114*float64(p.B)
			// Round rather than truncate: 0.299+0.587+0.114 doesn't sum to
			// exactly 1.0 in floating point, so pure white can compute as
			// 254.9999...  and truncate one ramp character short.
			idx := min(int(math.Round(gray/255*float64(len(rampChars)-1))), len(rampChars)-1)
			b.WriteByte(rampChars[idx])
		}
		b.WriteByte('\n')
	}
	return b.String(), nil
}

// RenderFile is Render reading from a file path.
func RenderFile(path string, maxWidth, maxHeight int) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("asciiart: %w", err)
	}
	return Render(data, maxWidth, maxHeight)
}

// GridFile is GridFit reading an image from a file path — the entry point
// most callers doing truecolor block rendering want.
func GridFile(path string, maxWidth, maxHeight int) ([][]Pixel, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("asciiart: %w", err)
	}
	img, err := Decode(data)
	if err != nil {
		return nil, err
	}
	return GridFit(img, maxWidth, maxHeight)
}
