package runner

import (
	"fmt"
	"sort"
	"strings"

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

	// Drawer is set while a draw event runs (RunDraw).
	Drawer Drawer
	// SpriteWidth gives a sprite's width (the loader's; life images).
	SpriteWidth func(name string) int
	draw        drawState
	// Timelines are the game's timelines by name (the loader's).
	Timelines map[string]*Timeline
	// HasNextRoom tells whether a room comes after the named one (the room
	// order is the loader's); nil: assume one does.
	HasNextRoom func(room string) bool
	// RoomOrder is the rooms in the game's order (room_next / room_previous).
	RoomOrder []string
	// RoomIDs are the rooms' GM ids (their index in the project) by name:
	// a room's name in code is its id, as in GM 5; room is a number.
	RoomIDs map[string]int
	// IDs are the other resources' ids (sprites …).
	IDs *IDs
	// Highscores is GM's ten-entry table, best first (Clear fills it).
	Highscores [10]Highscore
	// AskString answers get_string (the input backend's); nil: the default.
	AskString func(question, def string) string
	// Constants are the game's own constants (its global settings).
	Constants map[string]gml.Value
	// Message shows show_message's text (the backend's); nil: not shown.
	Message func(string)
	// Debug receives show_debug_message's text (a tool collecting what the
	// game says about itself; nil: dropped).
	Debug func(string)
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

func (g *Game) soundName(v gml.Value) string { return g.IDs.Name("sound", v) }

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
		o.Index = i
		if o.IDs == nil {
			o.IDs = g.IDs
		}
	}
}

// FuncGroups are the functions GameViber has, by kind (for the reference):
// GM 5's built-in functions, the runner's, the drag-and-drop actions, the
// drawing ones.
func (g *Game) FuncGroups() map[string][]string {
	names := func(m map[string]gml.Func) []string {
		var out []string
		for k := range m {
			out = append(out, k)
		}
		sort.Strings(out)
		return out
	}
	return map[string][]string{
		"built-in": names(gml.Builtins), "game": names(g.runnerFuncs()),
		"actions": names(g.actionFuncs()), "drawing": names(g.drawFuncs()),
	}
}

