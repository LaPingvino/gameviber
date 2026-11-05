package runner

import "sort"

// Key is a keyboard key as Game Maker numbers them (virtual key codes).
type Key int

// Keys Stelo and Pacmanio use.
const (
	KeyLeft  Key = 37
	KeyUp    Key = 38
	KeyRight Key = 39
	KeyDown  Key = 40
	KeySpace Key = 32
	KeyPgDn  Key = 34
	KeyEnter Key = 13
)

// Input is the keyboard as a step sees it.
type Input interface {
	Held(Key) bool    // keyboard event: the key is down
	Pressed(Key) bool // key press event: it went down since the last step
}

// Handler is an event's code: the room and the instance it runs as (self).
type Handler func(r *Room, self *Instance)

// CollisionHandler also gets the other instance.
type CollisionHandler func(r *Room, self, other *Instance)

// Events are an object's event handlers.
type Events struct {
	Create      Handler
	Alarm       [12]Handler
	Key         map[Key]Handler // keyboard (held)
	KeyPress    map[Key]Handler
	Step        Handler
	OutsideRoom Handler
	Collision   map[string]CollisionHandler // by the other object's name
	EndStep     Handler
	// BeginStep runs first in a step; Destroy when the instance is
	// destroyed; RoomStart / RoomEnd (Room.Start, Room.End); NoMoreLives
	// after a step in which the lives ran out (other events 4, 5, 6).
	BeginStep, Destroy, RoomStart, RoomEnd, NoMoreLives Handler
	// Draw replaces the default drawing (the sprite) when set.
	Draw Handler
	// KeyRelease runs when a key goes up; Mouse by GM's mouse event numbers
	// (0–2 a button held on the instance, 4–6 pressed, 7–9 released, 3 none;
	// 50–52 / 53–55 / 56–58 the same anywhere).
	KeyRelease map[Key]Handler
	Mouse      map[int]Handler
	// Other holds the other events by GM's number not given a field above:
	// 1 intersect boundary, 2 game start, 3 game end, 7 animation end, 10–25 user events 0–15.
	Other map[int]Handler
}

// Create places a new instance and runs its create event (instance_create).
func (r *Room) Create(obj *Object, x, y float64) *Instance {
	in := NewInstance(obj, x, y)
	if r.G != nil {
		r.G.lastID++
		in.ID = FirstInstanceID + r.G.lastID - 1
	}
	r.Instances = append(r.Instances, in)
	if h := obj.handler(func(e *Events) Handler { return e.Create }); h != nil {
		h(r, in)
	}
	return in
}

// handler finds an event's handler on the object or, failing that, its nearest
// parent that has one.
func (o *Object) handler(get func(*Events) Handler) Handler {
	for _, p := range o.chain() {
		if h := get(&p.Events); h != nil {
			return running(get, p, h)
		}
	}
	return nil
}

// eventCtx is an event being run on an instance: which event, and the object
// up the parent chain whose handler runs (event_inherited goes on from its
// parent).
type eventCtx struct {
	get   func(*Events) Handler
	owner *Object
}

// running wraps a handler so the instance knows which event it is in.
func running(get func(*Events) Handler, owner *Object, h Handler) Handler {
	return func(r *Room, i *Instance) {
		i.events = append(i.events, eventCtx{get, owner})
		defer func() { i.events = i.events[:len(i.events)-1] }()
		h(r, i)
	}
}

// Inherited runs the parent's handler for the event the instance is in
// (event_inherited); nothing outside an event or without a parent's.
func (i *Instance) Inherited(r *Room) {
	if len(i.events) == 0 {
		return
	}
	top := i.events[len(i.events)-1]
	for p := top.owner.Parent; p != nil; p = p.Parent {
		if h := top.get(&p.Events); h != nil {
			running(top.get, p, h)(r, i)
			return
		}
	}
}

// EventGetter is event_perform's (type, number) as an Events field; nil for
// events GameViber has no single handler for (collisions).
func EventGetter(typ, n int) func(*Events) Handler {
	switch typ {
	case 0:
		return func(e *Events) Handler { return e.Create }
	case 1:
		return func(e *Events) Handler { return e.Destroy }
	case 2:
		if n < 0 || n >= 12 {
			return nil
		}
		return func(e *Events) Handler { return e.Alarm[n] }
	case 3:
		switch n {
		case 1:
			return func(e *Events) Handler { return e.BeginStep }
		case 2:
			return func(e *Events) Handler { return e.EndStep }
		}
		return func(e *Events) Handler { return e.Step }
	case 5:
		return func(e *Events) Handler { return e.Key[Key(n)] }
	case 6:
		return func(e *Events) Handler { return e.Mouse[n] }
	case 7:
		switch n {
		case 0:
			return func(e *Events) Handler { return e.OutsideRoom }
		case 4:
			return func(e *Events) Handler { return e.RoomStart }
		case 5:
			return func(e *Events) Handler { return e.RoomEnd }
		case 6:
			return func(e *Events) Handler { return e.NoMoreLives }
		}
		return func(e *Events) Handler { return e.Other[n] }
	case 8:
		return func(e *Events) Handler { return e.Draw }
	case 9:
		return func(e *Events) Handler { return e.KeyPress[Key(n)] }
	case 10:
		return func(e *Events) Handler { return e.KeyRelease[Key(n)] }
	}
	return nil
}

