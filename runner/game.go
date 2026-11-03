package runner

import (
	"fmt"
	"strconv"

	"github.com/LaPingvino/gameviber/gml"
)

// Game ties the runner to the GML interpreter: the object table (an object's
// id is its index, as Game Maker numbers objects), the functions code can
// call, and the errors code ran into.
type Game struct {
	Objects []*Object
	// Funcs are added to the runner's own functions (sound, drawing, …).
	Funcs map[string]gml.Func
	// Seed seeds random() for every piece of code (replays).
	Seed int64
	// Errors are the runtime errors so far (the game goes on, as Game Maker's
	// "ignore" did; the first ones are kept).
	Errors []error

	// Highscores is GM's ten-entry table, best first (Clear fills it).
	Highscores [10]Highscore
	// AskString answers get_string (the input backend's); nil: the default.
	AskString func(question, def string) string
	// Constants are the game's own constants (its global settings).
	Constants map[string]gml.Value
	// Sound plays the game's sounds (the backend's); nil: silent.
	Sound Sound
	// Resources are the other resources' names (sprites, sounds, rooms …):
	// they read as their names, as strings, until the importer brings ids.
	Resources map[string]bool
	// ScriptNames are the scripts by id (resource ids of action arguments).
	ScriptNames []string

	scripts map[string][]gml.Stmt
	byName  map[string]int
	globals map[*Globals]*GlobalVars
	in      *gml.Interp
	room    *Room
}

// Highscore is one entry of the table.
type Highscore struct {
	Name  string
	Score int
}

// ClearHighscores is highscore_clear: ten "<nobody>" with 0.
func (g *Game) ClearHighscores() {
	for i := range g.Highscores {
		g.Highscores[i] = Highscore{"<nobody>", 0}
	}
}

// AddHighscore is highscore_add: inserted in order if it beats the last.
func (g *Game) AddHighscore(name string, score int) {
	i := len(g.Highscores)
	for i > 0 && score > g.Highscores[i-1].Score {
		i--
	}
	if i == len(g.Highscores) {
		return
	}
	copy(g.Highscores[i+1:], g.Highscores[i:len(g.Highscores)-1])
	g.Highscores[i] = Highscore{name, score}
}

// Sound is what the runner asks of the audio backend.
type Sound interface {
	Play(name string, loop bool)
	Stop(name string)
	StopAll()
	Playing(name string) bool
}

func (g *Game) soundName(v gml.Value) string {
	if v.IsStr {
		return v.S
	}
	return strconv.Itoa(int(v.N))
}

// Object returns the object of that name (nil if none).
func (g *Game) Object(name string) *Object {
	g.index()
	if i, ok := g.byName[name]; ok {
		return g.Objects[i]
	}
	return nil
}

func (g *Game) index() {
	if len(g.byName) == len(g.Objects) {
		return
	}
	g.byName = map[string]int{}
	for i, o := range g.Objects {
		g.byName[o.Name] = i
	}
}

// Script adds a script: callable by name (name(args…)) and by
// action_execute_script.
func (g *Game) Script(name, src string) error {
	list, err := gml.Parse(src)
	if err != nil {
		return err
	}
	if g.scripts == nil {
		g.scripts = map[string][]gml.Stmt{}
	}
	if g.Funcs == nil {
		g.Funcs = map[string]gml.Func{}
	}
	g.scripts[name] = list
	g.Funcs[name] = func(in *gml.Interp, a []gml.Value) (gml.Value, error) { return in.RunArgs(list, a) }
	if g.in != nil {
		g.in.Funcs[name] = g.Funcs[name]
	}
	return nil
}

// Code compiles a piece of GML into an event handler (create, step, alarm …).
func (g *Game) Code(src string) (Handler, error) {
	list, err := gml.Parse(src)
	if err != nil {
		return nil, err
	}
	return func(r *Room, self *Instance) { g.run(r, self, nil, list) }, nil
}

// CollisionCode compiles a collision event's GML (other is the instance hit).
func (g *Game) CollisionCode(src string) (CollisionHandler, error) {
	list, err := gml.Parse(src)
	if err != nil {
		return nil, err
	}
	return func(r *Room, self, other *Instance) { g.run(r, self, other, list) }, nil
}

