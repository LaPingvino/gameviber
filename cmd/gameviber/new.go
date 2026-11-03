package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// cmdNew writes a small starter project: a player (arrow keys) collecting
// coins in a walled room — every file plain text or a PNG drawn here, so it
// is GameViber's own and free to share.
func cmdNew(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: gameviber new DIR")
	}
	dir := args[0]
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("%s exists already", dir)
	}
	name := filepath.Base(dir)
	files := map[string]string{
		"game.toml":                         fmt.Sprintf("name = %q\ngm_version = 530\nroom_order = [\"level\"]\n\n[settings]\n  author = \"\"\n", name),
		"sprites/wall/sprite.toml":          spriteTOML,
		"sprites/player/sprite.toml":        spriteTOML,
		"sprites/coin/sprite.toml":          "frames = 1\nframe_w = 32\nframe_h = 32\norigin = [0, 0]\nbbox_lrbt = [8, 23, 23, 8]\nprecise = false\ntransparent = true\n",
		"objects/wall/object.toml":          "sprite = \"wall\"\nsolid = true\nvisible = true\ndepth = 0\npersistent = false\nparent = \"-1\"\nmask = \"-1\"\n",
		"objects/coin/object.toml":          "sprite = \"coin\"\nsolid = false\nvisible = true\ndepth = 0\npersistent = false\nparent = \"-1\"\nmask = \"-1\"\n",
		"objects/player/object.toml":        "sprite = \"player\"\nsolid = false\nvisible = true\ndepth = -1\npersistent = false\nparent = \"-1\"\nmask = \"-1\"\n",
		"objects/player/create.gml":         "score = 0\n",
		"objects/player/keyboard-37.gml":    "if place_free(x - 4, y) x -= 4\n",
		"objects/player/keyboard-39.gml":    "if place_free(x + 4, y) x += 4\n",
		"objects/player/keyboard-38.gml":    "if place_free(x, y - 4) y -= 4\n",
		"objects/player/keyboard-40.gml":    "if place_free(x, y + 4) y += 4\n",
		"objects/player/collision-coin.gml": "score += 10\nwith (other) instance_destroy()\n",
		"objects/player/step.gml":           "// every coin collected: back to the start, the coins again\nif instance_number(coin) = 0 room_goto(room)\n",
		"AGENTS.md":                         agentsMD,
	}
	var room strings.Builder
	room.WriteString("caption = \"Collect the coins\"\nwidth = 640\nheight = 480\nspeed = 30\npersistent = false\nbackground_colour = \"#20304A\"\ndraw_background_colour = true\nviews_enabled = false\n")
	inst := func(obj string, x, y int) {
		fmt.Fprintf(&room, "\n[[instance]]\n  object = %q\n  x = %d\n  y = %d\n", obj, x, y)
	}
	for x := 0; x < 640; x += 32 {
		inst("wall", x, 0)
		inst("wall", x, 448)
	}
	for y := 32; y < 448; y += 32 {
		inst("wall", 0, y)
		inst("wall", 608, y)
	}
	for x := 160; x < 480; x += 32 { // a wall across the middle, a gap at each end
		inst("wall", x, 224)
	}
	inst("player", 64, 64)
	for _, c := range [][2]int{{288, 96}, {512, 96}, {96, 352}, {320, 352}, {544, 352}, {544, 224}} {
		inst("coin", c[0], c[1])
	}
	files["rooms/level/room.toml"] = room.String()
	for path, content := range files {
		if err := writeFile(filepath.Join(dir, path), []byte(content)); err != nil {
			return err
		}
	}
	for name, img := range map[string]image.Image{"wall": wallImage(), "player": playerImage(), "coin": coinImage()} {
		var b bytes.Buffer
		png.Encode(&b, img)
		if err := writeFile(filepath.Join(dir, "sprites", name, "strip.png"), b.Bytes()); err != nil {
			return err
		}
	}
	fmt.Printf("a new game in %s — try: gameviber check %s; gameviber play -keys \"right:60,down:60\" -shot frame.png %s; gameviber run %s\n", dir, dir, dir, dir)
	return nil
}

const spriteTOML = "frames = 1\nframe_w = 32\nframe_h = 32\norigin = [0, 0]\nbbox_lrbt = [0, 31, 31, 0]\nprecise = false\ntransparent = false\n"

const agentsMD = `# For agents

A GameViber project (GameMaker-style). Objects are objects/<name>/: object.toml (sprite, solid, depth …) and
one .gml file per event: create.gml, step.gml, keyboard-<key code>.gml (37 left, 38 up, 39 right, 40 down),
collision-<object>.gml, alarm-<n>.gml, draw.gml … Rooms are rooms/<name>/room.toml (instances, size, speed);
game.toml lists the room order. GML is GameMaker 5's language.

The loop: edit → ` + "`gameviber check .`" + ` (syntax, unknown functions, with file:line) →
` + "`gameviber play -keys \"right:40,down:30\" -json .`" + ` (headless: score, lives, instances, errors) or
` + "`-shot frame.png`" + ` to look → ` + "`gameviber run .`" + ` to play it. What GML and functions GameViber understands: ` + "`gameviber reference`" + `
(or docs/reference.md in GameViber's repository).
`

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func wallImage() image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := range 32 {
		for x := range 32 {
			c := color.NRGBA{0x5a, 0x6e, 0x8c, 255}
			if y%16 == 0 || (x+(y/16)*16)%32 == 0 { // brick joints
				c = color.NRGBA{0x3a, 0x48, 0x60, 255}
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

func playerImage() image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := range 32 {
		for x := range 32 {
			dx, dy := float64(x)-15.5, float64(y)-15.5
			if dx*dx+dy*dy < 15*15 {
				img.SetNRGBA(x, y, color.NRGBA{0x5c, 0xd6, 0x8a, 255})
			}
			if (math.Abs(dx-5) < 2 || math.Abs(dx+5) < 2) && math.Abs(dy+4) < 3 { // eyes
				img.SetNRGBA(x, y, color.NRGBA{0x10, 0x20, 0x18, 255})
			}
		}
	}
	return img
}

func coinImage() image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := range 32 {
		for x := range 32 {
			dx, dy := float64(x)-15.5, float64(y)-15.5
			if d := dx*dx/64 + dy*dy/100; d < 1 {
				c := color.NRGBA{0xf5, 0xc8, 0x3a, 255}
				if d > 0.7 {
					c = color.NRGBA{0xc8, 0x96, 0x1e, 255}
				}
				img.SetNRGBA(x, y, c)
			}
		}
	}
	return img
}
