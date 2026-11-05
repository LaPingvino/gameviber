package main

import (
	"bytes"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPlatformerTemplate plays the platformer template through: every coin of
// level 1 by the jumps a player would make, then the flag into level 2; and
// level 2's gap jumped (its coin taken in the air) to the far platform.
func TestPlatformerTemplate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "plat")
	if err := cmdNew([]string{"-template", "platformer", dir}); err != nil {
		t.Fatal(err)
	}
	var rep strings.Builder
	if n, err := checkProjectVariant(dir, "", &rep); err != nil || n != 0 {
		t.Fatalf("check: %v %d problems\n%s", err, n, rep.String())
	}
	play := func(room, keys string) map[string]any {
		st, err := playProjectOpts(dir, playOpts{Room: room, Keys: keys, Steps: 300, Expect: "player.x * 1000 + player.y"})
		if err != nil {
			t.Fatal(err)
		}
		return st
	}
	level1 := "right:22,right+up:18,right:20,right+up:2,right:20,right:18,right+up:2,right:20,right:20,left:30,right:60"
	if st := play("", level1); st["room"] != "level2" || st["score"] != 40 {
		t.Errorf("level 1 through: room %v score %v, want level2 and 40", st["room"], st["score"])
	}
	gap := "right:53,right+up:2,right:26,right:6,right+up:2,right:22"
	if st := play("level2", gap); st["score"] != 20 || st["expect"].(map[string]any)["value"] != "472320" {
		t.Errorf("level 2's gap: score %v at %v, want 20 on the far platform (472,320)", st["score"], st["expect"])
	}
	if err := cmdNew([]string{"-template", "nonesuch", filepath.Join(t.TempDir(), "x")}); err == nil {
		t.Error("an unknown template made a game")
	}
}

// TestTopdownTemplate: three lives at the start, a gem taken by walking to it,
// and a ghost's catch costing a life and sending the player back to the start.
func TestTopdownTemplate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "td")
	if err := cmdNew([]string{"-template", "topdown", dir}); err != nil {
		t.Fatal(err)
	}
	var rep strings.Builder
	if n, err := checkProjectVariant(dir, "", &rep); err != nil || n != 0 {
		t.Fatalf("check: %v %d problems\n%s", err, n, rep.String())
	}
	play := func(keys string, steps int) (map[string]any, string) {
		st, err := playProjectOpts(dir, playOpts{Keys: keys, Steps: steps, Expect: "lives * 1000000 + player.x * 1000 + player.y"})
		if err != nil {
			t.Fatal(err)
		}
		return st, st["expect"].(map[string]any)["value"].(string)
	}
	if st, v := play("right:30", 40); st["score"] != 10 || v != "3160040" {
		t.Errorf("the first gem: score %v, lives/x/y %v; want 10, 3160040", st["score"], v)
	}
	if _, v := play("right:30", 300); v != "2040040" {
		t.Errorf("waiting for a ghost: lives/x/y %v, want 2040040 (a life less, back at the start)", v)
	}
}

// TestShooterTemplate: sweeping fire scores; left alone, enemies keep coming
// (the spawner's alarm re-arms itself) and those that get past cost lives.
func TestShooterTemplate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sh")
	if err := cmdNew([]string{"-template", "shooter", dir}); err != nil {
		t.Fatal(err)
	}
	var rep strings.Builder
	if n, err := checkProjectVariant(dir, "", &rep); err != nil || n != 0 {
		t.Fatalf("check: %v %d problems\n%s", err, n, rep.String())
	}
	sweep := "space+left:30,space+right:60,space+left:60,space+right:60,space+left:60,space+right:60"
	st, err := playProjectOpts(dir, playOpts{Keys: sweep, Steps: 340})
	if err != nil {
		t.Fatal(err)
	}
	if s, _ := st["score"].(int); s <= 0 {
		t.Errorf("sweeping fire: score %v", st["score"])
	}
	st, err = playProjectOpts(dir, playOpts{Steps: 300, Expect: "lives"})
	if err != nil {
		t.Fatal(err)
	}
	if n := st["instances"].(map[string]int)["enemy"]; n < 3 {
		t.Errorf("left alone for 300 steps: %d enemies on screen, want the spawner to keep sending them", n)
	}
	if v := st["expect"].(map[string]any)["value"]; v != "2" {
		t.Errorf("lives after one enemy got past: %v, want 2", v)
	}
}