// keyHandlers merges key maps up the parent chain (the child's own win).
func (o *Object) keyHandlers(get func(*Events) map[Key]Handler) map[Key]Handler {
	m := map[Key]Handler{}
	ch := o.chain()
	for i := len(ch) - 1; i >= 0; i-- {
		for k, h := range get(&ch[i].Events) {
			m[k] = h
		}
	}
	return m
}

// collisionHandlers merges collision events up the parent chain.
func (o *Object) collisionHandlers() map[string]CollisionHandler {
	m := map[string]CollisionHandler{}
	ch := o.chain()
	for i := len(ch) - 1; i >= 0; i-- {
		for k, h := range ch[i].Events.Collision {
			m[k] = h
		}
	}
	return m
}

// Step runs one step in Game Maker's order [engine]: alarms, keyboard, key
// press, step, movement, outside room, collisions, end step. Each phase runs
// over the instances in creation order; instances created during a phase
// take part from the next phase on, destroyed ones are skipped and removed at
// the end of the step.
//
// ASSUMPTION (from the GM manual as recalled, to check against the original):
// on a collision with a solid instance, the moving instance is first put back
// at its previous position, then the event runs, and afterwards it moves on by
// its (possibly changed) speed if that place is free.
func (r *Room) Step(in Input) {
	defer r.UpdateViews() // the views follow at the end of the step
	r.In = in
	if r.Sleep > 0 {
		r.Sleep--
		for _, i := range r.Paused { // the paused picture stays alive: animations run
			i.ImageIndex += i.ImageSpeed
		}
		for b := range r.Backgrounds { // and the backgrounds keep scrolling
			r.Backgrounds[b].X += r.Backgrounds[b].HSpeed
			r.Backgrounds[b].Y += r.Backgrounds[b].VSpeed
		}
		if r.Sleep == 0 {
			r.Paused, r.PausedG = nil, nil
		}
		return
	}
	each := func(f func(*Instance)) {
		for _, i := range r.Instances[:len(r.Instances):len(r.Instances)] {
			if !i.Destroyed {
				f(i)
			}
		}
	}
	for b := range r.Backgrounds { // the backgrounds scroll
		r.Backgrounds[b].X += r.Backgrounds[b].HSpeed
		r.Backgrounds[b].Y += r.Backgrounds[b].VSpeed
	}
	if m, ok := in.(MouseInput); ok {
		r.mouseVars(m)
	}
	each(func(i *Instance) { // begin step
		if h := i.Obj.handler(func(e *Events) Handler { return e.BeginStep }); h != nil {
			h(r, i)
		}
	})
	each(func(i *Instance) { // alarms
		for a := range i.Alarm {
			if i.Alarm[a] < 0 {
				continue
			}
			if i.Alarm[a]--; i.Alarm[a] == 0 {
				i.Alarm[a] = -1
				if h := i.Obj.handler(func(e *Events) Handler { return e.Alarm[a] }); h != nil {
					h(r, i)
				}
			}
		}
	})
	each(func(i *Instance) {
		c := i.Obj.cached()
		keys := c.key
		for _, k := range c.keyOrder {
			if in.Held(k) {
				keys[k](r, i)
			}
		}
	})
	each(func(i *Instance) {
		c := i.Obj.cached()
		keys := c.press
		for _, k := range c.pressOrder {
			if in.Pressed(k) {
				keys[k](r, i)
			}
		}
	})
	if rel, ok := in.(KeyReleaser); ok { // key release events
		each(func(i *Instance) {
			for k, h := range i.Obj.keyHandlers(func(e *Events) map[Key]Handler { return e.KeyRelease }) {
				if rel.Released(k) {
					h(r, i)
				}
			}
		})
	}
	if m, ok := in.(MouseInput); ok { // mouse events
		r.mouse(m, each)
	}
	each(func(i *Instance) {
		if h := i.Obj.handler(func(e *Events) Handler { return e.Step }); h != nil {
			h(r, i)
		}
	})
	// Timelines: the moment at the current position runs, then the position
	// advances by 1. ASSUMPTION [engine]: after the step event, before
	// movement; the first moment checked is position 0.
	each(func(i *Instance) {
		if t := i.Timeline; t != nil {
			if h := t.Moments[i.TimelinePos]; h != nil {
				h(r, i)
			}
			if i.Timeline == t {
				i.TimelinePos++
			}
		}
	})
	each(func(i *Instance) {
		i.XPrevious, i.YPrevious = i.X, i.Y
		if i.Path != nil { // on a path: it moves along it (instead of its speed)
			if i.stepPath() {
				if h := i.Obj.handler(func(e *Events) Handler { return e.Other[8] }); h != nil {
					h(r, i)
				}
			}
			return
		}
		i.Move()
	})
	each(func(i *Instance) {
		if h := i.Obj.handler(func(e *Events) Handler { return e.OutsideRoom }); h != nil && r.outside(i) {
			h(r, i)
		}
	})
	each(func(i *Instance) { // intersect boundary (other 1)
		if h := i.Obj.handler(func(e *Events) Handler { return e.Other[1] }); h != nil && r.crossesEdge(i) {
			h(r, i)
		}
	})
	each(func(i *Instance) {
		c := i.Obj.cached()
		cols := c.coll
		for _, name := range c.collOrder {
			h := cols[name]
			for _, o := range r.Instances {
				if o == i || o.Destroyed || i.Destroyed || !o.Obj.IsA(name) || !i.Obj.Mask || !o.Obj.Mask {
					continue
				}
				if !touches(i, i.X, i.Y, o) {
					continue
				}
				if o.Solid {
					i.X, i.Y = i.XPrevious, i.YPrevious
					h(r, i, o)
					if nx, ny := i.X+i.HSpeed, i.Y+i.VSpeed; (i.HSpeed != 0 || i.VSpeed != 0) && r.PlaceFree(i, nx, ny) {
						i.X, i.Y = nx, ny
					}
					continue
				}
				h(r, i, o)
			}
		}
	})
	each(func(i *Instance) {
		if h := i.Obj.handler(func(e *Events) Handler { return e.EndStep }); h != nil {
			h(r, i)
		}
	})
	each(func(i *Instance) { // animation: image_index advances by image_speed
		i.ImageIndex += i.ImageSpeed
		if r.FrameCount == nil {
			return
		}
		// Past the last frame: back round, and the animation end event
		// (other 7) — GM 5 wraps image_index the same way.
		if n := float64(r.FrameCount(i.Sprite)); n > 0 && i.ImageIndex >= n {
			i.ImageIndex -= n
			if h := i.Obj.handler(func(e *Events) Handler { return e.Other[7] }); h != nil {
				h(r, i)
			}
		}
	})
	live := r.Instances[:0]
	for _, i := range r.Instances {
		if !i.Destroyed {
			live = append(live, i)
		}
	}
	r.Instances = live
}

