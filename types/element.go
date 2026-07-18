// Package types defines the declarative element tree shared by tuigo's
// layout, renderer, and reconciler packages.
package types

// ElementType identifies the kind of node an Element represents.
type ElementType int

const (
	ElementTypeBox ElementType = iota
	ElementTypeText
	ElementTypeFragment
	// ElementTypeCanvas is a leaf that paints its own cells into its
	// laid-out rect via the Paint callback instead of Box/Text logic. It
	// participates in flex layout like a Box (fills its allocated rect,
	// honours Width/Height, defaults to flex:1) but has no children and no
	// text. TerminalPane is built on it.
	ElementTypeCanvas
)

func (t ElementType) String() string {
	switch t {
	case ElementTypeBox:
		return "Box"
	case ElementTypeText:
		return "Text"
	case ElementTypeFragment:
		return "Fragment"
	case ElementTypeCanvas:
		return "Canvas"
	default:
		return "Unknown"
	}
}

// Surface is the clipped drawing target handed to a Canvas element's Paint
// callback. Coordinates are ABSOLUTE terminal cells (matching layout.Rect);
// writes outside the pane's rect or its ancestor clip are silently dropped.
// It is defined here (not in renderer) so the types package stays leaf —
// the renderer supplies the concrete implementation that wraps its Buffer.
type Surface interface {
	// Bounds reports the pane's absolute laid-out rect (x, y, w, h). A
	// painter uses w×h to size whatever it composites (e.g. a child PTY).
	Bounds() (x, y, w, h int)
	// Set paints one cell at absolute (x, y). A zero rune is treated as a
	// space. Out-of-bounds writes are dropped.
	Set(x, y int, r rune, fg, bg Color, bold, italic, underline bool)
}

// Props holds arbitrary per-element data (e.g. event handlers) that isn't
// part of the core layout/render contract.
type Props map[string]any

// Element is the immutable data structure produced by component functions.
// Constructors never mutate a shared Element; each call returns an
// independent tree fragment.
type Element struct {
	Type          ElementType
	Key           string
	Text          string
	Style         Style
	Props         Props
	Children      []Element
	Handlers      []KeyHandler
	MouseHandlers []MouseHandler
	Focusable     bool
	// Paint is invoked by the renderer for an ElementTypeCanvas node with a
	// Surface clipped to the node's laid-out rect. Nil for every other type.
	Paint func(Surface)
}
