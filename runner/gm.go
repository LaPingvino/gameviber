// Package runner is the GameMaker world: rooms, instances, events in Game
// Maker 5.3's step order. It began as Stelo's internal/gm, the part of the engine Stelo's objects rely
// on: instances with speed and gravity, bounding-box collisions, place_free,
// move_contact_solid, and the order of a step. docs/STELO-RULES.md §2 marks
// these as [engine]; they come from Game Maker's documented behaviour.
//
// ASSUMPTION (to check against the original running): a bounding box is taken
// at the instance's position rounded to the nearest pixel.
package runner

import "math"

// Box is a bounding box in pixels, all edges inclusive (Game Maker's
// bbox_left, bbox_right, bbox_top, bbox_bottom).
type Box struct{ L, R, T, B int }

// Overlaps reports whether two boxes share a pixel.
func (a Box) Overlaps(b Box) bool { return a.L <= b.R && b.L <= a.R && a.T <= b.B && b.T <= a.B }

// Object is what the engine needs of an object: its name, whether it is solid
// and its sprite's bounding box (relative to the instance's position; every
// Stelo sprite has its origin at 0, 0).
type Object struct {
	Name  string
	Solid bool
	BBox  Box
	// Mask reports whether the object collides at all (an object without a
	// sprite has no box).
	Mask bool
	// Events are the object's event handlers.
	Events Events
	// Sprite, Visible and Depth are the object's drawing defaults.
	Sprite  string
	Visible bool
	Depth   int
	// Parent: the object's parent. [engine] A child without its own handler for
	// an event runs its parent's; "collision with P" and with(P) also cover
	// P's children.
	Parent *Object
	// cache holds the merged handlers (built on first use: events are all
	// set up before the first step).
	cache *objCache
}

// IsA reports whether the object is the named object or one of its children.
func (o *Object) IsA(name string) bool {
	for p := o; p != nil; p = p.Parent {
		if p.Name == name {
			return true
		}
	}
	return false
}

// handlers walks the object and its parents, nearest first.
func (o *Object) chain() []*Object {
	var c []*Object
	for p := o; p != nil; p = p.Parent {
		c = append(c, p)
	}
	return c
}

// Instance is one instance in the room.
type Instance struct {
	Obj                  *Object
	X, Y                 float64
	XPrevious, YPrevious float64
	XStart, YStart       float64
	HSpeed, VSpeed       float64
	Gravity              float64
	GravityDirection     float64 // degrees, 270 = down
	Alarm                [12]int // -1 = off
	Destroyed            bool
	// Solid is the instance's own solid flag (GM 5 keeps it per instance:
	// code may change it — Pacmanio's ghosts toggle walls); from the object.
	Solid bool
	// Sprite is sprite_index (starts as the object's); ImageIndex the frame.
	Sprite     string
	ImageIndex float64
	ImageSpeed float64
	// Vars holds the object's own variables (used, timelineaktiva, …).
	Vars map[string]float64
	// VarsStr holds string variables (tekstilo's teksto).
	VarsStr map[string]string
	// Timeline is the running timeline (nil: none) and TimelinePos its
	// position (timeline_index, timeline_position; speed 1).
	Timeline    *Timeline
	TimelinePos int
}

// Timeline is a Game Maker timeline: handlers at step positions (moments).
type Timeline struct {
	Name    string
	Moments map[int]Handler
}

// NewInstance places an instance; its start and previous position are where it
// is placed.
func NewInstance(obj *Object, x, y float64) *Instance {
	in := &Instance{Obj: obj, X: x, Y: y, XPrevious: x, YPrevious: y, XStart: x, YStart: y, GravityDirection: 270,
		Sprite: obj.Sprite, ImageSpeed: 1, Solid: obj.Solid, Vars: map[string]float64{}, VarsStr: map[string]string{}}
	for i := range in.Alarm {
		in.Alarm[i] = -1
	}
	return in
}

// BoxAt is the instance's bounding box if it stood at (x, y).
func (in *Instance) BoxAt(x, y float64) Box {
	ix, iy := int(math.Round(x)), int(math.Round(y))
	b := in.Obj.BBox
	return Box{ix + b.L, ix + b.R, iy + b.T, iy + b.B}
}

// Room holds the instances.
type Room struct {
	Name string
	W, H int
	// Speed is the room's steps per second (action_sleep counts in steps).
	Speed int
	// Backgrounds are the room's background layers (GM 5 has 8).
	Backgrounds []Background
	// Caption is room_caption.
	Caption string
	// Persistent is room_persistent (PARTIAL: kept, not yet acted on when leaving).
	Persistent bool
	// Views: shown when ViewsEnabled (GM 5 has 8).
	ViewsEnabled bool
	Views        []View
	Instances    []*Instance
	// Goto asks the game to change rooms after this step (room_goto); "" = stay.
	Goto string
	// Sleep is action_sleep: steps during which the game is frozen (nothing
	// moves, nothing is drawn anew).
	Sleep int
	// Paused is the instances, PausedG the globals, as they were when the
	// current pause began (nil outside a pause). See SleepMS.
	Paused  []*Instance
	PausedG *Globals
	// G holds what lives across rooms: score, lives, global variables.
	G *Globals
}

