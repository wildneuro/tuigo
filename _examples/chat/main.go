// Command chat is a mock two-sided chat UI demonstrating TUIML's state
// management (UseState), event handling (OnAnyKey/OnSpecialKey/OnScroll),
// and widget layer (Panel/Badge/Clock/Spinner/ProgressBar/Menu/Dialog) on
// top of the style/flex system. Run it with example-chat.sh from the repo
// root.
//
// Input state lives at the App root rather than in a separate focusable
// sub-component: the "/" command menu changes how much vertical space the
// message list gets, and that's a sibling-sizing decision only the parent
// can make, so the parent needs to see the live input value. The input Box
// still carries WithKey/Focusable so it has a stable identity for the
// focus system once more of the tree is built from real components.
package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	tuigo "github.com/wildneuro/tuigo"
	"github.com/wildneuro/tuigo/asciiart"
	"github.com/wildneuro/tuigo/audio"
)

type message struct {
	sender string
	text   string
	at     string
	mine   bool
}

var cannedReplies = []string{
	"Got it — give me a sec.",
	"Sounds good!",
	"Interesting, tell me more.",
	"I'm just a mock bot, but I'm listening.",
	"Noted. Anything else?",
}

var commands = []tuigo.MenuItem{
	{Label: "/help", Hint: "show keybindings"},
	{Label: "/clear", Hint: "clear the conversation"},
	{Label: "/music", Hint: "open the music + gallery PiPs"},
	{Label: "/quit", Hint: "exit tuigo chat"},
}

const maxHistoryForBar = 20 // purely decorative: what the footer ProgressBar treats as "full"

var logWords = []string{
	"connect", "heartbeat", "sync", "ack", "flush", "gc",
	"poll", "retry", "cache", "auth", "route", "probe",
}

func randomLogLine(seq int) string {
	return fmt.Sprintf("[%04d] %s.%s ok", seq, logWords[rand.Intn(len(logWords))], logWords[rand.Intn(len(logWords))])
}

// clampFrac keeps a random-walked fraction (e.g. a fake CPU/mem meter) in
// [0.05, 0.95] so ProgressBar never shows a suspiciously flat 0% or 100%.
func clampFrac(f float64) float64 {
	return max(min(f, 0.95), 0.05)
}

func main() {
	defer func() { activeStop() }() // stop any still-playing track after the terminal is restored
	if err := tuigo.Render(App); err != nil {
		fmt.Println("tuigo: " + err.Error())
	}
}

func matchingCommands(input string) []tuigo.MenuItem {
	if !strings.HasPrefix(input, "/") {
		return nil
	}
	var out []tuigo.MenuItem
	for _, c := range commands {
		if strings.HasPrefix(c.Label, input) {
			out = append(out, c)
		}
	}
	return out
}