func (g *Game) run(r *Room, self, other *Instance, list []gml.Stmt) {
	in := g.interp(r)
	in.Self, in.Other = self, nil
	if other != nil {
		in.Other = other
	}
	if _, err := in.Run(list); err != nil && len(g.Errors) < 100 {
		g.Errors = append(g.Errors, err)
	}
}

func (g *Game) interp(r *Room) *gml.Interp {
	g.index()
	g.room = r
	if g.globals == nil {
		g.globals = map[*Globals]*GlobalVars{}
	}
	gv := g.globals[r.G]
	if gv == nil {
		gv = &GlobalVars{G: r.G}
		g.globals[r.G] = gv
	}
	if g.in == nil {
		g.in = &gml.Interp{Seed: g.Seed, Funcs: map[string]gml.Func{}}
		for k, f := range g.runnerFuncs() {
			g.in.Funcs[k] = f
		}
		for k, f := range g.actionFuncs() {
			g.in.Funcs[k] = f
		}
		for k, f := range g.Funcs {
			g.in.Funcs[k] = f
		}
		g.in.Lookup = func(name string) (gml.Value, bool) {
			if i, ok := g.byName[name]; ok {
				return gml.RealV(float64(i)), true
			}
			if _, ok := g.scripts[name]; ok { // a script named as a value (action arguments)
				return gml.StrV(name), true
			}
			if g.Resources[name] {
				return gml.StrV(name), true
			}
			if v, ok := g.Constants[name]; ok {
				return v, true
			}
			return gml.Value{}, false
		}
		g.in.Instances = func(id float64) []gml.Instance {
			var out []gml.Instance
			if int(id) == gml.AllID {
				for _, in := range g.room.Instances {
					if !in.Destroyed {
						out = append(out, in)
					}
				}
				return out
			}
			// PARTIAL: instance ids (100001 …) come with the importer; ids
			// here are object indices.
			if i := int(id); i >= 0 && i < len(g.Objects) {
				for _, in := range g.room.All(g.Objects[i].Name) {
					out = append(out, in)
				}
			}
			return out
		}
	}
	g.in.Global = gv
	return g.in
}

func (g *Game) self() *Instance { return g.in.Self.(*Instance) }

func (g *Game) obj(v gml.Value) (*Object, error) {
	if i := int(v.N); !v.IsStr && i >= 0 && i < len(g.Objects) {
		return g.Objects[i], nil
	}
	return nil, fmt.Errorf("no object %s", v)
}

