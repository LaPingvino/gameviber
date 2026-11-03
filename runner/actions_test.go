package runner

import "testing"

// The D&D actions, called as the importer will write them (the relative box
// as one more argument; a question guarding the next action with 'if').
func TestActions(t *testing.T) {
	wall := &Object{Name: "wall", Solid: true, BBox: Box{0, 31, 0, 31}, Mask: true}
	dot := &Object{Name: "point", BBox: Box{12, 18, 12, 18}, Mask: true}
	pac := &Object{Name: "pacman", BBox: Box{1, 30, 1, 30}, Mask: true}
	g := &Game{Objects: []*Object{wall, dot, pac}}
	code, err := g.Code(`
action_set_score(200, 1); action_set_score(10, 1)
action_set_life(1, 1)
action_set_alarm(500, 0, 0); action_set_alarm(-100, 0, 1)
action_move_to(32, 0, 1)
action_set_hspeed(4, 0); action_reverse_xdir()
if action_if_empty(32, 0, 1, 1) a = 1
if !action_if_empty(-96, 0, 1, 1) b = 1
if action_if_number(point, 2, 0) c = 1
if action_if_life(4, 0) d = 1
action_create_object(point, 64, 0, 1)
action_move_start()`)
	if err != nil {
		t.Fatal(err)
	}
	r := &Room{W: 640, H: 480, G: &Globals{Lives: 3, Vars: map[string]float64{}}}
	r.Create(wall, 0, 0)
	r.Create(dot, 300, 300)
	r.Create(dot, 400, 300)
	p := r.Create(pac, 64, 0)
	code(r, p)
	if len(g.Errors) > 0 {
		t.Fatal(g.Errors)
	}
	if r.G.Score != 210 || r.G.Lives != 4 || p.Alarm[0] != 400 || p.HSpeed != -4 {
		t.Errorf("score %d, lives %d, alarm %d, hspeed %v", r.G.Score, r.G.Lives, p.Alarm[0], p.HSpeed)
	}
	if p.Vars["a"] != 1 || p.Vars["b"] != 1 || p.Vars["c"] != 1 || p.Vars["d"] != 1 {
		t.Errorf("questions: %v", p.Vars)
	}
	if n := len(r.All("point")); n != 3 || p.X != 64 || p.Y != 0 {
		t.Errorf("points %d, back at (%v, %v)", n, p.X, p.Y)
	}
	if last := r.All("point")[2]; last.X != 160 {
		t.Errorf("created relative at x %v, want 160 (96 + 64)", last.X)
	}
}