func App(ctx *tuigo.Ctx) tuigo.Element {
	messages, setMessages := tuigo.UseState(ctx, []message{
		{sender: "Bot", text: "Hey! This is a tuigo TUIML chat mock. Try / for commands.", at: "09:00"},
	})
	input, setInput := tuigo.UseState(ctx, "")
	menuIndex, setMenuIndex := tuigo.UseState(ctx, 0)
	scrollOffset, setScrollOffset := tuigo.UseState(ctx, 0)
	showHelp, setShowHelp := tuigo.UseState(ctx, false)
	frame, setFrame := tuigo.UseState(ctx, 0)
	focusInit, setFocusInit := tuigo.UseState(ctx, false)

	// The message list is declared before the input in the tree (visually
	// above it), so without this it would win the "first focusable in tree
	// order" default and typing wouldn't work until the user pressed Tab
	// once. Runs exactly once — see the PiP state comment below for why a
	// "run once" guard is safe under the double component() call per frame.
	if !focusInit {
		setFocusInit(true)
		ctx.Focus("input")
	}

	// PiP state. These advance on wall-clock elapsed time rather than via a
	// background goroutine or Ctx.After closure chain: Render already
	// re-renders once a second (for the header Clock) and on every
	// key/mouse/resize event, and reading "has enough time passed since the
	// last tick" fresh from state on each of those renders sidesteps the
	// classic stale-closure trap a self-rescheduling callback would hit
	// here (a closure capturing UseState's returned slice/value snapshots
	// the value at schedule time, not at fire time).
	telemetry, setTelemetry := tuigo.UseState(ctx, []string{})
	telemetrySeq, setTelemetrySeq := tuigo.UseState(ctx, 0)
	lastTelemetryAt, setLastTelemetryAt := tuigo.UseState(ctx, time.Time{})
	cpu, setCPU := tuigo.UseState(ctx, 0.35)
	mem, setMem := tuigo.UseState(ctx, 0.55)
	lastStatsAt, setLastStatsAt := tuigo.UseState(ctx, time.Time{})
	toast, setToast := tuigo.UseState(ctx, "")
	toastUntil, setToastUntil := tuigo.UseState(ctx, time.Time{})

	// Media PiP state: a music player (track switching via audio.PlayAsync)
	// and an image gallery (asciiart truecolor blocks, auto-advancing on
	// the same elapsed-time pattern as the telemetry/stats PiPs above).
	mediaOpen, setMediaOpen := tuigo.UseState(ctx, false)
	trackIndex, setTrackIndex := tuigo.UseState(ctx, 0)
	playing, setPlaying := tuigo.UseState(ctx, false)
	stopAudio, setStopAudio := tuigo.UseState(ctx, func() {})
	imgIndex, setImgIndex := tuigo.UseState(ctx, 0)
	lastImgAt, setLastImgAt := tuigo.UseState(ctx, time.Time{})

	now := time.Now()
	if now.Sub(lastTelemetryAt) > 700*time.Millisecond {
		setLastTelemetryAt(now)
		setTelemetrySeq(telemetrySeq + 1)
		lines := append(append([]string{}, telemetry...), randomLogLine(telemetrySeq))
		if len(lines) > 5 {
			lines = lines[len(lines)-5:]
		}
		setTelemetry(lines)
	}
	if now.Sub(lastStatsAt) > 900*time.Millisecond {
		setLastStatsAt(now)
		setCPU(clampFrac(cpu + (rand.Float64()-0.5)*0.2))
		setMem(clampFrac(mem + (rand.Float64()-0.5)*0.1))
	}
	if toast != "" && now.After(toastUntil) {
		setToast("")
	}
	if mediaOpen && now.Sub(lastImgAt) > time.Second {
		setLastImgAt(now)
		setImgIndex((imgIndex + 1) % len(galleryImages))
	}

	stopPlayback := func() {
		stopAudio()
		activeStop = func() {}
		setStopAudio(func() {})
		setPlaying(false)
	}
	playTrack := func(idx int) {
		stopAudio()
		path, err := trackPath(tracks[idx].file)
		if err != nil {
			setToast("audio: " + err.Error())
			setToastUntil(now.Add(3 * time.Second))
			return
		}
		stop, err := audio.PlayAsync(path)
		if err != nil {
			setToast("audio: " + err.Error())
			setToastUntil(now.Add(3 * time.Second))
			return
		}
		activeStop = stop // let main() clean up on exit even if the user never closes media mode
		setStopAudio(stop)
		setPlaying(true)
	}
	prevTrack := func() {
		idx := (trackIndex - 1 + len(tracks)) % len(tracks)
		setTrackIndex(idx)
		if playing {
			playTrack(idx)
		}
	}
	nextTrack := func() {
		idx := (trackIndex + 1) % len(tracks)
		setTrackIndex(idx)
		if playing {
			playTrack(idx)
		}
	}
	togglePlay := func() {
		if playing {
			stopPlayback()
		} else {
			playTrack(trackIndex)
		}
	}
	closeMedia := func() {
		stopPlayback()
		setMediaOpen(false)
	}

	width, height := ctx.Size()
	const headerH, inputH, footerH = 3, 3, 1

	// PiP overlays, composited on top of whatever tree this render returns
	// (including the help Dialog below) — see overlay.go: each gets its
	// own independent layout+draw pass, then is stamped onto the main
	// frame buffer before the cell diff. Media mode swaps the ambient
	// telemetry/stats PiPs for the music+gallery ones rather than stacking
	// all four — screen space is tight and they'd overlap.
	if mediaOpen {
		var grid [][]asciiart.Pixel
		if data, err := assets.ReadFile("assets/" + galleryImages[imgIndex]); err == nil {
			if img, err := asciiart.Decode(data); err == nil {
				grid, _ = asciiart.GridFit(img, 28, 11)
			}
		}
		ctx.Overlay(musicPanel(tracks[trackIndex], playing, trackIndex, len(tracks), prevTrack, togglePlay, nextTrack), 2, headerH+1)
		ctx.Overlay(galleryPanel(grid), 2, headerH+8)
	} else {
		ctx.Overlay(telemetryPanel(telemetry), width-30, height-10)
		ctx.Overlay(statsPanel(cpu, mem), 2, height-9)
	}
	if toast != "" {
		ctx.Overlay(toastPanel(toast, func() { setToast("") }), width-26, headerH+1)
	}

	matches := matchingCommands(input)
	menuOpen := len(matches) > 0
	if menuIndex >= len(matches) {
		menuIndex = max(len(matches)-1, 0)
	}

	send := func() {
		if input == "" {
			return
		}
		at := now.Format("15:04")
		next := append(append([]message{}, messages...), message{sender: "You", text: input, at: at, mine: true})
		reply := cannedReplies[len(next)%len(cannedReplies)]
		next = append(next, message{sender: "Bot", text: reply, at: at})
		setMessages(next)
		setInput("")
		setScrollOffset(0)
		setFrame(frame + 1)
		setToast("Message sent")
		setToastUntil(now.Add(2 * time.Second))
	}

	runCommand := func(cmd string) {
		switch cmd {
		case "/help":
			setShowHelp(true)
		case "/clear":
			setMessages(nil)
		case "/music":
			setMediaOpen(true)
		case "/quit":
			ctx.Exit()
		}
		setInput("")
	}

	if showHelp {
		return helpDialog(ctx, width, height, setShowHelp)
	}

	menuH := 0
	if menuOpen {
		menuH = len(matches) + 2 // +2 for the Menu's own border
	}
	messagesH := max(height-headerH-inputH-footerH-menuH, 1)

	maxScroll := max(len(messages)-messagesH, 0)
	if scrollOffset > maxScroll {
		scrollOffset = maxScroll
	}
	end := len(messages) - scrollOffset
	start := max(end-messagesH, 0)
	visible := messages[start:end]

	var rows []tuigo.Element
	for _, m := range visible {
		rows = append(rows, messageRow(m, width))
	}

	scrollUp := func() { setScrollOffset(min(scrollOffset+1, maxScroll)) }
	scrollDown := func() { setScrollOffset(max(scrollOffset-1, 0)) }

	var menuChild tuigo.Element
	if menuOpen {
		menuChild = tuigo.Menu(matches, menuIndex, func(i int) { runCommand(matches[i].Label) })
	} else {
		menuChild = tuigo.Fragment()
	}

	// Keyboard is focus-scoped, not one global catch-all: Ctrl+C is the only
	// app-wide shortcut (Global()). Everything else belongs to whichever
	// element Tab has focused — typing/menu-nav on the input, arrow-key
	// scrolling on the message list — exactly the VCL-style "events go to
	// the focused component" model (see tuigo-design-philosophy-vcl-2026).
	messagesPane := tuigo.Box(
		tuigo.WithKey("messages"), tuigo.Focusable(),
		tuigo.FlexColumn(), tuigo.Height(messagesH),
		tuigo.OnScroll(func(delta int) {
			setScrollOffset(max(min(scrollOffset+delta, maxScroll), 0))
		}),
		tuigo.OnSpecialKey(tuigo.KeyUp, scrollUp),
		tuigo.OnSpecialKey(tuigo.KeyDown, scrollDown),
		tuigo.Children(rows...),
	)

	inputHandler := tuigo.OnAnyKey(func(k tuigo.Key) {
		if menuOpen {
			switch k.Special {
			case tuigo.KeyUp:
				setMenuIndex((menuIndex - 1 + len(matches)) % len(matches))
			case tuigo.KeyDown:
				setMenuIndex((menuIndex + 1) % len(matches))
			case tuigo.KeyEnter:
				runCommand(matches[menuIndex].Label)
			case tuigo.KeyEsc:
				setInput("")
			case tuigo.KeyBackspace:
				if len(input) > 0 {
					setInput(input[:len(input)-1])
				}
			case tuigo.KeyNone:
				if k.Rune != 0 {
					setInput(input + string(k.Rune))
				}
			}
			return
		}
		switch k.Special {
		case tuigo.KeyEnter:
			send()
		case tuigo.KeyBackspace:
			if len(input) > 0 {
				setInput(input[:len(input)-1])
			}
		case tuigo.KeyNone:
			if k.Rune == '?' && input == "" {
				setShowHelp(true)
				return
			}
			if k.Rune != 0 {
				setInput(input + string(k.Rune))
			}
		}
	})

	return tuigo.Box(
		tuigo.FlexColumn(),
		tuigo.Width(width), tuigo.Height(height),
		tuigo.ColorBg(tuigo.ColorBlack),
		tuigo.Global()(tuigo.OnSpecialKey(tuigo.KeyCtrlC, ctx.Exit)),
		// Only claims keys that are otherwise unused by typing/menu-nav
		// (Left/Right/Esc) — Space or a letter like 'm' would silently
		// swallow that character out of chat messages while media is open,
		// since this fires alongside the input's own focused handler, not
		// instead of it. Play/pause has no keyboard binding for that
		// reason; use the clickable ⏯ button in the music PiP.
		tuigo.Global()(tuigo.OnAnyKey(func(k tuigo.Key) {
			if !mediaOpen {
				return
			}
			switch k.Special {
			case tuigo.KeyLeft:
				prevTrack()
			case tuigo.KeyRight:
				nextTrack()
			case tuigo.KeyEsc:
				closeMedia()
			}
		})),
		tuigo.Children(
			header(headerH, frame),
			messagesPane,
			menuChild,
			inputBar(inputH, input, inputHandler),
			footer(float64(len(messages))/maxHistoryForBar),
		),
	)
}

