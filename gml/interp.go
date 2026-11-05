package gml

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"strconv"
	"strings"
)

// Value is a GML value: a real or a string (GM 5 has no other types).
type Value struct {
	S     string
	N     float64
	IsStr bool
}

// RealV and StrV make values.
func RealV(n float64) Value { return Value{N: n} }
func StrV(s string) Value   { return Value{S: s, IsStr: true} }

// True is GML's truth test: a real above 0.5.
// ASSUMPTION [engine]: GM 5 takes > 0.5 as true (strings are false).
func (v Value) True() bool { return !v.IsStr && v.N > 0.5 }

func (v Value) String() string {
	if v.IsStr {
		return v.S
	}
	return strconv.FormatFloat(v.N, 'f', -1, 64)
}

func boolVal(b bool) Value {
	if b {
		return RealV(1)
	}
	return RealV(0)
}

// Instance is what the runner gives the interpreter for an instance: its
// variables (arrays as name plus one or two indices).
type Instance interface {
	Get(name string, idx []int) (Value, bool)
	Set(name string, idx []int, v Value)
}

// Special instance values (GM 5's constants).
const (
	SelfID   = -1
	OtherID  = -2
	AllID    = -3
	NooneID  = -4
	GlobalID = -5
)

// Func is a built-in or script: called with the interpreter (for self,
// other, …) and its arguments.
type Func func(in *Interp, args []Value) (Value, error)

// Interp runs GML for one instance at a time. The runner fills the hooks.
type Interp struct {
	Self, Other Instance
	Global      Instance
	Funcs       map[string]Func
	// Lookup resolves other names (objects, sprites, sounds … to their ids;
	// built-in constants are handled here).
	Lookup func(name string) (Value, bool)
	// Instances turns an instance or object id (or AllID) into the instances
	// it means (for with and x.y).
	Instances func(id float64) []Instance

	// Seed seeds random() (the same seed replays the same run).
	Seed int64

	rng      *rand.Rand
	locals   map[string]Value
	globals  map[string]bool // globalvar names
	returned Value
}

type control int

const (
	ctlNone control = iota
	ctlBreak
	ctlContinue
	ctlExit
)

// RuntimeError is an error while running: where, and what went wrong.
type RuntimeError struct {
	Pos Pos
	Msg string
}

func (e *RuntimeError) Error() string { return e.Pos.String() + ": " + e.Msg }

func rerr(n Node, format string, args ...any) error {
	return &RuntimeError{n.At(), fmt.Sprintf(format, args...)}
}

// Run runs a piece of code (an event or a script) as Self; it returns the
// value of 'return' (0 when none).
func (in *Interp) Run(list []Stmt) (Value, error) { return in.RunArgs(list, nil) }

// RunArgs runs a script with arguments: argument0 … argument9 (and
// argument[i]) are its locals, as in GM 5.
func (in *Interp) RunArgs(list []Stmt, args []Value) (Value, error) {
	saved, savedRet := in.locals, in.returned
	defer func() { in.returned = savedRet }()
	in.locals = map[string]Value{}
	for i := range 10 {
		v := RealV(0)
		if i < len(args) {
			v = args[i]
		}
		in.locals["argument"+strconv.Itoa(i)] = v
	}
	in.returned = RealV(0)
	defer func() { in.locals = saved }()
	_, err := in.block(list)
	return in.returned, err
}

func (in *Interp) block(list []Stmt) (control, error) {
	for _, s := range list {
		c, err := in.stmt(s)
		if err != nil || c != ctlNone {
			return c, err
		}
	}
	return ctlNone, nil
}

// maxLoop guards against runaway loops in the interpreter's own tests and
// tools (a game that loops forever would hang in Game Maker too).
var maxLoop = 10_000_000

var errLoop = errors.New("loop ran more than 10,000,000 times")

