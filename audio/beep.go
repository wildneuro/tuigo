package audio

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"time"
)

// GenerateChirpWAV synthesizes a short two-tone sweep from startFreq to
// endFreq as a 16-bit mono PCM WAV, entirely in Go — no external tools, so
// it always works even where Record's ffmpeg dependency doesn't. Useful
// for UI feedback sounds (sent/received chirps) without shipping audio
// assets.
func GenerateChirpWAV(startFreq, endFreq float64, dur time.Duration) []byte {
	const sampleRate = 44100
	numSamples := int(float64(sampleRate) * dur.Seconds())

	var buf bytes.Buffer
	buf.WriteString("RIFF")
	binary.Write(&buf, binary.LittleEndian, uint32(36+numSamples*2))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	binary.Write(&buf, binary.LittleEndian, uint32(16))
	binary.Write(&buf, binary.LittleEndian, uint16(1)) // PCM
	binary.Write(&buf, binary.LittleEndian, uint16(1)) // mono
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate))
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate*2))
	binary.Write(&buf, binary.LittleEndian, uint16(2))
	binary.Write(&buf, binary.LittleEndian, uint16(16))
	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, uint32(numSamples*2))

	phase1, phase2 := 0.0, 0.0
	fade := sampleRate / 40
	for i := range numSamples {
		progress := float64(i) / float64(numSamples)
		freq := startFreq + (endFreq-startFreq)*progress
		phase1 += 2 * math.Pi * freq / sampleRate
		phase2 += 2 * math.Pi * freq * 1.5 / sampleRate

		env := 1.0
		if i < fade {
			env = float64(i) / float64(fade)
		} else if i > numSamples-fade {
			env = float64(numSamples-i) / float64(fade)
		}
		wave := math.Sin(phase1)*0.7 + math.Sin(phase2)*0.3
		binary.Write(&buf, binary.LittleEndian, int16(wave*32767*env*0.25))
	}
	return buf.Bytes()
}

// Beep synthesizes a chirp with GenerateChirpWAV and plays it via Play,
// through a temp file (native players take a path, not a byte stream).
func Beep(startFreq, endFreq float64, dur time.Duration) error {
	tmp := filepath.Join(os.TempDir(), "tuigo-audio-beep.wav")
	if err := os.WriteFile(tmp, GenerateChirpWAV(startFreq, endFreq, dur), 0o644); err != nil {
		return err
	}
	defer os.Remove(tmp)
	return Play(tmp)
}