func header(h, frame int) tuigo.Element {
	return tuigo.Box(
		tuigo.Height(h), tuigo.Border(tuigo.BorderSingle), tuigo.FlexRow(),
		tuigo.Children(
			tuigo.With(tuigo.Text(" tuigo chat "), tuigo.Bold(), tuigo.ColorFg(tuigo.ColorBrightWhite)),
			tuigo.Badge("BETA", tuigo.ColorMagenta),
			tuigo.Box(), // flex spacer: pushes everything after it to the right edge
			tuigo.Spinner(frame),
			tuigo.With(tuigo.Text(" online "), tuigo.ColorFg(tuigo.ColorGreen)),
			tuigo.Clock(" 15:04:05 "),
		),
	)
}

func footer(capacityFrac float64) tuigo.Element {
	return tuigo.Box(
		tuigo.FlexRow(), tuigo.Height(1),
		tuigo.Children(
			tuigo.With(tuigo.Text(" ⏎ send   ⌫ delete   / commands   ? help   ⌃C quit"), tuigo.ColorFg(tuigo.ColorGray)),
			tuigo.Box(),
			tuigo.Box(tuigo.Width(18), tuigo.Children(tuigo.ProgressBar(capacityFrac, 14))),
		),
	)
}

// telemetryPanel is a Picture-in-Picture window: a small independently
// laid-out log tail, composited over the bottom-right corner of the main
// chat via Ctx.Overlay regardless of what the main tree looks like.
func telemetryPanel(lines []string) tuigo.Element {
	var rows []tuigo.Element
	for _, l := range lines {
		rows = append(rows, tuigo.With(tuigo.Text("%s", l), tuigo.Truncate(), tuigo.ColorFg(tuigo.ColorGreen)))
	}
	return tuigo.Panel(
		"Telemetry",
		tuigo.Width(30), tuigo.Height(8),
		tuigo.Border(tuigo.BorderDouble), tuigo.ColorBg(tuigo.ColorBlack), tuigo.ColorFg(tuigo.ColorGray),
		tuigo.Children(rows...),
	)
}

