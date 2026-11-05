// Package js lets a game's events be written in JavaScript (ES2015+, run by
// goja — pure Go, so the browser build keeps it). Importing it registers the
// ".js" extension in runner.Langs.
//
// The code sees GameViber as GML code in the same place does: GameViber's
// functions under their GML names (instance_create, draw_text, place_free …),
// the instance's variables as plain names (x += 4; vspeed = 2), constants and
// resource names as values (vk_left, c_red, obj_wall), `other` in collision
// events and `global` for globals. `this` is the instance too. Assigning a
// name that is not declared makes an instance variable, as in GML; locals are
// declared with let, const or var. JavaScript's own globals (Math, JSON …)
// come first.
package js

import (
	"fmt"

	"github.com/dop251/goja"

	"github.com/LaPingvino/gameviber/gml"
	"github.com/LaPingvino/gameviber/runner"
)

func init() { runner.Langs[".js"] = Compile }

// vm is one game's JavaScript runtime: every event of that game runs in it,
// one at a time (the game is single-threaded).
type vm struct {
	rt    *goja.Runtime
	orig  *goja.Object          // JavaScript's own global object
	in    *gml.Interp           // the running event's context (nil between events)
	funcs map[string]goja.Value // GameViber's functions, wrapped once
}

var vms = map[*runner.Game]*vm{}

func vmFor(g *runner.Game) *vm {
	if v := vms[g]; v != nil {
		return v
	}
	v := &vm{rt: goja.New(), funcs: map[string]goja.Value{}}
	v.orig = v.rt.GlobalObject()
	v.rt.SetGlobalObject(v.rt.NewDynamicObject(&scope{v}))
	vms[g] = v
	return v
}

// Compile turns one event's JavaScript into a handler. file names it in
// errors ("objects/player/step.js:3:5 …").
func Compile(g *runner.Game, file, src string) (runner.CollisionHandler, error) {
	// The event is a function's body, so its var / let / const are its own
	// and `this` can be the instance. The wrapper stays on line 1, so error
	// lines are the file's.
	prog, err := goja.Compile(file, "(function(){"+src+"\n})", false)
	if err != nil {
		return nil, err
	}
	v := vmFor(g)
	fv, err := v.rt.RunProgram(prog)
	if err != nil {
		return nil, err
	}
	fn, ok := goja.AssertFunction(fv)
	if !ok {
		return nil, fmt.Errorf("%s: not a function", file)
	}
	return func(r *runner.Room, self, other *runner.Instance) {
		g.Enter(r, self, other, func(in *gml.Interp) error {
			outer := v.in
			v.in = in
			defer func() { v.in = outer }()
			_, err := fn(v.instance(in.Self))
			return err
		})
	}, nil
}

// scope is the global object event code sees.
type scope struct{ v *vm }

func (s *scope) Get(key string) goja.Value {
	v := s.v
	if p := v.orig.Get(key); p != nil {
		return p
	}
	in := v.in
	if in == nil {
		return nil
	}
	switch key {
	case "self":
		return v.instance(in.Self)
	case "other":
		if in.Other != nil {
			return v.instance(in.Other)
		}
		return nil
	case "global":
		return v.instance(in.Global)
	}
	if f := in.Func(key); f != nil {
		return v.function(key)
	}
	if x, ok := in.Var(key); ok {
		return v.value(x)
	}
	return nil
}

func (s *scope) Set(key string, val goja.Value) bool {
	if s.v.in == nil {
		return s.v.orig.Set(key, val) == nil
	}
	s.v.in.SetVar(key, s.v.gml(val))
	return true
}

func (s *scope) Has(key string) bool { return s.Get(key) != nil }
func (s *scope) Delete(string) bool  { return false }
func (s *scope) Keys() []string      { return nil }

// function is GameViber's function name as a JavaScript function, calling it
// in the running event's context.
func (v *vm) function(name string) goja.Value {
	if f := v.funcs[name]; f != nil {
		return f
	}
	f := v.rt.ToValue(func(call goja.FunctionCall) goja.Value {
		args := make([]gml.Value, len(call.Arguments))
		for i, a := range call.Arguments {
			args[i] = v.gml(a)
		}
		res, err := v.in.Call(name, args)
		if err != nil {
			panic(v.rt.NewGoError(fmt.Errorf("%s: %w", name, err)))
		}
		return v.value(res)
	})
	v.funcs[name] = f
	return f
}

// instance is a GML instance (or the globals) as a JavaScript object whose
// properties are its variables.
func (v *vm) instance(i gml.Instance) goja.Value {
	if i == nil {
		return goja.Undefined()
	}
	return v.rt.NewDynamicObject(&inst{v, i})
}

type inst struct {
	v *vm
	i gml.Instance
}

func (o *inst) Get(key string) goja.Value {
	if x, ok := o.i.Get(key, nil); ok {
		return o.v.value(x)
	}
	return nil
}
func (o *inst) Set(key string, val goja.Value) bool { o.i.Set(key, nil, o.v.gml(val)); return true }
func (o *inst) Has(key string) bool {
	_, ok := o.i.Get(key, nil)
	return ok
}
func (o *inst) Delete(string) bool { return false }
func (o *inst) Keys() []string     { return nil }

// value is a GML value in JavaScript: a number or a string.
func (v *vm) value(x gml.Value) goja.Value {
	if x.IsStr {
		return v.rt.ToValue(x.S)
	}
	return v.rt.ToValue(x.N)
}

// gml is a JavaScript value in GML: strings stay strings, booleans are 1 / 0,
// an instance object is its id, nothing is 0, the rest a number.
func (v *vm) gml(x goja.Value) gml.Value {
	if x == nil || goja.IsUndefined(x) || goja.IsNull(x) {
		return gml.RealV(0)
	}
	if o, ok := x.(*goja.Object); ok {
		if in, ok := o.Export().(*inst); ok {
			if id, ok := in.i.Get("id", nil); ok {
				return id
			}
		}
	}
	switch e := x.Export().(type) {
	case string:
		return gml.StrV(e)
	case bool:
		if e {
			return gml.RealV(1)
		}
		return gml.RealV(0)
	}
	return gml.RealV(x.ToFloat())
}
