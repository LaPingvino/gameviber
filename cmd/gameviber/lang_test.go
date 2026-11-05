package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/LaPingvino/gameviber/import/gmfile"
	"github.com/LaPingvino/gameviber/load"
	"github.com/LaPingvino/gameviber/runner"
)

// The same game written in GML and in JavaScript plays the same: the coins
// starter and the coins-js template (its player's events in JavaScript), driven by one key script through
// the coins, compared every step (the player's position, the score, the
// instances left).
func TestStarterGMLvsJavaScript(t *testing.T) { starterVersus(t, "coins-js", "step.js") }

// The same for Lua: the coins-lua template against the GML starter.
func TestStarterGMLvsLua(t *testing.T) { starterVersus(t, "coins-lua", "step.lua") }

func starterVersus(t *testing.T, tmpl, stepFile string) {
	base := t.TempDir()
	gmlDir, jsDir := filepath.Join(base, "gml"), filepath.Join(base, "js")
	if err := cmdNew([]string{gmlDir}); err != nil {
		t.Fatal(err)
	}
	if err := cmdNew([]string{"-template", tmpl, jsDir}); err != nil { // the other language's starting point
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(jsDir, "objects", "player", stepFile)); err != nil {
		t.Fatalf("%s has no %s", tmpl, stepFile)
	}
	start := func(dir string) (*load.Game, *load.Play) {
		p, err := gmfile.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		g, err := load.FromProject(p)
		if err != nil {
			t.Fatal(err)
		}
		pl, err := g.StartAt("level")
		if err != nil {
			t.Fatal(err)
		}
		return g, pl
	}
	gG, gPl := start(gmlDir)
	jG, jPl := start(jsDir)

	spans, err := parseKeys("right:60,down:75,right:60,down:10,left:120,up:30,left:60,down:40,right:200,up:80,right:30")
	if err != nil {
		t.Fatal(err)
	}
	state := func(pl *load.Play) string {
		var b strings.Builder
		for _, in := range pl.Room.All("player") {
			fmt.Fprintf(&b, "player %v,%v ", in.X, in.Y)
		}
		var coins []string
		for _, in := range pl.Room.All("coin") {
			coins = append(coins, fmt.Sprintf("%v,%v", in.X, in.Y))
		}
		sort.Strings(coins)
		fmt.Fprintf(&b, "score %d coins [%s] room %s", pl.Globals.Score, strings.Join(coins, " "), pl.Room.Name)
		return b.String()
	}
	gk, jk := &keySet{held: map[runner.Key]bool{}}, &keySet{held: map[runner.Key]bool{}}
	step, startScore := 0, ""
	for _, sp := range spans {
		for range sp.steps {
			for _, k := range []*keySet{gk, jk} {
				k.prev, k.held = k.held, map[runner.Key]bool{}
				for _, key := range sp.keys {
					k.held[key] = true
				}
			}
			if err := gPl.Step(gk); err != nil {
				t.Fatal(err)
			}
			if err := jPl.Step(jk); err != nil {
				t.Fatal(err)
			}
			step++
			if a, b := state(gPl), state(jPl); a != b {
				t.Fatalf("step %d differs:\n GML   %s\n %s %s", step, a, tmpl, b)
			}
			if step == 1 {
				startScore = state(gPl)
			}
		}
	}
	if len(gG.Errors) > 0 || len(jG.Errors) > 0 {
		t.Fatalf("errors: GML %v, %s %v", gG.Errors, tmpl, jG.Errors)
	}
	t.Logf("%d steps identical; first %s; last %s", step, startScore, state(gPl))
	if gPl.Globals.Score == 0 {
		t.Error("the key script collected no coin: the comparison saw no collision event")
	}
}