func (in *Interp) stmt(s Stmt) (control, error) {
	switch s := s.(type) {
	case *Block:
		return in.block(s.List)
	case *Var:
		for _, n := range s.Names {
			if s.Global {
				if in.globals == nil {
					in.globals = map[string]bool{}
				}
				in.globals[n] = true
			} else {
				in.locals[n] = RealV(0)
			}
		}
	case *Assign:
		return ctlNone, in.assign(s)
	case *ExprStmt:
		_, err := in.eval(s.X)
		return ctlNone, err
	case *If:
		c, err := in.eval(s.Cond)
		if err != nil {
			return ctlNone, err
		}
		if c.True() {
			return in.stmt(s.Then)
		} else if s.Else != nil {
			return in.stmt(s.Else)
		}
	case *While:
		for n := 0; ; n++ {
			if n > maxLoop {
				return ctlNone, rerr(s, "%v", errLoop)
			}
			c, err := in.eval(s.Cond)
			if err != nil || !c.True() {
				return ctlNone, err
			}
			if ctl, err := in.stmt(s.Body); err != nil || ctl == ctlBreak || ctl == ctlExit {
				return exitOnly(ctl), err
			}
		}
	case *Do:
		for n := 0; ; n++ {
			if n > maxLoop {
				return ctlNone, rerr(s, "%v", errLoop)
			}
			if ctl, err := in.stmt(s.Body); err != nil || ctl == ctlBreak || ctl == ctlExit {
				return exitOnly(ctl), err
			}
			c, err := in.eval(s.Until)
			if err != nil || c.True() {
				return ctlNone, err
			}
		}
	case *Repeat:
		c, err := in.eval(s.Count)
		if err != nil {
			return ctlNone, err
		}
		for i := 0; i < int(math.Round(c.N)); i++ {
			if ctl, err := in.stmt(s.Body); err != nil || ctl == ctlBreak || ctl == ctlExit {
				return exitOnly(ctl), err
			}
		}
	case *For:
		if _, err := in.stmt(s.Init); err != nil {
			return ctlNone, err
		}
		for n := 0; ; n++ {
			if n > maxLoop {
				return ctlNone, rerr(s, "%v", errLoop)
			}
			c, err := in.eval(s.Cond)
			if err != nil || !c.True() {
				return ctlNone, err
			}
			if ctl, err := in.stmt(s.Body); err != nil || ctl == ctlBreak || ctl == ctlExit {
				return exitOnly(ctl), err
			}
			if _, err := in.stmt(s.Step); err != nil {
				return ctlNone, err
			}
		}
	case *With:
		t, err := in.eval(s.Target)
		if err != nil {
			return ctlNone, err
		}
		self, other := in.Self, in.Other
		defer func() { in.Self, in.Other = self, other }()
		for _, inst := range in.instancesOf(t.N) {
			in.Self, in.Other = inst, self
			ctl, err := in.stmt(s.Body)
			if err != nil || ctl == ctlExit {
				return ctl, err
			}
			if ctl == ctlBreak {
				break
			}
		}
	case *Switch:
		x, err := in.eval(s.X)
		if err != nil {
			return ctlNone, err
		}
		start := -1
		for i, c := range s.Cases {
			for _, v := range c.Values {
				cv, err := in.eval(v)
				if err != nil {
					return ctlNone, err
				}
				if equal(x, cv) && start < 0 {
					start = i
				}
			}
		}
		if start < 0 {
			for i, c := range s.Cases {
				if c.Values == nil {
					start = i
				}
			}
		}
		if start >= 0 {
			for _, c := range s.Cases[start:] { // falls through, as in C
				ctl, err := in.block(c.Body)
				if err != nil || ctl == ctlExit || ctl == ctlContinue {
					return ctl, err
				}
				if ctl == ctlBreak {
					break
				}
			}
		}
	case *Exit:
		return ctlExit, nil
	case *Break:
		return ctlBreak, nil
	case *Continue:
		return ctlContinue, nil
	case *Return:
		v, err := in.eval(s.X)
		in.returned = v
		return ctlExit, err
	}
	return ctlNone, nil
}

func exitOnly(c control) control {
	if c == ctlExit {
		return ctlExit
	}
	return ctlNone
}

func equal(a, b Value) bool {
	if a.IsStr != b.IsStr {
		return false
	}
	if a.IsStr {
		return a.S == b.S
	}
	return a.N == b.N
}

