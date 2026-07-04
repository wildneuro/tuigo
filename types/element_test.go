package types

import "testing"

func TestElementZeroValue(t *testing.T) {
	var e Element
	if e.Type != ElementTypeBox {
		t.Errorf("zero-value ElementType = %v, want ElementTypeBox", e.Type)
	}
	if e.Key != "" || e.Text != "" || e.Children != nil || e.Props != nil {
		t.Errorf("zero-value Element has non-zero field: %+v", e)
	}
	if e.Style != (Style{}) {
		t.Errorf("zero-value Element.Style = %+v, want zero Style", e.Style)
	}
}

func TestStyleZeroValue(t *testing.T) {
	var s Style
	if s.FlexDir != FlexDirectionRow {
		t.Errorf("zero-value FlexDirection = %v, want FlexDirectionRow", s.FlexDir)
	}
	if s.Border != BorderNone {
		t.Errorf("zero-value BorderStyle = %v, want BorderNone", s.Border)
	}
	if s.Bold || s.Italic || s.Underline {
		t.Errorf("zero-value Style has a text attribute set: %+v", s)
	}
}
