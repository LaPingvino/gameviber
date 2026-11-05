// Package lua lets a game's events and scripts be written in Lua (5.1, run by
// gopher-lua — pure Go, so the browser build keeps it). Importing it
// registers the ".lua" extension in runner.Langs and runner.ScriptLangs.
//
// The code sees GameViber as GML code in the same place does: GameViber's
// functions under their GML names, the instance's variables as plain names
// (x = x + 4), constants and resource names as values, `other` in collision
// events, `self`, and `global` for globals. Lua's own globals (math, string,
// table …) come first. Assigning a name not declared local makes an instance
// variable, as in GML. A truth value from GameViber (place_free, a
// comparison's result) is a Lua boolean, since 0 is true in Lua. Called as a
// method on an instance — other:instance_destroy() — a GameViber function runs
// as that instance, as GML's with (other) does. A script reads its arguments
// as argument0 … argument15 (or ...) and may return a value.
package lua

import (
	"fmt"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/LaPingvino/gameviber/gml"
	"github.com/LaPingvino/gameviber/runner"
)

func init() {
	runner.Langs[".lua"] = Compile
	runner.ScriptLangs[".lua"] = CompileScript
}

// vm is one game's Lua state: every event of that game runs in it, one at a
// time (the game is single-threaded).
type vm struct {
	L        *lua.LState
	env      *lua.LTable // the environment event code runs in
	instMeta *lua.LTable // instances' metatable
	in       *gml.Interp // the running event's context (nil between events)
}

var vms = map[*runner.Game]*vm{}

func vmFor(g *runner.Game) *vm {
	if v := vms[g]; v != nil {
		return v
	}
	L := lua.NewState()
	v := &vm{L: L, env: L.NewTable(), instMeta: L.NewTable()}
	meta := L.NewTable()
	L.SetField(meta, "__index", L.NewFunction(v.envIndex))
	L.SetField(meta, "__newindex", L.NewFunction(v.envNewIndex))
	L.SetMetatable(v.env, meta)
	L.SetField(v.instMeta, "__index", L.NewFunction(v.instIndex))
	L.SetField(v.instMeta, "__newindex", L.NewFunction(v.instNewIndex))
	vms[g] = v
	return v
}

// load compiles src as a chunk named file, running in the game's environment.
func (v *vm) load(file, src string) (*lua.LFunction, error) {
	fn, err := v.L.Load(strings.NewReader(src), file)
	if err != nil {
		return nil, err
	}
	v.L.SetFEnv(fn, v.env)
	return fn, nil
}

// call runs fn in an event's context and returns its first result.
func (v *vm) call(in *gml.Interp, fn *lua.LFunction, args ...lua.LValue) (lua.LValue, error) {
	outer := v.in
	v.in = in
	defer func() { v.in = outer }()
	if err := v.L.CallByParam(lua.P{Fn: fn, NRet: 1, Protect: true}, args...); err != nil {
		return lua.LNil, err
	}
	ret := v.L.Get(-1)
	v.L.Pop(1)
	return ret, nil
}

// Compile turns one event's Lua into a handler. file names it in errors
// ("objects/player/step.lua:3: …").
func Compile(g *runner.Game, file, src string) (runner.CollisionHandler, error) {
	v := vmFor(g)
	fn, err := v.load(file, src)
	if err != nil {
		return nil, err
	}
	return func(r *runner.Room, self, other *runner.Instance) {
		g.Enter(r, self, other, func(in *gml.Interp) error {
			_, err := v.call(in, fn)
			return err
		})
	}, nil
}

// scriptHead declares a script's arguments as GML names it; it stays on the
// script's first line, so error lines are the file's.
const scriptHead = "local argument0, argument1, argument2, argument3, argument4, argument5, argument6, argument7, " +
	"argument8, argument9, argument10, argument11, argument12, argument13, argument14, argument15 = ... "

// CompileScript turns a script's Lua into a function called as a GML script
// is: as the calling code's instance, with its arguments; what it returns is
// the call's value (nothing: 0).
func CompileScript(g *runner.Game, file, src string) (gml.Func, error) {
	v := vmFor(g)
	fn, err := v.load(file, scriptHead+src)
	if err != nil {
		return nil, err
	}
	return func(in *gml.Interp, args []gml.Value) (gml.Value, error) {
		la := make([]lua.LValue, len(args))
		for i, a := range args {
			la[i] = v.value(a)
		}
		ret, err := v.call(in, fn, la...)
		if err != nil {
			return gml.Value{}, err
		}
		return v.gml(ret), nil
	}, nil
}

// Check is a Lua event or script file's syntax: nil, or the first error with
// the file's line.
func Check(file, src string) error {
	L := lua.NewState()
	defer L.Close()
	_, err := L.Load(strings.NewReader(src), file)
	return err
}