// KnownFuncs are the names code can call: GM 5's functions GameViber has
// (the runner's, the actions, the drawing, the built-ins) and the scripts.
func (g *Game) KnownFuncs() map[string]bool {
	out := map[string]bool{}
	for _, m := range []map[string]gml.Func{g.runnerFuncs(), g.actionFuncs(), g.drawFuncs(), gml.Builtins, g.Funcs} {
		for k := range m {
			out[k] = true
		}
	}
	for k := range g.scripts {
		out[k] = true
	}
	return out
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

// Eval is a GML expression's value in room r, as the room's code would see it
// (globals, instance_number(…), …) — what a tool's expectation checks. self is
// the room's first instance (or none).
func (g *Game) Eval(r *Room, expr string) (gml.Value, error) {
	list, err := gml.Parse("return (" + expr + ")")
	if err != nil {
		return gml.Value{}, err
	}
	in := g.interp(r)
	in.Self, in.Other = nil, nil
	for _, i := range r.Instances {
		if !i.Destroyed {
			in.Self = i
			break
		}
	}
	return in.Run(list)
}

// run runs an event's code as self. Events nest — instance_create runs the
// new instance's create event, instance_destroy its destroy event — so the
// running code's self, other and room are put back afterwards: in GM the code
// after instance_create still runs as its own instance.
func (g *Game) run(r *Room, self, other *Instance, list []gml.Stmt) {
	var outerSelf, outerOther gml.Instance
	outerRoom := g.room
	if g.in != nil {
		outerSelf, outerOther = g.in.Self, g.in.Other
	}
	in := g.interp(r)
	defer func() { in.Self, in.Other, g.room = outerSelf, outerOther, outerRoom }()
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
	gv := g.globalsOf(r)
	if g.in == nil {
		g.in = &gml.Interp{Seed: g.Seed, Funcs: map[string]gml.Func{}}
		for k, f := range g.runnerFuncs() {
			g.in.Funcs[k] = f
		}
		for k, f := range g.actionFuncs() {
			g.in.Funcs[k] = f
		}
		for k, f := range g.drawFuncs() {
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
			if id, ok := g.RoomIDs[name]; ok {
				return gml.RealV(float64(id)), true
			}
			if id, ok := g.IDs.Lookup(name); ok { // sprites, sounds, backgrounds, timelines
				return gml.RealV(float64(id)), true
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
			if int(id) >= FirstInstanceID { // an instance's id
				if in := g.room.ByID(int(id)); in != nil {
					out = append(out, in)
				}
				return out
			}
			if i := int(id); i >= 0 && i < len(g.Objects) { // an object: all its instances
				for _, in := range g.room.All(g.Objects[i].Name) {
					out = append(out, in)
				}
			}
			return out
		}
	}
	gv.Room = r
	gv.roomValue = g.RoomValue
	gv.ids = g.IDs
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
	m := g.coreFuncs()
	for k, f := range g.motionFuncs() {
		m[k] = f
	}
	m["keyboard_check_direct"] = m["keyboard_check"] // the hardware's state: the same here
	return m
}

func (g *Game) coreFuncs() map[string]gml.Func {
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
			return gml.RealV(float64(g.room.Create(o, a[0].N, a[1].N).ID)), nil
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
		"instance_exists": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (object or instance id)
			if len(a) == 0 {
				return b(false), nil
			}
			if id := int(a[0].N); !a[0].IsStr && id >= FirstInstanceID {
				return b(g.room.ByID(id) != nil), nil
			}
			o, err := g.obj(a[0])
			if err != nil {
				return b(false), nil
			}
			return b(len(g.room.All(o.Name)) > 0), nil
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
		// screen_redraw / screen_refresh draw at once in GM 5 (e.g. before a
		// long wait); the frame is drawn every step anyway: nothing to do.
		"screen_redraw":  func(*gml.Interp, []gml.Value) (gml.Value, error) { return gml.RealV(0), nil },
		"screen_refresh": func(*gml.Interp, []gml.Value) (gml.Value, error) { return gml.RealV(0), nil },
		// keyboard_wait waits for a key in GM 5; headless, it goes on at once
		// (PARTIAL: the input backend will wait).
		// The window and the message loop are the backend's: nothing to do here.
		"set_graphics_mode": func(*gml.Interp, []gml.Value) (gml.Value, error) { return gml.RealV(0), nil },
		"io_handle":         func(*gml.Interp, []gml.Value) (gml.Value, error) { return gml.RealV(0), nil },
		"keyboard_wait":     func(*gml.Interp, []gml.Value) (gml.Value, error) { return gml.RealV(0), nil },
		"event_user": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (n 0–15): self's user event n
			if len(a) == 0 {
				return gml.RealV(0), nil
			}
			n := int(a[0].N)
			s := g.self()
			if h := s.Obj.handler(func(e *Events) Handler { return e.Other[10+n] }); h != nil {
				h(g.room, s)
			}
			return gml.RealV(0), nil
		},
		"show_error": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (message, abort)
			msg := ""
			if len(a) > 0 {
				msg = a[0].String()
			}
			return gml.RealV(0), fmt.Errorf("show_error: %s", msg)
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
		"room_goto": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (room)
			if len(a) > 0 {
				g.room.Goto = g.RoomName(a[0])
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
		"keyboard_check": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (key): held now; vk_anykey / vk_nokey
			if len(a) == 0 || g.room.In == nil {
				return b(len(a) > 0 && int(a[0].N) == 0), nil
			}
			k := int(a[0].N)
			if k == 0 || k == 1 {
				any := false
				for c := 1; c < 256 && !any; c++ {
					any = g.room.In.Held(Key(c))
				}
				return b(any == (k == 1)), nil
			}
			return b(g.room.In.Held(Key(k))), nil
		},
		"room_next": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (room): the one after it, -1 if none
			return g.roomStep(a, 1), nil
		},
		"room_previous": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (room): the one before it, -1 if none
			return g.roomStep(a, -1), nil
		},
		"room_restart": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // the room afresh
			g.room.Goto = g.room.Name
			return gml.RealV(0), nil
		},
		"show_debug_message": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (text): for the developer (GameMaker 6+; here for tools and assistants)
			if g.Debug != nil && len(a) > 0 {
				g.Debug(a[0].String())
			}
			return gml.RealV(0), nil
		},
		"show_message": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (text): the backend shows it; headless, nothing
			if g.Message != nil && len(a) > 0 {
				g.Message(a[0].String())
			}
			return gml.RealV(0), nil
		},
		// The keyboard state is the backend's each step: nothing to clear.
		"keyboard_clear": func(*gml.Interp, []gml.Value) (gml.Value, error) { return gml.RealV(0), nil },
		"io_clear":       func(*gml.Interp, []gml.Value) (gml.Value, error) { return gml.RealV(0), nil },
		"room_goto_next": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			g.room.Goto = "@next"
			return gml.RealV(0), nil
		},
	}
}

