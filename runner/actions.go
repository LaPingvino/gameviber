package runner

import (
	"fmt"
	"math"

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
		"action_move": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (directions, speed, rel)
			// directions: nine '0'/'1' in numeric-keypad order, bottom row
			// first (225 270 315 / 180 stop 0 / 135 90 45); one of the chosen
			// is picked at random. ASSUMPTION: the order as ENIGMA's
			// action_move has it; Pacmanio's ghosts ('010101010' = the four
			// straight directions) agree.
			dirs := []float64{225, 270, 315, 180, -1, 0, 135, 90, 45}
			pattern := ""
			if len(a) > 0 {
				pattern = a[0].S
			}
			var chosen []int
			for i, c := range pattern {
				if i < 9 && c == '1' {
					chosen = append(chosen, i)
				}
			}
			if len(chosen) == 0 {
				return ok, nil
			}
			rv, err := gml.Builtins["random"](in, []gml.Value{gml.RealV(float64(len(chosen)))})
			if err != nil {
				return gml.Value{}, err
			}
			d := dirs[chosen[int(rv.N)]]
			s := g.self()
			speed := num(a, 1)
			if rel(a, 2) {
				speed += math.Hypot(s.HSpeed, s.VSpeed)
			}
			if d < 0 {
				s.HSpeed, s.VSpeed = 0, 0
				return ok, nil
			}
			r := d * math.Pi / 180
			s.HSpeed, s.VSpeed = clean(speed*math.Cos(r)), clean(-speed*math.Sin(r))
			return ok, nil
		},
		"action_execute_script": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (script, arguments …)
			if len(a) == 0 {
				return gml.Value{}, fmt.Errorf("needs a script")
			}
			name := a[0].S
			if !a[0].IsStr {
				if i := int(a[0].N); i >= 0 && i < len(g.ScriptNames) {
					name = g.ScriptNames[i]
				}
			}
			sc, okk := g.scripts[name]
			if !okk {
				return gml.Value{}, fmt.Errorf("no script %q", name)
			}
			return in.RunArgs(sc, a[1:])
		},
		"action_sound": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (sound, loop)
			if g.Sound != nil && len(a) > 0 {
				g.Sound.Play(g.soundName(a[0]), rel(a, 1))
			}
			return ok, nil
		},
		"action_end_sound": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (sound)
			if g.Sound != nil && len(a) > 0 {
				g.Sound.Stop(g.soundName(a[0]))
			}
			return ok, nil
		},
		"action_set_sprite": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (sprite, subimage: -1 keeps the frame)
			s := g.self()
			if len(a) > 0 {
				s.Sprite = g.IDs.SpriteName(a[0])
			}
			if sub := num(a, 1); len(a) > 1 && sub != -1 {
				s.ImageIndex = sub
			}
			return ok, nil
		},
		"action_sleep": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (milliseconds, redraw)
			speed := g.room.Speed
			if speed <= 0 {
				speed = 30
			}
			g.room.SleepMS(int(num(a, 0)), speed)
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
			g.room.Destroy(g.self())
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
		"action_if_empty": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (x, y, objects, rel)
			// objects is a menu: 0 only solid, 1 all (GM's "Check Empty").
			s := g.self()
			x, y := num(a, 0), num(a, 1)
			if rel(a, 3) {
				x, y = x+s.X, y+s.Y
			}
			if num(a, 2) == 0 {
				return boolV(g.room.PlaceFree(s, x, y)), nil
			}
			return boolV(g.room.PlaceEmpty(s, x, y)), nil
		},
		"action_if_collision": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (x, y, objects, rel): GM's "Check Collision", the opposite of Check Empty
			s := g.self()
			x, y := num(a, 0), num(a, 1)
			if rel(a, 3) {
				x, y = x+s.X, y+s.Y
			}
			if num(a, 2) == 0 {
				return boolV(!g.room.PlaceFree(s, x, y)), nil
			}
			return boolV(!g.room.PlaceEmpty(s, x, y)), nil
		},
		"action_if_object": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (object, x, y, rel)
			o, err := g.obj(a[0])
			if err != nil {
				return gml.Value{}, err
			}
			s := g.self()
			x, y := num(a, 1), num(a, 2)
			if rel(a, 3) {
				x, y = x+s.X, y+s.Y
			}
			return boolV(g.room.PlaceMeeting(s, x, y, o.Name) != nil), nil
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
		"action_another_room": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (room, transition)
			if len(a) > 0 {
				g.room.Goto = g.RoomName(a[0])
			}
			return ok, nil
		},
		"action_previous_room": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			g.room.Goto = "@previous"
			return ok, nil
		},
		"action_restart_game": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			g.room.Goto = "@restart"
			return ok, nil
		},
		"action_if_next_room": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // a next room exists
			return boolV(g.HasNextRoom == nil || g.HasNextRoom(g.room.Name)), nil
		},
		"action_set_caption": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (score, scorecaption, lives, livescaption, health, healthcaption): the window caption's
			gv := g.globalsOf(g.room)
			for k, n := range []string{"show_score", "", "show_lives", "", "show_health", ""} {
				if n != "" && k < len(a) {
					gv.G.Vars[n] = boolF(a[k].True())
				}
			}
			for k, n := range map[int]string{1: "caption_score", 3: "caption_lives", 5: "caption_health"} {
				if k < len(a) {
					gv.Set(n, nil, gml.StrV(a[k].String()))
				}
			}
			return ok, nil
		},
		"action_set_timeline": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (timeline, position)
			s := g.self()
			if len(a) == 0 {
				return ok, nil
			}
			name := g.IDs.Name("timeline", a[0])
			t := g.Timelines[name]
			if t == nil && name != "" {
				return gml.Value{}, fmt.Errorf("no timeline %s", a[0].String())
			}
			s.Timeline, s.TimelinePos = t, int(num(a, 1))
			return ok, nil
		},
		// The path actions (LOOP item 12), by GameMaker's function names — an
		// imported game's actions call them by the name stored in its file.
		// ASSUMPTION [engine]: GameMaker 6–8's argument order (path, speed,
		// end action, absolute); 5.3's are called the same way in the games
		// we have (action_move, action_set_timeline …).
		"action_path": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (path, speed, endaction, absolute)
			if len(a) == 0 {
				return ok, nil
			}
			name := g.IDs.Name("path", a[0])
			p := g.Paths[name]
			if p == nil {
				if name == "" {
					g.self().Path = nil
					return ok, nil
				}
				return gml.Value{}, fmt.Errorf("no path %s", a[0].String())
			}
			g.self().PathStart(p, num(a, 1), int(num(a, 2)), num(a, 3) > 0.5)
			return ok, nil
		},
		"action_path_end": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			g.self().Path = nil
			return ok, nil
		},
		"action_path_position": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (position, relative)
			s := g.self()
			v := num(a, 0)
			if num(a, 1) > 0.5 {
				v += s.PathPos
			}
			s.PathPos = math.Max(0, math.Min(1, v))
			return ok, nil
		},
		"action_path_speed": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (speed, relative)
			s := g.self()
			v := num(a, 0)
			if num(a, 1) > 0.5 {
				v += s.PathSpeed
			}
			s.PathSpeed = v
			return ok, nil
		},
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
