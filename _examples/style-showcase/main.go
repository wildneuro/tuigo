// Command style-showcase exercises every style option (colors, text
// attributes, border, flex row vs column) as a reference for TUIML's style
// system. It prints the tree structure; once the renderer lands this
// becomes a visual reference too.
package main

import (
	"fmt"

	tuigo "github.com/wildneuro/tuigo"
)

func main() {
	tree := tuigo.Box(
		tuigo.FlexColumn(), tuigo.Gap(1), tuigo.Padding(2),
		tuigo.Children(
			tuigo.Box(
				tuigo.FlexRow(), tuigo.Gap(2), tuigo.Border(tuigo.BorderSingle),
				tuigo.Children(
					tuigo.With(tuigo.Text("bold"), tuigo.Bold()),
					tuigo.With(tuigo.Text("italic"), tuigo.Italic()),
					tuigo.With(tuigo.Text("underline"), tuigo.Underline()),
				),
			),
			tuigo.Box(
				tuigo.Border(tuigo.BorderDouble), tuigo.Padding(1),
				tuigo.Children(
					tuigo.With(tuigo.Text("fg+bg color"), tuigo.ColorFg(2), tuigo.ColorBg(4)),
				),
			),
		),
	)

	describe(tree, 0)
}

func describe(e tuigo.Element, depth int) {
	indent := ""
	for i := 0; i < depth; i++ {
		indent += "  "
	}
	fmt.Printf("%s%v style=%+v", indent, e.Type, e.Style)
	if e.Text != "" {
		fmt.Printf(" text=%q", e.Text)
	}
	fmt.Println()
	for _, child := range e.Children {
		describe(child, depth+1)
	}
}
