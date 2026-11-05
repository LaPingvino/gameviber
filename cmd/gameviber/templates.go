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
var templates = []string{"coins", "coins-js", "coins-lua", "platformer", "topdown", "shooter"}

// starterJS is the coins starter's player events in JavaScript, one for one
// with its GML (TestStarterGMLvsJavaScript plays both and finds them the same):
// the "coins-js" template.
var starterJS = map[string]string{
	"create":         "score = 0;\n",
	"keyboard-37":    "if (place_free(x - 4, y)) x -= 4;\n",
	"keyboard-39":    "if (place_free(x + 4, y)) x += 4;\n",
	"keyboard-38":    "if (place_free(x, y - 4)) y -= 4;\n",
	"keyboard-40":    "if (place_free(x, y + 4)) y += 4;\n",
	"collision-coin": "score += 10;\nother.instance_destroy(); // GML: with (other) instance_destroy()\n",
	"step":           "// every coin collected: back to the start, the coins again\nif (instance_number(coin) == 0) room_goto(room);\n",
}

// starterLua is the same in Lua: the "coins-lua" template.
var starterLua = map[string]string{
	"create":         "score = 0\n",
	"keyboard-37":    "if place_free(x - 4, y) then x = x - 4 end\n",
	"keyboard-39":    "if place_free(x + 4, y) then x = x + 4 end\n",
	"keyboard-38":    "if place_free(x, y - 4) then y = y - 4 end\n",
	"keyboard-40":    "if place_free(x, y + 4) then y = y + 4 end\n",
	"collision-coin": "score = score + 10\nother:instance_destroy() -- GML: with (other) instance_destroy()\n",
	"step":           "-- every coin collected: back to the start, the coins again\nif instance_number(coin) == 0 then room_goto(room) end\n",
}

// starterLangs are the coins starter's player events in other languages, by
// template name and file extension.
var starterLangs = map[string]struct {
	ext    string
	events map[string]string
}{"coins-js": {".js", starterJS}, "coins-lua": {".lua", starterLua}}

// templateFiles makes each template's files and pictures (all but "coins").
var templateFiles = map[string]func(name string) (map[string]string, map[string]image.Image){
	"platformer": platformerFiles,
	"topdown":    topdownFiles,
	"shooter":    shooterFiles,
}

func objectTOML(sprite string, solid bool, depth int) string {
	return fmt.Sprintf("sprite = %q\nsolid = %v\nvisible = true\ndepth = %d\npersistent = false\nparent = \"-1\"\nmask = \"-1\"\n", sprite, solid, depth)
}