// TestPathsInAGame: a path in the project (paths/NAME.toml), started from GML,
// moves the player; its end event runs; path_index names it while on it.
func TestPathsInAGame(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "g")
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(dir, "paths"), 0o755)
	os.WriteFile(filepath.Join(dir, "paths", "tour.toml"), []byte("smooth = false\nclosed = false\nprecision = 4\npoints = [[0, 0, 100], [64, 0, 100]]\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "objects", "player", "create.gml"), []byte("score = 0\npath_start(tour, 8, 0, false)\nwas = path_index\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "objects", "player", "other-8.gml"), []byte("score += 1\nshow_debug_message(\"end at \" + string(x))\n"), 0o644)
	var rep strings.Builder
	if n, err := checkProjectVariant(dir, "", &rep); err != nil || n != 0 {
		t.Fatalf("check: %v %d\n%s", err, n, rep.String())
	}
	st, err := playProjectOpts(dir, playOpts{Steps: 20, Expect: "player.was"})
	if err != nil {
		t.Fatal(err)
	}
	if st["score"] != 1 || st["expect"].(map[string]any)["value"] != "0" {
		t.Errorf("score %v, path_index while on it %v", st["score"], st["expect"])
	}
	if d := st["debug"].([]string); len(d) != 1 || d[0] != "end at 128" {
		t.Errorf("debug %q (the player starts at 64; 64 px along)", d)
	}
}

// TestPathActions: the D&D path actions as an imported game's converted
// actions call them — action_path (relative), action_path_speed relative,
// action_path_position, action_path_end.
func TestPathActions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "g")
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(dir, "paths"), 0o755)
	os.WriteFile(filepath.Join(dir, "paths", "tour.toml"), []byte("points = [[0, 0, 100], [100, 0, 100]]\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "objects", "player", "create.gml"), []byte("score = 0\naction_path(tour, 4, 0, 0)\naction_path_speed(6, 1)\naction_path_position(0.5, 0)\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "objects", "player", "other-8.gml"), []byte("score += 1\nshow_debug_message(string(x))\naction_path_end()\n"), 0o644)
	st, err := playProjectOpts(dir, playOpts{Steps: 6, Expect: "player.x"})
	if err != nil {
		t.Fatal(err)
	}
	// starts at 64 (the player's place), jumps to halfway (+50), then 10 px a step: the end (164) at step 5
	if st["score"] != 1 || st["expect"].(map[string]any)["value"] != "164" {
		t.Errorf("score %v x %v, debug %v", st["score"], st["expect"], st["debug"])
	}
}

// TestDrawShapes: GM 5's draw_circle / draw_ellipse / draw_line / draw_point
// from a draw event, in the brush and pen colours, seen in a headless shot.
func TestDrawShapes(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "g")
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "objects", "player", "draw.gml"), []byte(`brush_color = c_red
pen_color = c_white
draw_circle(320, 240, 40)
draw_ellipse(100, 100, 200, 150)
pen_color = c_yellow
draw_line(400, 100, 500, 150)
draw_point(600, 400)
`), 0o644)
	var shot bytes.Buffer
	if _, err := playProjectOpts(dir, playOpts{Steps: 2, Shot: &shot}); err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(&shot)
	if err != nil {
		t.Fatal(err)
	}
	at := func(x, y int) [3]uint32 { r, g, b, _ := img.At(x, y).RGBA(); return [3]uint32{r >> 8, g >> 8, b >> 8} }
	red, white, yellow := [3]uint32{255, 0, 0}, [3]uint32{255, 255, 255}, [3]uint32{255, 255, 0}
	if at(320, 240) != red || at(320, 200) != white || at(320, 280) != white {
		t.Errorf("circle: centre %v top %v bottom %v", at(320, 240), at(320, 200), at(320, 280))
	}
	if at(150, 125) != red || at(100, 125) != white {
		t.Errorf("ellipse: centre %v left %v", at(150, 125), at(100, 125))
	}
	if at(450, 125) != yellow || at(600, 400) != yellow {
		t.Errorf("line middle %v, point %v", at(450, 125), at(600, 400))
	}
}

