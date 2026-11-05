package render

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/bmp"

	"github.com/LaPingvino/gameviber/import/gmfile"
	"github.com/LaPingvino/gameviber/runner"
)

// Shot renders the room as the screen shows it — through view 0 when views
// are on, the room's colour, the texts in the frame — into a new image.
func Shot(p *gmfile.Project, g *runner.Game, r *runner.Room, a *Assets) *image.RGBA {
	w, h := r.W, r.H
	if r.ViewsEnabled && len(r.Views) > 0 && r.Views[0].Visible {
		w, h = r.Views[0].W, r.Views[0].H
	}
	var colour int32
	drawColour := true
	for _, x := range p.Rooms {
		if x != nil && x.Name == r.Name {
			colour, drawColour = x.BGColor, x.DrawBGColor
		}
	}
	in := *a
	in.TextOver = false // a saved screen has its texts
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	Frame(img, g, r, &in, colour, drawColour)
	return img
}

// SaveScreen writes a shot to a file: a BMP, as GM 5's screen_save does
// whatever the name, or a PNG when the name ends in .png.
func SaveScreen(path string, img image.Image) error {
	if d := filepath.Dir(path); d != "." {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if strings.EqualFold(filepath.Ext(path), ".png") {
		err = png.Encode(f, img)
	} else {
		err = bmp.Encode(f, img)
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}
