package tuigo

import (
	"strings"
	"time"

	"github.com/wildneuro/tuigo/layout"
)

// This file holds composed widgets built entirely from Box/Text/Option —
// no new core primitives, per STYLEGUIDE.md rule 1. They exist to give
// TUIML apps a consistent, polished starting point instead of every app
// hand-rolling borders and colors from scratch.

// Panel is a titled, bordered container in the spirit of a Turbo
// Vision / Borland-era window frame: a full-width colored title bar rather
// than a plain text line, so the title reads as a window's chrome instead
// of just another content row. The bar uses Theme.Accent — a Box, not a
// bare Text, since a standalone Text's ColorBg only paints behind its own
// characters (see renderer/draw.go's drawText), not the full row; a Box's
// fillBackground paints its entire rect. Pass Children(...) and any other
// Box options (Width, Height, Padding, ...) as opts; Border defaults to
// BorderRounded and FlexColumn if not overridden by a later option.
func Panel(title string, opts ...Option) Element {
	base := append([]Option{FlexColumn(), Border(BorderRounded), PaddingLeft(1), PaddingRight(1)}, opts...)
	body := Box(base...)
	titleBar := Box(
		Height(1), ColorBg(Theme.Accent),
		Children(With(Text(" %s", title), Bold(), ColorFg(ColorBrightWhite))),
	)
	body.Children = append([]Element{titleBar}, body.Children...)
	return body
}

// Divider draws a horizontal rule width cells wide, e.g. to separate
// sections inside a Panel without a full nested border.
func Divider(width int) Element {
	if width < 0 {
		width = 0
	}
	return With(Text("%s", strings.Repeat("─", width)), ColorFg(ColorGray))
}

// Badge is a small colored pill label, e.g. for status tags or counts.
func Badge(text string, bg Color) Element {
	return Box(
		PaddingLeft(1), PaddingRight(1), ColorBg(bg),
		Children(With(Text("%s", text), ColorFg(ColorBrightWhite), Bold())),
	)
}

// ProgressBar renders a fraction in [0,1] as a two-tone bar of the given
// width plus a percentage label. Zero-width segments are omitted rather
// than passed as Width(0), since Style's zero value means "unset" (see
// types.Style) and would otherwise be treated as a flex child instead of
// literally zero-width.
func ProgressBar(fraction float64, width int) Element {
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	filled := min(int(float64(width)*fraction+0.5), width)
	var segments []Element
	if filled > 0 {
		segments = append(segments, Box(Width(filled), ColorBg(ColorGreen)))
	}
	if width-filled > 0 {
		segments = append(segments, Box(Width(width-filled), ColorBg(ColorGray)))
	}
	segments = append(segments, With(Text(" %3.0f%%", fraction*100), ColorFg(ColorGray)))
	return Box(FlexRow(), Height(1), Children(segments...))
}

var spinnerFrames = []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")

// Spinner returns one frame of a braille spinner glyph. It's pure — the
// caller drives animation by incrementing frame (e.g. once per received
// key, or once per tick once TODO.md item 3's timers land) and re-rendering.
func Spinner(frame int) Element {
	g := spinnerFrames[((frame%len(spinnerFrames))+len(spinnerFrames))%len(spinnerFrames)]
	return With(Text("%c", g), ColorFg(ColorCyan))
}

// Clock renders the current wall-clock time formatted per the time package
// layout convention (e.g. "15:04:05"). Render's main loop ticks once a
// second specifically so a Clock element updates on its own without any
// user input driving a re-render.
func Clock(layout string) Element {
	return With(Text("%s", time.Now().Format(layout)), ColorFg(ColorGray))
}

// MeasureContent returns the minimum content height needed to render el when
// laid out at the given width — the exact primitive Dialog uses internally
// (layout.MinContentHeight) to size a dialogH<=0 auto-sizing dialog. It's
// exposed standalone so a caller building their own Panel+Border tree for
// Ctx.Overlay (a true floating panel — see Overlay's doc comment) can size
// that tree before constructing the Overlay, without going through Dialog's
// full-viewport takeover path:
//
//	body := Panel("Title", Children(Text("hello")))
//	h := MeasureContent(body, 40)
//	ctx.Overlay(With(body, Width(40), Height(h)), x, y)
func MeasureContent(el Element, width int) int {
	return layout.MinContentHeight(el, width)
}

