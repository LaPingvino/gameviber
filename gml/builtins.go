package gml

import (
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
	"strings"
	"unicode"
)

// Builtins are GM 5's functions that need no runner: math, strings, random,
// variable checks. The runner adds the rest (instances, rooms, drawing,
// sound) to Interp.Funcs.
//
// ASSUMPTION [engine]: GM 5 was written in Delphi, whose Round rounds halves
// to even; round() does the same here. String positions are 1-based.
var Builtins = map[string]Func{
	"abs":      math1(math.Abs),
	"sign":     math1(func(x float64) float64 { return float64(cmp(x, 0)) }),
	"round":    math1(math.RoundToEven),
	"floor":    math1(math.Floor),
	"ceil":     math1(math.Ceil),
	"frac":     math1(func(x float64) float64 { return x - math.Trunc(x) }),
	"sqrt":     math1(math.Sqrt),
	"sqr":      math1(func(x float64) float64 { return x * x }),
	"exp":      math1(math.Exp),
	"ln":       math1(math.Log),
	"log2":     math1(math.Log2),
	"log10":    math1(math.Log10),
	"sin":      math1(math.Sin),
	"cos":      math1(math.Cos),
	"tan":      math1(math.Tan),
	"arcsin":   math1(math.Asin),
	"arccos":   math1(math.Acos),
	"arctan":   math1(math.Atan),
	"degtorad": math1(func(x float64) float64 { return x * math.Pi / 180 }),
	"radtodeg": math1(func(x float64) float64 { return x * 180 / math.Pi }),
	"power":    math2(math.Pow),
	"arctan2":  math2(math.Atan2),
	"logn":     math2(func(n, x float64) float64 { return math.Log(x) / math.Log(n) }),
	"min":      reduce(math.Min),
	"max":      reduce(math.Max),
	"mean": func(in *Interp, a []Value) (Value, error) {
		if len(a) == 0 {
			return RealV(0), nil
		}
		s := 0.0
		for _, v := range a {
			s += v.N
		}
		return RealV(s / float64(len(a))), nil
	},
	"point_direction": func(in *Interp, a []Value) (Value, error) {
		if err := need(a, 4); err != nil {
			return Value{}, err
		}
		d := math.Atan2(a[1].N-a[3].N, a[2].N-a[0].N) * 180 / math.Pi // y grows downwards
		if d < 0 {
			d += 360
		}
		return RealV(d), nil
	},
	"point_distance": func(in *Interp, a []Value) (Value, error) {
		if err := need(a, 4); err != nil {
			return Value{}, err
		}
		return RealV(math.Hypot(a[2].N-a[0].N, a[3].N-a[1].N)), nil
	},
	"median": func(in *Interp, a []Value) (Value, error) { // (v0, v1, …): the middle value (the lower of the two middle ones)
		if err := need(a, 1); err != nil {
			return Value{}, err
		}
		ns := make([]float64, len(a))
		for i, v := range a {
			ns[i] = v.N
		}
		sort.Float64s(ns)
		return RealV(ns[(len(ns)-1)/2]), nil
	},
	"choose": func(in *Interp, a []Value) (Value, error) { // (v0, v1, …): one of them at random
		if err := need(a, 1); err != nil {
			return Value{}, err
		}
		return a[min(int(in.random()*float64(len(a))), len(a)-1)], nil
	},
	"lengthdir_x": func(in *Interp, a []Value) (Value, error) { // (length, direction°)
		if err := need(a, 2); err != nil {
			return Value{}, err
		}
		return RealV(a[0].N * math.Cos(a[1].N*math.Pi/180)), nil
	},
	"lengthdir_y": func(in *Interp, a []Value) (Value, error) { // y grows downwards
		if err := need(a, 2); err != nil {
			return Value{}, err
		}
		return RealV(-a[0].N * math.Sin(a[1].N*math.Pi/180)), nil
	},
	"random": func(in *Interp, a []Value) (Value, error) {
		if err := need(a, 1); err != nil {
			return Value{}, err
		}
		return RealV(in.random() * a[0].N), nil
	},
	"is_real":   func(in *Interp, a []Value) (Value, error) { return boolVal(len(a) > 0 && !a[0].IsStr), nil },
	"is_string": func(in *Interp, a []Value) (Value, error) { return boolVal(len(a) > 0 && a[0].IsStr), nil },
	"real": func(in *Interp, a []Value) (Value, error) {
		if err := need(a, 1); err != nil {
			return Value{}, err
		}
		if !a[0].IsStr {
			return a[0], nil
		}
		var f float64
		fmt.Sscan(strings.TrimSpace(a[0].S), &f)
		return RealV(f), nil
	},
	"string": func(in *Interp, a []Value) (Value, error) {
		if err := need(a, 1); err != nil {
			return Value{}, err
		}
		if a[0].IsStr {
			return a[0], nil
		}
		// ASSUMPTION [engine]: GM 5 writes reals with up to two decimals.
		s := fmt.Sprintf("%.2f", a[0].N)
		s = strings.TrimSuffix(strings.TrimRight(s, "0"), ".")
		return StrV(s), nil
	},
	"chr": func(in *Interp, a []Value) (Value, error) {
		if err := need(a, 1); err != nil {
			return Value{}, err
		}
		return StrV(string(rune(int(a[0].N)))), nil
	},
	"ord": str1(func(s string) Value {
		if s == "" {
			return RealV(0)
		}
		return RealV(float64([]rune(s)[0]))
	}),
	"string_length": str1(func(s string) Value { return RealV(float64(len([]rune(s)))) }),
	"string_lower":  str1(func(s string) Value { return StrV(strings.ToLower(s)) }),
	"string_upper":  str1(func(s string) Value { return StrV(strings.ToUpper(s)) }),
	"string_letters": str1(func(s string) Value {
		return StrV(strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) {
				return r
			}
			return -1
		}, s))
	}),
	"string_digits": str1(func(s string) Value {
		return StrV(strings.Map(func(r rune) rune {
			if unicode.IsDigit(r) {
				return r
			}
			return -1
		}, s))
	}),
	"string_pos": func(in *Interp, a []Value) (Value, error) { // (substr, str)
		if err := need(a, 2); err != nil {
			return Value{}, err
		}
		i := strings.Index(a[1].S, a[0].S)
		if i < 0 {
			return RealV(0), nil
		}
		return RealV(float64(len([]rune(a[1].S[:i])) + 1)), nil
	},
	"string_count": func(in *Interp, a []Value) (Value, error) { // (substr, str)
		if err := need(a, 2); err != nil {
			return Value{}, err
		}
		if a[0].S == "" {
			return RealV(0), nil
		}
		return RealV(float64(strings.Count(a[1].S, a[0].S))), nil
	},
	"string_copy": func(in *Interp, a []Value) (Value, error) { // (str, index, count)
		if err := need(a, 3); err != nil {
			return Value{}, err
		}
		r := []rune(a[0].S)
		i, n := clampSpan(len(r), int(a[1].N), int(a[2].N))
		return StrV(string(r[i : i+n])), nil
	},
	"string_char_at": func(in *Interp, a []Value) (Value, error) { // (str, index)
		if err := need(a, 2); err != nil {
			return Value{}, err
		}
		r := []rune(a[0].S)
		i, n := clampSpan(len(r), int(a[1].N), 1)
		return StrV(string(r[i : i+n])), nil
	},
	"string_delete": func(in *Interp, a []Value) (Value, error) { // (str, index, count)
		if err := need(a, 3); err != nil {
			return Value{}, err
		}
		r := []rune(a[0].S)
		i, n := clampSpan(len(r), int(a[1].N), int(a[2].N))
		return StrV(string(r[:i]) + string(r[i+n:])), nil
	},
	"string_insert": func(in *Interp, a []Value) (Value, error) { // (substr, str, index)
		if err := need(a, 3); err != nil {
			return Value{}, err
		}
		r := []rune(a[1].S)
		i := min(max(int(a[2].N)-1, 0), len(r))
		return StrV(string(r[:i]) + a[0].S + string(r[i:])), nil
	},
	"string_replace": func(in *Interp, a []Value) (Value, error) { // (str, substr, newstr)
		if err := need(a, 3); err != nil {
			return Value{}, err
		}
		return StrV(strings.Replace(a[0].S, a[1].S, a[2].S, 1)), nil
	},
	"string_replace_all": func(in *Interp, a []Value) (Value, error) {
		if err := need(a, 3); err != nil {
			return Value{}, err
		}
		return StrV(strings.ReplaceAll(a[0].S, a[1].S, a[2].S)), nil
	},
	"string_repeat": func(in *Interp, a []Value) (Value, error) {
		if err := need(a, 2); err != nil {
			return Value{}, err
		}
		return StrV(strings.Repeat(a[0].S, max(int(a[1].N), 0))), nil
	},
	"variable_global_exists": func(in *Interp, a []Value) (Value, error) {
		if err := need(a, 1); err != nil {
			return Value{}, err
		}
		_, ok := in.Global.Get(a[0].S, nil)
		return boolVal(ok), nil
	},
	"variable_local_exists": func(in *Interp, a []Value) (Value, error) {
		if err := need(a, 1); err != nil {
			return Value{}, err
		}
		_, ok := in.Self.Get(a[0].S, nil)
		return boolVal(ok), nil
	},
}

