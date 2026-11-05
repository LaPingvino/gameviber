package lua_test

import (
	"strings"
	"testing"

	_ "github.com/LaPingvino/gameviber/lang/lua"
	"github.com/LaPingvino/gameviber/runner"
)

func room() *runner.Room {
	return &runner.Room{W: 64, H: 64, G: &runner.Globals{Vars: map[string]float64{}}}
}

// A Lua create event reads and writes the instance, makes instance variables,
// keeps its locals, calls GameViber's functions and sees Lua's own globals.
func TestEvent(t *testing.T) {
	o := &runner.Object{Name: "player", Depth: 5}
	g := &runner.Game{Objects: []*runner.Object{o}}
	h, err := g.CodeIn("objects/player/create.lua", `
		local step = 4          -- a local
		x = x + step            -- the instance's x
		self.y = 10
		was = depth             -- a new instance variable, as in GML
		big = math.max(3, 7)    -- Lua's own math
		r = abs(-2.5)           -- a GameViber function
		s = tostring(12) .. "!" -- strings stay strings (Lua's string library comes first)
		for i = 1, 3 do count = (count or 0) + 1 end
	`)
	if err != nil {
		t.Fatal(err)
	}
	o.Events.Create = h
	in := room().Create(o, 0, 0)
	if len(g.Errors) > 0 {
		t.Fatal(g.Errors)
	}
	for name, want := range map[string]float64{"x": 4, "y": 10, "was": 5, "big": 7, "r": 2.5, "count": 3} {
		if got, ok := in.Get(name, nil); !ok || got.N != want {
			t.Errorf("%s = %v (%v), want %v", name, got, ok, want)
		}
	}
	if s, _ := in.Get("s", nil); s.S != "12!" {
		t.Errorf("s = %v", s)
	}
	for _, local := range []string{"step", "i"} {
		if _, ok := in.Vars[local]; ok {
			t.Errorf("the local %s became an instance variable", local)
		}
	}
}

// GameViber's truth values are Lua booleans: 0 is true in Lua, so a number
// would take every if.
func TestTruth(t *testing.T) {
	o := &runner.Object{Name: "p"}
	g := &runner.Game{Objects: []*runner.Object{o}}
	h, err := g.CodeIn("objects/p/create.lua", `
		if place_free(x, y) then a = 1 else a = 2 end
		if x > 100 then b = 1 else b = 2 end
		if instance_exists(12345) then c = 1 else c = 2 end
	`)
	if err != nil {
		t.Fatal(err)
	}
	o.Events.Create = h
	in := room().Create(o, 0, 0)
	if len(g.Errors) > 0 {
		t.Fatal(g.Errors)
	}
	if in.Vars["a"] != 1 || in.Vars["b"] != 2 || in.Vars["c"] != 2 {
		t.Errorf("a %v (free: 1), b %v (x > 100 false: 2), c %v (no instance: 2)", in.Vars["a"], in.Vars["b"], in.Vars["c"])
	}
}

// other in a collision; other:instance_destroy() runs as other (GML's with).
func TestCollisionOther(t *testing.T) {
	a := &runner.Object{Name: "a"}
	b := &runner.Object{Name: "b"}
	g := &runner.Game{Objects: []*runner.Object{a, b}}
	h, err := g.CollisionCodeIn("objects/a/collision-b.lua", "hit = other.x\nother.hp = 3\nother:instance_destroy()\nafter = x")
	if err != nil {
		t.Fatal(err)
	}
	r := room()
	ia, ib := r.Create(a, 1, 1), r.Create(b, 20, 1)
	h(r, ia, ib)
	if len(g.Errors) > 0 {
		t.Fatal(g.Errors)
	}
	if ia.Vars["hit"] != 20 || ib.Vars["hp"] != 3 || !ib.Destroyed || ia.Destroyed || ia.Vars["after"] != 1 {
		t.Errorf("hit %v, hp %v, b destroyed %v, a destroyed %v, after %v", ia.Vars["hit"], ib.Vars["hp"], ib.Destroyed, ia.Destroyed, ia.Vars["after"])
	}
}

// A Lua script called from GML and from Lua, with its arguments and a value.
func TestScript(t *testing.T) {
	o := &runner.Object{Name: "p"}
	g := &runner.Game{Objects: []*runner.Object{o}}
	if err := g.ScriptIn("scripts/half.lua", "half", "return argument0 / 2 + select('#', ...) - 1"); err != nil {
		t.Fatal(err)
	}
	h, err := g.CodeIn("objects/p/create.gml", "fromgml = half(10)")
	if err != nil {
		t.Fatal(err)
	}
	o.Events.Create = h
	in := room().Create(o, 0, 0)
	if len(g.Errors) > 0 {
		t.Fatal(g.Errors)
	}
	if in.Vars["fromgml"] != 5 {
		t.Errorf("half(10) from GML = %v", in.Vars["fromgml"])
	}
}

// Errors carry the file and the line.
func TestErrors(t *testing.T) {
	o := &runner.Object{Name: "p"}
	g := &runner.Game{Objects: []*runner.Object{o}}
	if _, err := g.CodeIn("objects/p/step.lua", "x = 1\nif x then"); err == nil || !strings.Contains(err.Error(), "objects/p/step.lua") {
		t.Errorf("syntax error: %v", err)
	}
	h, err := g.CodeIn("objects/p/step.lua", "x = 1\n\nx = nosuchfunction(1)")
	if err != nil {
		t.Fatal(err)
	}
	o.Events.Create = h
	room().Create(o, 0, 0)
	if len(g.Errors) != 1 || !strings.Contains(g.Errors[0].Error(), "step.lua:3") {
		t.Errorf("runtime error: %v", g.Errors)
	}
}
