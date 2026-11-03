package gml

import (
	"fmt"
	"strings"
	"testing"
)

// vars is a map-backed Instance for tests.
type vars map[string]Value

func (v vars) key(name string, idx []int) string { return fmt.Sprint(name, idx) }
func (v vars) Get(name string, idx []int) (Value, bool) {
	x, ok := v[v.key(name, idx)]
	return x, ok
}
func (v vars) Set(name string, idx []int, x Value) { v[v.key(name, idx)] = x }

func run(t *testing.T, src string) (vars, vars, Value, error) {
	t.Helper()
	list, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	self, glob, other := vars{}, vars{}, vars{}
	other.Set("x", nil, RealV(7))
	var log []string
	in := &Interp{Self: self, Global: glob, Other: other,
		Funcs: map[string]Func{
			"show": func(in *Interp, a []Value) (Value, error) { log = append(log, a[0].String()); return RealV(0), nil },
			"max": func(in *Interp, a []Value) (Value, error) {
				m := a[0].N
				for _, v := range a[1:] {
					m = max(m, v.N)
				}
				return RealV(m), nil
			},
		},
		Lookup:    func(n string) (Value, bool) { return RealV(100), n == "wall" },
		Instances: func(id float64) []Instance { return []Instance{other} },
	}
	v, err := in.Run(list)
	self.Set("log", nil, StrV(strings.Join(log, ",")))
	return self, glob, v, err
}

func TestInterp(t *testing.T) {
	for _, c := range []struct{ src, name, want string }{
		{"a = 1 + 2 * 3", "a", "7"},
		{"a = 7 div 2 + 7 mod 2", "a", "4"},
		{"a = 'ab' + 'c'", "a", "abc"},
		{"a = 0; repeat 4 a += 2", "a", "8"},
		{"a = 0; for (i = 0; i < 10; i += 1) { if i = 5 break; a += i }", "a", "10"},
		{"a = 0; i = 0; while i < 5 { i += 1; if i mod 2 = 0 continue; a += i }", "a", "9"},
		{"a = 0; do a += 1 until a >= 3", "a", "3"},
		{"switch (2) { case 1: a = 1; break; case 2: a = 2; case 3: a = 3; break; default: a = 9 }", "a", "3"},
		{"switch (5) { case 1: a = 1; default: a = 9 }", "a", "9"},
		{"var t; t = 4; a = t * 2", "a", "8"},
		{"a = other.x + wall", "a", "107"},
		{"arr[2] = 5; arr[3] = 6; a = arr[2] + arr[3]", "a", "11"},
		{"a = not (1 > 2) and (3 >= 3)", "a", "1"},
		{"a = max(1, 9, 4); show(a); show('hi')", "log", "9,hi"},
		{"a = 1; exit; a = 2", "a", "1"},
		{"with (other) x = 3; a = other.x", "a", "3"},
	} {
		self, _, _, err := run(t, c.src)
		if err != nil {
			t.Errorf("%q: %v", c.src, err)
			continue
		}
		if v, _ := self.Get(c.name, nil); v.String() != c.want {
			t.Errorf("%q: %s = %s, want %s", c.src, c.name, v, c.want)
		}
	}
}

func TestInterpGlobalsAndReturn(t *testing.T) {
	self, glob, ret, err := run(t, "globalvar g; g = 5; global.h = 6; return g + global.h")
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := glob.Get("g", nil); v.N != 5 {
		t.Errorf("global g = %v", v)
	}
	if _, ok := self.Get("g", nil); ok {
		t.Error("g landed on self")
	}
	if ret.N != 11 {
		t.Errorf("return %v", ret)
	}
}

func TestInterpErrors(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{"a = b", "1:5: unknown variable b"},
		{"a = 1 / 0", "division by zero"},
		{"a = 'x' - 1", "between a string and a number"},
		{"nope(1)", "unknown function nope"},
	} {
		_, _, _, err := run(t, c.src)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q: %v, want …%s…", c.src, err, c.want)
		}
	}
}
