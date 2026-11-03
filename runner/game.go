package runner

import (
	"fmt"

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

	byName  map[string]int
	globals map[*Globals]*GlobalVars
	in      *gml.Interp
	room    *Room
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
		for k, f := range g.Funcs {
			g.in.Funcs[k] = f
		}
		g.in.Lookup = func(name string) (gml.Value, bool) {
			if i, ok := g.byName[name]; ok {
				return gml.RealV(float64(i)), true
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
			g.self().Destroyed = true
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
		"room_goto_next": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			g.room.Goto = "@next"
			return gml.RealV(0), nil
		},
	}
}
