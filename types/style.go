package types

// Style holds every layout- and text-affecting property an Element can
// carry. Layout-affecting fields (Padding, Margin, Gap, Border,
// Width/Height, FlexDir) apply only to the node they're set on and never
// inherit. Text-styling fields (FgColor, BgColor, Bold, Italic, Underline)
// cascade from an ancestor Box down to Text children that don't set their
// own value.
type Style struct {
	Padding   [4]int
	Margin    [4]int
	Gap       int
	FlexDir   FlexDirection
	Width     int
	Height    int
	MinWidth  int
	MinHeight int
	FgColor   Color
	BgColor   Color
	Bold      bool
	Italic    bool
	Underline bool
	Border    BorderStyle
	Truncate  bool
}

// FlexDirection selects how a Box lays out its children.
type FlexDirection int

const (
	FlexDirectionRow FlexDirection = iota
	FlexDirectionColumn
)

// BorderStyle selects the border glyphs a Box draws around itself.
type BorderStyle int

const (
	BorderNone BorderStyle = iota
	BorderSingle
	BorderDouble
	BorderRounded
)

// Color is a terminal color index; the zero value means "unset" so styles
// can cascade without an explicit sentinel.
// Positive values (1-255) are xterm 256-color palette indices.
// Values with rgbFlag set encode 24-bit RGB colors in their low 24 bits
// (0xRRGGBB) — a flag bit rather than the sign, since r<<16|g<<8|b is
// never negative for byte inputs.
type Color int

const rgbFlag = 1 << 24

func RGB(r, g, b byte) Color {
	return Color(rgbFlag | int(r)<<16 | int(g)<<8 | int(b))
}

func (c Color) IsRGB() bool {
	return int(c)&rgbFlag != 0
}

func (c Color) RGB() (r, g, b byte) {
	if !c.IsRGB() {
		return 0, 0, 0
	}
	v := int(c)
	return byte(v >> 16), byte(v >> 8), byte(v)
}