// Globals are Game Maker's game-wide state.
type Globals struct {
	Score, Lives int
	// NoMoreLives is set when lives drop to 0 or below: the "no more lives"
	// event is due (the game runs it after the step).
	NoMoreLives bool
	Vars        map[string]float64
	// Sound plays a sound by name (action_sound); nil = silent.
	Sound func(name string)
	// KeyboardString is keyboard_string: the characters typed (Game Maker
	// keeps the last 1024).
	KeyboardString string
}

// Type appends typed characters to keyboard_string; '\b' (Backspace)
// deletes the last one [engine: Game Maker's keyboard_string does so].
func (g *Globals) Type(chars []rune) {
	for _, c := range chars {
		if c == '\b' {
			if r := []rune(g.KeyboardString); len(r) > 0 {
				g.KeyboardString = string(r[:len(r)-1])
			}
			continue
		}
		g.KeyboardString += string(c)
	}
	if r := []rune(g.KeyboardString); len(r) > 1024 {
		g.KeyboardString = string(r[len(r)-1024:])
	}
}

// AddLives is action_set_life(n, relative).
func (g *Globals) AddLives(n int) {
	g.Lives += n
	if g.Lives <= 0 {
		g.NoMoreLives = true
	}
}

// Play is action_sound(name).
func (g *Globals) Play(name string) {
	if g != nil && g.Sound != nil {
		g.Sound(name)
	}
}

// Change is action_change_object(obj, perform): the instance becomes an
// instance of obj in place, taking obj's sprite; with perform it runs obj's
// create event (and the old object's destroy event, which no Stelo object
// has).
func (r *Room) Change(in *Instance, obj *Object, perform bool) {
	// ASSUMPTION [engine]: the new object's sprite and solid flag come
	// with the change (as the sprite did in Stelo's engine).
	in.Obj, in.Sprite, in.ImageIndex, in.Solid = obj, obj.Sprite, 0, obj.Solid
	if perform {
		if h := obj.handler(func(e *Events) Handler { return e.Create }); h != nil {
			h(r, in)
		}
	}
}

// SleepMS is action_sleep(ms) at the room's speed (steps per second).
//
// Quality of life (Joop 2026-11-02: "keep the beat, no freeze"): the pause
// keeps its length and the game logic waits exactly as in the original, but
// the screen stays alive — what was showing when the pause began (Paused, a
// copy of the instances at that moment) is drawn with its animations running,
// instead of a frozen picture; PausedG keeps the score and lives of that
// moment for the HUD.
func (r *Room) SleepMS(ms, speed int) {
	if r.Sleep == 0 {
		r.Paused = make([]*Instance, 0, len(r.Instances))
		for _, i := range r.Instances {
			c := *i
			r.Paused = append(r.Paused, &c)
		}
		if r.G != nil {
			g := *r.G
			r.PausedG = &g
		}
	}
	r.Sleep += ms * speed / 1000
}

// ShownG is the globals to show (score, lives): during a pause those of the
// moment it began.
func (r *Room) ShownG() *Globals {
	if r.PausedG != nil {
		return r.PausedG
	}
	return r.G
}

// Drawn is what to draw: during a pause the moment it began, else the room.
func (r *Room) Drawn() []*Instance {
	if r.Paused != nil {
		return r.Paused
	}
	return r.Instances
}

func (r *Room) live() []*Instance {
	out := r.Instances[:0:0]
	for _, o := range r.Instances {
		if !o.Destroyed {
			out = append(out, o)
		}
	}
	return out
}

// PlaceFree is place_free(x, y): in would overlap no solid instance there.
func (r *Room) PlaceFree(in *Instance, x, y float64) bool {
	if !in.Obj.Mask {
		return true
	}
	b := in.BoxAt(x, y)
	for _, o := range r.Instances {
		if o.Destroyed {
			continue
		}
		if o != in && o.Solid && o.Obj.Mask && b.Overlaps(o.BoxAt(o.X, o.Y)) {
			return false
		}
	}
	return true
}

