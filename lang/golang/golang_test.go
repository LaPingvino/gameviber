package golang_test

import (
	"strings"
	"testing"

	"github.com/LaPingvino/gameviber/gml"
	"github.com/LaPingvino/gameviber/gv"
	"github.com/LaPingvino/gameviber/lang/golang"
	"github.com/LaPingvino/gameviber/runner"
)

func room() *runner.Room {
	return &runner.Room{W: 64, H: 64, G: &runner.Globals{Vars: map[string]float64{}}}
}

// A Go create event: the instance's variables through g, GameViber's
// functions, Go's own locals, and math.
func TestEvent(t *testing.T) {
	o := &runner.Object{Name: "player", Depth: 5}
	g := &runner.Game{Objects: []*runner.Object{o}}
	h, err := g.CodeIn("objects/player/create.go", `
		step := 4.0
		g.Set("x", g.Num("x")+step)
		g.Set("was", g.Num("depth"))
		g.Set("r", g.Call("abs", -2.5))
		g.Set("s", g.Call("string", 12).Str()+"!")
		if g.Is("place_free", g.Num("x"), g.Num("y")) {
			g.Set("free", 1)
		}
		count := 0
		for i := 0; i < 3; i++ {
			count++
		}
		g.Set("count", count)
	`)
	if err != nil {
		t.Fatal(err)
	}
	o.Events.Create = h
	in := room().Create(o, 0, 0)
	if len(g.Errors) > 0 {
		t.Fatal(g.Errors)
	}
	for name, want := range map[string]float64{"x": 4, "was": 5, "r": 2.5, "free": 1, "count": 3} {
		if got, ok := in.Get(name, nil); !ok || got.N != want {
			t.Errorf("%s = %v (%v), want %v", name, got, ok, want)
		}
	}
	if s, _ := in.Get("s", nil); s.S != "12!" {
		t.Errorf("s = %v", s)
	}
}

// A whole file with its own imports.
func TestWholeFile(t *testing.T) {
	o := &runner.Object{Name: "p"}
	g := &runner.Game{Objects: []*runner.Object{o}}
	h, err := g.CodeIn("objects/p/create.go", "// a whole file\npackage main\n\nimport (\n\t\"gv\"\n\t\"math\"\n)\n\nfunc Event(g *gv.Ctx) {\n\tg.Set(\"h\", math.Hypot(3, 4))\n}\n")
	if err != nil {
		t.Fatal(err)
	}
	o.Events.Create = h
	in := room().Create(o, 0, 0)
	if len(g.Errors) > 0 || in.Vars["h"] != 5 {
		t.Errorf("h %v, errors %v", in.Vars["h"], g.Errors)
	}
}

// Other in a collision; a call through it runs as other (GML's with).
func TestCollisionOther(t *testing.T) {
	a := &runner.Object{Name: "a"}
	b := &runner.Object{Name: "b"}
	g := &runner.Game{Objects: []*runner.Object{a, b}}
	h, err := g.CollisionCodeIn("objects/a/collision-b.go", `
		o := g.Other()
		g.Set("hit", o.Num("x"))
		o.Set("hp", 3)
		o.Call("instance_destroy")
		g.Set("after", g.Num("x"))
	`)
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

// A Go script called from GML.
func TestScript(t *testing.T) {
	o := &runner.Object{Name: "p"}
	g := &runner.Game{Objects: []*runner.Object{o}}
	if err := g.ScriptIn("scripts/half.go", "half", "return gv.Value{}"); err != nil {
		t.Fatal(err)
	}
	if err := g.ScriptIn("scripts/twice.go", "twice", "return g.Call(\"real\", args[0].Num()*2)"); err != nil {
		t.Fatal(err)
	}
	h, err := g.CodeIn("objects/p/create.gml", "t = twice(21)")
	if err != nil {
		t.Fatal(err)
	}
	o.Events.Create = h
	in := room().Create(o, 0, 0)
	if len(g.Errors) > 0 || in.Vars["t"] != 42 {
		t.Errorf("twice(21) = %v, errors %v", in.Vars["t"], g.Errors)
	}
}

// Errors: a compile error names the file and line; a failing call stops the
// event with the file named.
func TestErrors(t *testing.T) {
	o := &runner.Object{Name: "p"}
	g := &runner.Game{Objects: []*runner.Object{o}}
	if _, err := g.CodeIn("objects/p/step.go", "x := 1\n\ny := nosuch"); err == nil || !strings.Contains(err.Error(), "objects/p/step.go:3") {
		t.Errorf("compile error: %v", err)
	}
	h, err := g.CodeIn("objects/p/step.go", `g.Call("no_such_function")`)
	if err != nil {
		t.Fatal(err)
	}
	o.Events.Create = h
	room().Create(o, 0, 0)
	if len(g.Errors) != 1 || !strings.Contains(g.Errors[0].Error(), "step.go") || !strings.Contains(g.Errors[0].Error(), "no_such_function") {
		t.Errorf("runtime error: %v", g.Errors)
	}
}

// A compiled stand-in: an event file the program has compiled in runs as
// native Go — its source is not even read — with the same gv API, and a panic
// in it is the event's error with the file's name.
func TestCompiledStandIn(t *testing.T) {
	golang.Compiled["objects/c/create.go"] = func(g *gv.Ctx) {
		g.Set("x", g.Num("x")+7)
		g.Set("r", g.Call("abs", -1.5))
	}
	golang.CompiledScripts["scripts/twice.go"] = func(g *gv.Ctx, args []gv.Value) gv.Value {
		return gv.Wrap(gml.RealV(args[0].Num() * 2))
	}
	golang.Compiled["objects/c/step.go"] = func(g *gv.Ctx) { panic("boom") }
	defer func() {
		delete(golang.Compiled, "objects/c/create.go")
		delete(golang.Compiled, "objects/c/step.go")
		delete(golang.CompiledScripts, "scripts/twice.go")
	}()
	o := &runner.Object{Name: "c"}
	g := &runner.Game{Objects: []*runner.Object{o}}
	if err := g.ScriptIn("scripts/twice.go", "twice", "this is not Go"); err != nil {
		t.Fatal(err)
	}
	h, err := g.CodeIn("objects/c/create.go", "neither is this")
	if err != nil {
		t.Fatal(err)
	}
	o.Events.Create = h
	rm := room()
	in := rm.Create(o, 0, 0)
	if len(g.Errors) > 0 {
		t.Fatal(g.Errors)
	}
	if x, _ := in.Get("x", nil); x.N != 7 {
		t.Errorf("x = %v", x)
	}
	if r, _ := in.Get("r", nil); r.N != 1.5 {
		t.Errorf("r = %v", r)
	}
	// the script, called from GML
	gh, err := g.CodeIn("objects/c/draw.gml", "t = twice(21)")
	if err != nil {
		t.Fatal(err)
	}
	gh(rm, in)
	if v, _ := in.Get("t", nil); v.N != 42 {
		t.Errorf("twice(21) = %v", v)
	}
	sh, _ := g.CodeIn("objects/c/step.go", "")
	sh(rm, in)
	if len(g.Errors) == 0 || !strings.Contains(g.Errors[0].Error(), "objects/c/step.go: boom") {
		t.Errorf("errors %q", g.Errors)
	}
}
