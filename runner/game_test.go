package runner

import (
	"fmt"
	"math"
	"testing"

	"github.com/LaPingvino/gameviber/gml"
)

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

// Intersect boundary (other 1): while the box is partly (or wholly) outside.
func TestIntersectBoundary(t *testing.T) {
	ball := &Object{Name: "ball", BBox: Box{0, 15, 0, 15}, Mask: true}
	g := &Game{Objects: []*Object{ball}}
	h, err := g.Code("edges += 1")
	if err != nil {
		t.Fatal(err)
	}
	ball.Events.Other = map[int]Handler{1: h}
	r := &Room{W: 100, H: 100, G: &Globals{Vars: map[string]float64{}}}
	in := r.Create(ball, 70, 50)
	in.Vars["edges"] = 0
	in.HSpeed = 5
	for range 4 { // x 75, 80, 85 (box to 100: out), 90
		r.Step(noKeys)
	}
	if in.Vars["edges"] != 2 {
		t.Errorf("edges %v, want 2", in.Vars["edges"])
	}
}

// Instance ids: instance_create returns one; n.var, with (n) and
// instance_exists(n) reach that instance only; id is its own.
func TestInstanceIDs(t *testing.T) {
	maker := &Object{Name: "maker"}
	ball := &Object{Name: "ball"}
	g := &Game{Objects: []*Object{maker, ball}}
	h, err := g.Code(`
a = instance_create(10, 10, ball)
b = instance_create(20, 20, ball)
b.hspeed = 3
with (a) { mine = id }
alive = instance_exists(b)
with (b) instance_destroy()
gone = instance_exists(b)
any = instance_exists(ball)`)
	if err != nil {
		t.Fatal(err)
	}
	maker.Events.Create = h
	r := &Room{W: 64, H: 64, G: &Globals{Vars: map[string]float64{}}}
	m := r.Create(maker, 0, 0)
	if len(g.Errors) > 0 {
		t.Fatal(g.Errors)
	}
	a, b := r.Instances[1], r.Instances[2]
	if m.ID != 100001 || m.Vars["a"] != 100002 || m.Vars["b"] != 100003 {
		t.Errorf("ids: maker %d, a %v, b %v", m.ID, m.Vars["a"], m.Vars["b"])
	}
	if b.HSpeed != 3 || a.HSpeed != 0 || a.Vars["mine"] != 100002 {
		t.Errorf("b.hspeed %v, a.hspeed %v, a's id %v", b.HSpeed, a.HSpeed, a.Vars["mine"])
	}
	if m.Vars["alive"] != 1 || m.Vars["gone"] != 0 || m.Vars["any"] != 1 {
		t.Errorf("exists: before %v, after %v, any ball %v", m.Vars["alive"], m.Vars["gone"], m.Vars["any"])
	}
}

// Motion, distance and finding: GM 5's functions on a small room.
func TestMotionAndFinding(t *testing.T) {
	me := &Object{Name: "me", BBox: Box{0, 9, 0, 9}, Mask: true}
	coin := &Object{Name: "coin", BBox: Box{0, 9, 0, 9}, Mask: true}
	g := &Game{Objects: []*Object{me, coin}}
	h, err := g.Code(`
c1 = instance_create(100, 0, coin)
c2 = instance_create(30, 0, coin)
near = instance_nearest(x, y, coin)
second = instance_find(coin, 1)
d = distance_to_object(coin)
p = distance_to_point(13, 4)
hit = collision_point(35, 5, coin, false, true)
miss = collision_rectangle(50, 50, 60, 60, coin, false, true)
lx = lengthdir_x(10, 90); ly = lengthdir_y(10, 90)
pick = choose(7, 7, 7)
move_towards_point(0, 50, 2)`)
	if err != nil {
		t.Fatal(err)
	}
	me.Events.Create = h
	r := &Room{W: 200, H: 200, G: &Globals{Vars: map[string]float64{}}}
	m := r.Create(me, 0, 0)
	if len(g.Errors) > 0 {
		t.Fatal(g.Errors)
	}
	v := m.Vars
	switch {
	case v["near"] != v["c2"], v["second"] != v["c2"]:
		t.Errorf("nearest %v, second %v; c2 %v", v["near"], v["second"], v["c2"])
	case v["d"] != 21, v["p"] != 4:
		t.Errorf("distance to the coin %v (want 21), to the point %v (want 4)", v["d"], v["p"])
	case v["hit"] != v["c2"], v["miss"] != gml.NooneID:
		t.Errorf("collision_point %v, collision_rectangle %v", v["hit"], v["miss"])
	case math.Abs(v["lx"]) > 1e-9, v["ly"] != -10, v["pick"] != 7:
		t.Errorf("lengthdir %v,%v; choose %v", v["lx"], v["ly"], v["pick"])
	case math.Abs(m.HSpeed) > 1e-9 || math.Abs(m.VSpeed-2) > 1e-9:
		t.Errorf("move_towards_point: %v,%v", m.HSpeed, m.VSpeed)
	}
}