func (in *Interp) instancesOf(id float64) []Instance {
	switch int(id) {
	case SelfID:
		return []Instance{in.Self}
	case OtherID:
		if in.Other != nil {
			return []Instance{in.Other}
		}
		return nil
	case GlobalID:
		return []Instance{in.Global}
	case NooneID:
		return nil
	}
	if in.Instances == nil {
		return nil
	}
	return in.Instances(id)
}

func (in *Interp) indices(list []Expr) ([]int, error) {
	var idx []int
	for _, e := range list {
		v, err := in.eval(e)
		if err != nil {
			return nil, err
		}
		idx = append(idx, int(math.Round(v.N)))
	}
	return idx, nil
}

func (in *Interp) assign(s *Assign) error {
	v, err := in.eval(s.Value)
	if err != nil {
		return err
	}
	if s.Op != "=" && s.Op != ":=" {
		old, err := in.eval(s.Target)
		if err != nil {
			return err
		}
		if v, err = arith(s, strings.TrimSuffix(s.Op, "="), old, v); err != nil {
			return err
		}
	}
	target, name, idx, err := in.place(s.Target)
	if err != nil {
		return err
	}
	if target == nil { // a local or self
		if _, ok := in.locals[name]; ok && idx == nil {
			in.locals[name] = v
			return nil
		}
		if in.globals[name] || GlobalBuiltins[name] {
			in.Global.Set(name, idx, v)
			return nil
		}
		in.Self.Set(name, idx, v)
		return nil
	}
	for _, t := range target {
		t.Set(name, idx, v)
	}
	return nil
}

// place is where an assignment goes: the instances (nil: the plain name's
// scope), the variable and its indices.
func (in *Interp) place(e Expr) ([]Instance, string, []int, error) {
	switch e := e.(type) {
	case *Name:
		return nil, e.Name, nil, nil
	case *Field:
		x, err := in.eval(e.X)
		if err != nil {
			return nil, "", nil, err
		}
		return in.instancesOf(x.N), e.Name, nil, nil
	case *Index:
		ts, name, _, err := in.place(e.X)
		if err != nil {
			return nil, "", nil, err
		}
		idx, err := in.indices(e.I)
		return ts, name, idx, err
	}
	return nil, "", nil, rerr(e, "can't assign to %s", Format(e))
}

// GlobalBuiltins are GM 5's game-wide built-in variables: read and written
// through Interp.Global whoever runs the code.
var GlobalBuiltins = map[string]bool{
	"score": true, "instance_count": true, "instance_id": true, "lives": true, "health": true, "keyboard_string": true, "keyboard_key": true,
	"keyboard_lastkey": true, "keyboard_lastchar": true, "mouse_x": true, "mouse_y": true, "mouse_button": true,
	"room": true, "room_speed": true, "room_caption": true, "room_persistent": true, "view_left": true, "view_top": true, "view_width": true, "view_height": true, "view_visible": true, "view_enabled": true,
	"view_xview": true, "view_yview": true, "view_wview": true, "view_hview": true, "room_width": true, "room_height": true,
	"background_color": true, "brush_color": true, "pen_color": true, "font_color": true, "background_x": true, "background_y": true, "background_hspeed": true,
	"background_vspeed": true, "background_visible": true, "background_foreground": true, "background_index": true,
	"background_htiled": true, "background_vtiled": true, "show_score": true, "show_lives": true, "caption_score": true, "caption_lives": true,
}

