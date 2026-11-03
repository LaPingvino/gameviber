package runner

import "testing"

var (
	block  = &Object{Name: "block", Solid: true, Mask: true, BBox: Box{0, 31, 0, 31}}
	player = &Object{Name: "character", Mask: true, BBox: Box{7, 23, 2, 31}} // sprite_right's box
	ladder = &Object{Name: "ladder", Mask: true, BBox: Box{6, 25, 0, 31}}
)

func TestBoxesInclusive(t *testing.T) {
	a := Box{0, 31, 0, 31}
	if !a.Overlaps(Box{31, 40, 0, 5}) || a.Overlaps(Box{32, 40, 0, 5}) {
		t.Error("edges are inclusive: touching at pixel 31 overlaps, 32 doesn't")
	}
}

func TestPlaceFreeAndMeeting(t *testing.T) {
	r := &Room{W: 640, H: 480}
	floor := NewInstance(block, 64, 96)
	p := NewInstance(player, 64, 64)
	l := NewInstance(ladder, 64, 64)
	r.Instances = []*Instance{floor, p, l}
	if !r.PlaceFree(p, 64, 64) {
		t.Error("standing on the floor is free (box bottom 95, floor top 96)")
	}
	if r.PlaceFree(p, 64, 65) {
		t.Error("one pixel down overlaps the floor")
	}
	if r.PlaceMeeting(p, 64, 64, "ladder") != l {
		t.Error("meets the ladder")
	}
	if r.PlaceMeeting(p, 64+40, 64, "ladder") != nil {
		t.Error("40 px right is off the ladder")
	}
}

func TestMoveContactSolid(t *testing.T) {
	r := &Room{W: 640, H: 480}
	floor := NewInstance(block, 64, 96)
	p := NewInstance(player, 64, 50)
	r.Instances = []*Instance{floor, p}
	r.MoveContactSolid(p, 270, 12)
	if p.Y != 62 {
		t.Errorf("12 px down from 50: y = %v, want 62 (not yet touching)", p.Y)
	}
	r.MoveContactSolid(p, 270, 12)
	if p.Y != 64 {
		t.Errorf("stops touching the floor: y = %v, want 64", p.Y)
	}
}

func TestGravityAndCap(t *testing.T) {
	p := NewInstance(player, 0, 0)
	p.Gravity = 0.5
	for range 4 {
		p.Move()
	}
	// vspeed 0.5, 1, 1.5, 2 → y 0.5 + 1 + 1.5 + 2 = 5
	if p.VSpeed != 2 || p.Y != 5 || p.X != 0 {
		t.Errorf("after 4 steps: vspeed %v, y %v, x %v", p.VSpeed, p.Y, p.X)
	}
	if d := p.Direction(); d != 270 {
		t.Errorf("falling direction %v, want 270", d)
	}
}

type keys struct{ held, pressed map[Key]bool }

func (k keys) Held(x Key) bool    { return k.held[x] }
func (k keys) Pressed(x Key) bool { return k.pressed[x] }

var noKeys = keys{}

func TestAlarmFiresAfterNSteps(t *testing.T) {
	fired := 0
	obj := &Object{Name: "dead"}
	obj.Events.Alarm[0] = func(r *Room, self *Instance) { fired++; self.Destroyed = true }
	r := &Room{W: 640, H: 480}
	in := r.Create(obj, 0, 0)
	in.Alarm[0] = 10 // monster_dead: action_set_alarm(10, 0)
	for s := 1; s <= 10; s++ {
		r.Step(noKeys)
		if (s < 10) != (fired == 0) {
			t.Fatalf("step %d: fired %d", s, fired)
		}
	}
	if len(r.Instances) != 0 {
		t.Error("destroyed instance removed at the end of the step")
	}
}

