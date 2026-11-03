package ebiten

import (
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