// envIndex resolves a name event code reads: Lua's own globals, then self,
// other, global, GameViber's functions, and what GML would read (a global,
// self's variable, a constant, a resource).
func (v *vm) envIndex(L *lua.LState) int {
	key := L.CheckString(2)
	if g := L.GetGlobal(key); g != lua.LNil {
		L.Push(g)
		return 1
	}
	in := v.in
	if in == nil {
		L.Push(lua.LNil)
		return 1
	}
	switch key {
	case "self":
		L.Push(v.instance(in.Self))
		return 1
	case "other":
		L.Push(v.instance(in.Other))
		return 1
	case "global":
		L.Push(v.instance(in.Global))
		return 1
	}
	if in.Func(key) != nil {
		L.Push(v.function(key, nil))
		return 1
	}
	if x, ok := in.Var(key); ok {
		L.Push(v.value(x))
		return 1
	}
	L.Push(lua.LNil)
	return 1
}

// envNewIndex assigns a name as GML would: a global, else self's variable.
func (v *vm) envNewIndex(L *lua.LState) int {
	key := L.CheckString(2)
	if v.in == nil {
		L.RawSet(L.CheckTable(1), lua.LString(key), L.Get(3))
		return 0
	}
	v.in.SetVar(key, v.gml(L.Get(3)))
	return 0
}

// inst is an instance (or the globals) as Lua userdata.
type inst struct{ i gml.Instance }

func (v *vm) instance(i gml.Instance) lua.LValue {
	if i == nil {
		return lua.LNil
	}
	ud := v.L.NewUserData()
	ud.Value = &inst{i}
	v.L.SetMetatable(ud, v.instMeta)
	return ud
}

func (v *vm) instIndex(L *lua.LState) int {
	o := L.CheckUserData(1).Value.(*inst)
	key := L.CheckString(2)
	if x, ok := o.i.Get(key, nil); ok {
		L.Push(v.value(x))
		return 1
	}
	if v.in != nil && v.in.Func(key) != nil { // other:instance_destroy(): runs as that instance
		L.Push(v.function(key, o.i))
		return 1
	}
	L.Push(lua.LNil)
	return 1
}

func (v *vm) instNewIndex(L *lua.LState) int {
	o := L.CheckUserData(1).Value.(*inst)
	o.i.Set(L.CheckString(2), nil, v.gml(L.Get(3)))
	return 0
}

// function is GameViber's function name in Lua. Bound to an instance (got
// through it: other.f, other:f), it runs as that instance with the caller as
// other — GML's with — and drops the instance itself when the call passes it
// first (the method form other:f()).
func (v *vm) function(name string, target gml.Instance) *lua.LFunction {
	return v.L.NewFunction(func(L *lua.LState) int {
		first := 1
		if target != nil && L.GetTop() >= 1 {
			if ud, ok := L.Get(1).(*lua.LUserData); ok {
				if o, ok := ud.Value.(*inst); ok && o.i == target {
					first = 2
				}
			}
		}
		args := make([]gml.Value, 0, L.GetTop())
		for i := first; i <= L.GetTop(); i++ {
			args = append(args, v.gml(L.Get(i)))
		}
		in := v.in
		if target != nil && target != in.Self {
			self, otherWas := in.Self, in.Other
			in.Self, in.Other = target, self
			defer func() { in.Self, in.Other = self, otherWas }()
		}
		res, err := in.Call(name, args)
		if err != nil {
			L.RaiseError("%s: %v", name, err)
			return 0
		}
		L.Push(v.value(res))
		return 1
	})
}

// value is a GML value in Lua: a string, a boolean for a truth value, else a
// number.
func (v *vm) value(x gml.Value) lua.LValue {
	switch {
	case x.IsStr:
		return lua.LString(x.S)
	case x.Bool:
		return lua.LBool(x.N != 0)
	}
	return lua.LNumber(x.N)
}

// gml is a Lua value in GML: strings stay strings, booleans are 1 / 0 (marked),
// an instance is its id, nil is 0, a number itself.
func (v *vm) gml(x lua.LValue) gml.Value {
	switch x := x.(type) {
	case lua.LString:
		return gml.StrV(string(x))
	case lua.LNumber:
		return gml.RealV(float64(x))
	case lua.LBool:
		return gml.BoolV(bool(x))
	case *lua.LUserData:
		if o, ok := x.Value.(*inst); ok {
			if id, ok := o.i.Get("id", nil); ok {
				return id
			}
		}
	}
	if x == lua.LNil {
		return gml.RealV(0)
	}
	return gml.StrV(fmt.Sprint(x))
}
