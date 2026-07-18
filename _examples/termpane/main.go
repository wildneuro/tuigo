// Command termpane is a minimal terminal COMPOSITOR built with tuigo: it
// embeds a live child shell (bash by default, or the program named on the
// command line) inside a tuigo TerminalPane and lays it out beside a status
// column — tmux/zellij-as-a-library in ~40 lines.
//
// The pane is a real embedded terminal: it spawns the child in a PTY, paints
// its live screen every frame, reflows the child when the window resizes, and
// forwards your keystrokes to it while focused. A block cursor marks the
// child's cursor position when the pane holds focus.
//
// Run it from the repo root:
//
//	go run ./_examples/termpane            # embeds bash
//	go run ./_examples/termpane vim        # embeds vim
//
// Keys: type to drive the child; Tab moves focus between the pane and the
// status column; Ctrl-C is forwarded to the child (so it interrupts the child,
// not the compositor). The compositor exits when the child does.
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

	tuigo.Render(func(ctx *tuigo.Ctx) tuigo.Element {
		return tuigo.Box(
			tuigo.FlexRow(),
			tuigo.Children(
				// The embedded terminal: flex:1, filling most of the width.
				tuigo.TerminalPane(ctx, argv,
					tuigo.WithKey("term"),
					// When the child exits, so does the compositor.
					tuigo.OnPaneExit(func(error) { ctx.Exit() }),
				),
				// A fixed-width status column beside it.
				tuigo.With(
					tuigo.Panel("termpane",
						tuigo.Width(26),
						tuigo.Children(
							tuigo.Text("Embedded child:"),
							tuigo.With(tuigo.Text("  %v", argv), tuigo.Bold()),
							tuigo.Text(""),
							tuigo.Text("Tab  switch focus"),
							tuigo.Text("type drive the child"),
							tuigo.Text("C-c  interrupt child"),
							tuigo.Text(""),
							tuigo.With(tuigo.Text("child exit closes app"),
								tuigo.ColorFg(tuigo.ColorGray)),
						),
					),
					tuigo.Width(26),
				),
			),
		)
	})
}
