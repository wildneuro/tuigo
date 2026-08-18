package tuigo

import (
	"strings"
	"testing"

	"github.com/wildneuro/tuigo/layout"
)

func TestPanelPrependsTitleBar(t *testing.T) {
	p := Panel("Chat", Children(Text("body")))
	if len(p.Children) != 2 {
		t.Fatalf("got %d children, want 2 (title bar + body)", len(p.Children))
	}
	titleBar := p.Children[0]
	if titleBar.Style.BgColor != Theme.Accent {
		t.Errorf("title bar BgColor = %v, want Theme.Accent (%v)", titleBar.Style.BgColor, Theme.Accent)
	}
	if len(titleBar.Children) != 1 || titleBar.Children[0].Text != " Chat" {
		t.Errorf("title bar children = %+v, want a single \" Chat\" text", titleBar.Children)
	}
	if p.Children[1].Text != "body" {
		t.Errorf("body row text = %q, want %q", p.Children[1].Text, "body")
	}
	if p.Style.Border != BorderRounded {
		t.Errorf("Panel border = %v, want BorderRounded default", p.Style.Border)
	}
}

func TestDividerRepeatsRuleToWidth(t *testing.T) {
	d := Divider(5)
	if got := []rune(d.Text); len(got) != 5 {
		t.Errorf("Divider(5).Text = %q, want 5 runes", d.Text)
	}
}

func TestDividerNegativeWidthIsEmpty(t *testing.T) {
	d := Divider(-3)
	if d.Text != "" {
		t.Errorf("Divider(-3).Text = %q, want empty", d.Text)
	}
}

func TestBadgeAppliesBackgroundColor(t *testing.T) {
	b := Badge("NEW", ColorMagenta)
	if b.Style.BgColor != ColorMagenta {
		t.Errorf("Badge BgColor = %v, want ColorMagenta", b.Style.BgColor)
	}
	if len(b.Children) != 1 || b.Children[0].Text != "NEW" {
		t.Errorf("Badge children = %+v, want a single 'NEW' text", b.Children)
	}
}

func TestProgressBarZeroFractionOmitsFilledSegment(t *testing.T) {
	p := ProgressBar(0, 10)
	for _, c := range p.Children {
		if c.Style.BgColor == ColorGreen {
			t.Errorf("ProgressBar(0, 10) has a green (filled) segment: %+v", c)
		}
	}
}

func TestProgressBarFullFractionOmitsEmptySegment(t *testing.T) {
	p := ProgressBar(1, 10)
	for _, c := range p.Children {
		if c.Style.BgColor == ColorGray {
			t.Errorf("ProgressBar(1, 10) has a gray (empty) segment: %+v", c)
		}
	}
}

func TestProgressBarClampsOutOfRangeFraction(t *testing.T) {
	under := ProgressBar(-0.5, 10)
	over := ProgressBar(1.5, 10)
	// Both should behave like their clamped 0/1 equivalents: no panic, and
	// each has exactly one bar segment (the other is omitted) plus a label.
	if len(under.Children) != 2 {
		t.Errorf("ProgressBar(-0.5, 10) has %d children, want 2 (one segment + label)", len(under.Children))
	}
	if len(over.Children) != 2 {
		t.Errorf("ProgressBar(1.5, 10) has %d children, want 2 (one segment + label)", len(over.Children))
	}
}

func TestSpinnerCyclesFrames(t *testing.T) {
	first := Spinner(0)
	wrapped := Spinner(len(spinnerFrames))
	if first.Text != wrapped.Text {
		t.Errorf("Spinner(0) = %q, Spinner(len(frames)) = %q, want equal (wraps around)", first.Text, wrapped.Text)
	}
}

func TestSpinnerHandlesNegativeFrame(t *testing.T) {
	// Must not panic on a negative frame (e.g. a decrementing counter).
	_ = Spinner(-1)
}

