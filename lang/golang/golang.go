// Package golang lets a game's events and scripts be written in Go, run by
// yaegi (an interpreter in pure Go, so the browser build keeps it).
// Importing it registers ".go" in runner.Langs and runner.ScriptLangs.
//
// Go is typed, so GameViber reaches the code through a context, g (package
// "gv"): g.Num("x") and g.Str("name") read a name as GML would (the
// instance's variable, a global, a constant, a resource), g.Set("x", 4)
// assigns it as GML would, g.Call("place_free", x, y) calls a GameViber
// function (its result a gv.Value: .Num(), .Str(), .Bool()), g.Is(…) is a
// call's truth, g.Other() is the other instance in a collision — and calls
// made through it run as that instance, GML's with (other).
//
// An event file holds the event's statements (they become the body of
// func(g *gv.Ctx)), or a whole Go file — `package main`, its imports, and
// func Event(g *gv.Ctx). A script file's statements see args []gv.Value and
// may return a gv.Value. Besides gv, the code may import "math" (the common
// functions); each file runs in its own interpreter.
package golang

import (
	"fmt"
	"math"
	"reflect"
	"strings"

	"github.com/traefik/yaegi/interp"

	"github.com/LaPingvino/gameviber/gml"
	"github.com/LaPingvino/gameviber/runner"
)

func init() {
	runner.Langs[".go"] = Compile
	runner.ScriptLangs[".go"] = CompileScript
}

// Ctx is the code's view of GameViber: the running event's context, seen as
// one instance (self, unless got through Other).
type Ctx struct {
	in   *gml.Interp
	inst gml.Instance // nil: self as the event runs
}

// Value is a GameViber value.
type Value struct{ v gml.Value }

// Num is the value as a number; Str as a string; Bool as a truth (non-zero).
func (v Value) Num() float64   { return v.v.N }
func (v Value) Str() string    { return v.v.S }
func (v Value) Bool() bool     { return !v.v.IsStr && v.v.N != 0 }
func (v Value) String() string { return v.v.String() }

// Get reads a name as GML would: the instance's variable (through Other: that
// instance's), a global, a constant, a resource; zero when there is none.
func (g *Ctx) Get(name string) Value {
	if g.inst != nil {
		if x, ok := g.inst.Get(name, nil); ok {
			return Value{x}
		}
		return Value{}
	}
	x, _ := g.in.Var(name)
	return Value{x}
}

// Num is Get(name).Num(); Str is Get(name).Str().
func (g *Ctx) Num(name string) float64 { return g.Get(name).Num() }
func (g *Ctx) Str(name string) string  { return g.Get(name).Str() }

// Set assigns a name as GML would: a global, else the instance's variable
// (made if new). v is a number, a string, a bool or a Value.
func (g *Ctx) Set(name string, v any) {
	if g.inst != nil {
		g.inst.Set(name, nil, toGML(v))
		return
	}
	g.in.SetVar(name, toGML(v))
}

// Call calls a GameViber function — through Other(), as that instance (the
// caller as other), as GML's with does. A failure stops the event with its
// error, as in GML.
func (g *Ctx) Call(fn string, args ...any) Value {
	in := g.in
	if g.inst != nil && g.inst != in.Self {
		self, otherWas := in.Self, in.Other
		in.Self, in.Other = g.inst, self
		defer func() { in.Self, in.Other = self, otherWas }()
	}
	vals := make([]gml.Value, len(args))
	for i, a := range args {
		vals[i] = toGML(a)
	}
	res, err := in.Call(fn, vals)
	if err != nil {
		panic(fmt.Errorf("%s: %w", fn, err))
	}
	return Value{res}
}

// Is is Call(fn, args...).Bool().
func (g *Ctx) Is(fn string, args ...any) bool { return g.Call(fn, args...).Bool() }

// Other is the other instance (a collision's); nil when there is none.
func (g *Ctx) Other() *Ctx {
	if g.in.Other == nil {
		return nil
	}
	return &Ctx{in: g.in, inst: g.in.Other}
}

// toGML is a Go value in GML.
func toGML(v any) gml.Value {
	switch v := v.(type) {
	case Value:
		return v.v
	case *Ctx:
		inst := v.inst
		if inst == nil {
			inst = v.in.Self
		}
		if id, ok := inst.Get("id", nil); ok {
			return id
		}
	case string:
		return gml.StrV(v)
	case bool:
		return gml.BoolV(v)
	case int:
		return gml.RealV(float64(v))
	case float64:
		return gml.RealV(v)
	case float32:
		return gml.RealV(float64(v))
	}
	return gml.RealV(0)
}

