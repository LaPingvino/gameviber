package runner

import (
	"fmt"

	"github.com/LaPingvino/gameviber/gml"
)

// GM 5's drag-and-drop actions as functions (action_*), for the actions the
// sample games use. Convention: an action's "relative" box is passed as one
// more argument after its own (the importer adds it; 1 = relative); the
// "applies to" choice (self / other / object) is a with() around the call,
// and a question action's effect on the next action is the importer's 'if'.
// Names and argument orders are GM 6's action library (lib/*.lib), which
// kept GM 5's actions.

func rel(a []gml.Value, n int) bool { return len(a) > n && a[n].True() }

func (g *Game) actionFuncs() map[string]gml.Func {
	num := func(a []gml.Value, n int) float64 {
		if n < len(a) {
			return a[n].N
		}
		return 0
	}
	set := func(old *float64, v float64, relative bool) {
		if relative {
			*old += v
		} else {
			*old = v
		}
	}
	ok := gml.RealV(0)
	return map[string]gml.Func{
		// move
		"action_move_to": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (x, y, rel)
			s := g.self()
			set(&s.X, num(a, 0), rel(a, 2))
			set(&s.Y, num(a, 1), rel(a, 2))
			return ok, nil
		},
		"action_move_start": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			s := g.self()
			s.X, s.Y = s.XStart, s.YStart
			return ok, nil
		},
		"action_set_hspeed": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			set(&g.self().HSpeed, num(a, 0), rel(a, 1))
			return ok, nil
		},
		"action_set_vspeed": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			set(&g.self().VSpeed, num(a, 0), rel(a, 1))
			return ok, nil
		},
		"action_reverse_xdir": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			g.self().HSpeed *= -1
			return ok, nil
		},
		"action_reverse_ydir": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			g.self().VSpeed *= -1
			return ok, nil
		},
		"action_set_gravity": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (direction, gravity, rel)
			s := g.self()
			set(&s.GravityDirection, num(a, 0), rel(a, 2))
			set(&s.Gravity, num(a, 1), rel(a, 2))
			return ok, nil
		},
		"action_move_contact": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (direction, maximum, solid only)
			max := num(a, 1)
			if max <= 0 {
				max = 1000 // GM: -1 = as far as it goes
			}
			g.room.MoveContactSolid(g.self(), num(a, 0), max)
			return ok, nil
		},
		// instances
		"action_kill_object": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			g.self().Destroyed = true
			return ok, nil
		},
		"action_create_object": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (object, x, y, rel)
			o, err := g.obj(a[0])
			if err != nil {
				return gml.Value{}, err
			}
			x, y := num(a, 1), num(a, 2)
			if rel(a, 3) {
				x, y = x+g.self().X, y+g.self().Y
			}
			g.room.Create(o, x, y)
			return ok, nil
		},
		"action_change_object": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (object, perform events)
			o, err := g.obj(a[0])
			if err != nil {
				return gml.Value{}, err
			}
			g.room.Change(g.self(), o, rel(a, 1))
			return ok, nil
		},
		// timing
		"action_set_alarm": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (steps, alarm, rel)
			s := g.self()
			n := int(num(a, 1))
			if n < 0 || n >= len(s.Alarm) {
				return gml.Value{}, fmt.Errorf("no alarm %d", n)
			}
			if rel(a, 2) {
				s.Alarm[n] += int(num(a, 0))
			} else {
				s.Alarm[n] = int(num(a, 0))
			}
			return ok, nil
		},
		// score and lives
		"action_set_score": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (score, rel)
			if rel(a, 1) {
				g.room.G.Score += int(num(a, 0))
			} else {
				g.room.G.Score = int(num(a, 0))
			}
			return ok, nil
		},
		"action_set_life": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (lives, rel)
			n := int(num(a, 0))
			if !rel(a, 1) {
				n -= g.room.G.Lives
			}
			g.room.G.AddLives(n)
			return ok, nil
		},
		// questions: they return whether the next action runs (the importer
		// writes 'if action_if_…(…) next' — and 'if !…' for NOT)
		"action_if_empty": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (x, y, only solid, rel)
			s := g.self()
			x, y := num(a, 0), num(a, 1)
			if rel(a, 3) {
				x, y = x+s.X, y+s.Y
			}
			if rel(a, 2) { // only solid
				return boolV(g.room.PlaceFree(s, x, y)), nil
			}
			return boolV(g.room.PlaceEmpty(s, x, y)), nil
		},
		"action_if_number": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (object, number, operation: 0 =, 1 <, 2 >)
			o, err := g.obj(a[0])
			if err != nil {
				return gml.Value{}, err
			}
			n, want := float64(len(g.room.All(o.Name))), num(a, 1)
			switch int(num(a, 2)) {
			case 1:
				return boolV(n < want), nil
			case 2:
				return boolV(n > want), nil
			}
			return boolV(n == want), nil
		},
		"action_if_life": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (value, operation)
			return boolV(compare(float64(g.room.G.Lives), num(a, 0), int(num(a, 1)))), nil
		},
		"action_if_score": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			return boolV(compare(float64(g.room.G.Score), num(a, 0), int(num(a, 1)))), nil
		},
		"action_if_variable": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (value of the variable, value, operation)
			return boolV(compare(num(a, 0), num(a, 1), int(num(a, 2)))), nil
		},
		"action_if": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (expression)
			return boolV(len(a) > 0 && a[0].True()), nil
		},
		"action_if_aligned": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (hsnap, vsnap)
			s := g.self()
			h, v := int(num(a, 0)), int(num(a, 1))
			return boolV((h <= 0 || int(s.X)%h == 0) && (v <= 0 || int(s.Y)%v == 0)), nil
		},
		// rooms
		"action_next_room": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			g.room.Goto = "@next"
			return ok, nil
		},
	}
}

// compare is the questions' operation: 0 equal, 1 smaller, 2 larger.
func compare(a, b float64, op int) bool {
	switch op {
	case 1:
		return a < b
	case 2:
		return a > b
	}
	return a == b
}

func boolV(b bool) gml.Value {
	if b {
		return gml.RealV(1)
	}
	return gml.RealV(0)
}
