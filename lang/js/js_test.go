package js_test

import (
	"strings"
	"testing"

	_ "github.com/LaPingvino/gameviber/lang/js"
	"github.com/LaPingvino/gameviber/runner"
)

func room() *runner.Room {
	return &runner.Room{W: 64, H: 64, G: &runner.Globals{Vars: map[string]float64{}}}
}

// A JavaScript create event reads and writes the instance, makes new
// instance variables, keeps its own locals, calls GameViber's functions and
// sees JavaScript's own globals.
func TestEvent(t *testing.T) {
	o := &runner.Object{Name: "player", Depth: 5}
	g := &runner.Game{Objects: []*runner.Object{o}}
	h, err := g.CodeIn("objects/player/create.js", `
		let step = 4;            // a local
		x += step;               // the instance's x
		this.y = 10;
		was = depth;             // a new instance variable, as in GML
		big = Math.max(3, 7);    // JavaScript's own Math
		r = abs(-2.5);           // a GameViber function
		s = string(12) + "!";    // strings stay strings
		yes = place_free(x, y);  // booleans … GML numbers
		for (var i = 0; i < 3; i++) count = (typeof count === "number" ? count : 0) + 1;
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
		got, ok := in.Get(name, nil)
		if !ok || got.N != want {
			t.Errorf("%s = %v (%v), want %v", name, got, ok, want)
		}
	}
	if s, _ := in.Get("s", nil); s.S != "12!" {
		t.Errorf("s = %v", s)
	}
	if _, ok := in.Vars["i"]; ok {
		t.Error("the local i became an instance variable")
	}
}

// other is the instance hit in a collision event.
func TestCollisionOther(t *testing.T) {
	a := &runner.Object{Name: "a"}
	b := &runner.Object{Name: "b"}
	g := &runner.Game{Objects: []*runner.Object{a, b}}
	h, err := g.CollisionCodeIn("objects/a/collision-b.js", `hit = other.x; other.hp = 3;`)
	if err != nil {
		t.Fatal(err)
	}
	r := room()
	ia, ib := r.Create(a, 1, 1), r.Create(b, 20, 1)
	h(r, ia, ib)
	if len(g.Errors) > 0 {
		t.Fatal(g.Errors)
	}
	if ia.Vars["hit"] != 20 || ib.Vars["hp"] != 3 {
		t.Errorf("hit %v, hp %v", ia.Vars["hit"], ib.Vars["hp"])
	}
}

// Errors carry the file and the line: a syntax error when compiling, a
// runtime one (an unknown name, a failing function) in the game's errors.
func TestErrors(t *testing.T) {
	o := &runner.Object{Name: "p"}
	g := &runner.Game{Objects: []*runner.Object{o}}
	if _, err := g.CodeIn("objects/p/step.js", "x = 1;\nif (x {"); err == nil || !strings.Contains(err.Error(), "objects/p/step.js:2") {
		t.Errorf("syntax error: %v", err)
	}
	h, err := g.CodeIn("objects/p/step.js", "x = 1;\n\nx = nosuchname + 1;")
	if err != nil {
		t.Fatal(err)
	}
	o.Events.Create = h
	room().Create(o, 0, 0)
	if len(g.Errors) != 1 || !strings.Contains(g.Errors[0].Error(), "nosuchname") || !strings.Contains(g.Errors[0].Error(), "step.js:3") {
		t.Errorf("runtime error: %v", g.Errors)
	}
	if _, err := g.CodeIn("objects/p/step.rb", "x = 1"); err == nil {
		t.Error("no error for an unknown language")
	}
}

// A GameViber function called on an instance runs as that instance (GML's
// with (other)): other.instance_destroy() and instance_destroy.call(other)
// destroy the other one, not the caller.
func TestCallOnInstance(t *testing.T) {
	for _, src := range []string{"other.instance_destroy();", "instance_destroy.call(other);"} {
		a := &runner.Object{Name: "a"}
		b := &runner.Object{Name: "b"}
		g := &runner.Game{Objects: []*runner.Object{a, b}}
		h, err := g.CollisionCodeIn("objects/a/collision-b.js", src+" after = x;")
		if err != nil {
			t.Fatal(err)
		}
		r := room()
		ia, ib := r.Create(a, 3, 1), r.Create(b, 20, 1)
		h(r, ia, ib)
		if len(g.Errors) > 0 {
			t.Fatal(g.Errors)
		}
		if ia.Destroyed || !ib.Destroyed || ia.Vars["after"] != 3 {
			t.Errorf("%s: a destroyed %v, b destroyed %v, self afterwards x %v", src, ia.Destroyed, ib.Destroyed, ia.Vars["after"])
		}
	}
}
