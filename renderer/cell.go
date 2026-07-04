// Package renderer turns a laid-out Element tree into a cell buffer, diffs
// it against the previous frame, and flushes only the changed cells to a
// terminal.
package renderer

import "github.com/wildneuro/tuigo/types"

// Cell is one terminal character position and its resolved style.
type Cell struct {
	Rune                    rune
	Fg, Bg                  types.Color
	Bold, Italic, Underline bool
}
