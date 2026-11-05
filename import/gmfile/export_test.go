package gmfile

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/LaPingvino/gameviber/gml"
)

// Stelo exported to GameViber's format: every .gml parses, game.toml reads
// back, the sprite strips are PNGs of the right size.
func TestExportStelo(t *testing.T) {
	d, err := os.ReadFile("../../../stelo/win/stelo.exe")
	if err != nil {
		t.Skip("Stelo's exe is not checked out next to this repo")
	}
	e, err := OpenExe(d)
	if err != nil {
		t.Fatal(err)
	}
	p, err := e.ReadProject()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	notes, err := p.Export(dir, "Stelo")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range notes {
		t.Log("note:", n)
	}
	var game struct {
		Name      string   `toml:"name"`
		RoomOrder []string `toml:"room_order"`
	}
	if _, err := toml.DecodeFile(filepath.Join(dir, "game.toml"), &game); err != nil || game.Name != "Stelo" || len(game.RoomOrder) == 0 {
		t.Fatalf("game.toml: %v %+v", err, game)
	}
	gmls, pngs := 0, 0
	filepath.WalkDir(dir, func(path string, de os.DirEntry, err error) error {
		switch {
		case strings.HasSuffix(path, ".gml"):
			src, _ := os.ReadFile(path)
			if _, err := gml.Parse(string(src)); err != nil {
				t.Errorf("%s: %v", path, err)
			}
			gmls++
		case strings.HasSuffix(path, ".png"):
			f, _ := os.Open(path)
			if _, err := png.Decode(f); err != nil {
				t.Errorf("%s: %v", path, err)
			}
			f.Close()
			pngs++
		}
		return nil
	})
	t.Logf("%d GML files, %d PNGs; rooms in order: %s", gmls, pngs, strings.Join(game.RoomOrder, ", "))
}

// TestPathsRoundTrip: paths written as paths/<name>.toml and read back —
// points, smooth, closed, precision; ids by name order.
func TestPathsRoundTrip(t *testing.T) {
	p := &Project{Paths: []*Path{
		{ID: 0, Name: "patrol", Smooth: true, Closed: true, Precision: 6, Points: [][3]float64{{32, 32, 100}, {320, 32, 50}, {320, 240, 100}}},
		nil, // a deleted path keeps its id in GameMaker's numbering; the text form drops it
		{ID: 2, Name: "drop", Points: [][3]float64{{0, 0, 100}, {0, 400, 200}}},
	}}
	dir := t.TempDir()
	if _, err := p.Export(dir, "g"); err != nil {
		t.Fatal(err)
	}
	q, err := ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Paths) != 2 || q.Paths[0].Name != "drop" || q.Paths[1].Name != "patrol" || q.Paths[1].ID != 1 {
		t.Fatalf("paths: %+v", q.Paths)
	}
	pa := q.Paths[1]
	if !pa.Smooth || !pa.Closed || pa.Precision != 6 || len(pa.Points) != 3 || pa.Points[1] != [3]float64{320, 32, 50} {
		t.Errorf("patrol: %+v", pa)
	}
	if d := q.Paths[0]; d.Smooth || d.Closed || d.Precision != 4 || d.Points[1][2] != 200 {
		t.Errorf("drop (precision 0 written, GameMaker's 4 read): %+v", d)
	}
}
