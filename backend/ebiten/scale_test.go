package ebiten

import (
	"image"
	"testing"
)

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

// The buttons sit in the margin, never over the frame; none without room.
func TestButtonRects(t *testing.T) {
	for _, c := range []struct {
		sw, sh int
		want   bool
	}{{1920, 1080, true}, {1080, 1920, true}, {640, 480, false}} {
		v := fit(640, 480, c.sw, c.sh)
		snd, full := buttonRects(v, 640, 480, c.sw, c.sh)
		if snd.Empty() == c.want {
			t.Errorf("%dx%d: buttons %v, want %v", c.sw, c.sh, !snd.Empty(), c.want)
			continue
		}
		frame := image.Rect(int(v.ox), int(v.oy), int(v.ox+640*v.scale), int(v.oy+480*v.scale))
		if snd.Overlaps(frame) || full.Overlaps(frame) || snd.Overlaps(full) {
			t.Errorf("%dx%d: sound %v full %v frame %v", c.sw, c.sh, snd, full, frame)
		}
	}
}
