package layout

import "strings"

// WrapText breaks text into lines no wider than width, breaking on spaces
// where possible and hard-breaking a single word that's wider than width on
// its own.
func WrapText(text string, width int) []string {
	if width <= 0 {
		width = 1
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}

	var rough []string
	cur := words[0]
	for _, w := range words[1:] {
		if len([]rune(cur))+1+len([]rune(w)) <= width {
			cur += " " + w
		} else {
			rough = append(rough, cur)
			cur = w
		}
	}
	rough = append(rough, cur)

	var lines []string
	for _, l := range rough {
		r := []rune(l)
		for len(r) > width {
			lines = append(lines, string(r[:width]))
			r = r[width:]
		}
		lines = append(lines, string(r))
	}
	return lines
}