func TestSolidCollisionPutsBackThenEvent(t *testing.T) {
	r := &Room{W: 640, H: 480}
	p := &Object{Name: "character", Mask: true, BBox: Box{7, 23, 2, 31}}
	var sawY float64
	p.Events.Collision = map[string]CollisionHandler{"block": func(r *Room, self, other *Instance) {
		sawY = self.Y
		r.MoveContactSolid(self, self.Direction(), 12) // character's block event
		self.VSpeed = 0
	}}
	r.Create(block, 64, 96)
	in := r.Create(p, 64, 60)
	in.VSpeed = 6 // falling: 60 → 66 overlaps the floor (box bottom 97)
	r.Step(noKeys)
	if sawY != 60 {
		t.Errorf("event ran at y %v, want the previous position 60", sawY)
	}
	if in.Y != 64 || in.VSpeed != 0 {
		t.Errorf("after the event: y %v vspeed %v, want 64 (on the floor) and 0", in.Y, in.VSpeed)
	}
}

func TestOutsideRoomAndKeys(t *testing.T) {
	r := &Room{W: 640, H: 480}
	p := &Object{Name: "character", Mask: true, BBox: Box{7, 23, 2, 31}}
	out := 0
	p.Events.OutsideRoom = func(r *Room, self *Instance) { out++ }
	p.Events.Key = map[Key]Handler{KeyRight: func(r *Room, self *Instance) { self.X += 4 }}
	in := r.Create(p, 600, 470)
	r.Step(keys{held: map[Key]bool{KeyRight: true}})
	if in.X != 604 || out != 0 {
		t.Errorf("x %v outside %d", in.X, out)
	}
	in.Y = 480 - 1 // box top at 481: entirely below the room
	r.Step(noKeys)
	if out != 1 {
		t.Errorf("outside event: %d", out)
	}
}

// Pacmanio's ghosts: scared is a child of monster, intermonster of scared.
func TestParents(t *testing.T) {
	monster := &Object{Name: "monster", Mask: true, BBox: Box{1, 30, 0, 31}}
	scared := &Object{Name: "scared", Mask: true, BBox: Box{1, 30, 0, 31}, Parent: monster}
	inter := &Object{Name: "intermonster", Mask: true, BBox: Box{1, 30, 0, 31}, Parent: scared}
	stepped := 0
	monster.Events.Step = func(r *Room, self *Instance) { stepped++ }
	hits := map[string]int{}
	pac := &Object{Name: "pacman", Mask: true, BBox: Box{2, 30, 2, 30}}
	pac.Events.Collision = map[string]CollisionHandler{
		"monster": func(r *Room, self, other *Instance) { hits["monster"]++ },
		"scared":  func(r *Room, self, other *Instance) { hits["scared"]++ },
	}
	r := &Room{W: 640, H: 480}
	r.Create(pac, 64, 64)
	r.Create(inter, 64, 64)
	r.Step(noKeys)
	if stepped != 1 {
		t.Errorf("intermonster runs monster's step event (inherited): %d", stepped)
	}
	// An intermonster is a scared and a monster, so both of pacman's events
	// see it. [engine] Whether Game Maker 5.3 really runs both is the open
	// point in Pacmanio's rules (§3); this records what the core does now.
	if hits["monster"] != 1 || hits["scared"] != 1 {
		t.Errorf("collision events seen: %v", hits)
	}
	if n := len(r.All("monster")); n != 1 {
		t.Errorf("with(monster) covers children: %d", n)
	}
	if !inter.IsA("monster") || monster.IsA("scared") {
		t.Error("IsA follows parents only upwards")
	}
}

func TestSleepFreezesSteps(t *testing.T) {
	r := &Room{W: 640, H: 480}
	in := r.Create(player, 0, 0)
	in.HSpeed = 1
	r.SleepMS(1000, 30) // action_sleep(1000) at room speed 30
	for range 30 {
		r.Step(noKeys)
	}
	if in.X != 0 {
		t.Errorf("moved during the sleep: x %v", in.X)
	}
	r.Step(noKeys)
	if in.X != 1 {
		t.Errorf("moves after the sleep: x %v", in.X)
	}
}

// keyboard_string: Backspace deletes the last character (not past empty).
func TestTypeBackspace(t *testing.T) {
	g := &Globals{}
	g.Type([]rune("abĉ\b\bd"))
	if g.KeyboardString != "ad" {
		t.Errorf("got %q", g.KeyboardString)
	}
	g.Type([]rune("\b\b\b"))
	if g.KeyboardString != "" {
		t.Errorf("got %q", g.KeyboardString)
	}
}