// PlaceMeeting is place_meeting(x, y, obj): the first instance of the named
// object in overlaps at (x, y), or nil.
func (r *Room) PlaceMeeting(in *Instance, x, y float64, obj string) *Instance {
	if !in.Obj.Mask {
		return nil
	}
	b := in.BoxAt(x, y)
	for _, o := range r.Instances {
		if o.Destroyed {
			continue
		}
		if o != in && o.Obj.IsA(obj) && o.Obj.Mask && b.Overlaps(o.BoxAt(o.X, o.Y)) {
			return o
		}
	}
	return nil
}

// All is with(obj): the live instances of the object and its children, in
// creation order.
func (r *Room) All(obj string) []*Instance {
	var out []*Instance
	for _, o := range r.Instances {
		if o.Destroyed {
			continue
		}
		if o.Obj.IsA(obj) {
			out = append(out, o)
		}
	}
	return out
}

// MoveContactSolid is move_contact_solid(dir, maxdist): move one pixel at a
// time in the direction (degrees, 0 = right, 90 = up) until the next pixel
// would touch a solid, at most maxdist pixels. If the instance already touches
// a solid it doesn't move.
func (r *Room) MoveContactSolid(in *Instance, dir, maxdist float64) {
	if !r.PlaceFree(in, in.X, in.Y) {
		return
	}
	dx, dy := math.Cos(dir*math.Pi/180), -math.Sin(dir*math.Pi/180)
	for i := 0; i < int(maxdist); i++ {
		nx, ny := in.X+dx, in.Y+dy
		if !r.PlaceFree(in, nx, ny) {
			return
		}
		in.X, in.Y = nx, ny
	}
}

// Move is the movement part of a step: gravity is added to the speed (in its
// direction), then the speed to the position.
func (in *Instance) Move() {
	in.HSpeed += in.Gravity * math.Cos(in.GravityDirection*math.Pi/180)
	in.VSpeed -= in.Gravity * math.Sin(in.GravityDirection*math.Pi/180)
	in.HSpeed, in.VSpeed = clean(in.HSpeed), clean(in.VSpeed)
	in.X += in.HSpeed
	in.Y += in.VSpeed
}

// clean removes the floating-point noise of cos/sin at right angles.
func clean(v float64) float64 { return math.Round(v*1e9) / 1e9 }

// Direction is the direction of motion in degrees (Game Maker's direction).
func (in *Instance) Direction() float64 {
	if in.HSpeed == 0 && in.VSpeed == 0 {
		return 0
	}
	d := math.Atan2(-in.VSpeed, in.HSpeed) * 180 / math.Pi
	if d < 0 {
		d += 360
	}
	return d
}

// PlaceEmpty is place_empty(x, y): no other instance (solid or not) overlaps
// in at (x, y).
func (r *Room) PlaceEmpty(in *Instance, x, y float64) bool {
	if !in.Obj.Mask {
		return true
	}
	b := in.BoxAt(x, y)
	for _, o := range r.Instances {
		if !o.Destroyed && o != in && o.Obj.Mask && b.Overlaps(o.BoxAt(o.X, o.Y)) {
			return false
		}
	}
	return true
}

// Background is one of a room's background layers: which image, where,
// tiling, its scrolling speed (moved each step), shown or not.
type Background struct {
	Name                 string
	Visible, Foreground  bool
	X, Y, HSpeed, VSpeed float64
	TileH, TileV         bool
	Stretch              bool
}

// View is one of a room's views: the part of the room shown, the object it
// follows (with the border to keep and the speed it may move; −1 no limit).
type View struct {
	Visible                          bool
	X, Y, W, H                       int
	PortX, PortY, PortW, PortH       int
	BorderH, BorderV, SpeedH, SpeedV int
	Follow                           string // an object's name; "" none
}

// UpdateViews moves each view after the instance it follows, as Game Maker
// does at the end of a step (Stelo's World.UpdateView, which is step-exact):
// nearer than the border to an edge, the view moves to bring it back to the
// border, at most the view speed a step, and stays in the room.
func (r *Room) UpdateViews() {
	if !r.ViewsEnabled || r.Paused != nil {
		return
	}
	step := func(pos, size, room, objPos, border, speed int) int {
		target := pos
		if objPos-pos < border {
			target = objPos - border
		} else if pos+size-objPos < border {
			target = objPos + border - size
		}
		d := target - pos
		if speed >= 0 {
			d = max(-speed, min(speed, d))
		}
		return max(0, min(room-size, pos+d))
	}
	for i := range r.Views {
		v := &r.Views[i]
		if !v.Visible || v.Follow == "" {
			continue
		}
		f := r.All(v.Follow)
		if len(f) == 0 {
			continue
		}
		v.X = step(v.X, v.W, r.W, int(f[0].X), v.BorderH, v.SpeedH)
		v.Y = step(v.Y, v.H, r.H, int(f[0].Y), v.BorderV, v.SpeedV)
	}
}