func TestDialogCentersWithExplicitDimensions(t *testing.T) {
	d := Dialog(40, 20, 20, 8, "Confirm")
	if len(d.Children) != 3 {
		t.Fatalf("Dialog root has %d children, want 3 (top spacer, row, bottom spacer)", len(d.Children))
	}
	row := d.Children[1]
	if row.Style.Height != 8 {
		t.Errorf("Dialog middle row Height = %d, want 8 (matches dialogH)", row.Style.Height)
	}
	if len(row.Children) != 3 {
		t.Fatalf("Dialog middle row has %d children, want 3 (left spacer, box, right spacer)", len(row.Children))
	}
	box := row.Children[1]
	if box.Style.Width != 20 || box.Style.Height != 8 {
		t.Errorf("Dialog box size = %dx%d, want 20x8", box.Style.Width, box.Style.Height)
	}
}

func TestMenuHighlightsSelectedItem(t *testing.T) {
	m := Menu([]MenuItem{{Label: "help"}, {Label: "clear"}}, 1, nil)
	if len(m.Children) != 2 {
		t.Fatalf("Menu has %d rows, want 2", len(m.Children))
	}
	if m.Children[0].Style.BgColor == ColorBlue {
		t.Errorf("unselected row 0 has the selected highlight color")
	}
	if m.Children[1].Style.BgColor != ColorBlue {
		t.Errorf("selected row 1 BgColor = %v, want ColorBlue highlight", m.Children[1].Style.BgColor)
	}
}

func TestMenuItemWithHintAddsExtraChild(t *testing.T) {
	m := Menu([]MenuItem{{Label: "help", Hint: "show commands"}}, 0, nil)
	row := m.Children[0]
	if len(row.Children) != 3 {
		t.Errorf("row with a hint has %d children, want 3 (label, spacer, hint)", len(row.Children))
	}
}

func TestMenuOnSelectFiresWithClickedIndex(t *testing.T) {
	var got = -1
	m := Menu([]MenuItem{{Label: "a"}, {Label: "b"}}, 0, func(i int) { got = i })
	row1 := m.Children[1]
	if len(row1.MouseHandlers) != 1 {
		t.Fatalf("row 1 has %d mouse handlers, want 1", len(row1.MouseHandlers))
	}
	row1.MouseHandlers[0].Handle(MouseEvent{})
	if got != 1 {
		t.Errorf("onSelect got %d, want 1", got)
	}
}

func TestMenuNilOnSelectAddsNoHandlers(t *testing.T) {
	m := Menu([]MenuItem{{Label: "a"}}, 0, nil)
	if len(m.Children[0].MouseHandlers) != 0 {
		t.Errorf("row has %d mouse handlers, want 0 for nil onSelect", len(m.Children[0].MouseHandlers))
	}
}

func TestClockFormatsCurrentTime(t *testing.T) {
	c := Clock("15:04:05")
	if len(c.Text) != len("15:04:05") {
		t.Errorf("Clock(\"15:04:05\").Text = %q, want 8 characters", c.Text)
	}
}