// keyKeys holds the keys given.
type keyKeys map[Key]bool

func (k keyKeys) Held(c Key) bool  { return k[c] }
func (k keyKeys) Pressed(Key) bool { return false }

// keyboard_check reads the step's keys (vk_anykey / vk_nokey too);
// room_restart asks for the same room again; median.
func TestKeyboardCheckAndRestart(t *testing.T) {
	o := &Object{Name: "o"}
	g := &Game{Objects: []*Object{o}}
	h, err := g.Code(`left = keyboard_check(vk_left); up = keyboard_check(vk_up); any = keyboard_check(vk_anykey)
none = keyboard_check_direct(vk_nokey); m = median(5, 1, 3, 9); if (left) room_restart()`)
	if err != nil {
		t.Fatal(err)
	}
	o.Events.Step = h
	r := &Room{Name: "level", W: 64, H: 64, G: &Globals{Vars: map[string]float64{}}}
	in := r.Create(o, 0, 0)
	r.Step(keyKeys{37: true})
	if len(g.Errors) > 0 {
		t.Fatal(g.Errors)
	}
	v := in.Vars
	if v["left"] != 1 || v["up"] != 0 || v["any"] != 1 || v["none"] != 0 || v["m"] != 3 || r.Goto != "level" {
		t.Errorf("%v, goto %q", v, r.Goto)
	}
}

// room_next / room_previous follow the game's room order; -1 at the ends.
func TestRoomNextPrevious(t *testing.T) {
	o := &Object{Name: "o"}
	g := &Game{Objects: []*Object{o}, RoomOrder: []string{"title", "level1", "level2"}, RoomIDs: map[string]int{"level2": 0, "title": 1, "level1": 2}}
	h, err := g.Code(`n = room_next(level1); p = room_previous(level1); e = room_next(level2); s = room_previous(title); same = (room_next(title) = level1); here = room; go = (room = level1)`)
	if err != nil {
		t.Fatal(err)
	}
	o.Events.Create = h
	r := &Room{Name: "level1", W: 64, H: 64, G: &Globals{Vars: map[string]float64{}}}
	in := r.Create(o, 0, 0)
	if len(g.Errors) > 0 {
		t.Fatal(g.Errors)
	}
	// ids are the project's indices (not the order): level2 0, title 1, level1 2
	if in.Vars["n"] != 0 || in.Vars["p"] != 1 || in.Vars["e"] != -1 || in.Vars["s"] != -1 || in.Vars["same"] != 1 || in.Vars["here"] != 2 || in.Vars["go"] != 1 {
		t.Errorf("vars %v, strings %v", in.Vars, in.VarsStr)
	}
}