// random is the interpreter's own generator, seeded (Seed) so that runs
// replay exactly (docs/DESIGN.md §4).
func (in *Interp) random() float64 {
	if in.rng == nil {
		in.rng = rand.New(rand.NewPCG(uint64(in.Seed), 0x9e3779b97f4a7c15))
	}
	return in.rng.Float64()
}

func need(a []Value, n int) error {
	if len(a) < n {
		return fmt.Errorf("needs %d arguments, got %d", n, len(a))
	}
	return nil
}

func math1(f func(float64) float64) Func {
	return func(in *Interp, a []Value) (Value, error) {
		if err := need(a, 1); err != nil {
			return Value{}, err
		}
		return RealV(f(a[0].N)), nil
	}
}

func math2(f func(a, b float64) float64) Func {
	return func(in *Interp, a []Value) (Value, error) {
		if err := need(a, 2); err != nil {
			return Value{}, err
		}
		return RealV(f(a[0].N, a[1].N)), nil
	}
}

func reduce(f func(a, b float64) float64) Func {
	return func(in *Interp, a []Value) (Value, error) {
		if err := need(a, 1); err != nil {
			return Value{}, err
		}
		m := a[0].N
		for _, v := range a[1:] {
			m = f(m, v.N)
		}
		return RealV(m), nil
	}
}

func str1(f func(string) Value) Func {
	return func(in *Interp, a []Value) (Value, error) {
		if err := need(a, 1); err != nil {
			return Value{}, err
		}
		return f(a[0].S), nil
	}
}

// clampSpan turns a 1-based index and a count into a valid [i, i+n) span.
func clampSpan(length, index, count int) (int, int) {
	i := min(max(index-1, 0), length)
	return i, min(max(count, 0), length-i)
}

func cmp(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
