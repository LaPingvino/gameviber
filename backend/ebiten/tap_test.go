package ebiten

import "testing"

// touch = "tap": a finger is the left button (GM's 1) — pressed the step it
// lands, held while down, released the step after it lifts; never the right.
func TestTapIsLeftButton(t *testing.T) {
	in := input{}
	steps := []struct {
		tap                     bool
		pressed, held, released bool
	}{
		{false, false, false, false},
		{true, true, true, false},
		{true, false, true, false},
		{false, false, false, true},
		{false, false, false, false},
	}
	for i, s := range steps {
		in.tapPrev, in.tap = in.tap, s.tap
		if in.MousePressed(1) != s.pressed || in.MouseHeld(1) != s.held || in.MouseReleased(1) != s.released {
			t.Errorf("step %d: pressed %v held %v released %v", i, in.MousePressed(1), in.MouseHeld(1), in.MouseReleased(1))
		}
		if in.MouseHeld(2) || in.MousePressed(2) {
			t.Errorf("step %d: a finger pressed the right button", i)
		}
	}
}
