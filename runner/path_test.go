package runner

import (
	"math"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

// TestPathGeometry: a straight open path's length and points along it (speed
// interpolated), a closed one back to its start, a smooth one through the ends
// and within the points' hull.
func TestPathGeometry(t *testing.T) {
	p := &Path{Points: [][3]float64{{0, 0, 100}, {100, 0, 50}, {100, 100, 100}}}
	if !near(p.Length(), 200) {
		t.Errorf("length %v", p.Length())
	}
	if x, y, sp := p.At(0.25); !near(x, 50) || !near(y, 0) || !near(sp, 75) {
		t.Errorf("At(0.25) = %v,%v speed %v", x, y, sp)
	}
	if x, y, _ := p.At(0.75); !near(x, 100) || !near(y, 50) {
		t.Errorf("At(0.75) = %v,%v", x, y)
	}
	c := &Path{Points: p.Points, Closed: true}
	if !near(c.Length(), 200+math.Hypot(100, 100)) {
		t.Errorf("closed length %v", c.Length())
	}
	if x, y, _ := c.At(1); !near(x, 0) || !near(y, 0) {
		t.Errorf("closed end %v,%v", x, y)
	}
	s := &Path{Points: p.Points, Smooth: true, Precision: 4}
	if x, y, _ := s.At(0); x != 0 || y != 0 {
		t.Errorf("smooth start %v,%v", x, y)
	}
	if x, y, _ := s.At(1); !near(x, 100) || !near(y, 100) {
		t.Errorf("smooth end %v,%v", x, y)
	}
	if l := s.Length(); l >= 200 || l < 141 {
		t.Errorf("smooth length %v: shorter than the corner, longer than the diagonal", l)
	}
	if x, y, _ := s.At(0.5); x > 100 || y < 0 || x < 50 || y > 50 {
		t.Errorf("smooth middle %v,%v", x, y)
	}
}

// TestPathStep: an instance on a relative path moves by path_speed (times the
// point's speed), ends with the end event due; stop leaves it at the end, off
// the path; restart goes round again; reverse turns back.
func TestPathStep(t *testing.T) {
	p := &Path{Name: "line", Points: [][3]float64{{0, 0, 100}, {100, 0, 100}}}
	in := NewInstance(&Object{Name: "o"}, 10, 20)
	in.PathStart(p, 25, PathStop, false)
	ends := 0
	for range 4 {
		if in.stepPath() {
			ends++
		}
	}
	if ends != 1 || in.Path != nil || !near(in.X, 110) || !near(in.Y, 20) {
		t.Errorf("stop: ends %d path %v at %v,%v", ends, in.Path, in.X, in.Y)
	}
	in = NewInstance(&Object{Name: "o"}, 0, 0)
	in.PathStart(p, 30, PathRestart, true)
	for range 4 {
		in.stepPath()
	}
	if in.Path == nil || !near(in.X, 20) {
		t.Errorf("restart: at %v (want 20 after 120 px on a 100 px path)", in.X)
	}
	in = NewInstance(&Object{Name: "o"}, 0, 0)
	in.PathStart(p, 30, PathReverse, true)
	for range 4 {
		in.stepPath()
	}
	if in.PathSpeed != -30 || !near(in.X, 80) {
		t.Errorf("reverse: speed %v at %v (want -30 at 80)", in.PathSpeed, in.X)
	}
}
