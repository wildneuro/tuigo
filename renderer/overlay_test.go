package renderer

import "testing"

func TestStampCopiesSourceOntoDestAtOffset(t *testing.T) {
	dst := NewBuffer(5, 5)
	src := NewBuffer(2, 2)
	src.Set(0, 0, Cell{Rune: 'a'})
	src.Set(1, 1, Cell{Rune: 'b'})

	Stamp(dst, src, 2, 2)

	if got := dst.At(2, 2).Rune; got != 'a' {
		t.Errorf("dst.At(2,2) = %q, want 'a'", got)
	}
	if got := dst.At(3, 3).Rune; got != 'b' {
		t.Errorf("dst.At(3,3) = %q, want 'b'", got)
	}
	if got := dst.At(0, 0).Rune; got != ' ' {
		t.Errorf("dst.At(0,0) = %q, want blank (untouched)", got)
	}
}

func TestStampClipsAtDestBounds(t *testing.T) {
	dst := NewBuffer(3, 3)
	src := NewBuffer(4, 4)
	src.Set(3, 3, Cell{Rune: 'z'})

	// Must not panic even though src extends past dst's bounds at (1,1)+4x4.
	Stamp(dst, src, 1, 1)

	if got := dst.At(1, 1).Rune; got != ' ' {
		t.Errorf("dst.At(1,1) = %q, want blank (src's default cell)", got)
	}
}

func TestStampNegativeOffsetClips(t *testing.T) {
	dst := NewBuffer(3, 3)
	src := NewBuffer(2, 2)
	src.Set(0, 0, Cell{Rune: 'x'})
	src.Set(1, 1, Cell{Rune: 'y'})

	// Top-left corner of src falls off dst; must not panic.
	Stamp(dst, src, -1, -1)

	if got := dst.At(0, 0).Rune; got != 'y' {
		t.Errorf("dst.At(0,0) = %q, want 'y' (the in-bounds part of src)", got)
	}
}
