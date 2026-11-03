package gml

import "testing"

func TestBuiltins(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{"a = round(2.5) + round(3.5)", "6"},
		{"a = floor(-1.5)", "-2"},
		{"a = max(3, 8, 1) - min(3, 8, 1)", "7"},
		{"a = point_direction(0, 0, 10, -10)", "45"},
		{"a = point_distance(0, 0, 3, 4)", "5"},
		{"a = string_copy('Stelo', 2, 3)", "tel"},
		{"a = string_pos('lo', 'Stelo')", "4"},
		{"a = string_count('o', 'Pacmanio oo')", "3"},
		{"a = string_delete('Pacmanio', 4, 4)", "Paco"},
		{"a = string_insert('X', 'ab', 2)", "aXb"},
		{"a = ord('A') + string_length('ĉapelo')", "71"},
		{"a = string(12.5) + string(3)", "12.53"},
		{"a = real('42') + 1", "43"},
		{"a = string_upper(chr(97))", "A"},
		{"a = random(10) >= 0 and random(10) < 10", "1"},
		{"global.x = 1; a = variable_global_exists('x') + variable_global_exists('y')", "1"},
	} {
		self, _, _, err := run(t, c.src)
		if err != nil {
			t.Errorf("%q: %v", c.src, err)
			continue
		}
		if v, _ := self.Get("a", nil); v.String() != c.want {
			t.Errorf("%q: a = %s, want %s", c.src, v, c.want)
		}
	}
}

// The same seed gives the same random numbers (replays).
func TestRandomSeeded(t *testing.T) {
	draw := func(seed int64) [3]float64 {
		in := &Interp{Seed: seed}
		return [3]float64{in.random(), in.random(), in.random()}
	}
	if draw(7) != draw(7) || draw(7) == draw(8) {
		t.Error("random() isn't reproducible by seed")
	}
}
