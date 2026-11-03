package load

import (
	"os"
	"testing"

	"github.com/LaPingvino/gameviber/import/gmfile"
)

// Rendering MIDI takes seconds a tune: not in these tests.
func TestMain(m *testing.M) {
	gmfile.RenderMIDI = false
	os.Exit(m.Run())
}
