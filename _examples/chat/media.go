package main

import (
	"embed"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"time"

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

// eqBars are the classic Winamp block-graphic levels, quietest to loudest.
var eqBars = []rune("▁▂▃▄▅▆▇█")

const eqBarCount = 12

// equalizerRow renders eqBarCount independently-colored bars — like
// galleryPanel below, each bar needs its own color (bright green normally,
// dimming as it "peaks" is overkill here, so a flat LCD green is used) and
// a Text element carries exactly one style for its whole string (see
// STYLEGUIDE.md), so this is one Element per bar rather than one per row.
// Heights are derived from eqSeed with a per-bar-index seeded RNG rather
// than a running counter or goroutine, so this stays a pure function of
// its inputs (see the elapsed-wall-clock-tick comment in main.go: eqSeed
// itself is what advances over time, driven by the same pattern as the
// telemetry/stats PiPs).
func equalizerRow(playing bool, eqSeed int) tuigo.Element {
	cells := make([]tuigo.Element, eqBarCount)
	for i := range cells {
		level := 0
		if playing {
			r := rand.New(rand.NewSource(int64(eqSeed)*1000 + int64(i)))
			level = r.Intn(len(eqBars))
		}
		cells[i] = tuigo.With(tuigo.Text("%c", eqBars[level]), tuigo.ColorFg(tuigo.ColorBrightGreen), tuigo.ColorBg(tuigo.Theme.Surface))
	}
	return tuigo.Box(tuigo.FlexRow(), tuigo.Height(1), tuigo.Children(cells...))
}

// musicPanel is styled like a classic Winamp mini-player: black background,
// bright-green LCD-style track text, an animated equalizer, and a looping
// fake seek bar built from the existing ProgressBar widget.
func musicPanel(t track, playing bool, idx, total int, onPrev, onToggle, onNext func(), playStartedAt time.Time, eqSeed int) tuigo.Element {
	status := "⏸ paused"
	if playing {
		status = "▶ playing"
	}
	seekFrac := 0.0
	if playing && !playStartedAt.IsZero() {
		const loopSeconds = 30.0
		seekFrac = math.Mod(time.Since(playStartedAt).Seconds(), loopSeconds) / loopSeconds
	}
	return tuigo.Panel(
		"Now Playing",
		tuigo.Width(musicW), tuigo.Height(musicH),
		tuigo.Border(tuigo.BorderRounded), tuigo.ColorBg(tuigo.Theme.Surface),
		tuigo.Children(
			tuigo.With(tuigo.Text(" %d/%d %s", idx+1, total, t.title), tuigo.Truncate(), tuigo.ColorFg(tuigo.ColorBrightGreen), tuigo.Bold()),
			tuigo.With(tuigo.Text(" %s", status), tuigo.ColorFg(tuigo.ColorBrightGreen)),
			tuigo.Box(tuigo.FlexRow(), tuigo.Height(1), tuigo.Children(
				tuigo.With(tuigo.Text(" « prev "), tuigo.ColorFg(tuigo.ColorCyan), tuigo.OnClick(func(tuigo.MouseEvent) { onPrev() })),
				tuigo.With(tuigo.Text(" ⏯ "), tuigo.ColorFg(tuigo.ColorYellow), tuigo.OnClick(func(tuigo.MouseEvent) { onToggle() })),
				tuigo.With(tuigo.Text(" next » "), tuigo.ColorFg(tuigo.ColorCyan), tuigo.OnClick(func(tuigo.MouseEvent) { onNext() })),
			)),
			equalizerRow(playing, eqSeed),
			tuigo.Box(tuigo.FlexRow(), tuigo.Height(1), tuigo.Children(tuigo.ProgressBar(seekFrac, musicW-8))),
			tuigo.With(tuigo.Text(" ←→ switch  ⏯ play  esc close"), tuigo.Truncate(), tuigo.ColorFg(tuigo.ColorGray)),
		),
	)
}

// galleryPanel renders grid as solid-block truecolor "pixels" — each cell
// is its own single-rune Text element with its own ColorFg, since a Text
// element carries exactly one style for its whole string (see
// STYLEGUIDE.md's style-inheritance rule): a color image needs one
// Element per cell, not one Element per row.
// galleryChrome is how many rows galleryPanel reserves beyond the image
// grid itself: Panel's own title row + top/bottom border + this panel's
// prev/next controls row. Callers positioning the panel (main.go) need
// this to avoid overlapping whatever sits below it.
const galleryChrome = 4

func galleryPanel(grid [][]asciiart.Pixel, idx, total int, onPrev, onNext func()) tuigo.Element {
	if len(grid) == 0 || len(grid[0]) == 0 {
		return tuigo.Panel("Gallery", tuigo.Width(20), tuigo.Height(galleryChrome+1), tuigo.Children(tuigo.Text("no image")))
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
	controls := tuigo.Box(tuigo.FlexRow(), tuigo.Height(1), tuigo.Gap(1), tuigo.Children(
		tuigo.With(tuigo.Text(" ‹ prev "), tuigo.ColorFg(tuigo.ColorCyan), tuigo.OnClick(func(tuigo.MouseEvent) { onPrev() })),
		tuigo.With(tuigo.Text("%d/%d", idx+1, total), tuigo.ColorFg(tuigo.ColorGray)),
		tuigo.With(tuigo.Text(" next › "), tuigo.ColorFg(tuigo.ColorCyan), tuigo.OnClick(func(tuigo.MouseEvent) { onNext() })),
	))
	return tuigo.Panel(
		"Gallery",
		tuigo.Width(max(w+2, 20)), tuigo.Height(h+galleryChrome),
		tuigo.Border(tuigo.BorderRounded),
		tuigo.Children(append([]tuigo.Element{controls}, rows...)...),
	)
}
