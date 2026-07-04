package layout

import (
	"reflect"
	"testing"

	"github.com/wildneuro/tuigo/types"
)

func box(opts ...func(*types.Element)) types.Element {
	e := types.Element{Type: types.ElementTypeBox}
	for _, o := range opts {
		o(&e)
	}
	return e
}

func text(s string) types.Element {
	return types.Element{Type: types.ElementTypeText, Text: s}
}

func withChildren(children ...types.Element) func(*types.Element) {
	return func(e *types.Element) { e.Children = children }
}

func withStyle(s types.Style) func(*types.Element) {
	return func(e *types.Element) { e.Style = s }
}

func TestLayoutRootFillsViewport(t *testing.T) {
	n := Layout(box(), 80, 24)
	want := Rect{0, 0, 80, 24}
	if n.Rect != want {
		t.Errorf("root Rect = %+v, want %+v", n.Rect, want)
	}
}

func TestLayoutColumnStacksChildrenWithGap(t *testing.T) {
	el := box(
		withStyle(types.Style{FlexDir: types.FlexDirectionColumn, Gap: 1}),
		withChildren(
			box(withStyle(types.Style{Height: 2})),
			box(withStyle(types.Style{Height: 3})),
		),
	)
	n := Layout(el, 10, 20)
	if len(n.Children) != 2 {
		t.Fatalf("got %d children, want 2", len(n.Children))
	}
	if got := n.Children[0].Rect; got != (Rect{0, 0, 10, 2}) {
		t.Errorf("child 0 Rect = %+v, want {0 0 10 2}", got)
	}
	if got := n.Children[1].Rect; got != (Rect{0, 3, 10, 3}) {
		t.Errorf("child 1 Rect = %+v, want {0 3 10 3}", got)
	}
}

func TestLayoutFlexChildrenShareRemainingSpace(t *testing.T) {
	el := box(
		withStyle(types.Style{FlexDir: types.FlexDirectionColumn}),
		withChildren(box(), box()),
	)
	n := Layout(el, 10, 10)
	if got := n.Children[0].Rect.H; got != 5 {
		t.Errorf("flex child 0 height = %d, want 5", got)
	}
	if got := n.Children[1].Rect.H; got != 5 {
		t.Errorf("flex child 1 height = %d, want 5", got)
	}
}

func TestLayoutTextIntrinsicHeightWrapsToContentWidth(t *testing.T) {
	el := box(
		withStyle(types.Style{FlexDir: types.FlexDirectionColumn}),
		withChildren(text("one two three four")),
	)
	n := Layout(el, 8, 20) // "one two"/"three" won't fit one line at width 8
	got := n.Children[0].Rect.H
	want := len(WrapText("one two three four", 8))
	if got != want {
		t.Errorf("text height = %d, want %d (wrapped: %v)", got, want, WrapText("one two three four", 8))
	}
}

func TestLayoutRowIntrinsicWidthIsRuneLength(t *testing.T) {
	el := box(
		withStyle(types.Style{FlexDir: types.FlexDirectionRow}),
		withChildren(text("hi")),
	)
	n := Layout(el, 20, 5)
	if got := n.Children[0].Rect.W; got != 2 {
		t.Errorf("row text width = %d, want 2", got)
	}
}

func TestContentRectReservesBorderAndPadding(t *testing.T) {
	rect := Rect{0, 0, 10, 10}
	style := types.Style{Border: types.BorderSingle, Padding: [4]int{1, 2, 1, 2}}
	got := ContentRect(rect, style)
	want := Rect{X: 3, Y: 2, W: 4, H: 6} // -1 border each side, then -padding
	if got != want {
		t.Errorf("ContentRect = %+v, want %+v", got, want)
	}
}

func TestApplyMarginShrinksAndOffsetsRect(t *testing.T) {
	got := applyMargin(Rect{0, 0, 10, 10}, [4]int{1, 2, 3, 4})
	want := Rect{X: 4, Y: 1, W: 4, H: 6}
	if got != want {
		t.Errorf("applyMargin = %+v, want %+v", got, want)
	}
}

func TestExplicitSizeOverridesFlexDistribution(t *testing.T) {
	el := box(
		withStyle(types.Style{FlexDir: types.FlexDirectionRow}),
		withChildren(
			box(withStyle(types.Style{Width: 3})),
			box(),
		),
	)
	n := Layout(el, 10, 5)
	if got := n.Children[0].Rect.W; got != 3 {
		t.Errorf("explicit width child = %d, want 3", got)
	}
	if got := n.Children[1].Rect.W; got != 7 {
		t.Errorf("flex child = %d, want 7", got)
	}
}

func TestRectIntersectNonOverlappingHasZeroArea(t *testing.T) {
	a := Rect{0, 0, 5, 5}
	b := Rect{10, 10, 5, 5}
	got := a.Intersect(b)
	if got.W != 0 || got.H != 0 {
		t.Errorf("Intersect of disjoint rects = %+v, want zero area", got)
	}
}

func TestLayoutDeterministicAndNonMutating(t *testing.T) {
	el := box(
		withStyle(types.Style{FlexDir: types.FlexDirectionColumn}),
		withChildren(text("a"), text("b")),
	)
	before := el
	n1 := Layout(el, 10, 10)
	n2 := Layout(el, 10, 10)
	if !reflect.DeepEqual(n1, n2) {
		t.Errorf("Layout is not deterministic: %+v vs %+v", n1, n2)
	}
	if !reflect.DeepEqual(before, el) {
		t.Errorf("Layout mutated its input Element")
	}
}