// draw_triangle, draw_roundrect and draw_arrow in a frame: the brush inside,
// the pen on the edges; the shot kept in GAMEVIBER_SHOT_DIR when set (for a
// look).
func TestDrawTriangleRoundrectArrow(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "g")
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "objects", "player", "draw.gml"), []byte(`brush_color = c_red
pen_color = c_white
draw_triangle(100, 300, 200, 300, 150, 200)
draw_roundrect(300, 100, 460, 180)
pen_color = c_yellow
draw_arrow(100, 420, 300, 420, 20)
`), 0o644)
	var shot bytes.Buffer
	if _, err := playProjectOpts(dir, playOpts{Steps: 2, Shot: &shot}); err != nil {
		t.Fatal(err)
	}
	if d := os.Getenv("GAMEVIBER_SHOT_DIR"); d != "" {
		os.WriteFile(filepath.Join(d, "shapes2.png"), shot.Bytes(), 0o644)
	}
	img, err := png.Decode(bytes.NewReader(shot.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	at := func(x, y int) [3]uint32 { r, g, b, _ := img.At(x, y).RGBA(); return [3]uint32{r >> 8, g >> 8, b >> 8} }
	red, white, yellow := [3]uint32{255, 0, 0}, [3]uint32{255, 255, 255}, [3]uint32{255, 255, 0}
	if at(150, 270) != red || at(150, 300) != white || at(100, 250) == red {
		t.Errorf("triangle: inside %v, base %v, outside %v", at(150, 270), at(150, 300), at(100, 250))
	}
	if at(380, 140) != red || at(380, 100) != white || at(300, 140) != white || at(300, 100) == white {
		t.Errorf("roundrect: inside %v, top %v, left %v, corner %v", at(380, 140), at(380, 100), at(300, 140), at(300, 100))
	}
	if at(200, 420) != yellow || at(290, 415) != yellow || at(290, 425) != yellow {
		t.Errorf("arrow: line %v, head %v %v", at(200, 420), at(290, 415), at(290, 425))
	}
}

// draw_sprite_tiled covers the room with the wall; draw_sprite_stretched
// fills its box (the coin wide, the player tall): what the shot looked like.
func TestDrawSpriteTiledStretched(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "g")
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "objects", "player", "draw.gml"), []byte(`draw_sprite_tiled(wall, 0, 0, 0)
draw_sprite_stretched(coin, 0, 100, 100, 200, 120)
draw_sprite_stretched(player, 0, 340, 100, 60, 180)
`), 0o644)
	var shot bytes.Buffer
	if _, err := playProjectOpts(dir, playOpts{Steps: 2, Shot: &shot}); err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(&shot)
	if err != nil {
		t.Fatal(err)
	}
	at := func(x, y int) [3]uint32 { r, g, b, _ := img.At(x, y).RGBA(); return [3]uint32{r >> 8, g >> 8, b >> 8} }
	if a, b := at(600, 450), at(5, 5); a != b || a == at(200, 160) {
		t.Errorf("tiled wall: %v and %v should match (and differ from the coin %v)", a, b, at(200, 160))
	}
	if c := at(200, 160); c[0] < 200 || c[2] > 120 {
		t.Errorf("stretched coin centre %v, want yellow", c)
	}
	if p := at(370, 200); p[1] < 180 || p[0] > 150 {
		t.Errorf("stretched player %v, want green", p)
	}
}

// A test play's clock moves by steps: current_time reads the same in every
// run — 0 on the first step, 1000 a second of steps later (room speed 30).
func TestPlayClockRepeats(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "g")
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "objects", "player", "step.gml"), []byte(`show_debug_message(string(current_time) + " " + string(current_year))
`), 0o644)
	var runs [2][]string
	for i := range runs {
		res, err := playProjectOpts(dir, playOpts{Steps: 31})
		if err != nil {
			t.Fatal(err)
		}
		d, _ := res["debug"].([]string)
		runs[i] = d
	}
	if len(runs[0]) < 31 || fmt.Sprint(runs[0]) != fmt.Sprint(runs[1]) {
		t.Fatalf("runs differ or too short: %q / %q", runs[0], runs[1])
	}
	if runs[0][0] != "0 2026" || runs[0][30] != "1000 2026" {
		t.Errorf("first %q, after 30 steps %q", runs[0][0], runs[0][30])
	}
}
