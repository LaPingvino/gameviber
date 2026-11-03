package main

import (
	"github.com/LaPingvino/gameviber/runner"
	"os"
	"strings"
	"testing"
)

func TestParseKeys(t *testing.T) {
	sp, err := parseKeys("right:40, right+up:25,:10")
	if err != nil {
		t.Fatal(err)
	}
	if len(sp) != 3 || sp[0].steps != 40 || len(sp[1].keys) != 2 || sp[1].keys[1] != 38 || sp[2].keys != nil || sp[2].steps != 10 {
		t.Errorf("%+v", sp)
	}
	for _, bad := range []string{"jump:3", "right", "right:-1"} {
		if _, err := parseKeys(bad); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
}

// docs/reference.md is what gameviber reference writes (regenerate:
// go run ./cmd/gameviber reference docs/reference.md).
func TestReferenceUpToDate(t *testing.T) {
	var b strings.Builder
	writeReference(&b)
	got, err := os.ReadFile("../../docs/reference.md")
	if err != nil || string(got) != b.String() {
		t.Error("docs/reference.md is out of date: go run ./cmd/gameviber reference docs/reference.md")
	}
}

// check's hints: later GameMaker names get their 5.3 form, typos the nearest
// known name, and every 5.3 form named is a function GameViber has.
func TestHints(t *testing.T) {
	known := map[string]bool{"instance_create": true, "sound_play": true, "floor": true, "random": true}
	if h := hint("irandom", known); !strings.Contains(h, "floor(random(n + 1))") {
		t.Errorf("irandom: %q", h)
	}
	if h := hint("instance_crate", known); h != "did you mean instance_create?" {
		t.Errorf("typo: %q", h)
	}
	if h := hint("something_else_entirely", known); h != "" {
		t.Errorf("far: %q", h)
	}
	real := (&runner.Game{}).KnownFuncs()
	for fn := range laterGM {
		if real[fn] {
			t.Errorf("%s is in laterGM but GameViber knows it", fn)
		}
	}
}
