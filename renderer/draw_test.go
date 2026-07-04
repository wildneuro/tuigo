package renderer

import (
	"testing"

	"github.com/wildneuro/tuigo/layout"
	"github.com/wildneuro/tuigo/types"
)

func TestDrawWritesTextAtOrigin(t *testing.T) {
	el := types.Element{Type: types.ElementTypeText, Text: "hi"}
	node := layout.Layout(el, 5, 1)
	buf := Draw(node, 5, 1)
	if buf.At(0, 0).Rune != 'h' || buf.At(1, 0).Rune != 'i' {
		t.Errorf("At(0,0)=%q At(1,0)=%q, want 'h','i'", buf.At(0, 0).Rune, buf.At(1, 0).Rune)
	}
}

func TestDrawBorderDrawsCorners(t *testing.T) {
	el := types.Element{Type: types.ElementTypeBox, Style: types.Style{Border: types.BorderSingle}}
	node := layout.Layout(el, 4, 3)
	buf := Draw(node, 4, 3)
	if buf.At(0, 0).Rune != '┌' || buf.At(3, 0).Rune != '┐' {
		t.Errorf("top corners = %q %q, want ┌ ┐", buf.At(0, 0).Rune, buf.At(3, 0).Rune)
	}
	if buf.At(0, 2).Rune != '└' || buf.At(3, 2).Rune != '┘' {
		t.Errorf("bottom corners = %q %q, want └ ┘", buf.At(0, 2).Rune, buf.At(3, 2).Rune)
	}
}

func TestDrawCascadesFgColorToTextChildren(t *testing.T) {
	el := types.Element{
		Type:  types.ElementTypeBox,
		Style: types.Style{FgColor: 5},
		Children: []types.Element{
			{Type: types.ElementTypeText, Text: "x"},
		},
	}
	node := layout.Layout(el, 3, 1)
	buf := Draw(node, 3, 1)
	if got := buf.At(0, 0).Fg; got != 5 {
		t.Errorf("child Fg = %v, want 5 (cascaded)", got)
	}
}

func TestDrawChildOverridesInheritedColor(t *testing.T) {
	el := types.Element{
		Type:  types.ElementTypeBox,
		Style: types.Style{FgColor: 5},
		Children: []types.Element{
			{Type: types.ElementTypeText, Text: "x", Style: types.Style{FgColor: 9}},
		},
	}
	node := layout.Layout(el, 3, 1)
	buf := Draw(node, 3, 1)
	if got := buf.At(0, 0).Fg; got != 9 {
		t.Errorf("child Fg = %v, want 9 (own override)", got)
	}
}

func TestDrawClipsChildToParentContentArea(t *testing.T) {
	// A 3-wide bordered box has 1 column of content; a longer text child
	// must not spill past the border into the sibling to its right.
	el := types.Element{
		Type:  types.ElementTypeBox,
		Style: types.Style{Border: types.BorderSingle},
		Children: []types.Element{
			{Type: types.ElementTypeText, Text: "toolong"},
		},
	}
	node := layout.Layout(el, 3, 3)
	buf := Draw(node, 5, 3)
	if buf.At(4, 1).Rune != ' ' {
		t.Errorf("At(4,1) = %q, want blank (outside the 3-wide box)", buf.At(4, 1).Rune)
	}
}