// topdownFiles is a gem hunt seen from above: move in four directions among
// wall blocks, collect the gems; two ghosts wander (turning at walls) and
// chase the player when near; a ghost's touch costs a life and sends the
// player back to the start; three lives, then the game starts over; every
// gem collected: the room again.
func topdownFiles(name string) (map[string]string, map[string]image.Image) {
	small := "frames = 1\nframe_w = 32\nframe_h = 32\norigin = [0, 0]\nbbox_lrbt = [4, 27, 27, 4]\nprecise = false\ntransparent = true\n"
	files := map[string]string{
		"game.toml":                  fmt.Sprintf("name = %q\ngm_version = 530\nroom_order = [\"maze\"]\n\n[settings]\n  author = \"\"\n  icon = \"player\"\n", name),
		"sprites/wall/sprite.toml":   spriteTOML,
		"sprites/player/sprite.toml": small,
		"sprites/ghost/sprite.toml":  small,
		"sprites/gem/sprite.toml":    "frames = 1\nframe_w = 32\nframe_h = 32\norigin = [0, 0]\nbbox_lrbt = [8, 23, 23, 8]\nprecise = false\ntransparent = true\n",
		"objects/wall/object.toml":   objectTOML("wall", true, 0),
		"objects/gem/object.toml":    objectTOML("gem", false, 1),
		"objects/ghost/object.toml":  objectTOML("ghost", false, -1),
		"objects/player/object.toml": objectTOML("player", false, -2),
		"objects/player/other-2.gml": "// game start: three lives, shown in the caption with the score\nlives = 3\nshow_lives = true\n",
		"objects/player/other-6.gml": "// no more lives: start the whole game over\ngame_restart()\n",
		"objects/player/step.gml": `// move in four directions, not into walls
if keyboard_check(vk_left) && place_free(x - 4, y) x -= 4
if keyboard_check(vk_right) && place_free(x + 4, y) x += 4
if keyboard_check(vk_up) && place_free(x, y - 4) y -= 4
if keyboard_check(vk_down) && place_free(x, y + 4) y += 4
// every gem collected: the room again (gems and ghosts back)
if instance_number(gem) = 0 room_restart()
`,
		"objects/player/collision-gem.gml":   "score += 10\nwith (other) instance_destroy()\n",
		"objects/player/collision-ghost.gml": "// caught: a life less, back to the start (the ghost to its own, so it can't wait there)\nlives -= 1\nx = xstart\ny = ystart\nwith (other) { x = xstart; y = ystart }\n",
		"objects/ghost/create.gml":           "// the way it is going: dx, dy each -1, 0 or 1\ndx = choose(-1, 1)\ndy = 0\n",
		"objects/ghost/step.gml": `// near the player: head for them along the longer distance
if point_distance(x, y, player.x, player.y) < 160 {
  if abs(player.x - x) > abs(player.y - y) { dx = sign(player.x - x); dy = 0 } else { dx = 0; dy = sign(player.y - y) }
}
// go on, or at a wall pick another way at random
if place_free(x + dx * 2, y + dy * 2) { x += dx * 2; y += dy * 2 } else {
  d = choose(0, 1, 2, 3)
  dx = 0
  dy = 0
  if d = 0 dx = 1 else if d = 1 dx = -1 else if d = 2 dy = 1 else dy = -1
}
`,
		"AGENTS.md": agentsMD + "\nThis game is a top-down gem hunt: objects/player/step.gml moves the player, objects/ghost/step.gml has the\nghosts' wandering and chasing, objects/player/collision-ghost.gml what a catch costs.\n`gameviber play -keys \"right:30,down:20\" -expect \"score > 0\" .` tests a change.\n",
	}
	var b strings.Builder
	b.WriteString("caption = \"Gem hunt\"\nwidth = 640\nheight = 480\nspeed = 30\npersistent = false\nbackground_colour = \"#2E4A2E\"\ndraw_background_colour = true\nviews_enabled = false\n")
	inst := func(o string, x, y int) {
		fmt.Fprintf(&b, "\n[[instance]]\n  object = %q\n  x = %d\n  y = %d\n", o, x, y)
	}
	for x := 0; x < 640; x += 32 {
		inst("wall", x, 0)
		inst("wall", x, 448)
	}
	for y := 32; y < 448; y += 32 {
		inst("wall", 0, y)
		inst("wall", 608, y)
	}
	for _, bx := range []int{96, 224, 352, 480} { // 2x2 blocks, two rows
		for _, by := range []int{96, 256} {
			for _, d := range [][2]int{{0, 0}, {32, 0}, {0, 32}, {32, 32}} {
				inst("wall", bx+d[0], by+d[1])
			}
		}
	}
	for _, g := range [][2]int{{176, 48}, {432, 48}, {48, 192}, {304, 192}, {560, 192}, {176, 368}, {432, 368}, {560, 400}} {
		inst("gem", g[0], g[1])
	}
	inst("ghost", 304, 400)
	inst("ghost", 560, 48)
	inst("player", 40, 40)
	files["rooms/maze/room.toml"] = b.String()
	return files, map[string]image.Image{"wall": wallImage(), "player": playerImage(), "ghost": ghostImage(), "gem": gemImage()}
}

func ghostImage() image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	body := color.NRGBA{0xc0, 0x7c, 0xe8, 255}
	for y := range 32 {
		for x := range 32 {
			dx, dy := float64(x)-15.5, float64(y)-13
			in := (dy < 0 && dx*dx+dy*dy < 13*13) || (dy >= 0 && y < 28 && dx > -13 && dx < 13)
			if y >= 24 && y < 28 && (x/5)%2 == 1 { // the wavy hem
				in = false
			}
			if in {
				img.SetNRGBA(x, y, body)
			}
			if (dx+5)*(dx+5)+(dy+2)*(dy+2) < 6 || (dx-5)*(dx-5)+(dy+2)*(dy+2) < 6 { // eyes
				img.SetNRGBA(x, y, color.NRGBA{255, 255, 255, 255})
			}
		}
	}
	return img
}

func gemImage() image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := range 32 {
		for x := range 32 {
			dx, dy := x-16, y-16
			if dx < 0 {
				dx = -dx
			}
			if dy < 0 {
				dy = -dy
			}
			if dx+dy <= 8 {
				c := color.NRGBA{0x4a, 0xd8, 0xe8, 255}
				if x < 16 && y < 16 {
					c = color.NRGBA{0xb0, 0xf4, 0xfc, 255} // the light facet
				}
				img.SetNRGBA(x, y, c)
			}
		}
	}
	return img
}

