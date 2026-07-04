// Package audio plays and records audio by shelling out to platform-native
// tools rather than linking a CGo audio backend — no C toolchain, no
// cross-compile headaches, consistent with tuigo's "boring, testable
// layers" philosophy (see the root STYLEGUIDE.md). Playback uses whatever
// native player each OS ships (afplay on macOS, paplay/aplay/ffplay on
// Linux, Media.SoundPlayer via PowerShell on Windows); Record requires
// ffmpeg on PATH, since none of the three OSes ship a scriptable recorder.
package audio

import (
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

// Play plays the audio file at path and blocks until playback finishes.
func Play(path string) error {
	cmd, err := playCommand(path)
	if err != nil {
		return err
	}
	return cmd.Run()
}

// PlayAsync starts playing path in the background. The returned stop
// function kills playback early; it's always safe to call, including after
// playback has already finished on its own.
func PlayAsync(path string) (stop func(), err error) {
	cmd, err := playCommand(path)
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go cmd.Wait() // reap the child so it doesn't become a zombie; ignore the exit error
	return func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}, nil
}

func playCommand(path string) (*exec.Cmd, error) {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("afplay", path), nil
	case "linux":
		for _, name := range []string{"paplay", "aplay", "ffplay"} {
			p, err := exec.LookPath(name)
			if err != nil {
				continue
			}
			if name == "ffplay" {
				return exec.Command(p, "-nodisp", "-autoexit", "-loglevel", "quiet", path), nil
			}
			return exec.Command(p, path), nil
		}
		return nil, fmt.Errorf("audio: no supported player found on PATH (tried paplay, aplay, ffplay)")
	case "windows":
		script := fmt.Sprintf("(New-Object Media.SoundPlayer '%s').PlaySync();", path)
		return exec.Command("powershell", "-c", script), nil
	default:
		return nil, fmt.Errorf("audio: unsupported OS %q", runtime.GOOS)
	}
}

// Record captures duration of audio from the system's default microphone
// into path (the container/codec is inferred by ffmpeg from path's
// extension, e.g. ".wav"). Requires ffmpeg on PATH; the default input
// device name is OS-typical but not guaranteed on every machine — pass a
// nonempty device to override it (empty uses the OS default below).
func Record(path string, duration time.Duration, device string) error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("audio: Record requires ffmpeg on PATH: %w", err)
	}
	secs := fmt.Sprintf("%.2f", duration.Seconds())
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		if device == "" {
			device = ":0" // "video:audio" avfoundation syntax; no video, default audio input
		}
		cmd = exec.Command("ffmpeg", "-y", "-f", "avfoundation", "-i", device, "-t", secs, path)
	case "linux":
		if device == "" {
			device = "default"
		}
		cmd = exec.Command("ffmpeg", "-y", "-f", "pulse", "-i", device, "-t", secs, path)
	case "windows":
		if device == "" {
			device = "audio=Microphone"
		}
		cmd = exec.Command("ffmpeg", "-y", "-f", "dshow", "-i", device, "-t", secs, path)
	default:
		return fmt.Errorf("audio: unsupported OS %q", runtime.GOOS)
	}
	return cmd.Run()
}
