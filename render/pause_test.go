package render

import (
	"image"
	"image/color"
	"testing"

	"github.com/LaPingvino/gameviber/runner"
)

// During action_sleep the frame shows the moment the pause began: the event
// that paused (here: jump to 40,0) has run, but the picture keeps the old
// place until the pause is over.
func TestPauseShowsItsMoment(t *testing.T) {
	red := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for i := range red.Pix {
		red.Pix[i] = 255
		if i%4 == 1 || i%4 == 2 {
			red.Pix[i] = 0
		}
	}
	a := &Assets{Frames: map[string][]*image.NRGBA{"dot": {red}}, Origins: map[string]image.Point{}}
	o := &runner.Object{Name: "o", Sprite: "dot", Visible: true}
	g := &runner.Game{Objects: []*runner.Object{o}}
	h, err := g.Code("action_sleep(100, 1); x = 40")
	if err != nil {
		t.Fatal(err)
	}
	o.Events.Step = h
	r := &runner.Room{W: 64, H: 16, Speed: 30, G: &runner.Globals{Vars: map[string]float64{}}}
	r.Create(o, 0, 0)
	r.Step(noKeys{})
	if len(g.Errors) > 0 {
		t.Fatal(g.Errors)
	}
	isRed := func(img *image.RGBA, x int) bool { return img.RGBAAt(x+2, 2) == color.RGBA{255, 0, 0, 255} }
	dst := image.NewRGBA(image.Rect(0, 0, 64, 16))
	Frame(dst, g, r, a, 0, true)
	if r.Sleep == 0 || !isRed(dst, 0) || isRed(dst, 40) {
		t.Fatalf("in the pause (sleep %d): at 0 %v, at 40 %v — want the old place", r.Sleep, isRed(dst, 0), isRed(dst, 40))
	}
	for r.Sleep > 0 {
		r.Step(noKeys{})
	}
	dst = image.NewRGBA(image.Rect(0, 0, 64, 16))
	Frame(dst, g, r, a, 0, true)
	if isRed(dst, 0) || !isRed(dst, 40) {
		t.Errorf("after the pause: at 0 %v, at 40 %v — want the new place", isRed(dst, 0), isRed(dst, 40))
	}
}
