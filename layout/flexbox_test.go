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

// bordered returns a bordered FlexColumn box with rows text lines inside —
// the shape of tuigo's Menu widget, whose min-content along a column is
// border(2) + rows.
func bordered(rows int) types.Element {
	children := make([]types.Element, rows)
	for i := range children {
		children[i] = text("x")
	}
	return box(
		withStyle(types.Style{FlexDir: types.FlexDirectionColumn, Border: types.BorderSingle}),
		withChildren(children...),
	)
}

func TestLayoutFlexChildGetsMinContentFloorNotZero(t *testing.T) {
	// A 5-row Menu-shaped child (border=2 + 5 rows = min 7) inside a column
	// that only has 3 rows total to offer. Before the min-content floor, an
	// equal flex share (3/1 = 3) would collapse it to 3 rows, losing content
	// with no visual trace of why.
	el := box(
		withStyle(types.Style{FlexDir: types.FlexDirectionColumn}),
		withChildren(bordered(5)),
	)
	n := Layout(el, 10, 3)
	if got := n.Children[0].Rect.H; got != 7 {
		t.Errorf("bordered flex child height = %d, want 7 (min-content floor)", got)
	}
}

func TestLayoutFlexMinContentWaterfallShrinksOtherChildFirst(t *testing.T) {
	// Two flex children in a 10-row column: one has a big min-content (7),
	// the other has none. The plain child should absorb the shrink so the
	// min-content child still gets its floor.
	el := box(
		withStyle(types.Style{FlexDir: types.FlexDirectionColumn}),
		withChildren(bordered(5), box()),
	)
	n := Layout(el, 10, 10)
	if got := n.Children[0].Rect.H; got != 7 {
		t.Errorf("min-content child height = %d, want 7", got)
	}
	if got := n.Children[1].Rect.H; got != 3 {
		t.Errorf("plain flex child height = %d, want 3 (10 - 7)", got)
	}
}

func TestLayoutFlexMinContentBothOverflowGetTheirOwnMin(t *testing.T) {
	// Two bordered children whose combined min-content (7+7=14) exceeds the
	// 10 rows available. Each still gets its own min rather than being
	// squeezed below it — the row overflows and the renderer clips, which
	// beats every child silently collapsing to nothing.
	el := box(
		withStyle(types.Style{FlexDir: types.FlexDirectionColumn}),
		withChildren(bordered(5), bordered(5)),
	)
	n := Layout(el, 10, 10)
	if got := n.Children[0].Rect.H; got != 7 {
		t.Errorf("first overflowing child height = %d, want 7", got)
	}
	if got := n.Children[1].Rect.H; got != 7 {
		t.Errorf("second overflowing child height = %d, want 7", got)
	}
}

func TestLayoutFlexExplicitHeightStillWinsOverMinContent(t *testing.T) {
	// A bordered child whose min-content is 7 rows, but with an explicit
	// Height(2) set directly on it: explicit always wins, even below the
	// child's own min-content — that's the author asserting an exact size.
	child := box(
		withStyle(types.Style{FlexDir: types.FlexDirectionColumn, Border: types.BorderSingle, Height: 2}),
		withChildren(text("x"), text("x"), text("x"), text("x"), text("x")),
	)
	el := box(
		withStyle(types.Style{FlexDir: types.FlexDirectionColumn}),
		withChildren(child),
	)
	n := Layout(el, 10, 10)
	if got := n.Children[0].Rect.H; got != 2 {
		t.Errorf("explicit height child = %d, want 2", got)
	}
}

func TestLayoutFlexMinContentUnchangedWhenSpaceIsAmple(t *testing.T) {
	// Regression guard: with ample space, min-content-bearing flex children
	// behave exactly like TestLayoutFlexChildrenShareRemainingSpace's plain
	// case — equal split, no waterfall pinning kicks in.
	el := box(
		withStyle(types.Style{FlexDir: types.FlexDirectionColumn}),
		withChildren(bordered(1), bordered(1)),
	)
	n := Layout(el, 10, 20)
	if got := n.Children[0].Rect.H; got != 10 {
		t.Errorf("flex child 0 height = %d, want 10", got)
	}
	if got := n.Children[1].Rect.H; got != 10 {
		t.Errorf("flex child 1 height = %d, want 10", got)
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
