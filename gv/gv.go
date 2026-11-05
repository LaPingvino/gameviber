// Package gv is GameViber's game API for game code written in Go: the same
// package whether the code is interpreted (by yaegi, in the browser editor and
// the web player — lang/golang) or compiled into an exported desktop game.
//
// g.Num("x") and g.Str("name") read a name as GML would (the instance's
// variable, a global, a constant, a resource), g.Set("x", 4) assigns it as GML
// would, g.Call("place_free", x, y) calls a GameViber function (its result a
// Value: .Num(), .Str(), .Bool()), g.Is(…) is a call's truth, g.Other() is the
// other instance in a collision — calls made through it run as that instance,
// GML's with (other).
package gv

import (
	"fmt"

	"github.com/LaPingvino/gameviber/gml"
)

// New is the context of a running event (self).
func New(in *gml.Interp) *Ctx { return &Ctx{in: in} }

// Wrap is a GameViber value as a Value; GML is a Value's GameViber value.
func Wrap(v gml.Value) Value   { return Value{v} }
func (v Value) GML() gml.Value { return v.v }

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
		g.inst.Set(name, nil, ToGML(v))
		return
	}
	g.in.SetVar(name, ToGML(v))
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
		vals[i] = ToGML(a)
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

// ToGML is a Go value in GML.
func ToGML(v any) gml.Value {
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