// Precise masks: two 4×4 triangles whose boxes overlap but whose pixels do
// not, then do; a non-precise box against a mask touches only on its pixels.
func TestPreciseCollision(t *testing.T) {
	tri := func(upper bool) *PixelMask { // the upper-left or lower-right half
		m := &PixelMask{W: 4, H: 4, Bits: make([]bool, 16)}
		for y := 0; y < 4; y++ {
			for x := 0; x < 4; x++ {
				m.Bits[y*4+x] = (x+y < 3) == upper
			}
		}
		return m
	}
	a := &Object{Name: "a", BBox: Box{0, 3, 0, 3}, Mask: true, Pixels: []*PixelMask{tri(true)}}
	b := &Object{Name: "b", BBox: Box{0, 3, 0, 3}, Mask: true, Pixels: []*PixelMask{tri(false)}}
	box := &Object{Name: "box", BBox: Box{0, 0, 0, 0}, Mask: true}
	ia, ib := NewInstance(a, 0, 0), NewInstance(b, 0, 0)
	if touches(ia, 0, 0, ib) {
		t.Error("upper-left and lower-right halves on the same square touch")
	}
	if !touches(ia, 1, 1, ib) {
		t.Error("shifted by one they should overlap")
	}
	dot := NewInstance(box, 3, 3)
	if touches(dot, 3, 3, ia) || !touches(dot, 0, 0, ia) {
		t.Error("a 1-pixel box against the upper-left half: (3,3) is empty, (0,0) solid")
	}
}

// TestSelfAfterNestedEvent: instance_create runs the new instance's create
// event; the creating code then goes on as its own instance (self, other), as
// in GM — it ran as the new one before (found with the shooter template: a
// spawner's alarm set after instance_create went to the enemy).
func TestSelfAfterNestedEvent(t *testing.T) {
	maker := &Object{Name: "maker"}
	ball := &Object{Name: "ball"}
	g := &Game{Objects: []*Object{maker, ball}}
	mk, err := g.Code("instance_create(30, 40, ball)\nafter_x = x\nalarm[0] = 7\nmine = 1")
	if err != nil {
		t.Fatal(err)
	}
	bc, err := g.Code("born = 1")
	if err != nil {
		t.Fatal(err)
	}
	maker.Events.Create, ball.Events.Create = mk, bc
	r := &Room{W: 64, H: 64, G: &Globals{Vars: map[string]float64{}}}
	m := r.Create(maker, 5, 6)
	if len(g.Errors) > 0 {
		t.Fatal(g.Errors)
	}
	b := r.Instances[1]
	if m.Vars["after_x"] != 5 || m.Alarm[0] != 7 || m.Vars["mine"] != 1 {
		t.Errorf("the maker after instance_create: x seen %v, alarm %v, mine %v", m.Vars["after_x"], m.Alarm[0], m.Vars["mine"])
	}
	if b.Vars["born"] != 1 || b.Alarm[0] >= 0 || b.Vars["mine"] != 0 {
		t.Errorf("the ball: born %v, alarm %v, mine %v", b.Vars["born"], b.Alarm[0], b.Vars["mine"])
	}
}

// GM 5's script_execute (by name or by id) and instance_count / instance_id[]
// (mouse_check_button is GM 6+: cmd/gameviber/hints.go).
func TestScriptExecuteInstanceCount(t *testing.T) {
	o := &Object{Name: "o"}
	g := &Game{Objects: []*Object{o}, ScriptNames: []string{"twice"}}
	if err := g.Script("twice", "return argument0 * 2"); err != nil {
		t.Fatal(err)
	}
	h, err := g.Code(`a = script_execute(twice, 4); b = script_execute(0, 5)
n = instance_count; first = instance_id[0]; past = instance_id[5]`)
	if err != nil {
		t.Fatal(err)
	}
	o.Events.Step = h
	r := &Room{W: 64, H: 64, G: &Globals{Vars: map[string]float64{}}}
	in := r.Create(o, 0, 0)
	r.Create(o, 8, 0)
	gone := r.Create(o, 16, 0)
	gone.Destroyed = true
	r.Step(keyKeys{})
	if len(g.Errors) > 0 {
		t.Fatal(g.Errors)
	}
	v := in.Vars
	if v["a"] != 8 || v["b"] != 10 || v["n"] != 2 || v["first"] != float64(in.ID) || v["past"] != -4 {
		t.Errorf("%v", v)
	}
}

// volumeSounds records sound_volume calls.
type volumeSounds struct {
	playedSounds
	vol map[string]float64
}

func (v *volumeSounds) Volume(name string, x float64) { v.vol[name] = x }

