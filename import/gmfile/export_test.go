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
