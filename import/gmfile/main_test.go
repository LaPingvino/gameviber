package gmfile

import (
	"bytes"
	"os"
	"testing"
)

// Rendering MIDI takes seconds a tune: off, except in TestMIDIToOpus.
func TestMain(m *testing.M) {
	RenderMIDI = false
	os.Exit(m.Run())
}

// Stelo's shortest tune (finavenko, 3 kB of MIDI) renders to Ogg Opus.
func TestMIDIToOpus(t *testing.T) {
	d, err := os.ReadFile("../../../stelo/win/stelo.exe")
	if err != nil {
		t.Skip("the sample games are not checked out next to this repo")
	}
	e, _ := OpenExe(d)
	p, err := e.ReadProject()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range p.Sounds {
		if s == nil || s.Name != "finavenko" {
			continue
		}
		opus, err := midiToOpus(s.Data)
		if err != nil {
			t.Skip(err) // no fluidsynth / ffmpeg / sound font here
		}
		if !bytes.HasPrefix(opus, []byte("OggS")) || !bytes.Contains(opus[:200], []byte("OpusHead")) || len(opus) < 10000 {
			t.Errorf("not Ogg Opus: %d bytes, %q", len(opus), opus[:min(40, len(opus))])
		}
		return
	}
	t.Fatal("no finavenko in Stelo")
}
