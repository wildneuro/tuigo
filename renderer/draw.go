package renderer

import (
	"github.com/wildneuro/tuigo/layout"
	"github.com/wildneuro/tuigo/types"
)

type borderChars struct {
	TopLeft, TopRight, BottomLeft, BottomRight rune
	Horizontal, Vertical                       rune
}

var (
	singleBorder  = borderChars{'┌', '┐', '└', '┘', '─', '│'}
	doubleBorder  = borderChars{'╔', '╗', '╚', '╝', '═', '║'}
	roundedBorder = borderChars{'╭', '╮', '╰', '╯', '─', '│'}
)

// Draw walks a laid-out tree and renders it into a fresh Buffer sized
// width x height.
func Draw(root layout.Node, width, height int) *Buffer {
	buf := NewBuffer(width, height)
	drawNode(buf, root, types.Style{}, layout.Rect{X: 0, Y: 0, W: width, H: height})
	return buf
}

// drawNode resolves text-styling cascade from inherited, clips every write
// to clip (the intersection of every ancestor's content area), and recurses
// into children with the cascaded style and a tighter clip.
func drawNode(buf *Buffer, n layout.Node, inherited types.Style, clip layout.Rect) {
	resolved := resolveCascade(inherited, n.Element.Style)

	switch n.Element.Type {
	case types.ElementTypeBox:
		fillBackground(buf, n.Rect, clip, resolved)
		if n.Element.Style.Border != types.BorderNone {
			drawBorder(buf, n.Rect, clip, n.Element.Style.Border, resolved)
		}
		content := layout.ContentRect(n.Rect, n.Element.Style).Intersect(clip)
		for _, c := range n.Children {
			drawNode(buf, c, resolved, content)
		}
	case types.ElementTypeText:
		drawText(buf, n.Rect, clip, n.Element.Text, resolved)
	case types.ElementTypeFragment:
		for _, c := range n.Children {
			drawNode(buf, c, resolved, clip)
		}
	}
}

// resolveCascade applies STYLEGUIDE.md rule 4: text-styling fields cascade
// from ancestor to descendant unless the descendant sets its own; layout
// fields (not read here) never inherit.
func resolveCascade(inherited, own types.Style) types.Style {
	r := inherited
	if own.FgColor != 0 {
		r.FgColor = own.FgColor
	}
	if own.BgColor != 0 {
		r.BgColor = own.BgColor
	}
	if own.Bold {
		r.Bold = true
	}
	if own.Italic {
		r.Italic = true
	}
	if own.Underline {
		r.Underline = true
	}
	return r
}

func fillBackground(buf *Buffer, rect, clip layout.Rect, style types.Style) {
	if style.BgColor == 0 {
		return
	}
	r := rect.Intersect(clip)
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			buf.Set(x, y, Cell{Rune: ' ', Fg: style.FgColor, Bg: style.BgColor})
		}
	}
}

func drawBorder(buf *Buffer, rect, clip layout.Rect, kind types.BorderStyle, style types.Style) {
	if rect.W <= 0 || rect.H <= 0 {
		return
	}
	chars := singleBorder
	switch kind {
	case types.BorderDouble:
		chars = doubleBorder
	case types.BorderRounded:
		chars = roundedBorder
	}
	put := func(x, y int, r rune) {
		if x < clip.X || x >= clip.X+clip.W || y < clip.Y || y >= clip.Y+clip.H {
			return
		}
		buf.Set(x, y, Cell{Rune: r, Fg: style.FgColor, Bg: style.BgColor})
	}
	x0, y0 := rect.X, rect.Y
	x1, y1 := rect.X+rect.W-1, rect.Y+rect.H-1
	put(x0, y0, chars.TopLeft)
	put(x1, y0, chars.TopRight)
	put(x0, y1, chars.BottomLeft)
	put(x1, y1, chars.BottomRight)
	for x := x0 + 1; x < x1; x++ {
		put(x, y0, chars.Horizontal)
		put(x, y1, chars.Horizontal)
	}
	for y := y0 + 1; y < y1; y++ {
		put(x0, y, chars.Vertical)
		put(x1, y, chars.Vertical)
	}
}

func drawText(buf *Buffer, rect, clip layout.Rect, text string, style types.Style) {
	if style.Truncate {
		drawTextTruncate(buf, rect, clip, text, style)
		return
	}
	lines := layout.WrapText(text, max(rect.W, 1))
	for i, line := range lines {
		y := rect.Y + i
		if y >= rect.Y+rect.H || y < clip.Y || y >= clip.Y+clip.H {
			continue
		}
		for j, r := range []rune(line) {
			x := rect.X + j
			if x >= rect.X+rect.W || x < clip.X || x >= clip.X+clip.W {
				continue
			}
			buf.Set(x, y, Cell{Rune: r, Fg: style.FgColor, Bg: style.BgColor, Bold: style.Bold, Italic: style.Italic, Underline: style.Underline})
		}
	}
}

func drawTextTruncate(buf *Buffer, rect, clip layout.Rect, text string, style types.Style) {
	y := rect.Y
	if y < clip.Y || y >= clip.Y+clip.H {
		return
	}
	runes := []rune(text)
	avail := rect.W
	if avail <= 0 {
		return
	}
	if len(runes) > avail {
		runes = runes[:avail-1]
		buf.Set(rect.X+avail-1, y, Cell{Rune: '…', Fg: style.FgColor, Bg: style.BgColor, Bold: style.Bold, Italic: style.Italic, Underline: style.Underline})
	}
	for j, r := range runes {
		x := rect.X + j
		if x < clip.X || x >= clip.X+clip.W {
			continue
		}
		buf.Set(x, y, Cell{Rune: r, Fg: style.FgColor, Bg: style.BgColor, Bold: style.Bold, Italic: style.Italic, Underline: style.Underline})
	}
}