// outside reports whether the instance is entirely outside the room (its box,
// or its position without one).
func (r *Room) outside(i *Instance) bool {
	b := Box{int(i.X), int(i.X), int(i.Y), int(i.Y)}
	if i.Obj.Mask {
		b = i.BoxAt(i.X, i.Y)
	}
	return b.R < 0 || b.L >= r.W || b.B < 0 || b.T >= r.H
}

// crossesEdge: GM 5's intersect boundary — the instance lies at least partly
// outside the room (its box, or its position without one). ASSUMPTION
// [engine]: a box ending exactly on the last pixel row / column is inside.
func (r *Room) crossesEdge(i *Instance) bool {
	b := Box{int(i.X), int(i.X), int(i.Y), int(i.Y)}
	if i.Obj.Mask {
		b = i.BoxAt(i.X, i.Y)
	}
	return b.L < 0 || b.R >= r.W || b.T < 0 || b.B >= r.H
}

// Events run in a fixed order: keys by key code, collisions by the other
// object's name. [engine] Game Maker orders them by key code and by object
// index; ASSUMPTION: the name order is close enough until the object index is
// carried over (no Stelo object has two collisions that interact in one step).
func sortedKeys(m map[Key]Handler) []Key {
	ks := make([]Key, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Slice(ks, func(a, b int) bool { return ks[a] < ks[b] })
	return ks
}

func sortedNames(m map[string]CollisionHandler) []string {
	ns := make([]string, 0, len(m))
	for n := range m {
		ns = append(ns, n)
	}
	sort.Strings(ns)
	return ns
}

// objCache is an object's merged handlers, in the order Step runs them.
type objCache struct {
	key, press           map[Key]Handler
	keyOrder, pressOrder []Key
	coll                 map[string]CollisionHandler
	collOrder            []string
}

func (o *Object) cached() *objCache {
	if o.cache == nil {
		c := &objCache{
			key:   o.keyHandlers(func(e *Events) map[Key]Handler { return e.Key }),
			press: o.keyHandlers(func(e *Events) map[Key]Handler { return e.KeyPress }),
			coll:  o.collisionHandlers(),
		}
		c.keyOrder, c.pressOrder, c.collOrder = sortedKeys(c.key), sortedKeys(c.press), sortedNames(c.coll)
		o.cache = c
	}
	return o.cache
}

// Start runs the room start event of every instance (after all were created).
func (r *Room) Start() { r.every(func(e *Events) Handler { return e.RoomStart }) }

// End runs the room end event of every instance.
func (r *Room) End() { r.every(func(e *Events) Handler { return e.RoomEnd }) }

func (r *Room) every(get func(*Events) Handler) {
	for _, i := range r.Instances[:len(r.Instances):len(r.Instances)] {
		if !i.Destroyed {
			if h := i.Obj.handler(get); h != nil {
				h(r, i)
			}
		}
	}
}

// Destroy destroys an instance, running its destroy event first.
func (r *Room) Destroy(i *Instance) {
	if i.Destroyed {
		return
	}
	if h := i.Obj.handler(func(e *Events) Handler { return e.Destroy }); h != nil {
		h(r, i)
	}
	i.Destroyed = true
}

// NoMoreLivesDue runs the no-more-lives event when the lives ran out this
// step (GM 5 checks after the step); the flag is cleared.
func (r *Room) NoMoreLivesDue() {
	if r.G == nil || !r.G.NoMoreLives {
		return
	}
	r.G.NoMoreLives = false
	r.every(func(e *Events) Handler { return e.NoMoreLives })
}

// KeyReleaser is an Input that also tells which keys went up this step.
type KeyReleaser interface{ Released(Key) bool }

// MouseInput is an Input with the mouse: its position in the room and the
// buttons (1 left, 2 right, 3 middle) held, pressed and released this step.
type MouseInput interface {
	Mouse() (x, y float64)
	MouseHeld(b int) bool
	MousePressed(b int) bool
	MouseReleased(b int) bool
}

// mouseVars keeps mouse_x / mouse_y / mouse_button in the globals (read at
// the start of the step, before any event, as GM 5 does).
func (r *Room) mouseVars(m MouseInput) {
	if r.G == nil || r.G.Vars == nil {
		return
	}
	x, y := m.Mouse()
	r.G.Vars["mouse_x"], r.G.Vars["mouse_y"] = x, y
	r.G.Vars["mouse_button"] = 0
	for b := 1; b <= 3; b++ {
		if m.MouseHeld(b) {
			r.G.Vars["mouse_button"] = float64(b)
		}
	}
}

// mouse runs the mouse events (after the key events, as GM 5 does).
func (r *Room) mouse(m MouseInput, each func(func(*Instance))) {
	x, y := m.Mouse()
	pt := Box{int(x), int(x), int(y), int(y)}
	each(func(i *Instance) {
		hs := i.Obj.mouseHandlers()
		if len(hs) == 0 {
			return
		}
		on := i.Obj.Mask && i.BoxAt(i.X, i.Y).Overlaps(pt)
		run := func(ev int) {
			if h := hs[ev]; h != nil {
				h(r, i)
			}
		}
		none := true
		for b := 1; b <= 3; b++ {
			if m.MouseHeld(b) {
				none = false
				if on {
					run(b - 1)
				}
				run(49 + b)
			}
			if m.MousePressed(b) {
				if on {
					run(3 + b)
				}
				run(52 + b)
			}
			if m.MouseReleased(b) {
				if on {
					run(6 + b)
				}
				run(55 + b)
			}
		}
		if none && on {
			run(3)
		}
	})
}

// mouseHandlers merges the mouse events up the parent chain.
func (o *Object) mouseHandlers() map[int]Handler {
	m := map[int]Handler{}
	ch := o.chain()
	for i := len(ch) - 1; i >= 0; i-- {
		for k, h := range ch[i].Events.Mouse {
			m[k] = h
		}
	}
	return m
}

// Every runs an event of every instance (game start / end …).
func (r *Room) Every(get func(*Events) Handler) { r.every(get) }

// FirstInstanceID is the first instance id (GM 5: 100001; below it, numbers
// name objects).
const FirstInstanceID = 100001

// ByID finds a live instance by its id.
func (r *Room) ByID(id int) *Instance {
	for _, in := range r.Instances {
		if in.ID == id && !in.Destroyed {
			return in
		}
	}
	return nil
}