// platformerFiles is a platformer: run (left / right), jump (up) with
// gravity, land on walls, collect the coins, then touch the flag for the next
// level (after the last: the first again); falling off restarts the level.
func platformerFiles(name string) (map[string]string, map[string]image.Image) {
	obj := objectTOML
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

// shooterFiles is a shooter: the ship moves left / right along the bottom and
// fires with space (a short cooldown); enemies drop from the top, more often
// as the score rises (the spawner's alarm); a hit scores 10; an enemy that
// gets past or rams the ship costs a life; three lives, then the game starts
// over.
func shooterFiles(name string) (map[string]string, map[string]image.Image) {
	sprite := func(l, r, b, t int) string {
		return fmt.Sprintf("frames = 1\nframe_w = 32\nframe_h = 32\norigin = [16, 16]\nbbox_lrbt = [%d, %d, %d, %d]\nprecise = false\ntransparent = true\n", l, r, b, t)
	}
	files := map[string]string{
		"game.toml":                   fmt.Sprintf("name = %q\ngm_version = 530\nroom_order = [\"space\"]\n\n[settings]\n  author = \"\"\n  icon = \"ship\"\n", name),
		"sprites/ship/sprite.toml":    sprite(4, 27, 29, 6),
		"sprites/enemy/sprite.toml":   sprite(4, 27, 27, 4),
		"sprites/bullet/sprite.toml":  sprite(14, 17, 23, 8),
		"objects/ship/object.toml":    objectTOML("ship", false, -1),
		"objects/enemy/object.toml":   objectTOML("enemy", false, 0),
		"objects/bullet/object.toml":  objectTOML("bullet", false, 1),
		"objects/spawner/object.toml": objectTOML("-1", false, 0),
		"objects/ship/create.gml":     "// frames until the next shot may go\ncooldown = 0\n",
		"objects/ship/other-2.gml":    "// game start: three lives, shown in the caption with the score\nlives = 3\nshow_lives = true\n",
		"objects/ship/other-6.gml":    "// no more lives: start the whole game over\ngame_restart()\n",
		"objects/ship/step.gml": `// move along the bottom
if keyboard_check(vk_left) && x > 24 x -= 6
if keyboard_check(vk_right) && x < room_width - 24 x += 6
// fire, at most every 8 steps
cooldown -= 1
if keyboard_check(vk_space) && cooldown <= 0 {
  instance_create(x, y - 20, bullet)
  cooldown = 8
}
`,
		"objects/ship/collision-enemy.gml":   "// rammed: a life less, the enemy gone\nlives -= 1\nwith (other) instance_destroy()\n",
		"objects/bullet/create.gml":          "vspeed = -10\n",
		"objects/bullet/other-0.gml":         "// off the top: gone\ninstance_destroy()\n",
		"objects/bullet/collision-enemy.gml": "score += 10\nwith (other) instance_destroy()\ninstance_destroy()\n",
		"objects/enemy/create.gml":           "// down, a little faster or slower each\nvspeed = 1.5 + random(1.5)\n",
		"objects/enemy/other-0.gml":          "// got past the ship: a life less\nif y > room_height {\n  lives -= 1\n  instance_destroy()\n}\n",
		"objects/spawner/create.gml":         "alarm[0] = 30\n",
		"objects/spawner/alarm-0.gml": `// an enemy somewhere along the top, then the next one: sooner as the score rises
instance_create(32 + random(room_width - 64), -16, enemy)
alarm[0] = max(12, 45 - score / 20)
`,
		"AGENTS.md": agentsMD + "\nThis game is a shooter: objects/ship/step.gml moves and fires, objects/spawner/alarm-0.gml sends the enemies (how\noften), objects/enemy/create.gml how fast they fall. `gameviber play -keys \"space:120\" -expect \"score > 0\" .` tests a change.\n",
	}
	files["rooms/space/room.toml"] = "caption = \"Star defence\"\nwidth = 480\nheight = 640\nspeed = 30\npersistent = false\nbackground_colour = \"#0B1026\"\ndraw_background_colour = true\nviews_enabled = false\n" +
		"\n[[instance]]\n  object = \"spawner\"\n  x = 0\n  y = 0\n" +
		"\n[[instance]]\n  object = \"ship\"\n  x = 240\n  y = 596\n"
	return files, map[string]image.Image{"ship": shipImage(), "enemy": enemyImage(), "bullet": bulletImage()}
}

func shipImage() image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := 6; y < 30; y++ {
		half := (y - 6) / 2 // a triangle widening downwards
		for x := 16 - half; x < 16+half; x++ {
			img.SetNRGBA(x, y, color.NRGBA{0x9c, 0xd4, 0xff, 255})
		}
	}
	for y := 16; y < 22; y++ { // the cockpit
		for x := 14; x < 18; x++ {
			img.SetNRGBA(x, y, color.NRGBA{0x20, 0x40, 0x80, 255})
		}
	}
	return img
}

func enemyImage() image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := range 32 {
		for x := range 32 {
			dx, dy := float64(x)-15.5, float64(y)-15.5
			if dx*dx/144+dy*dy/64 < 1 { // a saucer
				img.SetNRGBA(x, y, color.NRGBA{0xf0, 0x60, 0x50, 255})
			}
			if dx*dx+(dy+4)*(dy+4) < 25 && dy < -2 { // its dome
				img.SetNRGBA(x, y, color.NRGBA{0xff, 0xd0, 0x80, 255})
			}
		}
	}
	return img
}

func bulletImage() image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := 8; y < 24; y++ {
		for x := 14; x < 18; x++ {
			img.SetNRGBA(x, y, color.NRGBA{0xff, 0xf0, 0x60, 255})
		}
	}
	return img
}
