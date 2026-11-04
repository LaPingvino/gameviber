package sfx

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

func rms(s []float32) float64 {
	t := 0.0
	for _, v := range s {
		t += float64(v) * float64(v)
	}
	return math.Sqrt(t / float64(max(len(s), 1)))
}

// crossings counts sign changes in s (a rough pitch).
func crossings(s []float32) int {
	n := 0
	for i := 1; i < len(s); i++ {
		if (s[i-1] < 0) != (s[i] < 0) {
			n++
		}
	}
	return n
}

func TestPresets(t *testing.T) {
	for _, kind := range Presets {
		for seed := uint64(1); seed <= 20; seed++ {
			s := Generate(Preset(kind, seed))
			if len(s) < 441 || len(s) > 44100*10 {
				t.Errorf("%s %d: %d samples", kind, seed, len(s))
			}
			if r := rms(s); r < 0.01 {
				t.Errorf("%s %d: silent (rms %.4f)", kind, seed, r)
			}
		}
		a, b := Generate(Preset(kind, 7)), Generate(Preset(kind, 7))
		if len(a) != len(b) || rms(a) != rms(b) {
			t.Errorf("%s: the same seed gave another sound", kind)
		}
		if c := Generate(Preset(kind, 8)); len(c) == len(a) && rms(c) == rms(a) {
			t.Errorf("%s: another seed gave the same sound", kind)
		}
	}
}

// TestPitchDirection: a laser falls, a power-up rises (zero crossings early vs late).
func TestPitchDirection(t *testing.T) {
	falls, rises := 0, 0
	for seed := uint64(1); seed <= 20; seed++ {
		if s := Generate(Preset("laser", seed)); crossings(s[:len(s)/4]) > crossings(s[len(s)*3/4:]) {
			falls++
		}
		if s := Generate(Preset("powerup", seed)); crossings(s[:len(s)/4]) < crossings(s[len(s)*3/4:]) {
			rises++
		}
	}
	if falls < 16 || rises < 14 {
		t.Errorf("of 20: lasers falling %d, power-ups rising %d", falls, rises)
	}
}

func TestWAV(t *testing.T) {
	s := Generate(Preset("coin", 1))
	w := WAV(s)
	if string(w[:4]) != "RIFF" || string(w[8:16]) != "WAVEfmt " || string(w[36:40]) != "data" {
		t.Fatal("not a WAV header")
	}
	if n := binary.LittleEndian.Uint32(w[40:]); int(n) != len(s)*2 || len(w) != 44+len(s)*2 {
		t.Errorf("data size %d for %d samples", n, len(s))
	}
	if !bytes.Equal(w[22:24], []byte{1, 0}) || binary.LittleEndian.Uint32(w[24:]) != 44100 {
		t.Error("not mono 44100 Hz")
	}
}
