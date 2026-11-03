package runner

import "testing"

// Objects whose events are GML, run in the step order: a ball moving right
// until it meets the wall, which it changes into a spark; create, step and
// collision code; with and object names.
func TestGameGML(t *testing.T) {
	wall := &Object{Name: "wall", Solid: true, BBox: Box{0, 31, 0, 31}, Mask: true}
	spark := &Object{Name: "spark", BBox: Box{0, 31, 0, 31}, Mask: true}
	ball := &Object{Name: "ball", BBox: Box{0, 31, 0, 31}, Mask: true}
	g := &Game{Objects: []*Object{wall, spark, ball}}
	var err error
	if ball.Events.Create, err = g.Code("hspeed = 8; hits = 0"); err != nil {
		t.Fatal(err)
	}
	if ball.Events.Step, err = g.Code("steps += 1; with (wall) seen = 1"); err != nil {
		t.Fatal(err)
	}
	coll, err := g.CollisionCode("hits += 1; hspeed = 0; with (other) instance_change(spark, 0); global.done = 1")
	if err != nil {
		t.Fatal(err)
	}
	ball.Events.Collision = map[string]CollisionHandler{"wall": coll}
	r := &Room{W: 640, H: 480, G: &Globals{Lives: 3, Vars: map[string]float64{}}}
	w := r.Create(wall, 96, 0)
	b := r.Create(ball, 0, 0)
	b.Vars["steps"] = 0
	for range 12 {
		r.Step(noKeys)
	}
	if len(g.Errors) > 0 {
		t.Fatal(g.Errors)
	}
	if b.Vars["hits"] != 1 || b.HSpeed != 0 || w.Obj != spark || r.G.Vars["done"] != 1 || w.Vars["seen"] != 1 {
		t.Errorf("hits %v, hspeed %v, wall now %s, done %v, seen %v", b.Vars["hits"], b.HSpeed, w.Obj.Name, r.G.Vars["done"], w.Vars["seen"])
	}
	if b.Vars["steps"] != 12 {
		t.Errorf("steps %v", b.Vars["steps"])
	}
}