// constants are GM 5's built-in constants (the rest come from Lookup).
var constants = map[string]float64{
	"true": 1, "false": 0, "self": SelfID, "other": OtherID, "all": AllID, "noone": NooneID, "global": GlobalID,
	"pi": math.Pi,
	// GameViber has no unregistered mode (GM 5's library actions check it).
	"gamemaker_registered": 1,
	// Mouse buttons and virtual keys (GM 5's constants; Windows key codes).
	"mb_any": -1, "mb_none": 0, "mb_left": 1, "mb_right": 2, "mb_middle": 3,
	"vk_nokey": 0, "vk_anykey": 1, "vk_backspace": 8, "vk_tab": 9, "vk_enter": 13, "vk_shift": 16, "vk_control": 17,
	"vk_alt": 18, "vk_pause": 19, "vk_escape": 27, "vk_space": 32, "vk_pageup": 33, "vk_pagedown": 34, "vk_end": 35,
	"vk_home": 36, "vk_left": 37, "vk_up": 38, "vk_right": 39, "vk_down": 40, "vk_printscreen": 44, "vk_insert": 45,
	"vk_delete": 46, "vk_numpad0": 96, "vk_numpad1": 97, "vk_numpad2": 98, "vk_numpad3": 99, "vk_numpad4": 100,
	"vk_numpad5": 101, "vk_numpad6": 102, "vk_numpad7": 103, "vk_numpad8": 104, "vk_numpad9": 105,
	"vk_multiply": 106, "vk_add": 107, "vk_subtract": 109, "vk_decimal": 110, "vk_divide": 111,
	"vk_f1": 112, "vk_f2": 113, "vk_f3": 114, "vk_f4": 115, "vk_f5": 116, "vk_f6": 117, "vk_f7": 118, "vk_f8": 119,
	"vk_f9": 120, "vk_f10": 121, "vk_f11": 122, "vk_f12": 123,
	// GM 5's colours, Windows order (0xBBGGRR).
	"c_aqua": 16776960, "c_black": 0, "c_blue": 16711680, "c_dkgray": 4210752, "c_fuchsia": 16711935,
	"c_gray": 8421504, "c_green": 32768, "c_lime": 65280, "c_ltgray": 12632256, "c_maroon": 128,
	"c_navy": 8388608, "c_olive": 32896, "c_purple": 8388736, "c_red": 255, "c_silver": 12632256,
	"c_teal": 8421376, "c_white": 16777215, "c_yellow": 65535,
}

func (in *Interp) eval(e Expr) (Value, error) {
	switch e := e.(type) {
	case *Num:
		if strings.HasPrefix(e.Text, "$") {
			n, err := strconv.ParseInt(e.Text[1:], 16, 64)
			if err != nil {
				return Value{}, rerr(e, "bad hex number %s", e.Text)
			}
			return RealV(float64(n)), nil
		}
		n, err := strconv.ParseFloat(e.Text, 64)
		if err != nil {
			return Value{}, rerr(e, "bad number %s", e.Text)
		}
		return RealV(n), nil
	case *Str:
		return StrV(e.Value), nil
	case *Name:
		return in.lookup(e, nil)
	case *Index:
		idx, err := in.indices(e.I)
		if err != nil {
			return Value{}, err
		}
		if n, ok := e.X.(*Name); ok {
			return in.lookup(n, idx)
		}
		if f, ok := e.X.(*Field); ok {
			return in.field(f, idx)
		}
		return Value{}, rerr(e, "can't index %s", Format(e.X))
	case *Field:
		return in.field(e, nil)
	case *Call:
		fn := in.Funcs[e.Fn]
		if fn == nil {
			fn = Builtins[e.Fn]
		}
		if fn == nil {
			return Value{}, rerr(e, "unknown function %s", e.Fn)
		}
		args := make([]Value, len(e.Args))
		for i, a := range e.Args {
			v, err := in.eval(a)
			if err != nil {
				return Value{}, err
			}
			args[i] = v
		}
		v, err := fn(in, args)
		if err != nil {
			var re *RuntimeError
			if !errors.As(err, &re) {
				err = rerr(e, "%s: %v", e.Fn, err)
			}
		}
		return v, err
	case *Unary:
		x, err := in.eval(e.X)
		if err != nil {
			return Value{}, err
		}
		if x.IsStr {
			return Value{}, rerr(e, "%s on a string", e.Op)
		}
		switch e.Op {
		case "!":
			return boolVal(!x.True()), nil
		case "-":
			return RealV(-x.N), nil
		case "~":
			return RealV(float64(^int64(x.N))), nil
		}
		return x, nil
	case *Binary:
		l, err := in.eval(e.L)
		if err != nil {
			return Value{}, err
		}
		switch e.Op { // GM 5 evaluates both sides; kept so side effects match
		case "&&", "||", "^^":
			r, err := in.eval(e.R)
			if err != nil {
				return Value{}, err
			}
			a, b := l.True(), r.True()
			switch e.Op {
			case "&&":
				return boolVal(a && b), nil
			case "||":
				return boolVal(a || b), nil
			}
			return boolVal(a != b), nil
		}
		r, err := in.eval(e.R)
		if err != nil {
			return Value{}, err
		}
		return arith(e, e.Op, l, r)
	}
	return Value{}, rerr(e, "can't evaluate this")
}

