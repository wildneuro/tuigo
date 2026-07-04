package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	tuigo "github.com/wildneuro/tuigo"
	"github.com/wildneuro/tuigo/asciiart"
)

//go:embed assets/track1.mp3 assets/track2.mp3 assets/track3.mp3 assets/photo1.png assets/photo2.png
var assets embed.FS

type track struct{ title, file string }

var tracks = []track{
	{"Clear Path Ahead", "track1.mp3"},
	{"Orbital Sunrise", "track2.mp3"},
	{"Orbital Briefing", "track3.mp3"},
}

var galleryImages = []string{"photo1.png", "photo2.png"}

// activeStop lets main clean up a still-playing background audio process
// on exit — tuigo has no component unmount lifecycle yet (see TODO.md),
// and Ctx.Exit alone would leave a native player process running after
// the TUI itself has closed.
var activeStop = func() {}

// trackPath extracts an embedded track to a stable temp path (once — later
// calls reuse the file) since audio.Play/PlayAsync shell out to native
// players that need a real path, not an in-memory byte stream.
func trackPath(file string) (string, error) {
	tmp := filepath.Join(os.TempDir(), "tuigo-chat-"+file)
	if _, err := os.Stat(tmp); err == nil {
		return tmp, nil
	}
	data, err := assets.ReadFile("assets/" + file)
	if err != nil {
		return "", fmt.Errorf("read embedded %s: %w", file, err)
	}
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", fmt.Errorf("write temp file for %s: %w", file, err)
	}
	return tmp, nil
}

func musicPanel(t track, playing bool, idx, total int, onPrev, onToggle, onNext func()) tuigo.Element {
	status := "⏸ paused"
	if playing {
		status = "▶ playing"
	}
	return tuigo.Panel(
		"Now Playing",
		tuigo.Width(32), tuigo.Height(7),
		tuigo.Border(tuigo.BorderDouble), tuigo.ColorBg(tuigo.ColorBlack),
		tuigo.Children(
			tuigo.With(tuigo.Text(" %d/%d %s", idx+1, total, t.title), tuigo.Truncate(), tuigo.ColorFg(tuigo.ColorBrightWhite)),
			tuigo.With(tuigo.Text(" %s", status), tuigo.ColorFg(tuigo.ColorGreen)),
			tuigo.Box(tuigo.FlexRow(), tuigo.Height(1), tuigo.Children(
				tuigo.With(tuigo.Text(" « prev "), tuigo.ColorFg(tuigo.ColorCyan), tuigo.OnClick(func(tuigo.MouseEvent) { onPrev() })),
				tuigo.With(tuigo.Text(" ⏯ "), tuigo.ColorFg(tuigo.ColorYellow), tuigo.OnClick(func(tuigo.MouseEvent) { onToggle() })),
				tuigo.With(tuigo.Text(" next » "), tuigo.ColorFg(tuigo.ColorCyan), tuigo.OnClick(func(tuigo.MouseEvent) { onNext() })),
			)),
			tuigo.With(tuigo.Text(" ←/→ or click switch   click ⏯   esc close"), tuigo.ColorFg(tuigo.ColorGray)),
		),
	)
}

// galleryPanel renders grid as solid-block truecolor "pixels" — each cell
// is its own single-rune Text element with its own ColorFg, since a Text
// element carries exactly one style for its whole string (see
// STYLEGUIDE.md's style-inheritance rule): a color image needs one
// Element per cell, not one Element per row.
func galleryPanel(grid [][]asciiart.Pixel) tuigo.Element {
	if len(grid) == 0 || len(grid[0]) == 0 {
		return tuigo.Panel("Gallery", tuigo.Width(20), tuigo.Height(4), tuigo.Children(tuigo.Text("no image")))
	}
	var rows []tuigo.Element
	for _, row := range grid {
		cells := make([]tuigo.Element, len(row))
		for i, p := range row {
			cells[i] = tuigo.With(tuigo.Text("█"), tuigo.ColorFg(tuigo.RGB(p.R, p.G, p.B)))
		}
		rows = append(rows, tuigo.Box(tuigo.FlexRow(), tuigo.Height(1), tuigo.Children(cells...)))
	}
	w, h := len(grid[0]), len(grid)
	return tuigo.Panel(
		"Gallery",
		tuigo.Width(w+2), tuigo.Height(h+3), // +2 border, +1 more for Panel's own title row
		tuigo.Border(tuigo.BorderDouble),
		tuigo.Children(rows...),
	)
}
