package tuigo

import "testing"

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
	if p.Style.Border != BorderSingle {
		t.Errorf("Panel border = %v, want BorderSingle default", p.Style.Border)
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
