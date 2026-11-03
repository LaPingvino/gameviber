package ebiten

import "testing"

// Whole steps when they fill ≥ 90%, else the largest fraction; centred.
func TestFit(t *testing.T) {
	for _, c := range []struct {
		sw, sh int
		scale  float64
	}{
		{640, 480, 1}, {1920, 1080, 2.25}, {1300, 1000, 2}, {1000, 700, 700.0 / 480}, {2560, 1440, 3},
	} {
		v := fit(640, 480, c.sw, c.sh)
		if v.scale != c.scale {
			t.Errorf("%dx%d: scale %v, want %v", c.sw, c.sh, v.scale, c.scale)
		}
		if x, y := v.toFrame(c.sw/2, c.sh/2); x != 320 || y != 240 {
			t.Errorf("%dx%d: the centre maps to %d,%d", c.sw, c.sh, x, y)
		}
	}
}
