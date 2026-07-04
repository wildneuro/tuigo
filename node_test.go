package tuigo

import (
	"testing"

	"github.com/wildneuro/tuigo/types"
)

func TestBoxProducesBoxType(t *testing.T) {
	e := Box()
	if e.Type != types.ElementTypeBox {
		t.Errorf("Box().Type = %v, want ElementTypeBox", e.Type)
	}
}

func TestTextFormatsAndSetsType(t *testing.T) {
	e := Text("count: %d", 3)
	if e.Type != types.ElementTypeText {
		t.Errorf("Text().Type = %v, want ElementTypeText", e.Type)
	}
	if e.Text != "count: 3" {
		t.Errorf("Text().Text = %q, want %q", e.Text, "count: 3")
	}
}

func TestFragmentGroupsChildrenWithoutOwnType(t *testing.T) {
	children := []Element{Text("a"), Text("b")}
	e := Fragment(children...)
	if e.Type != types.ElementTypeFragment {
		t.Errorf("Fragment().Type = %v, want ElementTypeFragment", e.Type)
	}
	if len(e.Children) != 2 {
		t.Fatalf("Fragment().Children has %d elements, want 2", len(e.Children))
	}
}

func TestChildrenAttachesToBox(t *testing.T) {
	child := Text("hi")
	e := Box(Children(child))
	if len(e.Children) != 1 || e.Children[0].Text != "hi" {
		t.Errorf("Box(Children(...)).Children = %+v, want [hi]", e.Children)
	}
}

func TestWithKeySetsStableIdentity(t *testing.T) {
	e := Box(WithKey("row-1"))
	if e.Key != "row-1" {
		t.Errorf("Box(WithKey(...)).Key = %q, want %q", e.Key, "row-1")
	}
}

func TestOptionsApplyInOrder(t *testing.T) {
	// A later option touching the same field must win.
	e := Box(Padding(1), Padding(4))
	if e.Style.Padding != [4]int{4, 4, 4, 4} {
		t.Errorf("Style.Padding = %v, want [4 4 4 4]", e.Style.Padding)
	}
}

func TestOptionsDoNotMutateSharedElement(t *testing.T) {
	base := Text("base")
	styled := With(base, Bold())
	if base.Style.Bold {
		t.Errorf("With mutated the original Element: %+v", base)
	}
	if !styled.Style.Bold {
		t.Errorf("With did not apply Bold() to the copy: %+v", styled)
	}
}

func TestWithAppliesMultipleOptions(t *testing.T) {
	e := With(Text("hi"), Bold(), Italic())
	if !e.Style.Bold || !e.Style.Italic {
		t.Errorf("With(Bold(), Italic()) = %+v, want both set", e.Style)
	}
}
