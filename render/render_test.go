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

// held is one key held down (pressed on its first step).
type held struct {
	key   runner.Key
	first bool
}

func (h *held) Held(k runner.Key) bool    { return k == h.key }
func (h *held) Pressed(k runner.Key) bool { return k == h.key && h.first }

// Pacmanio played from its first room the way a player starts it: up to
// eat komenco's dot, left to eat pu_bildo's, then the first level — reached
// through the rooms (the region set by pu_bildo), drawn after 30 steps;
// $GAMEVIBER_SHOT2 for the picture.
func TestPlayPacmanioIntoLevel(t *testing.T) {
	d, err := os.ReadFile("../../pacmanio/win/pacmanio.exe")
	if err != nil {
		t.Skip("Pacmanio's exe is not checked out next to this repo")
	}
	e, _ := gmfile.OpenExe(d)
	p, err := e.ReadProject()
	if err != nil {
		t.Fatal(err)
	}
	g, err := load.FromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	pl, err := g.Start()
	if err != nil {
		t.Fatal(err)
	}
	path := []string{pl.Room.Name}
	step := func(in runner.Input) {
		if err := pl.Step(in); err != nil {
			t.Fatal(err)
		}
		if n := pl.Room.Name; n != path[len(path)-1] {
			path = append(path, n)
		}
	}
	hold := func(k runner.Key, until string) {
		h := &held{key: k, first: true}
		for i := 0; i < 400 && pl.Room.Name != until; i++ {
			step(h)
			h.first = false
		}
	}
	// The start rooms are small mazes (komenco has a wall between Pacman and
	// its dot): instead of a scripted path, Pacman is put on the dot, as a
	// player who walked there; the game does the rest (eat, 2 s, next room).
	eat := func(until string) {
		for i := 0; i < 400 && pl.Room.Name != until; i++ {
			if pm, dots := pl.Room.All("pacman"), pl.Room.All("point"); len(pm) > 0 && len(dots) > 0 {
				pm[0].X, pm[0].Y = dots[0].X, dots[0].Y
			}
			step(noKeys{})
		}
	}
	eat("pu_bildo")
	eat("pu_1a_nivelo")
	_ = hold
	for range 30 {
		step(noKeys{})
	}
	t.Logf("rooms: %v; score %d, lives %d; errors %v", path, pl.Globals.Score, pl.Globals.Lives, g.Errors)
	if pl.Room.Name != "pu_1a_nivelo" {
		t.Fatalf("ended in %s", pl.Room.Name)
	}
	var room *gmfile.Room
	for _, x := range p.Rooms {
		if x != nil && x.Name == pl.Room.Name {
			room = x
		}
	}
	img := image.NewRGBA(image.Rect(0, 0, int(room.W), int(room.H)))
	Frame(img, pl.Room, LoadAssets(p), room.BGColor, room.DrawBGColor)
	if path := os.Getenv("GAMEVIBER_SHOT2"); path != "" {
		f, _ := os.Create(path)
		png.Encode(f, img)
		f.Close()
	}
}
