package render

import (
	"image"
	"image/png"
	"os"
	"testing"

	"github.com/LaPingvino/gameviber/import/gmfile"
	"github.com/LaPingvino/gameviber/load"
	"github.com/LaPingvino/gameviber/runner"
)

type noKeys struct{}

func (noKeys) Held(runner.Key) bool    { return false }
func (noKeys) Pressed(runner.Key) bool { return false }

// Pacmanio's first level, run 30 steps and drawn: the walls, the dots, the
// ghosts — written to $GAMEVIBER_SHOT if set, for a look.
func TestFramePacmanio(t *testing.T) {
	d, err := os.ReadFile("../../pacmanio/win/pacmanio.exe")
	if err != nil {
		t.Skip("Pacmanio's exe is not checked out next to this repo")
	}
	e, err := gmfile.OpenExe(d)
	if err != nil {
		t.Fatal(err)
	}
	p, err := e.ReadProject()
	if err != nil {
		t.Fatal(err)
	}
	g, err := load.FromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	r, err := g.Room("pu_1a_nivelo", &runner.Globals{Lives: 3, Vars: map[string]float64{"murigxu": 2}})
	if err != nil {
		t.Fatal(err)
	}
	for range 30 {
		r.Step(noKeys{})
	}
	var room *gmfile.Room
	for _, x := range p.Rooms {
		if x != nil && x.Name == "pu_1a_nivelo" {
			room = x
		}
	}
	img := image.NewRGBA(image.Rect(0, 0, int(room.W), int(room.H)))
	Frame(img, r, LoadAssets(p), room.BGColor, room.DrawBGColor)
	drawn := 0
	bg := img.RGBAAt(0, 0)
	for y := 0; y < img.Bounds().Dy(); y += 4 {
		for x := 0; x < img.Bounds().Dx(); x += 4 {
			if img.RGBAAt(x, y) != bg {
				drawn++
			}
		}
	}
	if drawn < 1000 {
		t.Errorf("only %d sampled pixels differ from the background", drawn)
	}
	if path := os.Getenv("GAMEVIBER_SHOT"); path != "" {
		f, _ := os.Create(path)
		png.Encode(f, img)
		f.Close()
	}
}
