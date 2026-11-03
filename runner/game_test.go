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

// mouseKeys is an Input with a mouse and key releases.
type mouseKeys struct {
	x, y                   float64
	held, pressed, release map[int]bool
	keyUp                  map[Key]bool
}

func (m mouseKeys) Held(Key) bool             { return false }
func (m mouseKeys) Pressed(Key) bool          { return false }
func (m mouseKeys) Released(k Key) bool       { return m.keyUp[k] }
func (m mouseKeys) Mouse() (float64, float64) { return m.x, m.y }
func (m mouseKeys) MouseHeld(b int) bool      { return m.held[b] }
func (m mouseKeys) MousePressed(b int) bool   { return m.pressed[b] }
func (m mouseKeys) MouseReleased(b int) bool  { return m.release[b] }

// Mouse events on and off an instance, the global ones, key release, and
// mouse_x / mouse_y in the globals.
func TestMouseAndKeyRelease(t *testing.T) {
	btn := &Object{Name: "button", BBox: Box{0, 31, 0, 31}, Mask: true}
	g := &Game{Objects: []*Object{btn}}
	code := func(src string) Handler {
		h, err := g.Code(src)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	btn.Events.Mouse = map[int]Handler{4: code("clicks += 1"), 53: code("anywhere += 1"), 3: code("idle += 1")}
	btn.Events.KeyRelease = map[Key]Handler{KeySpace: code("ups += 1; mx = mouse_x")}
	r := &Room{W: 640, H: 480, G: &Globals{Vars: map[string]float64{}}}
	b := r.Create(btn, 100, 100)
	b.Vars["clicks"], b.Vars["anywhere"], b.Vars["idle"], b.Vars["ups"] = 0, 0, 0, 0
	r.Step(mouseKeys{x: 110, y: 110, pressed: map[int]bool{1: true}, held: map[int]bool{1: true}})
	r.Step(mouseKeys{x: 300, y: 300, pressed: map[int]bool{1: true}, held: map[int]bool{1: true}})
	r.Step(mouseKeys{x: 105, y: 120, keyUp: map[Key]bool{KeySpace: true}})
	if len(g.Errors) > 0 {
		t.Fatal(g.Errors)
	}
	if b.Vars["clicks"] != 1 || b.Vars["anywhere"] != 2 || b.Vars["idle"] != 1 || b.Vars["ups"] != 1 || b.Vars["mx"] != 105 {
		t.Errorf("%v", b.Vars)
	}
}

// Animation end (other 7) when image_index passes the last frame (and wraps),
// and user events through event_user.
func TestAnimationEndAndUserEvents(t *testing.T) {
	spin := &Object{Name: "spin", Sprite: "four"}
	g := &Game{Objects: []*Object{spin}}
	code := func(src string) Handler {
		h, err := g.Code(src)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	spin.Events.Other = map[int]Handler{7: code("ends += 1; event_user(2)"), 12: code("user2 += 1")}
	r := &Room{W: 640, H: 480, G: &Globals{Vars: map[string]float64{}},
		FrameCount: func(s string) int { return map[string]int{"four": 4}[s] }}
	in := r.Create(spin, 0, 0)
	in.Vars["ends"], in.Vars["user2"] = 0, 0
	for range 9 { // image_speed 1, 4 frames: the end after steps 4 and 8
		r.Step(noKeys)
	}
	if len(g.Errors) > 0 {
		t.Fatal(g.Errors)
	}
	if in.Vars["ends"] != 2 || in.Vars["user2"] != 2 || in.ImageIndex != 1 {
		t.Errorf("ends %v, user2 %v, image_index %v", in.Vars["ends"], in.Vars["user2"], in.ImageIndex)
	}
}