// execute_string runs text as the instance (with arguments), get_integer
// takes a typed number (the default otherwise), sound_volume reaches the
// sound backend.
func TestExecuteStringGetIntegerSoundVolume(t *testing.T) {
	o := &Object{Name: "o"}
	snd := &volumeSounds{vol: map[string]float64{}}
	answers := []string{" 42 ", "lots"}
	g := &Game{Objects: []*Object{o}, Sound: snd, AskString: func(q, def string) string {
		a := answers[0]
		answers = answers[1:]
		return a
	}}
	h, err := g.Code(`execute_string("hp = argument0 + 1; x = 7", 9); n = get_integer("How many?", 3); m = get_integer("How many?", 5)
sound_volume("boom", 0.25)`)
	if err != nil {
		t.Fatal(err)
	}
	o.Events.Step = h
	r := &Room{W: 64, H: 64, G: &Globals{Vars: map[string]float64{}}}
	in := r.Create(o, 0, 0)
	r.Step(keyKeys{})
	if len(g.Errors) > 0 {
		t.Fatal(g.Errors)
	}
	if v := in.Vars; v["hp"] != 10 || in.X != 7 || v["n"] != 42 || v["m"] != 5 || snd.vol["boom"] != 0.25 {
		t.Errorf("%v x=%v vol=%v", v, in.X, snd.vol)
	}
}

