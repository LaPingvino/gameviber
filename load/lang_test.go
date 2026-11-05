package load

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/LaPingvino/gameviber/import/gmfile"
)

// A game folder may hold events in JavaScript: objects/<name>/<event>.js runs
// like the .gml file of the same name would, a collision's `other` included,
// and the folder written back keeps the .js file.
func TestJavaScriptEvents(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	img.Set(0, 0, color.NRGBA{255, 0, 0, 255})
	var box bytes.Buffer
	png.Encode(&box, img)
	fsys := fstest.MapFS{
		"sprites/box/strip.png":          {Data: box.Bytes()},
		"sprites/box/sprite.toml":        {Data: []byte("frames = 1\nframe_w = 16\nframe_h = 16\norigin = [0, 0]\nbbox_lrbt = [0, 15, 15, 0]\nprecise = false\ntransparent = false\n")},
		"scripts/double.js":              {Data: []byte("// twice its argument, plus one through a GML script\nreturn add1(argument0 * 2) - 1;\n")},
		"scripts/add1.gml":               {Data: []byte("return argument0 + 1\n")},
		"scripts/where_am_i.js":          {Data: []byte("return x + arguments.length;\n")},
		"game.toml":                      {Data: []byte("name = \"js\"\nroom_order = [\"r\"]\n")},
		"objects/mover/object.toml":      {Data: []byte("sprite = \"box\"\nvisible = true\n")},
		"objects/mover/create.js":        {Data: []byte("hits = 0;\nd = double(21);\nw = where_am_i(1, 2);\n")},
		"objects/mover/step.js":          {Data: []byte("x += 2; // two a step\nif (x > 40) instance_destroy();\n")},
		"objects/mover/collision-pad.js": {Data: []byte("hits += 1;\nother.touched = 1;\n")},
		"objects/pad/object.toml":        {Data: []byte("sprite = \"box\"\nvisible = true\n")},
		"objects/pad/create.gml":         {Data: []byte("touched = 0\ntwice = double(4)\n")},
		"rooms/r/room.toml": {Data: []byte("width = 64\nheight = 64\nspeed = 30\n" +
			"[[instance]]\nobject = \"mover\"\nx = 0\ny = 0\nid = 100001\n" +
			"[[instance]]\nobject = \"pad\"\nx = 0\ny = 0\nid = 100002\n")},
	}
	p, err := gmfile.ReadFS(fsys, ".")
	if err != nil {
		t.Fatal(err)
	}
	g, err := FromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	pl, err := g.StartAt("r")
	if err != nil {
		t.Fatal(err)
	}
	for range 5 {
		if err := pl.Step(noKeys{}); err != nil {
			t.Fatal(err)
		}
	}
	if len(g.Errors) > 0 {
		t.Fatal(g.Errors)
	}
	movers := pl.Room.All("mover")
	if len(movers) != 1 || movers[0].X != 10 {
		t.Fatalf("movers %d, x %v (want one at 10)", len(movers), func() float64 {
			if len(movers) > 0 {
				return movers[0].X
			}
			return -1
		}())
	}
	if hits, touched := movers[0].Vars["hits"], pl.Room.All("pad")[0].Vars["touched"]; hits < 1 || touched != 1 {
		t.Errorf("the JavaScript collision event: hits %v, pad touched %v", hits, touched)
	}

	// Scripts: a JS script from JS and from GML, a GML script from JS, self in a script.
	pad := pl.Room.All("pad")[0]
	if movers[0].Vars["d"] != 42 || pad.Vars["twice"] != 8 || movers[0].Vars["w"] != 2 {
		t.Errorf("scripts: double(21) %v, from GML double(4) %v, where_am_i(1, 2) at x 0 %v", movers[0].Vars["d"], pad.Vars["twice"], movers[0].Vars["w"])
	}

	// Written back out, the events and scripts keep their language.
	if _, err := os.Stat(filepath.Join(dir0(t, p), "scripts", "double.js")); err != nil {
		t.Errorf("script written back: %v", err)
	}
	dir := t.TempDir()
	if _, err := p.Export(dir, "js"); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"step.js", "collision-pad.js", "create.js"} {
		b, err := os.ReadFile(filepath.Join(dir, "objects", "mover", f))
		if err != nil {
			t.Fatalf("written folder: %v", err)
		}
		if f == "step.js" && !strings.Contains(string(b), "two a step") {
			t.Errorf("step.js changed: %q", b)
		}
	}
}

// dir0 is the project exported to a new temporary folder.
func dir0(t *testing.T, p *gmfile.Project) string {
	t.Helper()
	dir := t.TempDir()
	if _, err := p.Export(dir, "js"); err != nil {
		t.Fatal(err)
	}
	return dir
}
