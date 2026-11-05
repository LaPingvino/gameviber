package gml

import "fmt"

// The calls below let code in another language (runner.Langs: JavaScript, …)
// see names exactly as GML code running in the same place would.

// Var is a plain name's value as GML would read it here: a global, self's
// variable, a constant, then Lookup (objects, sprites, rooms …).
func (in *Interp) Var(name string) (Value, bool) {
	v, err := in.lookup(&Name{Name: name}, nil)
	return v, err == nil
}

// SetVar assigns a plain name as GML would: a global if it is one, else
// self's variable (created if new).
func (in *Interp) SetVar(name string, v Value) {
	if in.globals[name] || GlobalBuiltins[name] {
		in.Global.Set(name, nil, v)
		return
	}
	in.Self.Set(name, nil, v)
}

// Func is the function GML code would call by that name (the game's own
// table, then the language's built-ins); nil if there is none.
func (in *Interp) Func(name string) Func {
	if f := in.Funcs[name]; f != nil {
		return f
	}
	return Builtins[name]
}

// Call calls the named function as GML would.
func (in *Interp) Call(name string, args []Value) (Value, error) {
	f := in.Func(name)
	if f == nil {
		return Value{}, fmt.Errorf("unknown function %s", name)
	}
	return f(in, args)
}

// InstancesOf is the instances an id stands for, as in GML's "id.x": one
// instance, every instance of an object, self, other, all.
func (in *Interp) InstancesOf(id float64) []Instance { return in.instancesOf(id) }
