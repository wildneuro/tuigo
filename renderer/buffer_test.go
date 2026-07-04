package renderer

import "testing"

func TestNewBufferIsBlank(t *testing.T) {
	b := NewBuffer(3, 2)
	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			if got := b.At(x, y); got.Rune != ' ' {
				t.Errorf("At(%d,%d) = %+v, want blank", x, y, got)
			}
		}
	}
}

func TestSetAndAtRoundTrip(t *testing.T) {
	b := NewBuffer(3, 2)
	c := Cell{Rune: 'x', Bold: true}
	b.Set(1, 1, c)
	if got := b.At(1, 1); got != c {
		t.Errorf("At(1,1) = %+v, want %+v", got, c)
	}
}

func TestSetOutOfBoundsIsClipped(t *testing.T) {
	b := NewBuffer(2, 2)
	b.Set(-1, 0, Cell{Rune: 'x'})
	b.Set(5, 5, Cell{Rune: 'x'})
	for _, c := range b.Cells {
		if c.Rune != ' ' {
			t.Fatalf("out-of-bounds Set mutated the buffer: %+v", b.Cells)
		}
	}
}

func TestDiffNilPrevProducesFullFrame(t *testing.T) {
	next := NewBuffer(2, 2)
	next.Set(0, 0, Cell{Rune: 'a'})
	patches := Diff(nil, next)
	if len(patches) != 4 {
		t.Fatalf("got %d patches, want 4 (full frame)", len(patches))
	}
}

func TestDiffOnlyReportsChangedCells(t *testing.T) {
	prev := NewBuffer(2, 2)
	next := NewBuffer(2, 2)
	next.Set(1, 1, Cell{Rune: 'z'})
	patches := Diff(prev, next)
	if len(patches) != 1 {
		t.Fatalf("got %d patches, want 1", len(patches))
	}
	if patches[0].X != 1 || patches[0].Y != 1 || patches[0].Cell.Rune != 'z' {
		t.Errorf("patch = %+v, want {1 1 {z ...}}", patches[0])
	}
}

func TestDiffIdenticalBuffersProducesNoPatches(t *testing.T) {
	a := NewBuffer(4, 4)
	b := NewBuffer(4, 4)
	if patches := Diff(a, b); len(patches) != 0 {
		t.Errorf("got %d patches for identical buffers, want 0", len(patches))
	}
}

func TestDiffResizeProducesFullFrame(t *testing.T) {
	prev := NewBuffer(2, 2)
	next := NewBuffer(3, 3)
	patches := Diff(prev, next)
	if len(patches) != 9 {
		t.Fatalf("got %d patches, want 9 (full frame after resize)", len(patches))
	}
}
