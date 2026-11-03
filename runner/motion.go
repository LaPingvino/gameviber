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
	return map[string]gml.Func{
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
