package main

import (
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
