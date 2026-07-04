// Package tuigo is a declarative, component-based terminal UI framework.
package tuigo

import (
	"fmt"

	"github.com/wildneuro/tuigo/types"
)

// Element is the tree node produced by Box, Text, and Fragment.
type Element = types.Element

// Option mutates an Element during construction. Options apply in the
// order they're passed and never mutate a shared Element.
type Option func(*Element)

// Box is a flex container: it lays out its children and never renders text
// of its own.
func Box(opts ...Option) Element {
	e := Element{Type: types.ElementTypeBox}
	for _, opt := range opts {
		opt(&e)
	}
	return e
}

// Text renders a formatted string. Unset style fields cascade from the
// nearest ancestor Box.
func Text(format string, args ...any) Element {
	e := Element{Type: types.ElementTypeText, Text: fmt.Sprintf(format, args...)}
	return e
}

// Fragment groups children without introducing a layout box of its own.
func Fragment(children ...Element) Element {
	return Element{Type: types.ElementTypeFragment, Children: children}
}

// Children attaches child elements to a Box.
func Children(children ...Element) Option {
	return func(e *Element) {
		e.Children = children
	}
}

// WithKey assigns a stable identity used by the reconciler to match nodes
// across renders.
func WithKey(key string) Option {
	return func(e *Element) {
		e.Key = key
	}
}

// With applies style/key options to an already-constructed Element (most
// commonly a Text node, whose constructor reserves its variadic args for
// fmt.Sprintf). It never mutates the Element passed in.
func With(e Element, opts ...Option) Element {
	for _, opt := range opts {
		opt(&e)
	}
	return e
}
