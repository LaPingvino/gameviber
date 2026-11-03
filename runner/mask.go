package runner

import "math"

// Precise collisions (GM 5's "precise collision checking"): a precise
// sprite collides by its visible pixels, one mask per frame; a sprite that is
// not precise collides by its whole bounding box. Two instances touch when
// their boxes overlap and, inside the overlap, some pixel is solid in both.

// PixelMask is one frame's solid pixels; X0, Y0 is its top-left corner
// relative to the instance's position (minus the sprite's origin).
type PixelMask struct {
	X0, Y0, W, H int
	Bits         []bool
}

func (m *PixelMask) at(x, y int) bool {
	x, y = x-m.X0, y-m.Y0
	return x >= 0 && y >= 0 && x < m.W && y < m.H && m.Bits[y*m.W+x]
}

// solidAt: whether the instance standing at (ix, iy) covers room pixel
// (px, py) — inside its box, and on its mask when its sprite is precise.
func (in *Instance) solidAt(px, py, ix, iy int) bool {
	b := in.Obj.BBox
	if px < ix+b.L || px > ix+b.R || py < iy+b.T || py > iy+b.B {
		return false
	}
	ms := in.Obj.Pixels
	if len(ms) == 0 {
		return true
	}
	f := int(math.Floor(in.ImageIndex)) % len(ms)
	if f < 0 {
		f += len(ms)
	}
	return ms[f].at(px-ix, py-iy)
}

// touches reports whether a, standing at (ax, ay), meets b where b is.
func touches(a *Instance, ax, ay float64, b *Instance) bool {
	ba, bb := a.BoxAt(ax, ay), b.BoxAt(b.X, b.Y)
	if !ba.Overlaps(bb) {
		return false
	}
	if len(a.Obj.Pixels) == 0 && len(b.Obj.Pixels) == 0 {
		return true
	}
	aix, aiy := int(math.Round(ax)), int(math.Round(ay))
	bix, biy := int(math.Round(b.X)), int(math.Round(b.Y))
	for y := max(ba.T, bb.T); y <= min(ba.B, bb.B); y++ {
		for x := max(ba.L, bb.L); x <= min(ba.R, bb.R); x++ {
			if a.solidAt(x, y, aix, aiy) && b.solidAt(x, y, bix, biy) {
				return true
			}
		}
	}
	return false
}
