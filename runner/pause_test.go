package runner

import "testing"

// A pause (action_sleep) keeps its length and the logic waits, but what is
// drawn is the moment it began, animating: changes the event made after the
// pause began (the player back at the start, a life less) show only when the
// pause is over.
func TestPauseShowsTheMomentAlive(t *testing.T) {
	o := &Object{Name: "p", Mask: true, BBox: Box{0, 31, 0, 31}, Visible: true}
	r := &Room{W: 640, H: 480, G: &Globals{Lives: 3, Vars: map[string]float64{}}}
	p := r.Create(o, 100, 100)
	p.ImageSpeed = 0.5
	r.SleepMS(1000, 30)              // as die(): sound, the pause …
	p.X, p.Y = p.XStart+50, p.YStart // … then back to the start
	r.G.Lives--
	if r.Sleep != 30 || len(r.Drawn()) != 1 || r.Drawn()[0].X != 100 || r.ShownG().Lives != 3 {
		t.Fatalf("pause start: sleep %d, drawn %+v, lives shown %d", r.Sleep, r.Drawn()[0].X, r.ShownG().Lives)
	}
	r.SleepMS(500, 30) // a second sleep in the pause lengthens it, keeps the moment
	for range 44 {
		r.Step(pauseNoKeys{})
	}
	if r.Sleep != 1 || r.Drawn()[0].ImageIndex != 22 || p.ImageIndex != 0 || r.Drawn()[0].X != 100 {
		t.Fatalf("in the pause: sleep %d, drawn frame %v (live %v) at %v", r.Sleep, r.Drawn()[0].ImageIndex, p.ImageIndex, r.Drawn()[0].X)
	}
	r.Step(pauseNoKeys{})
	if r.Paused != nil || r.Drawn()[0] != p || r.ShownG().Lives != 2 {
		t.Fatal("after the pause: the room is drawn again")
	}
}

type pauseNoKeys struct{}

func (pauseNoKeys) Held(Key) bool    { return false }
func (pauseNoKeys) Pressed(Key) bool { return false }
