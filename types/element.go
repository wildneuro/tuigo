// Package types defines the declarative element tree shared by tuigo's
// layout, renderer, and reconciler packages.
package types

// ElementType identifies the kind of node an Element represents.
type ElementType int

const (
	ElementTypeBox ElementType = iota
	ElementTypeText
	ElementTypeFragment
)

func (t ElementType) String() string {
	switch t {
	case ElementTypeBox:
		return "Box"
	case ElementTypeText:
		return "Text"
	case ElementTypeFragment:
		return "Fragment"
	default:
		return "Unknown"
	}
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
}
