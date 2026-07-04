package renderer

// Stamp copies src's cells onto dst at offset (x, y), clipping to dst's
// bounds via Buffer.Set. This is how tuigo composites floating content —
// Picture-in-Picture panels, popups — on top of the main frame without
// needing full z-order support in the layout tree: render the overlay's
// own small Element tree into an independent Buffer via Draw, then Stamp
// it onto the frame buffer before diffing (see Ctx.Overlay in the root
// package). It runs after the main Draw call and before Diff, so an
// overlay naturally participates in the existing cell-diff — only the
// overlay's changed cells get re-flushed, same as everything else.
func Stamp(dst, src *Buffer, x, y int) {
	for sy := range src.Height {
		for sx := range src.Width {
			dst.Set(x+sx, y+sy, src.At(sx, sy))
		}
	}
}
