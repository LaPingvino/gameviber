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
	var rest struct {
		Objects []*struct {
			Name   string `json:"name"`
			Events []struct {
				Type    string `json:"type"`
				Actions []struct {
					Function string   `json:"function"`
					Args     []string `json:"args"`
				} `json:"actions"`
			} `json:"events"`
		} `json:"objects"`
		Rooms []*struct {
			Name      string `json:"name"`
			Instances []any  `json:"instances"`
		} `json:"rooms"`
		Scripts []*struct {
			Name string `json:"name"`
		} `json:"scripts"`
		Timelines []*struct {
			Name string `json:"name"`
		} `json:"timelines"`
		RoomOrder []int32 `json:"room_order"`
	}
	json.Unmarshal(js, &rest)
	if len(p.Objects) != len(rest.Objects) || len(p.Rooms) != len(rest.Rooms) || len(p.Scripts) != len(rest.Scripts) || len(p.Timelines) != len(rest.Timelines) {
		t.Fatalf("objects %d/%d rooms %d/%d scripts %d/%d timelines %d/%d", len(p.Objects), len(rest.Objects), len(p.Rooms), len(rest.Rooms), len(p.Scripts), len(rest.Scripts), len(p.Timelines), len(rest.Timelines))
	}
	actions := 0
	for i, o := range rest.Objects {
		g := p.Objects[i]
		if (o == nil) != (g == nil) || o == nil {
			continue
		}
		if g.Name != o.Name || len(g.Events) != len(o.Events) {
			t.Errorf("object %d: %s %d events; want %s %d", i, g.Name, len(g.Events), o.Name, len(o.Events))
			continue
		}
		for j, ev := range o.Events {
			if g.Events[j].Type != ev.Type || len(g.Events[j].Actions) != len(ev.Actions) {
				t.Errorf("%s event %d: %s %d actions; want %s %d", o.Name, j, g.Events[j].Type, len(g.Events[j].Actions), ev.Type, len(ev.Actions))
				continue
			}
			for k, a := range ev.Actions {
				actions++
				if g.Events[j].Actions[k].Function != a.Function || len(g.Events[j].Actions[k].Args) != len(a.Args) {
					t.Errorf("%s event %d action %d: %s, want %s", o.Name, j, k, g.Events[j].Actions[k].Function, a.Function)
				}
			}
		}
	}
	for i, rm := range rest.Rooms {
		if rm != nil && (p.Rooms[i].Name != rm.Name || len(p.Rooms[i].Instances) != len(rm.Instances)) {
			t.Errorf("room %d: %s %d instances; want %s %d", i, p.Rooms[i].Name, len(p.Rooms[i].Instances), rm.Name, len(rm.Instances))
		}
	}
	if len(p.RoomOrder) != len(rest.RoomOrder) {
		t.Errorf("room order %v, want %v", p.RoomOrder, rest.RoomOrder)
	}
	t.Logf("%d objects (%d actions), %d rooms, %d scripts, %d timelines", len(p.Objects), actions, len(p.Rooms), len(p.Scripts), len(p.Timelines))
}