// runnerFuncs are the functions that need the room (Game Maker 5's names).
func (g *Game) runnerFuncs() map[string]gml.Func {
	b := func(v bool) gml.Value {
		if v {
			return gml.RealV(1)
		}
		return gml.RealV(0)
	}
	return map[string]gml.Func{
		"instance_create": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			if len(a) < 3 {
				return gml.Value{}, fmt.Errorf("needs x, y, object")
			}
			o, err := g.obj(a[2])
			if err != nil {
				return gml.Value{}, err
			}
			g.room.Create(o, a[0].N, a[1].N)
			return gml.RealV(0), nil // PARTIAL: the new instance's id (instance ids come with the importer)
		},
		"instance_destroy": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			g.room.Destroy(g.self())
			return gml.RealV(0), nil
		},
		"instance_change": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			if len(a) < 2 {
				return gml.Value{}, fmt.Errorf("needs object, perform")
			}
			o, err := g.obj(a[0])
			if err != nil {
				return gml.Value{}, err
			}
			g.room.Change(g.self(), o, a[1].True())
			return gml.RealV(0), nil
		},
		"place_free": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			if len(a) < 2 {
				return gml.Value{}, fmt.Errorf("needs x, y")
			}
			return b(g.room.PlaceFree(g.self(), a[0].N, a[1].N)), nil
		},
		"place_meeting": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			if len(a) < 3 {
				return gml.Value{}, fmt.Errorf("needs x, y, object")
			}
			o, err := g.obj(a[2])
			if err != nil {
				return gml.Value{}, err
			}
			return b(g.room.PlaceMeeting(g.self(), a[0].N, a[1].N, o.Name) != nil), nil
		},
		"instance_number": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			if len(a) < 1 {
				return gml.Value{}, fmt.Errorf("needs object")
			}
			o, err := g.obj(a[0])
			if err != nil {
				return gml.Value{}, err
			}
			return gml.RealV(float64(len(g.room.All(o.Name)))), nil
		},
		"sound_play": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			if g.Sound != nil && len(a) > 0 {
				g.Sound.Play(g.soundName(a[0]), false)
			}
			return gml.RealV(0), nil
		},
		"sound_loop": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			if g.Sound != nil && len(a) > 0 {
				g.Sound.Play(g.soundName(a[0]), true)
			}
			return gml.RealV(0), nil
		},
		"sound_stop": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			if g.Sound != nil && len(a) > 0 {
				g.Sound.Stop(g.soundName(a[0]))
			}
			return gml.RealV(0), nil
		},
		"sound_stop_all": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			if g.Sound != nil {
				g.Sound.StopAll()
			}
			return gml.RealV(0), nil
		},
		"sound_isplaying": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			return b(g.Sound != nil && len(a) > 0 && g.Sound.Playing(g.soundName(a[0]))), nil
		},
		"highscore_clear": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			g.ClearHighscores()
			return gml.RealV(0), nil
		},
		"highscore_add": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (name, score)
			if len(a) < 2 {
				return gml.Value{}, fmt.Errorf("needs name, score")
			}
			g.AddHighscore(a[0].String(), int(a[1].N))
			return gml.RealV(0), nil
		},
		"highscore_value": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (place, 1–10)
			if len(a) == 0 {
				return gml.RealV(0), nil
			}
			if i := int(a[0].N) - 1; i >= 0 && i < len(g.Highscores) {
				return gml.RealV(float64(g.Highscores[i].Score)), nil
			}
			return gml.RealV(0), nil
		},
		"highscore_name": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			if len(a) == 0 {
				return gml.RealV(0), nil
			}
			if i := int(a[0].N) - 1; i >= 0 && i < len(g.Highscores) {
				return gml.StrV(g.Highscores[i].Name), nil
			}
			return gml.StrV(""), nil
		},
		"draw_highscore": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // drawing comes with the draw events
			return gml.RealV(0), nil
		},
		"get_string": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (question, default)
			q, def := "", ""
			if len(a) > 0 {
				q = a[0].String()
			}
			if len(a) > 1 {
				def = a[1].String()
			}
			if g.AskString != nil {
				return gml.StrV(g.AskString(q, def)), nil
			}
			return gml.StrV(def), nil
		},
		// GM 5's CD functions: GameViber has no CD drive, so they answer as a
		// machine without one (no CD present, no tracks; play does nothing) —
		// the games then take their own music, as they did in 2006 then.
		"cd_present": func(*gml.Interp, []gml.Value) (gml.Value, error) { return gml.RealV(0), nil },
		"cd_number":  func(*gml.Interp, []gml.Value) (gml.Value, error) { return gml.RealV(0), nil },
		"cd_playing": func(*gml.Interp, []gml.Value) (gml.Value, error) { return gml.RealV(0), nil },
		"cd_track":   func(*gml.Interp, []gml.Value) (gml.Value, error) { return gml.RealV(0), nil },
		"cd_init":    func(*gml.Interp, []gml.Value) (gml.Value, error) { return gml.RealV(0), nil },
		"cd_play":    func(*gml.Interp, []gml.Value) (gml.Value, error) { return gml.RealV(0), nil },
		"cd_stop":    func(*gml.Interp, []gml.Value) (gml.Value, error) { return gml.RealV(0), nil },
		"room_goto": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (room): by name (PARTIAL: names until ids)
			if len(a) > 0 {
				g.room.Goto = a[0].String()
			}
			return gml.RealV(0), nil
		},
		"room_goto_previous": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			g.room.Goto = "@previous"
			return gml.RealV(0), nil
		},
		"game_restart": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			g.room.Goto = "@restart"
			return gml.RealV(0), nil
		},
		"game_end": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			g.room.Goto = "@end"
			return gml.RealV(0), nil
		},
		"room_goto_next": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			g.room.Goto = "@next"
			return gml.RealV(0), nil
		},
	}
}
