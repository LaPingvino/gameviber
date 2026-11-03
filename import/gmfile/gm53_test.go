package gmfile

import (
	"encoding/json"
	"os"
	"testing"
)

// Stelo's 2006 executable (in the Stelo repo, a sibling checkout) opens:
// marker, seed, the project header and format 530.
func TestOpenExe(t *testing.T) {
	d, err := os.ReadFile("../../../stelo/win/stelo.exe")
	if err != nil {
		t.Skip("Stelo's exe is not checked out next to this repo")
	}
	e, err := OpenExe(d)
	if err != nil {
		t.Fatal(err)
	}
	if e.Version != 530 {
		t.Errorf("format %d, want 530", e.Version)
	}
	t.Logf("seed %d, project at file offset %d", e.Seed, e.P+e.Base)
}

func TestSwapTables(t *testing.T) {
	tab, inv := swapTables(12345)
	seen := map[byte]bool{}
	for i := range 256 {
		seen[tab[i]] = true
		if inv[tab[i]] != byte(i) {
			t.Fatalf("inverse wrong at %d", i)
		}
	}
	if len(seen) != 256 || tab[0] != 0 {
		t.Error("not a permutation keeping 0")
	}
}

func TestCP1252(t *testing.T) {
	if got := cp1252([]byte{'a', 0x80, 0xE9, 0x93}); got != "a€é“" {
		t.Errorf("%q", got)
	}
}

// Stelo's settings, sounds and sprites as gm53-reader read them
// (stelo/original/project.json).
func TestReadProjectStelo(t *testing.T) {
	d, err := os.ReadFile("../../../stelo/win/stelo.exe")
	if err != nil {
		t.Skip("Stelo's exe is not checked out next to this repo")
	}
	js, err := os.ReadFile("../../../stelo/original/project.json")
	if err != nil {
		t.Skip("Stelo's recovered project.json is not checked out")
	}
	var want struct {
		Settings struct {
			Esc    []bool `json:"f4_f1_esc_f5"`
			Author string `json:"author"`
		} `json:"settings"`
		Sounds []*struct {
			Name string `json:"name"`
		} `json:"sounds"`
		Sprites []*struct {
			Name   string  `json:"name"`
			BBox   []int32 `json:"bbox_lrbt"`
			Frames []any   `json:"frames"`
		} `json:"sprites"`
	}
	if err := json.Unmarshal(js, &want); err != nil {
		t.Fatal(err)
	}
	e, err := OpenExe(d)
	if err != nil {
		t.Fatal(err)
	}
	p, err := e.ReadProject()
	if err != nil {
		t.Fatal(err)
	}
	for i, b := range want.Settings.Esc {
		if p.Settings.F4F1EscF5[i] != b {
			t.Errorf("f4_f1_esc_f5[%d] = %v, want %v", i, p.Settings.F4F1EscF5[i], b)
		}
	}
	if p.Settings.Author != want.Settings.Author {
		t.Errorf("author %q, want %q", p.Settings.Author, want.Settings.Author)
	}
	if len(p.Sounds) != len(want.Sounds) || len(p.Sprites) != len(want.Sprites) {
		t.Fatalf("%d sounds, %d sprites; want %d, %d", len(p.Sounds), len(p.Sprites), len(want.Sounds), len(want.Sprites))
	}
	for i, s := range want.Sprites {
		g := p.Sprites[i]
		if (s == nil) != (g == nil) {
			t.Fatalf("sprite %d present %v, want %v", i, g != nil, s != nil)
		}
		if s == nil {
			continue
		}
		if g.Name != s.Name || len(g.Frames) != len(s.Frames) || g.BBox[0] != s.BBox[0] || g.BBox[3] != s.BBox[3] {
			t.Errorf("sprite %d: %s %d frames %v; want %s %d frames %v", i, g.Name, len(g.Frames), g.BBox, s.Name, len(s.Frames), s.BBox)
		}
	}
	for i, s := range want.Sounds {
		if (s == nil) != (p.Sounds[i] == nil) || s != nil && p.Sounds[i].Name != s.Name {
			t.Errorf("sound %d differs", i)
		}
	}
	t.Logf("%d sounds, %d sprites", len(p.Sounds), len(p.Sprites))
}