// statsPanel is a second, simultaneous PiP window — proof this isn't a
// one-off special case but a general overlay mechanism.
func statsPanel(cpu, mem float64) tuigo.Element {
	return tuigo.Panel(
		"Stats",
		tuigo.Width(26), tuigo.Height(6),
		tuigo.Border(tuigo.BorderSingle), tuigo.ColorBg(tuigo.ColorBlack),
		tuigo.Children(
			tuigo.Box(tuigo.FlexRow(), tuigo.Height(1), tuigo.Children(
				tuigo.With(tuigo.Text("cpu "), tuigo.ColorFg(tuigo.ColorGray)),
				tuigo.ProgressBar(cpu, 14),
			)),
			tuigo.Box(tuigo.FlexRow(), tuigo.Height(1), tuigo.Children(
				tuigo.With(tuigo.Text("mem "), tuigo.ColorFg(tuigo.ColorGray)),
				tuigo.ProgressBar(mem, 14),
			)),
		),
	)
}

// toastPanel is a third, transient PiP — a dialog-styled notification
// popup that appears over the header for a couple of seconds after
// sending a message, then Ctx.Overlay simply stops being called for it.
func toastPanel(text string, dismiss func()) tuigo.Element {
	return tuigo.Box(
		tuigo.Width(24), tuigo.Height(3),
		tuigo.Border(tuigo.BorderSingle), tuigo.ColorBg(tuigo.ColorBlue),
		tuigo.OnClick(func(tuigo.MouseEvent) { dismiss() }),
		tuigo.Children(tuigo.With(tuigo.Text(" %s", text), tuigo.ColorFg(tuigo.ColorBrightWhite))),
	)
}

