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
	"github.com/LaPingvino/gameviber/gv"
	"github.com/LaPingvino/gameviber/runner"
)

func init() {
	runner.Langs[".go"] = Compile
	runner.ScriptLangs[".go"] = CompileScript
}

// exports are the packages the code may import.
var exports = interp.Exports{
	"gv/gv": {
		"Ctx":   reflect.ValueOf((*gv.Ctx)(nil)),
		"Value": reflect.ValueOf((*gv.Value)(nil)),
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

// Compiled are event files compiled into this program (an exported desktop
// game registers them, by the file's path in the game folder): they run as
// native Go instead of through yaegi. Scripts are CompiledScripts.
var (
	Compiled        = map[string]func(g *gv.Ctx){}
	CompiledScripts = map[string]func(g *gv.Ctx, args []gv.Value) gv.Value{}
)

// call runs compiled code, a panic becoming the event's error (as run does).
func call(file string, f func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%s: %v", file, r)
		}
	}()
	f()
	return nil
}

// Compile turns one event's Go into a handler: its compiled stand-in when the
// program has one, else interpreted.
func Compile(g *runner.Game, file, src string) (runner.CollisionHandler, error) {
	if c, ok := Compiled[file]; ok {
		return func(r *runner.Room, self, other *runner.Instance) {
			g.Enter(r, self, other, func(in *gml.Interp) error {
				return call(file, func() { c(gv.New(in)) })
			})
		}, nil
	}
	f, err := load(file, src, "Event", "func Event(g *gv.Ctx) { ", "\n}")
	if err != nil {
		return nil, err
	}
	return func(r *runner.Room, self, other *runner.Instance) {
		g.Enter(r, self, other, func(in *gml.Interp) error {
			_, err := run(file, f, reflect.ValueOf(gv.New(in)))
			return err
		})
	}, nil
}

// CompileScript turns a script's Go into a function called as a GML script
// is: as the calling instance, its arguments as args, its result returned
// (none: 0).
func CompileScript(g *runner.Game, file, src string) (gml.Func, error) {
	if c, ok := CompiledScripts[file]; ok {
		return func(in *gml.Interp, args []gml.Value) (out gml.Value, err error) {
			vals := make([]gv.Value, len(args))
			for i, a := range args {
				vals[i] = gv.Wrap(a)
			}
			err = call(file, func() { out = c(gv.New(in), vals).GML() })
			return out, err
		}, nil
	}
	f, err := load(file, src, "Script", "func Script(g *gv.Ctx, args []gv.Value) gv.Value { ", "\nreturn gv.Value{}\n}")
	if err != nil {
		return nil, err
	}
	return func(in *gml.Interp, args []gml.Value) (gml.Value, error) {
		vals := make([]gv.Value, len(args))
		for i, a := range args {
			vals[i] = gv.Wrap(a)
		}
		out, err := run(file, f, reflect.ValueOf(gv.New(in)), reflect.ValueOf(vals))
		if err != nil {
			return gml.Value{}, err
		}
		return out[0].Interface().(gv.Value).GML(), nil
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
