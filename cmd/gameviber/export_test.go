package main

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaPingvino/gameviber/import/gmfile"
)

// TestGameIcon: [settings] icon names a sprite → the export's icons are drawn
// from its first frame; a sprite that doesn't exist is a check problem.
func TestGameIcon(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "g")
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	ents, _ := os.ReadDir(filepath.Join(dir, "sprites"))
	if len(ents) == 0 {
		t.Fatal("the starter game has no sprites")
	}
	sprite := ents[0].Name()
	toml := filepath.Join(dir, "game.toml")
	data, _ := os.ReadFile(toml)
	if !strings.Contains(string(data), "[settings]") {
		data = append(data, "\n[settings]\n"...)
	}
	os.WriteFile(toml, append(data, []byte("  icon = \""+sprite+"\"\n")...), 0o644)
	p, err := gmfile.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if p.IconSprite != sprite {
		t.Fatalf("IconSprite %q, want %q", p.IconSprite, sprite)
	}
	out := t.TempDir()
	if err := writeGameIcons(p, out); err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{192, 512} {
		b, err := os.ReadFile(filepath.Join(out, "icon-"+map[int]string{192: "192", 512: "512"}[n]+".png"))
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(bytes.NewReader(b))
		if err != nil || img.Bounds().Dx() != n {
			t.Fatalf("icon %d: %v %v", n, err, img.Bounds())
		}
		bg, _, _, _ := img.At(2, 2).RGBA()
		drawn := false
		for y := n / 3; y < 2*n/3 && !drawn; y++ {
			for x := n / 3; x < 2*n/3; x++ {
				if r, g, b, _ := img.At(x, y).RGBA(); r != bg || g != bg || b != bg {
					drawn = true
					break
				}
			}
		}
		if !drawn {
			t.Errorf("icon %d: nothing of the sprite in its middle", n)
		}
	}
	var buf bytes.Buffer
	if n := checkRefs(dir, &buf); n != 0 {
		t.Errorf("a valid icon: %d problems: %s", n, buf.String())
	}
	data, _ = os.ReadFile(toml)
	os.WriteFile(toml, []byte(strings.Replace(string(data), "icon = \""+sprite+"\"", "icon = \""+sprite+"x\"", 1)), 0o644)
	buf.Reset()
	if n := checkRefs(dir, &buf); n != 1 || !strings.Contains(buf.String(), "[settings] icon's sprite") {
		t.Errorf("a missing icon sprite: %d problems: %q", n, buf.String())
	}
}