func arith(n Node, op string, l, r Value) (Value, error) {
	switch op {
	case "==":
		return boolVal(equal(l, r)), nil
	case "!=":
		return boolVal(!equal(l, r)), nil
	case "<", "<=", ">", ">=":
		var c int
		if l.IsStr && r.IsStr {
			c = strings.Compare(l.S, r.S)
		} else if !l.IsStr && !r.IsStr {
			switch {
			case l.N < r.N:
				c = -1
			case l.N > r.N:
				c = 1
			}
		} else {
			return Value{}, rerr(n, "comparing a string with a number")
		}
		return boolVal(op == "<" && c < 0 || op == "<=" && c <= 0 || op == ">" && c > 0 || op == ">=" && c >= 0), nil
	case "+":
		if l.IsStr && r.IsStr {
			return StrV(l.S + r.S), nil
		}
	}
	if l.IsStr || r.IsStr {
		return Value{}, rerr(n, "%q between %s and %s", op, kind(l), kind(r))
	}
	a, b := l.N, r.N
	switch op {
	case "+":
		return RealV(a + b), nil
	case "-":
		return RealV(a - b), nil
	case "*":
		return RealV(a * b), nil
	case "/":
		if b == 0 {
			return Value{}, rerr(n, "division by zero")
		}
		return RealV(a / b), nil
	case "div":
		if b == 0 {
			return Value{}, rerr(n, "division by zero")
		}
		return RealV(math.Trunc(a / b)), nil
	case "mod":
		if b == 0 {
			return Value{}, rerr(n, "division by zero")
		}
		return RealV(math.Mod(a, b)), nil
	case "&":
		return RealV(float64(int64(a) & int64(b))), nil
	case "|":
		return RealV(float64(int64(a) | int64(b))), nil
	case "^":
		return RealV(float64(int64(a) ^ int64(b))), nil
	case "<<":
		return RealV(float64(int64(a) << uint(b))), nil
	case ">>":
		return RealV(float64(int64(a) >> uint(b))), nil
	}
	return Value{}, rerr(n, "unknown operator %s", op)
}

func kind(v Value) string {
	if v.IsStr {
		return "a string"
	}
	return "a number"
}

func (in *Interp) lookup(n *Name, idx []int) (Value, error) {
	if v, ok := in.locals[n.Name]; ok && idx == nil {
		return v, nil
	}
	if n.Name == "argument" && len(idx) == 1 {
		if v, ok := in.locals["argument"+strconv.Itoa(idx[0])]; ok {
			return v, nil
		}
	}
	if in.globals[n.Name] || GlobalBuiltins[n.Name] {
		if v, ok := in.Global.Get(n.Name, idx); ok {
			return v, nil
		}
	}
	if v, ok := in.Self.Get(n.Name, idx); ok {
		return v, nil
	}
	if c, ok := constants[n.Name]; ok {
		return RealV(c), nil
	}
	if in.Lookup != nil {
		if v, ok := in.Lookup(n.Name); ok {
			return v, nil
		}
	}
	return Value{}, rerr(n, "unknown variable %s", n.Name)
}

func (in *Interp) field(f *Field, idx []int) (Value, error) {
	x, err := in.eval(f.X)
	if err != nil {
		return Value{}, err
	}
	ts := in.instancesOf(x.N)
	if len(ts) == 0 {
		return Value{}, rerr(f, "no instance for %s", Format(f.X))
	}
	if v, ok := ts[0].Get(f.Name, idx); ok {
		return v, nil
	}
	return Value{}, rerr(f, "unknown variable %s.%s", Format(f.X), f.Name)
}

// Constants lists the built-in constants (for the reference).
func Constants() map[string]float64 { return constants }