// Dialog centers a titled Panel of exactly dialogW x dialogH within a
// viewportW x viewportH screen, using flex spacers on every side. It's a
// full-viewport takeover BY DESIGN, not a floating panel: it replaces the
// entire tree it's shown in while open (main content goes fully behind it),
// which is still the right call when a dialog should demand full attention
// (a blocking confirm, a fatal-error screen). For a true floating panel that
// draws on top of other content without blanking the rest of the session —
// a toast, a PiP widget, a popup menu — use Ctx.Overlay instead (see its doc
// comment); pair it with MeasureContent to size a hand-built Panel+Border
// tree, and DimBackdrop if the overlay should dim the backdrop behind it.
//
// dialogH <= 0 = size to content: wrapped Text is measured at the dialog's
// real inner width (dialogW minus Panel's border and left/right padding),
// so callers never hand-count rows — the classic bug this avoids is a body
// line that wraps to more rows than the caller guessed, pushing later rows
// (e.g. a button row) past the bottom of the panel where they get clipped.
// A positive dialogH keeps the exact legacy behavior (fixed size, no
// measurement). The result is clamped to at least 3 rows (enough for the
// border + title alone) and at most viewportH-2.
func Dialog(viewportW, viewportH, dialogW, dialogH int, title string, bodyOpts ...Option) Element {
	if dialogH <= 0 {
		// Mirror Panel's own body construction (Border + FlexColumn +
		// PaddingLeft/Right(1)) WITHOUT the titleBar it prepends, so
		// MinContentHeight measures only the caller's body content plus the
		// panel's own border/padding frame. The titleBar's fixed 1 row is
		// added back separately below — measuring the full Panel (titleBar
		// included) would double-count it, since titleBar is itself an
		// Height(1) child of the same FlexColumn body.
		bodyBox := Box(append([]Option{FlexColumn(), Border(BorderRounded), PaddingLeft(1), PaddingRight(1), Width(dialogW)}, bodyOpts...)...)
		frameH := layout.MinContentHeight(bodyBox, dialogW)
		dialogH = frameH + 1 // + titleBar row
		if vmax := viewportH - 2; dialogH > vmax {
			dialogH = vmax
		}
		if dialogH < 3 {
			dialogH = 3
		}
	}
	box := Panel(title, append([]Option{Width(dialogW), Height(dialogH)}, bodyOpts...)...)
	return Box(
		Width(viewportW), Height(viewportH), FlexColumn(),
		Children(
			Box(),
			Box(FlexRow(), Height(dialogH), Children(Box(), box, Box())),
			Box(),
		),
	)
}

// ButtonState is the visual state of a Button. Immediate-mode widgets have
// no persistent internal state of their own (see STYLEGUIDE.md) — the
// CALLER owns which state to render, typically by tracking a focused/
// pressed key in its own UseState and comparing it against this Button's
// identity each render. Pair ButtonPressed with Ctx.After to end a brief
// "press flash" a frame or two after a click, the same way a native GUI
// button flashes before its click handler's effect becomes visible.
type ButtonState int

const (
	// ButtonNormal is a button's resting look: no focus, no click in flight.
	ButtonNormal ButtonState = iota
	// ButtonFocused marks the keyboard-focus / hover target — the button
	// Enter/Space (or a mouse hover, once tuigo has one) would activate.
	ButtonFocused
	// ButtonPressed is the brief inverted flash shown right after a click,
	// before the caller's onClick side effect (and any resulting re-render)
	// lands. The caller is responsible for reverting to Normal/Focused after
	// a short Ctx.After delay; Button itself never times out on its own.
	ButtonPressed
)

// Button is a one-row clickable label — the primitive tuigo lacked, forcing
// consumers to hand-roll a Box+Text+OnClick every time they needed a
// pressable control (see e.g. tldrq's confirm dialogs). Pass onClick as nil
// for a purely decorative / disabled-looking button; it renders but never
// fires. Styling: ButtonNormal is a plain reverse-video-less row (bright
// white on black) so it reads as clickable without shouting; ButtonFocused
// swaps in Theme.Accent as the background so the keyboard-focus target is
// obvious; ButtonPressed inverts normal's colors (bright-white background,
// black text) for a flash that reads as "this just got clicked" even
// without a mouse-hover concept.
func Button(label string, state ButtonState, onClick func(MouseEvent)) Element {
	fg, bg := ColorBrightWhite, ColorBlack
	bold := false
	switch state {
	case ButtonFocused:
		bg = Theme.Accent
		bold = true
	case ButtonPressed:
		fg, bg = ColorBlack, ColorBrightWhite
		bold = true
	}
	var click Option = func(*Element) {}
	if onClick != nil {
		click = OnClick(onClick)
	}
	text := With(Text(" %s ", label), ColorFg(fg))
	if bold {
		text = With(text, Bold())
	}
	return Box(Height(1), ColorBg(bg), click, Children(text))
}

// MenuItem is one row in a Menu.
type MenuItem struct {
	Label string
	Hint  string // shown dim/right-aligned-ish after Label, e.g. a shortcut or description
}

// Menu renders a selectable list — a dropdown/popup style widget for things
// like a "/" slash-command palette in a chat input. It occupies its own
// row(s) in normal flow layout (tuigo has no floating overlays yet), so the
// idiomatic use is to conditionally insert it as a sibling just above/below
// the input it's triggered from, pushing other content rather than
// floating over it.
// Menu renders items with items[selected] highlighted. onSelect, if
// non-nil, fires with a row's index when it's clicked — pass nil for a
// keyboard-only menu.
func Menu(items []MenuItem, selected int, onSelect func(index int)) Element {
	rows := make([]Element, 0, len(items))
	for i, item := range items {
		bg := ColorBlack
		fg := ColorBrightWhite
		if i == selected {
			bg = ColorBlue
		}
		var click Option = func(*Element) {}
		if onSelect != nil {
			idx := i
			click = OnClick(func(MouseEvent) { onSelect(idx) })
		}
		label := With(Text(" %s", item.Label), ColorFg(fg), ColorBg(bg))
		if item.Hint == "" {
			rows = append(rows, Box(Height(1), ColorBg(bg), click, Children(label)))
			continue
		}
		hint := With(Text("%s ", item.Hint), ColorFg(ColorGray), ColorBg(bg))
		rows = append(rows, Box(FlexRow(), Height(1), ColorBg(bg), click, Children(label, Box(ColorBg(bg)), hint)))
	}
	return Box(FlexColumn(), Border(BorderRounded), Children(rows...))
}
