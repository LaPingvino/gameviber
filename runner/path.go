package runner

import "math"

// Paths (LOOP item 12b): a path resource, an instance moving along one, and
// the path end event (other 8).
//
// ASSUMPTION [engine]: GameMaker's path geometry as its manual describes it —
// straight paths join the points; smooth paths are quadratic curves through
// the midpoints of the segments (from the first point to the last on an open
// path), each cut into 2^precision pieces; a closed path returns to its first
// point. Each point's speed is a percentage of the instance's path_speed,
// interpolated along the way. A relative path (path_start's absolute false)
// starts where the instance is.

// Path is a path resource.
type Path struct {
	Name      string
	Points    [][3]float64 // x, y, speed (percent)
	Smooth    bool
	Closed    bool
	Precision int

	pts   []pathPt // the polyline it is followed along
	total float64  // its length
}

type pathPt struct{ x, y, sp, at float64 } // at: the length up to this point

func (p *Path) build() {
	if p.pts != nil || len(p.Points) == 0 {
		return
	}
	pt := func(q [3]float64) pathPt { return pathPt{x: q[0], y: q[1], sp: q[2]} }
	ctrl := append([][3]float64(nil), p.Points...)
	var out []pathPt
	if !p.Smooth || len(ctrl) < 3 {
		for _, q := range ctrl {
			out = append(out, pt(q))
		}
		if p.Closed && len(ctrl) > 1 {
			out = append(out, pt(ctrl[0]))
		}
	} else {
		mid := func(a, b [3]float64) [3]float64 {
			return [3]float64{(a[0] + b[0]) / 2, (a[1] + b[1]) / 2, (a[2] + b[2]) / 2}
		}
		n := len(ctrl)
		steps := 1 << max(1, min(8, p.Precision))
		piece := func(a, c, b [3]float64) { // a quadratic curve a → b pulled towards c
			for s := 0; s < steps; s++ {
				t := float64(s) / float64(steps)
				u := 1 - t
				var q [3]float64
				for k := range 3 {
					q[k] = u*u*a[k] + 2*u*t*c[k] + t*t*b[k]
				}
				out = append(out, pt(q))
			}
		}
		if p.Closed {
			for i := range n {
				prev, cur, next := ctrl[(i+n-1)%n], ctrl[i], ctrl[(i+1)%n]
				piece(mid(prev, cur), cur, mid(cur, next))
			}
			out = append(out, out[0])
		} else {
			for i := 1; i < n-1; i++ {
				a, b := mid(ctrl[i-1], ctrl[i]), mid(ctrl[i], ctrl[i+1])
				if i == 1 {
					a = ctrl[0]
				}
				if i == n-2 {
					b = ctrl[n-1]
				}
				piece(a, ctrl[i], b)
			}
			out = append(out, pt(ctrl[n-1]))
		}
	}
	for i := 1; i < len(out); i++ {
		out[i].at = out[i-1].at + math.Hypot(out[i].x-out[i-1].x, out[i].y-out[i-1].y)
	}
	p.pts, p.total = out, out[len(out)-1].at
}

// At is the point at a fraction (0–1) of the path's length, and the speed
// (percent) there.
func (p *Path) At(pos float64) (x, y, speed float64) {
	p.build()
	if len(p.pts) == 0 {
		return 0, 0, 100
	}
	if len(p.pts) == 1 || p.total == 0 {
		return p.pts[0].x, p.pts[0].y, p.pts[0].sp
	}
	d := math.Max(0, math.Min(1, pos)) * p.total
	i := 1
	for i < len(p.pts)-1 && p.pts[i].at < d {
		i++
	}
	a, b := p.pts[i-1], p.pts[i]
	t := 0.0
	if b.at > a.at {
		t = (d - a.at) / (b.at - a.at)
	}
	return a.x + (b.x-a.x)*t, a.y + (b.y-a.y)*t, a.sp + (b.sp-a.sp)*t
}

// Length is the path's length.
func (p *Path) Length() float64 { p.build(); return p.total }

// Path end actions (path_endaction).
const (
	PathStop     = 0 // stop at the end
	PathRestart  = 1 // continue from the start
	PathContinue = 2 // continue from here (the path again, from where it ended)
	PathReverse  = 3 // back the other way
)

// PathStart is path_start: the instance follows p at speed (pixels a step),
// with the end action; absolute: the path's own coordinates, else it starts
// where the instance is.
func (in *Instance) PathStart(p *Path, speed float64, end int, absolute bool) {
	in.Path, in.PathPos, in.PathPosPrev, in.PathSpeed, in.PathEnd = p, 0, 0, speed, end
	if in.PathScale == 0 {
		in.PathScale = 1
	}
	in.pathOX, in.pathOY = 0, 0
	if p != nil && !absolute {
		sx, sy, _ := p.At(0)
		in.pathOX, in.pathOY = in.X-sx*in.PathScale, in.Y-sy*in.PathScale
	}
	if p != nil && speed < 0 {
		in.PathPos, in.PathPosPrev = 1, 1
	}
}

// stepPath moves the instance along its path for one step; true when it
// reached an end (the path end event is due).
func (in *Instance) stepPath() bool {
	p := in.Path
	length := p.Length() * in.PathScale
	_, _, sp := p.At(in.PathPos)
	in.PathPosPrev = in.PathPos
	if length > 0 {
		in.PathPos += in.PathSpeed * sp / 100 / length
	} else {
		in.PathPos = 1
		if in.PathSpeed < 0 {
			in.PathPos = 0
		}
	}
	ended := false
	if in.PathPos >= 1 || in.PathPos <= 0 && in.PathSpeed < 0 {
		ended = true
		forward := in.PathPos >= 1
		switch in.PathEnd {
		case PathRestart:
			if forward {
				in.PathPos -= 1
			} else {
				in.PathPos += 1
			}
		case PathContinue: // the path again, shifted to start where it ended
			x0, y0, _ := p.At(0)
			x1, y1, _ := p.At(1)
			dx, dy := (x1-x0)*in.PathScale, (y1-y0)*in.PathScale
			if forward {
				in.pathOX, in.pathOY = in.pathOX+dx, in.pathOY+dy
				in.PathPos -= 1
			} else {
				in.pathOX, in.pathOY = in.pathOX-dx, in.pathOY-dy
				in.PathPos += 1
			}
		case PathReverse:
			in.PathSpeed = -in.PathSpeed
			if forward {
				in.PathPos = 2 - in.PathPos
			} else {
				in.PathPos = -in.PathPos
			}
		default: // stop
			in.PathPos = math.Max(0, math.Min(1, in.PathPos))
		}
	}
	x, y, _ := p.At(in.PathPos)
	in.X, in.Y = in.pathOX+x*in.PathScale, in.pathOY+y*in.PathScale
	if ended && in.PathEnd == PathStop {
		in.Path = nil
	}
	return ended
}
