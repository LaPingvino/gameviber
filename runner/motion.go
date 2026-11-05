package runner

import (
	"fmt"
	"math"

	"github.com/LaPingvino/gameviber/gml"
)

// motionFuncs are GM 5's motion, distance and instance-finding functions.
func (g *Game) motionFuncs() map[string]gml.Func {
	id := func(in *Instance) gml.Value {
		if in == nil {
			return gml.RealV(gml.NooneID)
		}
		return gml.RealV(float64(in.ID))
	}
	setMotion := func(s *Instance, dir, speed float64) {
		d := dir * math.Pi / 180
		s.HSpeed, s.VSpeed = speed*math.Cos(d), -speed*math.Sin(d)
	}
	need := func(a []gml.Value, n int) error {
		if len(a) < n {
			return fmt.Errorf("needs %d arguments", n)
		}
		return nil
	}
	random := func(in *gml.Interp, n float64) float64 {
		v, _ := gml.Builtins["random"](in, []gml.Value{gml.RealV(n)})
		return v.N
	}
	arg := func(a []gml.Value, i int) float64 {
		if i < len(a) {
			return a[i].N
		}
		return 0
	}
	// contact moves the instance pixel by pixel (dir, at most max; ≤ 0: 1000)
	// while free decides; outside moves while it doesn't (move_outside_*).
	step := func(s *Instance, dir, max float64, free func(x, y float64) bool, until bool) {
		if max <= 0 {
			max = 1000
		}
		dx, dy := math.Cos(dir*math.Pi/180), -math.Sin(dir*math.Pi/180)
		for i := 0; i < int(max); i++ {
			if until { // move_outside: stop once free
				if free(s.X, s.Y) {
					return
				}
			} else if !free(s.X+dx, s.Y+dy) { // move_contact: stop before touching
				return
			}
			s.X, s.Y = s.X+dx, s.Y+dy
		}
	}
	// bounce is move_bounce_*: when the next position isn't free, reverse the
	// speed along the blocked axis (both at a corner). GM 5's advanced
	// (precise) bounce is taken as this one.
	bounce := func(s *Instance, free func(x, y float64) bool) {
		if s.HSpeed == 0 && s.VSpeed == 0 || free(s.X+s.HSpeed, s.Y+s.VSpeed) {
			return
		}
		h := !free(s.X+s.HSpeed, s.Y)
		v := !free(s.X, s.Y+s.VSpeed)
		if h {
			s.HSpeed = -s.HSpeed
		}
		if v {
			s.VSpeed = -s.VSpeed
		}
		if !h && !v {
			s.HSpeed, s.VSpeed = -s.HSpeed, -s.VSpeed
		}
	}
	solidFree := func(s *Instance) func(x, y float64) bool {
		return func(x, y float64) bool { return g.room.PlaceFree(s, x, y) }
	}
	allFree := func(s *Instance) func(x, y float64) bool {
		return func(x, y float64) bool { return g.room.PlaceEmpty(s, x, y) }
	}
	snap := func(v, n float64) float64 {
		if n <= 0 {
			return v
		}
		return math.Round(v/n) * n
	}
	zero := gml.RealV(0)
	return map[string]gml.Func{
		"move_contact_solid": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (dir, maxdist): up to a solid
			s := g.self()
			step(s, arg(a, 0), arg(a, 1), solidFree(s), false)
			return zero, nil
		},
		"move_contact_all": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (dir, maxdist): up to any instance
			s := g.self()
			step(s, arg(a, 0), arg(a, 1), allFree(s), false)
			return zero, nil
		},
		"move_outside_solid": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (dir, maxdist): out of solids
			s := g.self()
			step(s, arg(a, 0), arg(a, 1), solidFree(s), true)
			return zero, nil
		},
		"move_outside_all": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (dir, maxdist): out of all instances
			s := g.self()
			step(s, arg(a, 0), arg(a, 1), allFree(s), true)
			return zero, nil
		},
		"move_bounce_solid": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (adv): bounce against solids
			s := g.self()
			bounce(s, solidFree(s))
			return zero, nil
		},
		"move_bounce_all": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (adv): bounce against all instances
			s := g.self()
			bounce(s, allFree(s))
			return zero, nil
		},
		"move_snap": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (hsnap, vsnap): to the nearest grid point
			s := g.self()
			s.X, s.Y = snap(s.X, arg(a, 0)), snap(s.Y, arg(a, 1))
			return zero, nil
		},
		"place_snapped": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (hsnap, vsnap): on the grid?
			s := g.self()
			ok := (arg(a, 0) <= 0 || math.Mod(s.X, arg(a, 0)) == 0) && (arg(a, 1) <= 0 || math.Mod(s.Y, arg(a, 1)) == 0)
			if ok {
				return gml.RealV(1), nil
			}
			return zero, nil
		},
		"move_random": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (hsnap, vsnap): a free place in the room, on the grid
			s := g.self()
			for try := 0; try < 100; try++ {
				x, y := snap(random(in, float64(g.room.W)), arg(a, 0)), snap(random(in, float64(g.room.H)), arg(a, 1))
				if g.room.PlaceFree(s, x, y) {
					s.X, s.Y = x, y
					break
				}
			}
			return zero, nil
		},
		"move_wrap": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (hor, vert, margin): out one side, in the other
			s, m := g.self(), arg(a, 2)
			w, h := float64(g.room.W), float64(g.room.H)
			if arg(a, 0) != 0 {
				if s.X < -m {
					s.X += w + 2*m
				} else if s.X >= w+m {
					s.X -= w + 2*m
				}
			}
			if arg(a, 1) != 0 {
				if s.Y < -m {
					s.Y += h + 2*m
				} else if s.Y >= h+m {
					s.Y -= h + 2*m
				}
			}
			return zero, nil
		},
		"motion_set": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (direction, speed)
			if err := need(a, 2); err != nil {
				return gml.Value{}, err
			}
			setMotion(g.self(), a[0].N, a[1].N)
			return gml.RealV(0), nil
		},
		"motion_add": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (direction, speed): added to the motion
			if err := need(a, 2); err != nil {
				return gml.Value{}, err
			}
			s, d := g.self(), a[0].N*math.Pi/180
			s.HSpeed += a[1].N * math.Cos(d)
			s.VSpeed -= a[1].N * math.Sin(d)
			return gml.RealV(0), nil
		},
		"move_towards_point": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (x, y, speed)
			if err := need(a, 3); err != nil {
				return gml.Value{}, err
			}
			s := g.self()
			setMotion(s, math.Atan2(-(a[1].N-s.Y), a[0].N-s.X)*180/math.Pi, a[2].N)
			return gml.RealV(0), nil
		},
		"distance_to_point": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (x, y): from the bounding box
			if err := need(a, 2); err != nil {
				return gml.Value{}, err
			}
			s := g.self()
			return gml.RealV(boxDistance(s.box(), Box{int(a[0].N), int(a[0].N), int(a[1].N), int(a[1].N)})), nil
		},
		"distance_to_object": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (object): to the nearest of its instances, box to box
			if err := need(a, 1); err != nil {
				return gml.Value{}, err
			}
			o, err := g.obj(a[0])
			if err != nil {
				return gml.Value{}, err
			}
			s, best := g.self(), 1000000.0 // GM 5: a large number when there is none
			for _, x := range g.room.All(o.Name) {
				if x != s {
					best = min(best, boxDistance(s.box(), x.box()))
				}
			}
			return gml.RealV(best), nil
		},
		"instance_nearest": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (x, y, object): by position; noone if none
			if err := need(a, 3); err != nil {
				return gml.Value{}, err
			}
			o, err := g.obj(a[2])
			if err != nil {
				return gml.Value{}, err
			}
			var near *Instance
			best := math.Inf(1)
			for _, x := range g.room.All(o.Name) {
				if d := math.Hypot(x.X-a[0].N, x.Y-a[1].N); d < best {
					near, best = x, d
				}
			}
			return id(near), nil
		},
		"instance_find": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (object, n): the n-th; noone if none
			if err := need(a, 2); err != nil {
				return gml.Value{}, err
			}
			o, err := g.obj(a[0])
			if err != nil {
				return gml.Value{}, err
			}
			all := g.room.All(o.Name)
			if n := int(a[1].N); n >= 0 && n < len(all) {
				return id(all[n]), nil
			}
			return id(nil), nil
		},
		"instance_place": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (x, y, object): whom self would touch there
			if err := need(a, 3); err != nil {
				return gml.Value{}, err
			}
			o, err := g.obj(a[2])
			if err != nil {
				return gml.Value{}, err
			}
			return id(g.room.PlaceMeeting(g.self(), a[0].N, a[1].N, o.Name)), nil
		},
		"collision_point": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (x, y, object, precise, notme)
			if err := need(a, 3); err != nil {
				return gml.Value{}, err
			}
			x, y := int(a[0].N), int(a[1].N)
			return g.collide(a, 2, Box{x, x, y, y})
		},
		"collision_rectangle": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (x1, y1, x2, y2, object, precise, notme)
			if err := need(a, 5); err != nil {
				return gml.Value{}, err
			}
			x1, y1, x2, y2 := int(a[0].N), int(a[1].N), int(a[2].N), int(a[3].N)
			return g.collide(a, 4, Box{min(x1, x2), max(x1, x2), min(y1, y2), max(y1, y2)})
		},
	}
}

