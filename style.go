package tuigo

import "github.com/wildneuro/tuigo/types"

// Re-exported so callers only ever import the tuigo package for TUIML.
type (
	Color         = types.Color
	BorderStyle   = types.BorderStyle
	FlexDirection = types.FlexDirection
)

var RGB = types.RGB

const (
	BorderNone    = types.BorderNone
	BorderSingle  = types.BorderSingle
	BorderDouble  = types.BorderDouble
	BorderRounded = types.BorderRounded
)

// Named xterm-256 colors. Color's zero value means "unset/inherit" (see
// types.Color), so plain black lives at the extended-palette index 16
// rather than the ambiguous 0.
const (
	ColorBlack   Color = 16
	ColorRed     Color = 1
	ColorGreen   Color = 2
	ColorYellow  Color = 3
	ColorBlue    Color = 4
	ColorMagenta Color = 5
	ColorCyan    Color = 6
	ColorWhite   Color = 7
	ColorGray    Color = 8

	ColorBrightRed     Color = 9
	ColorBrightGreen   Color = 10
	ColorBrightYellow  Color = 11
	ColorBrightBlue    Color = 12
	ColorBrightMagenta Color = 13
	ColorBrightCyan    Color = 14
	ColorBrightWhite   Color = 15
)

// Padding sets all four padding edges to n.
func Padding(n int) Option {
	return func(e *Element) { e.Style.Padding = [4]int{n, n, n, n} }
}

func PaddingTop(n int) Option    { return func(e *Element) { e.Style.Padding[0] = n } }
func PaddingRight(n int) Option  { return func(e *Element) { e.Style.Padding[1] = n } }
func PaddingBottom(n int) Option { return func(e *Element) { e.Style.Padding[2] = n } }
func PaddingLeft(n int) Option   { return func(e *Element) { e.Style.Padding[3] = n } }

// Margin sets all four margin edges to n.
func Margin(n int) Option {
	return func(e *Element) { e.Style.Margin = [4]int{n, n, n, n} }
}

func MarginTop(n int) Option    { return func(e *Element) { e.Style.Margin[0] = n } }
func MarginRight(n int) Option  { return func(e *Element) { e.Style.Margin[1] = n } }
func MarginBottom(n int) Option { return func(e *Element) { e.Style.Margin[2] = n } }
func MarginLeft(n int) Option   { return func(e *Element) { e.Style.Margin[3] = n } }

// Gap sets the spacing between a Box's children.
func Gap(n int) Option {
	return func(e *Element) { e.Style.Gap = n }
}

// FlexRow lays out children left-to-right. It's the zero-value direction,
// so this option exists mainly for explicitness.
func FlexRow() Option {
	return func(e *Element) { e.Style.FlexDir = types.FlexDirectionRow }
}

// FlexColumn lays out children top-to-bottom.
func FlexColumn() Option {
	return func(e *Element) { e.Style.FlexDir = types.FlexDirectionColumn }
}

func Width(n int) Option     { return func(e *Element) { e.Style.Width = n } }
func Height(n int) Option    { return func(e *Element) { e.Style.Height = n } }
func MinWidth(n int) Option  { return func(e *Element) { e.Style.MinWidth = n } }
func MinHeight(n int) Option { return func(e *Element) { e.Style.MinHeight = n } }

// Border sets the border style drawn around a Box.
func Border(b BorderStyle) Option {
	return func(e *Element) { e.Style.Border = b }
}

func Bold() Option      { return func(e *Element) { e.Style.Bold = true } }
func Italic() Option    { return func(e *Element) { e.Style.Italic = true } }
func Underline() Option { return func(e *Element) { e.Style.Underline = true } }

// Truncate disables word-wrapping for a Text element and instead renders a
// single line, replacing the last visible character with '…' if the text
// is wider than the available width.
func Truncate() Option { return func(e *Element) { e.Style.Truncate = true } }

// ColorFg sets the cascading foreground text color.
func ColorFg(c Color) Option {
	return func(e *Element) { e.Style.FgColor = c }
}

// ColorBg sets the background color.
func ColorBg(c Color) Option {
	return func(e *Element) { e.Style.BgColor = c }
}

// ThemeColors is a small, intentional color palette for tuigo applications.
// Prefer theme colors over raw palette indices; use RGB only for one-off
// visual effects that don't belong in the theme.
type ThemeColors struct {
	// Surfaces
	Background Color
	Surface    Color
	SurfaceAlt Color

	// Text
	TextPrimary Color
	TextMuted   Color

	// Accents
	Accent    Color
	AccentAlt Color

	// Semantic
	Success Color
	Warning Color
	Error   Color
}

var DarkTheme = ThemeColors{
	Background:  ColorBlack,
	Surface:     RGB(30, 30, 30),
	SurfaceAlt:  RGB(45, 45, 45),
	TextPrimary: ColorBrightWhite,
	TextMuted:   ColorGray,
	Accent:      RGB(0, 180, 216),
	AccentAlt:   RGB(0, 216, 180),
	Success:     ColorGreen,
	Warning:     ColorYellow,
	Error:       ColorRed,
}

var LightTheme = ThemeColors{
	Background:  RGB(245, 245, 245),
	Surface:     RGB(255, 255, 255),
	SurfaceAlt:  RGB(225, 225, 225),
	TextPrimary: RGB(20, 20, 20),
	TextMuted:   RGB(90, 90, 90),
	Accent:      RGB(0, 110, 170),
	AccentAlt:   RGB(0, 140, 120),
	Success:     RGB(20, 130, 60),
	Warning:     RGB(160, 110, 0),
	Error:       RGB(180, 30, 30),
}

// NamedThemes indexes the built-in themes for cycling/lookup (e.g. a
// "/theme" command).
var NamedThemes = map[string]ThemeColors{
	"dark":  DarkTheme,
	"light": LightTheme,
}

// Theme is the active palette. It starts as DarkTheme; call SetTheme to
// switch it — every subsequent render that reads Theme.* picks up the
// change immediately, since it's read fresh each render, not cached.
var Theme = DarkTheme

// SetTheme replaces the active theme wholesale.
func SetTheme(t ThemeColors) { Theme = t }
