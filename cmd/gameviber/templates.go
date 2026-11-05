package main

import (
	"fmt"
	"image"
	"image/color"
	"strings"
)

// Starting points (LOOP item 10): `gameviber new -template NAME DIR`. Each is
// a small but whole game in GameViber's own files and pictures.

// templates are the names -template takes ("coins" is the starter of before).
var templates = []string{"coins", "platformer"}

// platformerFiles is a platformer: run (left / right), jump (up) with
// gravity, land on walls, collect the coins, then touch the flag for the next
// level (after the last: the first again); falling off restarts the level.
func platformerFiles(name string) (map[string]string, map[string]image.Image) {
	obj := func(sprite string, solid bool, depth int) string {
		return fmt.Sprintf("sprite = %q\nsolid = %v\nvisible = true\ndepth = %d\npersistent = false\nparent = \"-1\"\nmask = \"-1\"\n", sprite, solid, depth)
	}
	files := map[string]string{
		"game.toml":                  fmt.Sprintf("name = %q\ngm_version = 530\nroom_order = [\"level1\", \"level2\"]\n\n[settings]\n  author = \"\"\n  icon = \"player\"\n", name),
		"sprites/wall/sprite.toml":   spriteTOML,
		"sprites/player/sprite.toml": "frames = 1\nframe_w = 32\nframe_h = 32\norigin = [0, 0]\nbbox_lrbt = [4, 27, 31, 2]\nprecise = false\ntransparent = true\n",
		"sprites/coin/sprite.toml":   "frames = 1\nframe_w = 32\nframe_h = 32\norigin = [0, 0]\nbbox_lrbt = [8, 23, 23, 8]\nprecise = false\ntransparent = true\n",
		"sprites/flag/sprite.toml":   "frames = 1\nframe_w = 32\nframe_h = 32\norigin = [0, 0]\nbbox_lrbt = [8, 23, 31, 0]\nprecise = false\ntransparent = true\n",
		"objects/wall/object.toml":   obj("wall", true, 0),
		"objects/coin/object.toml":   obj("coin", false, 0),
		"objects/flag/object.toml":   obj("flag", false, 1),
		"objects/player/object.toml": obj("player", false, -1),
		"objects/player/create.gml":  "// vertical speed: gravity adds to it, a jump sets it upwards\nvsp = 0\n",
		"objects/player/step.gml": `// run
if keyboard_check(vk_left) && place_free(x - 4, y) x -= 4
if keyboard_check(vk_right) && place_free(x + 4, y) x += 4
// jump, but only standing on something
if keyboard_check(vk_up) && !place_free(x, y + 1) vsp = -15
// gravity, up to a falling speed
vsp += 1
if vsp > 10 vsp = 10
// move up or down a pixel at a time: stop at walls (landing, or a head bump)
repeat (abs(vsp)) {
  if place_free(x, y + sign(vsp)) y += sign(vsp) else { vsp = 0; break }
}
// stay inside the room at the sides
x = max(0, min(x, room_width - 32))
// fell off the bottom: this level again
if y > room_height room_restart()
`,
		"objects/player/collision-coin.gml": "score += 10\nwith (other) instance_destroy()\n",
		"objects/player/collision-flag.gml": `// every coin collected: the next level (after the last, the first again)
if instance_number(coin) = 0 {
  if room_next(room) >= 0 room_goto_next() else room_goto(level1)
}
`,
		"AGENTS.md": agentsMD + "\nThis game is a platformer: objects/player/step.gml has the running, jumping and gravity; touch the flag after all\ncoins for the next level. `gameviber play -keys \"right:40,right+up:20\" -expect \"score > 0\" .` tests a change.\n",
	}
	level := func(caption string, platforms [][3]int, coins [][2]int, flag [2]int) string {
		var b strings.Builder
		fmt.Fprintf(&b, "caption = %q\nwidth = 640\nheight = 480\nspeed = 30\npersistent = false\nbackground_colour = \"#7EC8E3\"\ndraw_background_colour = true\nviews_enabled = false\n", caption)
		inst := func(o string, x, y int) {
			fmt.Fprintf(&b, "\n[[instance]]\n  object = %q\n  x = %d\n  y = %d\n", o, x, y)
		}
		for _, p := range platforms { // x from, x to, y
			for x := p[0]; x <= p[1]; x += 32 {
				inst("wall", x, p[2])
			}
		}
		for _, c := range coins {
			inst("coin", c[0], c[1])
		}
		inst("flag", flag[0], flag[1])
		inst("player", 32, 384)
		return b.String()
	}
	files["rooms/level1/room.toml"] = level("Level 1 — collect the coins, then the flag",
		[][3]int{{0, 608, 448}, {192, 288, 352}, {352, 448, 288}, {512, 608, 224}}, [][2]int{{224, 320}, {384, 256}, {576, 192}, {96, 416}}, [2]int{608, 416})
	files["rooms/level2/room.toml"] = level("Level 2 — mind the gap",
		[][3]int{{0, 224, 448}, {352, 608, 448}, {128, 192, 352}, {448, 512, 352}}, [][2]int{{160, 320}, {288, 352}, {480, 320}, {576, 416}}, [2]int{608, 416})
	return files, map[string]image.Image{"wall": wallImage(), "player": playerImage(), "coin": coinImage(), "flag": flagImage()}
}

func flagImage() image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := range 32 {
		for x := range 32 {
			switch {
			case x >= 8 && x <= 10: // the pole
				img.SetNRGBA(x, y, color.NRGBA{0x50, 0x50, 0x58, 255})
			case x > 10 && x <= 26 && y >= 2 && y <= 14 && (x-10) <= 16-(y-2)/2: // the pennant
				img.SetNRGBA(x, y, color.NRGBA{0xe8, 0x4a, 0x3c, 255})
			}
		}
	}
	return img
}