// globalsOf is the game-wide variables of a room's globals.
func (g *Game) globalsOf(r *Room) *GlobalVars {
	if g.globals == nil {
		g.globals = map[*Globals]*GlobalVars{}
	}
	gv := g.globals[r.G]
	if gv == nil {
		gv = &GlobalVars{G: r.G}
		g.globals[r.G] = gv
	}
	return gv
}

func boolF(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// Caption is the window's caption: the room's caption, then the score and
// lives when shown (show_score / show_lives, with caption_score /
// caption_lives before them). ASSUMPTION [engine]: GM 5 shows the score by
// default ("Score: "), not the lives ("Lives: "), and parts are two spaces
// apart.
func (g *Game) Caption(r *Room) string {
	if r == nil || r.G == nil {
		return ""
	}
	gv := g.globalsOf(r)
	str := func(name, def string) string {
		if v, ok := gv.Get(name, nil); ok && v.IsStr {
			return v.S
		}
		return def
	}
	on := func(name string, def bool) bool {
		if v, ok := r.G.Vars[name]; ok {
			return v > 0.5
		}
		return def
	}
	parts := []string{}
	if r.Caption != "" {
		parts = append(parts, r.Caption)
	}
	if on("show_score", true) {
		parts = append(parts, fmt.Sprintf("%s%d", str("caption_score", "Score: "), r.G.Score))
	}
	if on("show_lives", false) {
		parts = append(parts, fmt.Sprintf("%s%d", str("caption_lives", "Lives: "), r.G.Lives))
	}
	return strings.Join(parts, "  ")
}

// roomStep is the room d places from a[0] in the game's order, or -1.
func (g *Game) roomStep(a []gml.Value, d int) gml.Value {
	if len(a) == 0 {
		return gml.RealV(-1)
	}
	name := g.RoomName(a[0])
	for i, r := range g.RoomOrder {
		if r == name && i+d >= 0 && i+d < len(g.RoomOrder) {
			return g.RoomValue(g.RoomOrder[i+d])
		}
	}
	return gml.RealV(-1)
}

// RoomName is the room a value names: its id (a number) or, from older
// projects and actions, its name.
func (g *Game) RoomName(v gml.Value) string {
	if v.IsStr {
		return v.S
	}
	for name, id := range g.RoomIDs {
		if id == int(v.N) {
			return name
		}
	}
	return v.String()
}

// RoomValue is a room as code sees it: its id, or its name when it has none.
func (g *Game) RoomValue(name string) gml.Value {
	if id, ok := g.RoomIDs[name]; ok {
		return gml.RealV(float64(id))
	}
	return gml.StrV(name)
}
