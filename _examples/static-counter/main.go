// Command static-counter builds a small TUIML element tree and prints its
// structure. It has no interactivity yet — the app loop and useState hook
// are a later build step — but it demonstrates padding/gap/border
// composition, the core of the TUIML style.
package main

import (
	"fmt"

	tuigo "github.com/wildneuro/tuigo"
)

func main() {
	n := 3
	tree := tuigo.Box(
		tuigo.Padding(1), tuigo.FlexColumn(), tuigo.Gap(1), tuigo.Border(tuigo.BorderSingle),
		tuigo.Children(
			tuigo.Text("count: %d", n),
			tuigo.Text("press + to increment, q to quit"),
		),
	)

	printTree(tree, 0)
}

func printTree(e tuigo.Element, depth int) {
	indent := ""
	for i := 0; i < depth; i++ {
		indent += "  "
	}
	if e.Text != "" {
		fmt.Printf("%s%v %q\n", indent, e.Type, e.Text)
	} else {
		fmt.Printf("%s%v\n", indent, e.Type)
	}
	for _, child := range e.Children {
		printTree(child, depth+1)
	}
}
