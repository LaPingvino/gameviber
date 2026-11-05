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
