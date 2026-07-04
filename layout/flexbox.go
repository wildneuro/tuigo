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
	flexSize := 0
	if flexCount > 0 {
		flexSize = remaining / flexCount
	}

	nodes := make([]Node, n)
	pos := content.X
	if dir == types.FlexDirectionColumn {
		pos = content.Y
	}
	for i, c := range children {
		sz := sizes[i]
		if sz < 0 {
			sz = flexSize
		}
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
