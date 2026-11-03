package render

import (
	"image"
	"testing"

	"github.com/LaPingvino/gameviber/runner"
)

// Text is drawn into the frame: GM's colour (BGR), top-left at (x, y),
// '#' a new line.
func TestDrawText(t *testing.T) {
	dst := image.NewRGBA(image.Rect(0, 0, 200, 80))
	drawText(dst, TextCall{X: 10, Y: 10, S: "Score#99", Font: runner.Font{Name: "Arial", Size: 12, Colour: 0x0000FF}}) // red
	red, top, bottom := 0, 80, 0
	for y := range 80 {
		for x := range 200 {
			if c := dst.RGBAAt(x, y); c.R > 128 && c.G < 64 && c.B < 64 {
				red++
				top, bottom = min(top, y), max(bottom, y)
			}
		}
	}
	if red < 40 || top < 10 || bottom < 30 {
		t.Errorf("%d red pixels, rows %d–%d (want two lines from y 10)", red, top, bottom)
	}
}
