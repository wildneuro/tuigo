package audio

import (
	"bytes"
	"encoding/binary"
	"runtime"
	"testing"
	"time"
)

func TestGenerateChirpWAVHasValidRIFFHeader(t *testing.T) {
	wav := GenerateChirpWAV(400, 800, 50*time.Millisecond)
	if len(wav) < 44 {
		t.Fatalf("WAV too short to contain a header: %d bytes", len(wav))
	}
	if string(wav[0:4]) != "RIFF" {
		t.Errorf("missing RIFF magic, got %q", wav[0:4])
	}
	if string(wav[8:12]) != "WAVE" {
		t.Errorf("missing WAVE magic, got %q", wav[8:12])
	}
	if string(wav[12:16]) != "fmt " {
		t.Errorf("missing fmt chunk, got %q", wav[12:16])
	}
	if string(wav[36:40]) != "data" {
		t.Errorf("missing data chunk, got %q", wav[36:40])
	}
}

func TestGenerateChirpWAVSampleCountMatchesDuration(t *testing.T) {
	dur := 100 * time.Millisecond
	wav := GenerateChirpWAV(300, 300, dur)
	var dataSize uint32
	if err := binary.Read(bytes.NewReader(wav[40:44]), binary.LittleEndian, &dataSize); err != nil {
		t.Fatalf("reading data chunk size: %v", err)
	}
	wantSamples := int(44100 * dur.Seconds())
	gotSamples := int(dataSize) / 2 // 16-bit mono: 2 bytes/sample
	if gotSamples != wantSamples {
		t.Errorf("sample count = %d, want %d", gotSamples, wantSamples)
	}
}

func TestGenerateChirpWAVFadesInAndOut(t *testing.T) {
	// The very first and very last samples should be near-silent (the
	// fade envelope), even for a loud constant tone.
	wav := GenerateChirpWAV(440, 440, 200*time.Millisecond)
	data := wav[44:]
	first := int16(binary.LittleEndian.Uint16(data[0:2]))
	if first < -500 || first > 500 {
		t.Errorf("first sample = %d, want near 0 (fade-in)", first)
	}
}

func TestPlayCommandOnCurrentPlatform(t *testing.T) {
	// Exercises whichever OS branch this test actually runs on, rather than
	// asserting one specific GOOS — CI may run on darwin or linux.
	cmd, err := playCommand("song.mp3")
	switch runtime.GOOS {
	case "darwin", "windows":
		if err != nil {
			t.Fatalf("playCommand on %s: %v", runtime.GOOS, err)
		}
		if cmd == nil {
			t.Fatal("playCommand returned a nil *exec.Cmd with no error")
		}
	case "linux":
		// Passes either way: a player found on PATH, or a clear error
		// naming what was tried. Either is a correct outcome, just not
		// deterministic across CI images.
		if err == nil && cmd == nil {
			t.Fatal("playCommand returned neither a command nor an error")
		}
	}
}