func inputBar(h int, input string, handler tuigo.Option) tuigo.Element {
	return tuigo.Box(
		tuigo.WithKey("input"), tuigo.Focusable(), handler,
		tuigo.Height(h), tuigo.Border(tuigo.BorderSingle), tuigo.FlexRow(),
		tuigo.Children(
			tuigo.With(tuigo.Text("› "), tuigo.ColorFg(tuigo.ColorCyan), tuigo.Bold()),
			tuigo.Text("%s", input),
			tuigo.With(tuigo.Text(" "), tuigo.ColorBg(tuigo.ColorBrightWhite)), // block cursor
		),
	)
}

func helpDialog(ctx *tuigo.Ctx, width, height int, setShowHelp func(bool)) tuigo.Element {
	lines := []tuigo.Element{
		tuigo.Text("⏎  send message"),
		tuigo.Text("⌫  delete character"),
		tuigo.Text("/   open the command menu"),
		tuigo.Text("↑ ↓ navigate the command menu / scroll history"),
		tuigo.Text("⇥   Tab: switch focus between input and messages"),
		tuigo.Text("/music opens the music + gallery PiPs"),
		tuigo.Text("?   this help (when the input is empty)"),
		tuigo.Text("⌃C  quit"),
		tuigo.Text(""),
		tuigo.With(tuigo.Text("press any key to close"), tuigo.ColorFg(tuigo.ColorGray)),
	}
	dialogH := len(lines) + 2
	return tuigo.Box(
		tuigo.Width(width), tuigo.Height(height),
		tuigo.Global()(tuigo.OnAnyKey(func(tuigo.Key) { setShowHelp(false) })),
		tuigo.Children(tuigo.Dialog(width, height, min(width-4, 36), dialogH, "Help", tuigo.Children(lines...))),
	)
}

// messageRow renders one chat bubble: outgoing ("mine") bubbles are blue and
// pushed to the right edge with a flex spacer, incoming bubbles are gray and
// left-aligned, mirroring the classic iMessage/WhatsApp layout.
func messageRow(m message, width int) tuigo.Element {
	maxBubbleWidth := max(width*2/3, 8)
	bubbleWidth := min(maxBubbleWidth, len(m.text)+2)
	bubbleColor := tuigo.ColorGray
	if m.mine {
		bubbleColor = tuigo.ColorBlue
	}
	bubble := tuigo.Box(
		tuigo.Width(bubbleWidth),
		tuigo.PaddingLeft(1), tuigo.PaddingRight(1),
		tuigo.ColorBg(bubbleColor),
		tuigo.Children(tuigo.With(tuigo.Text("%s", m.text), tuigo.ColorFg(tuigo.ColorBrightWhite))),
	)
	timestamp := tuigo.With(tuigo.Text(" %s ", m.at), tuigo.ColorFg(tuigo.ColorGray))

	if m.mine {
		return tuigo.Box(tuigo.FlexRow(), tuigo.Height(1), tuigo.Children(tuigo.Box(), timestamp, bubble))
	}
	return tuigo.Box(tuigo.FlexRow(), tuigo.Height(1), tuigo.Children(bubble, timestamp))
}