// collide: the first instance of the object (a[k]) whose box meets b;
// a[k+2] notme leaves self out. PARTIAL: precise (a[k+1]) is taken as the
// bounding box, as all our collisions are.
func (g *Game) collide(a []gml.Value, k int, b Box) (gml.Value, error) {
	o, err := g.obj(a[k])
	if err != nil {
		return gml.Value{}, err
	}
	notMe := len(a) > k+2 && a[k+2].True()
	for _, x := range g.room.All(o.Name) {
		if notMe && x == g.self() || !x.Obj.Mask {
			continue
		}
		if x.BoxAt(x.X, x.Y).Overlaps(b) {
			return gml.RealV(float64(x.ID)), nil
		}
	}
	return gml.RealV(gml.NooneID), nil
}

// box is the instance's bounding box where it is (its position without a mask).
func (in *Instance) box() Box {
	if in.Obj.Mask {
		return in.BoxAt(in.X, in.Y)
	}
	return Box{int(in.X), int(in.X), int(in.Y), int(in.Y)}
}

// boxDistance is the gap between two boxes (0 when they touch or overlap).
func boxDistance(a, b Box) float64 {
	dx := max(0, a.L-b.R, b.L-a.R)
	dy := max(0, a.T-b.B, b.T-a.B)
	return math.Hypot(float64(dx), float64(dy))
}
