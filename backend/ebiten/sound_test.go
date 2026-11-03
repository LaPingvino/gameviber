package ebiten

import (
	"bytes"
	"io"

	"github.com/LaPingvino/gopus"
	"os"
	"testing"

	"github.com/LaPingvino/gameviber/import/gmfile"
)

// The sample games' WAVs decode (to 44.1 kHz stereo PCM); what doesn't is
// listed (their MIDI music).
func TestDecodeSounds(t *testing.T) {
	for _, exe := range []string{"../../../stelo/win/stelo.exe", "../../../pacmanio/win/pacmanio.exe"} {
		d, err := os.ReadFile(exe)
		if err != nil {
			t.Skip("the sample games are not checked out next to this repo")
		}
		e, _ := gmfile.OpenExe(d)
		p, err := e.ReadProject()
		if err != nil {
			t.Fatal(err)
		}
		pcm, skipped := DecodeSounds(p)
		if len(pcm) == 0 {
			t.Errorf("%s: no sound decoded; skipped %v", exe, skipped)
		}
		t.Logf("%s: %d decoded, skipped %v", exe, len(pcm), skipped)
	}
}

// Music imported as Ogg Opus (gameviber import renders the MIDI) decodes
// through gopus at the context's rate.
func TestOpusMusic(t *testing.T) {
	d, err := os.ReadFile("../../../stelo/win/stelo.exe")
	if err != nil {
		t.Skip("the sample games are not checked out next to this repo")
	}
	e, _ := gmfile.OpenExe(d)
	p, err := e.ReadProject()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range p.Sounds {
		if s == nil || s.Name != "finavenko" {
			continue
		}
		opus, err := gmfile.MIDIToOpus(s.Data)
		if err != nil {
			t.Skip(err)
		}
		st, err := gopus.NewStream(bytes.NewReader(opus))
		if err != nil {
			t.Fatal(err)
		}
		pcm, err := io.ReadAll(st)
		if err != nil {
			t.Fatal(err)
		}
		if st.SampleRate() != sampleRate || len(pcm) < sampleRate*4 { // over a second of 16-bit stereo
			t.Errorf("rate %d, %d bytes", st.SampleRate(), len(pcm))
		}
		return
	}
	t.Fatal("no finavenko")
}
