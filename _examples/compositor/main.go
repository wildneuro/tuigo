// Command compositor is the proof-of-concept for embedding a DEMANDING
// full-screen child (think Claude Code) in a tuigo TerminalPane and overlaying
// a dialog on top of it without corrupting the pane. It exercises the three
// compositor fixes landed in tuigo v0.1.16:
//
//   - FIX 1 — Tab reaches the pane. The embedded shell fills most of the
//     screen and GRABS input: Tab and Shift-Tab go to the child (an agent uses
//     them for its own modes), NOT to focus cycling. Focus is switched with a
//     distinct global chord, Ctrl-O (tuigo.FocusCycleKey), which always works
//     even over a grab-all pane.
//   - FIX 2 — the REAL hardware cursor. When the pane holds focus the terminal's
//     own cursor blinks at the child's cursor cell; focus the status bar (a
//     non-pane element) and the cursor disappears; open the dialog and it's
//     hidden so it can't poke through.
//   - FIX 3 — a dialog OVER the pane with clean restore. Ctrl-O to focus the
//     bar, Enter to open a Menu overlay stamped on top of the live shell;
//     pick an item or press Esc and the pane repaints from its Screen with no
//     artifacts.
//
// Run it from the repo root:
//
//	go run ./_examples/compositor          # embeds bash
//	go run ./_examples/compositor htop      # embeds htop (or any program)
//
// Keys:
//
//	(pane focused)  type / Tab / Shift-Tab / Ctrl-C  -> the child shell
//	Ctrl-O                                            -> switch focus (pane <-> bar)
//	(bar focused)   Enter                             -> open the dialog over the pane
//	(dialog open)   Up/Down move, Enter select, Esc   -> close; pane restored intact
package main

import (
	"os"

	tuigo "github.com/wildneuro/tuigo"
)

func main() {
	argv := os.Args[1:]
	if len(argv) == 0 {
		argv = []string{"bash"}
	}

	items := []tuigo.MenuItem{
		{Label: "Say hello", Hint: "echo"},
		{Label: "Clear screen", Hint: "clear"},
		{Label: "Quit compositor", Hint: "exit"},
	}

	tuigo.Render(func(ctx *tuigo.Ctx) tuigo.Element {
		menuOpen, setMenuOpen := tuigo.UseState(ctx, false)
		selected, setSelected := tuigo.UseState(ctx, 0)
		w, h := ctx.Size()

		barFocused := ctx.IsFocused("bar")

		// choose acts on a menu row, then closes the dialog. The pane repaints
		// itself next frame, so nothing needs to "erase" the overlay.
		choose := func(i int) {
			setMenuOpen(false)
			switch i {
			case 2: // Quit
				ctx.Exit()
			}
		}

		// The status bar is a focusable, NON-grab element (so the real cursor
		// hides when it's focused, and Enter/arrows reach it rather than the
		// shell). Its handlers depend on whether the dialog is open.
		barOpts := []tuigo.Option{
			tuigo.WithKey("bar"), tuigo.Focusable(),
			tuigo.Height(1), tuigo.ColorBg(tuigo.ColorBlue),
		}
		if menuOpen {
			barOpts = append(barOpts,
				tuigo.OnSpecialKey(tuigo.KeyUp, func() {
					if selected > 0 {
						setSelected(selected - 1)
					}
				}),
				tuigo.OnSpecialKey(tuigo.KeyDown, func() {
					if selected < len(items)-1 {
						setSelected(selected + 1)
					}
				}),
				tuigo.OnSpecialKey(tuigo.KeyEnter, func() { choose(selected) }),
				tuigo.OnSpecialKey(tuigo.KeyEsc, func() { setMenuOpen(false) }),
			)
		} else {
			barOpts = append(barOpts, tuigo.OnSpecialKey(tuigo.KeyEnter, func() {
				setSelected(0)
				setMenuOpen(true)
			}))
		}

		var barText string
		switch {
		case menuOpen:
			barText = " dialog open  —  Up/Down move · Enter select · Esc close"
		case barFocused:
			barText = " [bar focused]  Enter: open dialog · Ctrl-O: back to shell"
		default:
			barText = " [shell focused]  type to drive it · Tab reaches it · Ctrl-O: focus bar"
		}
		bar := tuigo.With(
			tuigo.Box(tuigo.Children(
				tuigo.With(tuigo.Text("%s", barText),
					tuigo.ColorFg(tuigo.ColorBrightWhite), tuigo.Bold()),
			)),
			barOpts...,
		)

		// The dialog is a real floating overlay stamped ON TOP of the pane —
		// the compositing path FIX 3 proves. When menuOpen flips false we simply
		// stop calling ctx.Overlay and the pane repaints clean.
		if menuOpen {
			const dw, dh = 30, 5 // menu border adds 2 rows: 3 items + top/bottom
			dx, dy := (w-dw)/2, (h-dh)/2
			if dx < 0 {
				dx = 0
			}
			if dy < 0 {
				dy = 0
			}
			menu := tuigo.With(
				tuigo.Menu(items, selected, func(i int) { choose(i) }),
				tuigo.Width(dw), tuigo.Height(dh),
			)
			ctx.Overlay(menu, dx, dy)
		}

		return tuigo.Box(
			tuigo.FlexColumn(),
			tuigo.Children(
				// The embedded terminal: flex:1, filling the screen above the bar.
				tuigo.TerminalPane(ctx, argv,
					tuigo.WithKey("term"),
					tuigo.OnPaneExit(func(error) { ctx.Exit() }),
				),
				bar,
			),
		)
	})
}
