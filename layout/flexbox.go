// Package layout computes pure Go flexbox geometry for an Element tree. It
// never touches a terminal or mutates its input.
package layout

import "github.com/wildneuro/tuigo/types"

// Rect is an axis-aligned region in terminal cells.
type Rect struct {
	X, Y, W, H int
}

// Intersect returns the overlapping region of r and o, with zero width/height
// if they don't overlap.
func (r Rect) Intersect(o Rect) Rect {
	x1, y1 := max(r.X, o.X), max(r.Y, o.Y)
	x2, y2 := min(r.X+r.W, o.X+o.W), min(r.Y+r.H, o.Y+o.H)
	if x2 < x1 {
		x2 = x1
	}
	if y2 < y1 {
		y2 = y1
	}
	return Rect{x1, y1, x2 - x1, y2 - y1}
}

// Node mirrors an Element with its computed Rect attached.
type Node struct {
	Element  types.Element
	Rect     Rect
	Children []Node
}

// Layout computes the position and size of el and every descendant within a
// width x height viewport. It is deterministic and never mutates el.
func Layout(el types.Element, width, height int) Node {
	return layoutNode(el, Rect{0, 0, width, height})
}

func layoutNode(el types.Element, rect Rect) Node {
	rect = applyMargin(rect, el.Style.Margin)
	if el.Style.Width > 0 {
		rect.W = el.Style.Width
	}
	if el.Style.Height > 0 {
		rect.H = el.Style.Height
	}

	n := Node{Element: el, Rect: rect}

	switch el.Type {
	case types.ElementTypeFragment:
		for _, c := range el.Children {
			n.Children = append(n.Children, layoutNode(c, rect))
		}
	case types.ElementTypeBox:
		content := ContentRect(rect, el.Style)
		n.Children = layoutChildren(el.Children, el.Style, content)
	}
	return n
}

// ContentRect returns the area inside a Box's border and padding.
func ContentRect(rect Rect, s types.Style) Rect {
	x, y, w, h := rect.X, rect.Y, rect.W, rect.H
	if s.Border != types.BorderNone {
		x++
		y++
		w -= 2
		h -= 2
	}
	x += s.Padding[3]
	y += s.Padding[0]
	w -= s.Padding[1] + s.Padding[3]
	h -= s.Padding[0] + s.Padding[2]
	return Rect{x, y, max(w, 0), max(h, 0)}
}

func applyMargin(rect Rect, m [4]int) Rect {
	return Rect{
		X: rect.X + m[3],
		Y: rect.Y + m[0],
		W: max(rect.W-m[1]-m[3], 0),
		H: max(rect.H-m[0]-m[2], 0),
	}
}

// layoutChildren distributes content's main-axis size among children:
// explicit Width/Height wins, Text nodes get their intrinsic (wrapped) size,
// and everything else shares the remaining space equally (flex:1 default).
// Every child stretches to fill the full cross-axis size.
func layoutChildren(children []types.Element, style types.Style, content Rect) []Node {
	n := len(children)
	if n == 0 {
		return nil
	}
	dir := style.FlexDir
	gap := style.Gap

	// For Row direction, wrapped Text children may need more height than
	// content.H provides. We first compute the max intrinsic cross-axis size
	// and grow the content area if needed. This is a single-pass algorithm:
	// we assume the content width doesn't change as a result of growing height,
	// which is correct for wrapped Text (wrapping depends on width, not height).
	if dir == types.FlexDirectionRow {
		maxH := content.H
		for _, c := range children {
			if h := intrinsicCrossSize(c, content.W); h > maxH {
				maxH = h
			}
		}
		if maxH > content.H {
			content.H = maxH
		}
	}

	mainSize := content.W
	if dir == types.FlexDirectionColumn {
		mainSize = content.H
	}

	sizes := make([]int, n)
	flexCount := 0
	used := gap * max(n-1, 0)
	for i, c := range children {
		sz := explicitMainSize(c, dir)
		if sz < 0 {
			sz = intrinsicMainSize(c, dir, content)
		}
		if sz < 0 {
			flexCount++
			sizes[i] = -1
			continue
		}
		sizes[i] = sz
		used += sz
	}
	remaining := max(mainSize-used, 0)

	// Give every flex child at least its min-content size before splitting
	// the rest equally. Without this, a bordered/padded flex child (e.g. a
	// Menu = FlexColumn+Border needing N+2 rows) inside a too-short parent
	// silently collapses toward 0 rows — its content vanishes with no
	// visual trace of why. This is a classic flexbox min-size "waterfall":
	// repeatedly pin the child whose min-content exceeds its current equal
	// share at that min, then re-split the remaining space (and remaining
	// budget) among the still-unpinned children, until no pinned child
	// remains oversized relative to a fresh equal split. At most flexCount
	// iterations (one child gets pinned per pass in the worst case).
	if flexCount > 0 {
		pinned := make([]bool, n)
		pinnedTotal := 0
		activeCount := flexCount
		for {
			if activeCount == 0 {
				break
			}
			share := (remaining - pinnedTotal) / activeCount
			changed := false
			for i, c := range children {
				if sizes[i] != -1 || pinned[i] {
					continue
				}
				m := minContentMainSize(c, dir, content)
				if m > share {
					sizes[i] = m
					pinned[i] = true
					pinnedTotal += m
					activeCount--
					changed = true
				}
			}
			if !changed {
				break
			}
		}
		// Whatever's left over the min-content floors is split equally among
		// the still-unpinned flex children. If pinnedTotal alone already
		// exceeds remaining, activeCount is 0 here (every child got pinned
		// at its min in the loop above) and the row/column simply overflows
		// — the renderer clips it, which beats every child silently
		// collapsing to nothing.
		if activeCount > 0 {
			leftoverShare := max(remaining-pinnedTotal, 0) / activeCount
			for i, sz := range sizes {
				if sz == -1 {
					sizes[i] = leftoverShare
				}
			}
		}
	}

	nodes := make([]Node, n)
	pos := content.X
	if dir == types.FlexDirectionColumn {
		pos = content.Y
	}
	for i, c := range children {
		// Every -1 (flex) slot was resolved above, either pinned to its
		// min-content size or given an equal leftover share.
		sz := sizes[i]
		var childRect Rect
		if dir == types.FlexDirectionRow {
			childRect = Rect{X: pos, Y: content.Y, W: sz, H: content.H}
		} else {
			childRect = Rect{X: content.X, Y: pos, W: content.W, H: sz}
		}
		nodes[i] = layoutNode(c, childRect)
		pos += sz + gap
	}
	return nodes
}