// TestDialogAutoHeightSizesToWrappedBody reproduces the real promote-confirm
// bug: a dw=50 dialog whose gray body Text wraps to 2 rows at the panel's
// real inner width. The caller used to have to hand-count "1 text row" and
// hardcode dialogH=6, which clipped the buttons row one line past the
// bottom of the content box. dialogH<=0 must instead measure the wrap and
// produce enough room for every row, including the buttons row landing
// inside the panel, not past it.
func TestDialogAutoHeightSizesToWrappedBody(t *testing.T) {
	const dw = 50
	innerW := dw - 2 /*border*/ - 2 /*PaddingLeft+PaddingRight*/
	grayText := "This action cannot be undone and will affect every downstream consumer"
	if got := len(layout.WrapText(grayText, innerW)); got != 2 {
		t.Fatalf("test setup: grayText wraps to %d lines at width %d, want exactly 2 (adjust fixture text)", got, innerW)
	}

	viewportW, viewportH := 80, 24
	dlg := Dialog(viewportW, viewportH, dw, 0, "Confirm", Children(
		With(Text("%s", grayText), ColorFg(ColorGray)),
		Text(""),
		Box(FlexRow(), Height(1), Children(Text("[Yes]"), Text("[No]"))),
	))

	// Locate the Panel box (viewport > middle row > [spacer, panel, spacer]).
	panel := dlg.Children[1].Children[1]
	if got := panel.Style.Height; got != 7 {
		t.Fatalf("auto dialogH = %d, want 7 (2 wrapped body rows + blank + buttons row = 4, +3 border/title chrome)", got)
	}

	// Assert the buttons row's laid-out rect actually falls inside the
	// panel, not past it — the exact failure mode of the original bug.
	root := layout.Layout(dlg, viewportW, viewportH)
	panelNode := root.Children[1].Children[1]
	buttonsNode := panelNode.Children[len(panelNode.Children)-1] // last child = buttons row
	panelBottom := panelNode.Rect.Y + panelNode.Rect.H
	buttonsBottom := buttonsNode.Rect.Y + buttonsNode.Rect.H
	if buttonsNode.Rect.Y < panelNode.Rect.Y || buttonsBottom > panelBottom {
		t.Errorf("buttons row rect %+v falls outside panel rect %+v (clipped)", buttonsNode.Rect, panelNode.Rect)
	}
}

func TestDialogAutoHeightClampsToTinyViewport(t *testing.T) {
	longText := strings.Repeat("wrap ", 40)
	dlg := Dialog(20, 6, 15, 0, "T", Children(With(Text("%s", longText), ColorFg(ColorGray))))
	panel := dlg.Children[1].Children[1]
	if got, want := panel.Style.Height, 4; got != want { // viewportH-2 = 4
		t.Errorf("clamped dialogH = %d, want %d (viewportH-2)", got, want)
	}
}

func TestDialogPositiveHeightUnchanged(t *testing.T) {
	dlg := Dialog(80, 24, 50, 6, "Confirm", Children(Text("body")))
	panel := dlg.Children[1].Children[1]
	if got := panel.Style.Height; got != 6 {
		t.Errorf("explicit dialogH=6 was overridden: got %d, want 6 (legacy behavior pinned)", got)
	}
}

func TestMeasureContentMatchesLayoutMinContentHeight(t *testing.T) {
	el := Panel("Title", Children(With(Text("%s", strings.Repeat("wrap ", 20)), ColorFg(ColorGray))))
	want := layout.MinContentHeight(el, 20)
	if got := MeasureContent(el, 20); got != want {
		t.Errorf("MeasureContent(el, 20) = %d, want %d (layout.MinContentHeight)", got, want)
	}
}

func TestMeasureContentAgreesWithDialogAutoHeight(t *testing.T) {
	// Dialog's dialogH<=0 path builds a bodyBox (Border+FlexColumn+padding,
	// no titleBar) and measures it, then adds 1 for the titleBar row.
	// MeasureContent on the same shape should reproduce that frame height.
	longText := strings.Repeat("wrap ", 40)
	const viewportW, viewportH, dialogW = 80, 24, 15
	dlg := Dialog(viewportW, viewportH, dialogW, 0, "T", Children(With(Text("%s", longText), ColorFg(ColorGray))))
	panel := dlg.Children[1].Children[1]

	bodyBox := Box(FlexColumn(), Border(BorderRounded), PaddingLeft(1), PaddingRight(1), Width(dialogW),
		Children(With(Text("%s", longText), ColorFg(ColorGray))))
	frameH := MeasureContent(bodyBox, dialogW) + 1 // + titleBar row
	if want := viewportH - 2; frameH > want {
		frameH = want
	}
	if frameH < 3 {
		frameH = 3
	}
	if got := panel.Style.Height; got != frameH {
		t.Errorf("Dialog auto-height = %d, MeasureContent-derived = %d, want equal", got, frameH)
	}
}