// exports are the packages the code may import.
var exports = interp.Exports{
	"gv/gv": {
		"Ctx":   reflect.ValueOf((*Ctx)(nil)),
		"Value": reflect.ValueOf((*Value)(nil)),
	},
	"math/math": {
		"Abs": reflect.ValueOf(math.Abs), "Sqrt": reflect.ValueOf(math.Sqrt), "Floor": reflect.ValueOf(math.Floor),
		"Ceil": reflect.ValueOf(math.Ceil), "Round": reflect.ValueOf(math.Round), "Min": reflect.ValueOf(math.Min),
		"Max": reflect.ValueOf(math.Max), "Pow": reflect.ValueOf(math.Pow), "Mod": reflect.ValueOf(math.Mod),
		"Sin": reflect.ValueOf(math.Sin), "Cos": reflect.ValueOf(math.Cos), "Tan": reflect.ValueOf(math.Tan),
		"Atan2": reflect.ValueOf(math.Atan2), "Hypot": reflect.ValueOf(math.Hypot), "Pi": reflect.ValueOf(math.Pi),
	},
}

// load compiles one file in its own interpreter and returns its function
// named fn: the source as written when it is a whole file, else wrapped as
// fn's body (on the first line, so error lines are the file's).
func load(file, src, fn, head, tail string) (reflect.Value, error) {
	i := interp.New(interp.Options{})
	if err := i.Use(exports); err != nil {
		return reflect.Value{}, err
	}
	if !strings.HasPrefix(strings.TrimSpace(stripComments(src)), "package") {
		src = `package main; import "gv"; ` + head + src + tail
	}
	if _, err := i.Eval(src); err != nil {
		return reflect.Value{}, fmt.Errorf("%s:%v", file, err)
	}
	f, err := i.Eval("main." + fn)
	if err != nil {
		return reflect.Value{}, fmt.Errorf("%s: no func %s", file, fn)
	}
	return f, nil
}

// stripComments drops leading // comment lines, to see whether a file starts
// with its package clause.
func stripComments(s string) string {
	for {
		t := strings.TrimSpace(s)
		if !strings.HasPrefix(t, "//") {
			return t
		}
		_, rest, ok := strings.Cut(t, "\n")
		if !ok {
			return ""
		}
		s = rest
	}
}

// run calls f, turning a panic (a failed Call, a runtime error in the code)
// into the event's error.
func run(file string, f reflect.Value, args ...reflect.Value) (out []reflect.Value, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%s: %v", file, r)
		}
	}()
	return f.Call(args), nil
}

// Compile turns one event's Go into a handler.
func Compile(g *runner.Game, file, src string) (runner.CollisionHandler, error) {
	f, err := load(file, src, "Event", "func Event(g *gv.Ctx) { ", "\n}")
	if err != nil {
		return nil, err
	}
	return func(r *runner.Room, self, other *runner.Instance) {
		g.Enter(r, self, other, func(in *gml.Interp) error {
			_, err := run(file, f, reflect.ValueOf(&Ctx{in: in}))
			return err
		})
	}, nil
}

// CompileScript turns a script's Go into a function called as a GML script
// is: as the calling instance, its arguments as args, its result returned
// (none: 0).
func CompileScript(g *runner.Game, file, src string) (gml.Func, error) {
	f, err := load(file, src, "Script", "func Script(g *gv.Ctx, args []gv.Value) gv.Value { ", "\nreturn gv.Value{}\n}")
	if err != nil {
		return nil, err
	}
	return func(in *gml.Interp, args []gml.Value) (gml.Value, error) {
		vals := make([]Value, len(args))
		for i, a := range args {
			vals[i] = Value{a}
		}
		out, err := run(file, f, reflect.ValueOf(&Ctx{in: in}), reflect.ValueOf(vals))
		if err != nil {
			return gml.Value{}, err
		}
		return out[0].Interface().(Value).v, nil
	}, nil
}

// Check is a Go event or script file's syntax and types: nil, or the first
// error with the file's line.
func Check(file, src string, script bool) error {
	if script {
		_, err := load(file, src, "Script", "func Script(g *gv.Ctx, args []gv.Value) gv.Value { ", "\nreturn gv.Value{}\n}")
		return err
	}
	_, err := load(file, src, "Event", "func Event(g *gv.Ctx) { ", "\n}")
	return err
}