func explicitMainSize(c types.Element, dir types.FlexDirection) int {
	if dir == types.FlexDirectionRow {
		if c.Style.Width > 0 {
			return c.Style.Width
		}
	} else if c.Style.Height > 0 {
		return c.Style.Height
	}
	return -1
}

// intrinsicMainSize returns a Text node's natural size: wrapped line count
// in a column, or single-line rune length in a row. Non-Text nodes without
// an explicit size are flex items, signaled by -1.
func intrinsicMainSize(c types.Element, dir types.FlexDirection, content Rect) int {
	if c.Type != types.ElementTypeText {
		return -1
	}
	if c.Style.Truncate {
		if dir == types.FlexDirectionColumn {
			return 1
		}
		return content.W
	}
	if dir == types.FlexDirectionColumn {
		return len(WrapText(c.Text, max(content.W, 1)))
	}
	return len([]rune(c.Text))
}

// maxMinContentDepth caps minContentMainSize's recursion. A pathological or
// cyclic-looking tree (there's no cycle possible in an immutable Element
// tree, but a very deep one is plausible from generated UI) degrades to
// "no floor" beyond this depth rather than blowing the stack.
const maxMinContentDepth = 32

// minContentMainSize returns the smallest size c can be laid out at along
// dir without losing content: for Text, its existing intrinsic (wrapped)
// size; for a Box, its border (2 cells, if any) plus padding along dir plus
// its children's min-content, summed if the Box's own flex direction
// matches dir (children stack, so their minimums add) or maxed if it's
// cross-wise (children overlap the main axis, so the largest one governs).
// An explicit Width/Height on c always wins outright — that's the author
// asserting an exact size, not a floor. This is what lets a bordered flex
// child (e.g. Menu) claim at least enough room for its border-plus-rows
// instead of collapsing to 0 when its parent is short on space.
func minContentMainSize(c types.Element, dir types.FlexDirection, content Rect) int {
	return minContentMainSizeDepth(c, dir, content, 0)
}

func minContentMainSizeDepth(c types.Element, dir types.FlexDirection, content Rect, depth int) int {
	if depth >= maxMinContentDepth {
		return 0
	}
	if sz := explicitMainSize(c, dir); sz >= 0 {
		return sz
	}
	if c.Type == types.ElementTypeText {
		if m := intrinsicMainSize(c, dir, content); m >= 0 {
			return m
		}
		return 0
	}
	if c.Type != types.ElementTypeBox {
		return 0 // Fragment/Canvas: no min-content floor of their own
	}

	min := 0
	if c.Style.Border != types.BorderNone {
		min += 2
	}
	if dir == types.FlexDirectionRow {
		min += c.Style.Padding[1] + c.Style.Padding[3] // right + left
	} else {
		min += c.Style.Padding[0] + c.Style.Padding[2] // top + bottom
	}

	if len(c.Children) == 0 {
		return min
	}
	// Shrink content by this box's own border/padding before recursing, so
	// grandchildren's Text-wrapping min-content is computed against the
	// space actually available to them, not the outer content rect.
	childContent := ContentRect(Rect{W: content.W, H: content.H}, c.Style)

	if c.Style.FlexDir == dir {
		childrenMin := c.Style.Gap * max(len(c.Children)-1, 0)
		for _, cc := range c.Children {
			childrenMin += minContentMainSizeDepth(cc, dir, childContent, depth+1)
		}
		return min + childrenMin
	}
	maxChild := 0
	for _, cc := range c.Children {
		if m := minContentMainSizeDepth(cc, dir, childContent, depth+1); m > maxChild {
			maxChild = m
		}
	}
	return min + maxChild
}

// intrinsicCrossSize returns the intrinsic cross-axis size for a child in a
// Row layout (i.e., the height a Text node needs when it wraps). Returns -1
// if the child should just use the parent's cross-axis size.
func intrinsicCrossSize(c types.Element, contentW int) int {
	if c.Type != types.ElementTypeText {
		return -1
	}
	if c.Style.Truncate {
		return 1
	}
	return len(WrapText(c.Text, max(contentW, 1)))
}