// GM 5's movement functions: contact / outside against a solid wall, bounce
// off it, snapping, wrapping, a random free place.
func TestMoveFunctions(t *testing.T) {
	wall := &Object{Name: "wall", Solid: true, BBox: Box{0, 15, 0, 15}, Mask: true}
	ball := &Object{Name: "ball", BBox: Box{0, 7, 0, 7}, Mask: true}
	g := &Game{Objects: []*Object{wall, ball}}
	code := func(src string) Handler {
		h, err := g.Code(src)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	r := &Room{W: 128, H: 96, G: &Globals{Vars: map[string]float64{}}}
	r.Create(wall, 64, 0)
	run := func(x, y float64, src string) *Instance {
		ball.Events.Step = code(src)
		b := r.Create(ball, x, y)
		r.Step(keyKeys{})
		r.Destroy(b)
		if len(g.Errors) > 0 {
			t.Fatal(g.Errors)
		}
		return b
	}
	if b := run(10, 4, "move_contact_solid(0, -1)"); b.X != 56 {
		t.Errorf("contact: x %v, want 56", b.X)
	}
	if b := run(60, 4, "move_outside_solid(180, 20)"); b.X != 56 {
		t.Errorf("outside: x %v, want 56", b.X)
	}
	if b := run(54, 4, "hspeed = 4; vspeed = 1; move_bounce_solid(false)"); b.HSpeed != -4 || b.VSpeed != 1 {
		t.Errorf("bounce: %v, %v", b.HSpeed, b.VSpeed)
	}
	if b := run(21, 30, "move_snap(16, 16); s = place_snapped(16, 16)"); b.X != 16 || b.Y != 32 || b.Vars["s"] != 1 {
		t.Errorf("snap: %v,%v s=%v", b.X, b.Y, b.Vars["s"])
	}
	if b := run(-10, 100, "move_wrap(true, true, 0)"); b.X != 118 || b.Y != 4 {
		t.Errorf("wrap: %v,%v", b.X, b.Y)
	}
	if b := run(0, 40, "move_random(8, 8)"); math.Mod(b.X, 8) != 0 || math.Mod(b.Y, 8) != 0 || !r.PlaceFree(b, b.X, b.Y) {
		t.Errorf("random: %v,%v", b.X, b.Y)
	}
}

// collision_line / collision_circle / collision_ellipse, instance_position
// and instance_furthest against two 16×16 walls.
func TestCollisionShapes(t *testing.T) {
	wall := &Object{Name: "wall", Solid: true, BBox: Box{0, 15, 0, 15}, Mask: true}
	probe := &Object{Name: "probe"}
	g := &Game{Objects: []*Object{wall, probe}}
	h, err := g.Code(`a = collision_line(0, 8, 100, 8, wall, false, true) != noone
b = collision_line(0, 40, 100, 40, wall, false, true) != noone
c = collision_line(70, 0, 30, 50, wall, false, true) != noone
d = collision_circle(50, 8, 15, wall, false, true) != noone
e = collision_circle(50, 8, 13, wall, false, true) != noone
f = collision_ellipse(30, 0, 66, 16, wall, false, true) != noone
p = instance_position(70, 10, wall); q = instance_position(10, 10, wall)
far = instance_furthest(0, 0, wall)`)
	if err != nil {
		t.Fatal(err)
	}
	probe.Events.Step = h
	r := &Room{W: 200, H: 100, G: &Globals{Vars: map[string]float64{}}}
	r.Create(wall, 64, 0)
	w2 := r.Create(wall, 150, 60)
	w1 := r.All("wall")[0]
	in := r.Create(probe, 0, 0)
	r.Step(keyKeys{})
	if len(g.Errors) > 0 {
		t.Fatal(g.Errors)
	}
	v := in.Vars
	if v["a"] != 1 || v["b"] != 0 || v["c"] != 1 || v["d"] != 1 || v["e"] != 0 || v["f"] != 1 ||
		v["p"] != float64(w1.ID) || v["q"] != gml.NooneID || v["far"] != float64(w2.ID) {
		t.Errorf("%v", v)
	}
}

// event_inherited runs the parent's handler for the same event (up a chain
// of three); event_perform runs an event by type and number.
func TestEventInheritedAndPerform(t *testing.T) {
	grand := &Object{Name: "grand"}
	parent := &Object{Name: "parent", Parent: grand}
	child := &Object{Name: "child", Parent: parent}
	g := &Game{Objects: []*Object{grand, parent, child}}
	code := func(src string) Handler {
		h, err := g.Code(src)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	grand.Events.Step = code("trail += 100")
	parent.Events.Step = code("trail += 10; event_inherited()")
	child.Events.Step = code("trail += 1; event_inherited(); event_perform(ev_alarm, 3); event_perform(ev_other, ev_user2)")
	parent.Events.Alarm[3] = code("alarmed = 1")
	child.Events.Other = map[int]Handler{12: code("user2 = 1; event_inherited()")}
	r := &Room{W: 64, H: 64, G: &Globals{Vars: map[string]float64{}}}
	in := r.Create(child, 0, 0)
	in.Vars["trail"] = 0
	r.Step(keyKeys{})
	if len(g.Errors) > 0 {
		t.Fatal(g.Errors)
	}
	if v := in.Vars; v["trail"] != 111 || v["alarmed"] != 1 || v["user2"] != 1 || len(in.events) != 0 {
		t.Errorf("%v, events left %d", v, len(in.events))
	}
}

// textDrawer records Text calls.
type textDrawer struct {
	Drawer
	lines []string
	ys    []float64
}

func (d *textDrawer) Text(x, y float64, s string, f Font) {
	d.lines, d.ys = append(d.lines, s), append(d.ys, y)
}

// draw_text_ext wraps words at the width and spaces lines by sep;
// string_width / string_height measure in the draw font (a stand-in
// measurer: 10 px a letter, 12 a line).
func TestDrawTextExtWraps(t *testing.T) {
	old := MeasureText
	defer func() { MeasureText = old }()
	MeasureText = func(s string, f Font) (float64, float64) {
		n := 1
		for _, c := range s {
			if c == '#' {
				n++
			}
		}
		return float64(len(s)) * 10, float64(n * 12)
	}
	o := &Object{Name: "o"}
	g := &Game{Objects: []*Object{o}}
	h, err := g.Code(`draw_text_ext(5, 10, "one two three four", 20, 90); w = string_width("abc"); hh = string_height("a#b")`)
	if err != nil {
		t.Fatal(err)
	}
	o.Events.Draw = h
	r := &Room{W: 64, H: 64, G: &Globals{Vars: map[string]float64{}}}
	in := r.Create(o, 0, 0)
	d := &textDrawer{}
	g.RunDraw(r, in, d)
	if len(g.Errors) > 0 {
		t.Fatal(g.Errors)
	}
	if want := []string{"one two", "three", "four"}; fmt.Sprint(d.lines) != fmt.Sprint(want) || fmt.Sprint(d.ys) != "[10 30 50]" {
		t.Errorf("lines %q at %v", d.lines, d.ys)
	}
	if in.Vars["w"] != 30 || in.Vars["hh"] != 24 {
		t.Errorf("w %v hh %v", in.Vars["w"], in.Vars["hh"])
	}
}
