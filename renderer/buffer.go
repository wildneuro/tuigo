package renderer

// Buffer is a 2D grid of cells, row-major.
type Buffer struct {
	Width, Height int
	Cells         []Cell
}

// NewBuffer allocates a w x h buffer filled with blank cells.
func NewBuffer(w, h int) *Buffer {
	cells := make([]Cell, w*h)
	for i := range cells {
		cells[i] = Cell{Rune: ' '}
	}
	return &Buffer{Width: w, Height: h, Cells: cells}
}

// At returns the cell at (x, y). Out-of-bounds coordinates return a blank
// cell rather than panicking, since layout can produce rects that graze the
// viewport edge.
func (b *Buffer) At(x, y int) Cell {
	if x < 0 || y < 0 || x >= b.Width || y >= b.Height {
		return Cell{Rune: ' '}
	}
	return b.Cells[y*b.Width+x]
}

// Set writes a cell at (x, y), silently clipping out-of-bounds writes.
func (b *Buffer) Set(x, y int, c Cell) {
	if x < 0 || y < 0 || x >= b.Width || y >= b.Height {
		return
	}
	b.Cells[y*b.Width+x] = c
}

// Patch is a single cell that changed between two frames.
type Patch struct {
	X, Y int
	Cell Cell
}

// Diff returns the cells that differ between prev and next, in row-major
// order. A nil or differently-sized prev produces a full-frame patch list.
func Diff(prev, next *Buffer) []Patch {
	if prev == nil || prev.Width != next.Width || prev.Height != next.Height {
		patches := make([]Patch, 0, len(next.Cells))
		for y := range next.Height {
			for x := range next.Width {
				patches = append(patches, Patch{X: x, Y: y, Cell: next.At(x, y)})
			}
		}
		return patches
	}
	var patches []Patch
	for i, c := range next.Cells {
		if prev.Cells[i] != c {
			patches = append(patches, Patch{X: i % next.Width, Y: i / next.Width, Cell: c})
		}
	}
	return patches
}
