package runner

import (
	"testing"

	"github.com/LaPingvino/gameviber/gml"
)

// GML run on a runner instance: built-in variables reach the fields, the
// rest the instance's own variables; globals reach score, lives, variables.
func TestBinding(t *testing.T) {
	obj := &Object{Name: "pacman", Sprite: "pac_left"}
	self := NewInstance(obj, 32, 64)
	g := &Globals{Lives: 3, Vars: map[string]float64{}}
	list, err := gml.Parse(`
hspeed = 4; vspeed = -3; teksto = 'hi'; arr[2] = x + y
alarm[0] = 160; image_single = floor(2.7)
global.murigxu = 2; score += 10; lives -= 3
a = speed; b = sprite_index`)
	if err != nil {
		t.Fatal(err)
	}
	in := &gml.Interp{Self: self, Global: &GlobalVars{G: g}}
	if _, err := in.Run(list); err != nil {
		t.Fatal(err)
	}
	if self.HSpeed != 4 || self.VSpeed != -3 || self.VarsStr["teksto"] != "hi" || self.Vars["arr[2]"] != 96 {
		t.Errorf("instance: %+v %v %v", self, self.Vars, self.VarsStr)
	}
	if self.Alarm[0] != 160 || self.ImageIndex != 2 || self.Vars["a"] != 5 || self.VarsStr["b"] != "pac_left" {
		t.Errorf("alarm %d, image %v, speed %v, sprite %q", self.Alarm[0], self.ImageIndex, self.Vars["a"], self.VarsStr["b"])
	}
	if g.Vars["murigxu"] != 2 || g.Score != 10 || g.Lives != 0 || !g.NoMoreLives {
		t.Errorf("globals: %+v", g)
	}
}

// solid is per instance: code changes one wall, not every wall.
func TestSolidPerInstance(t *testing.T) {
	wall := &Object{Name: "murox", Solid: true, BBox: Box{0, 31, 0, 31}, Mask: true}
	ghost := &Object{Name: "monster", BBox: Box{0, 31, 0, 31}, Mask: true}
	g := &Game{Objects: []*Object{wall, ghost}}
	code, err := g.Code("with (murox) solid = false; a = place_free(32, 0); with (murox) solid = true; b = place_free(32, 0)")
	if err != nil {
		t.Fatal(err)
	}
	r := &Room{W: 640, H: 480, G: &Globals{Vars: map[string]float64{}}}
	w1 := r.Create(wall, 32, 0)
	m := r.Create(ghost, 0, 0)
	code(r, m)
	if len(g.Errors) > 0 {
		t.Fatal(g.Errors)
	}
	if m.Vars["a"] != 1 || m.Vars["b"] != 0 || !w1.Solid || !wall.Solid {
		t.Errorf("free while not solid %v, after %v; wall solid %v, object %v", m.Vars["a"], m.Vars["b"], w1.Solid, wall.Solid)
	}
}

// depth starts as the object's and code can change it.
func TestDepthVariable(t *testing.T) {
	o := &Object{Name: "o", Depth: 5}
	g := &Game{Objects: []*Object{o}}
	h, err := g.Code("was = depth; depth = -2")
	if err != nil {
		t.Fatal(err)
	}
	o.Events.Create = h
	r := &Room{W: 64, H: 64, G: &Globals{Vars: map[string]float64{}}}
	in := r.Create(o, 0, 0)
	if len(g.Errors) > 0 {
		t.Fatal(g.Errors)
	}
	if in.Vars["was"] != 5 || in.Depth != -2 {
		t.Errorf("was %v, depth %v", in.Vars["was"], in.Depth)
	}
}

// Sprites as ids: a sprite's name is its id, sprite_index reads it and takes
// an id (or a name), object_index is the object's id.
func TestSpriteAndObjectIDs(t *testing.T) {
	ids := &IDs{Sprites: map[string]int{"spr_a": 0, "spr_b": 1}}
	wall := &Object{Name: "wall"}
	o := &Object{Name: "o", Sprite: "spr_a"}
	g := &Game{Objects: []*Object{wall, o}, IDs: ids}
	h, err := g.Code(`was = sprite_index; isA = (sprite_index = spr_a); sprite_index = spr_b; now = sprite_index
me = object_index; mine = (object_index = o); sprite_index = "spr_a"`)
	if err != nil {
		t.Fatal(err)
	}
	o.Events.Create = h
	r := &Room{W: 64, H: 64, G: &Globals{Vars: map[string]float64{}}}
	in := r.Create(o, 0, 0)
	if len(g.Errors) > 0 {
		t.Fatal(g.Errors)
	}
	v := in.Vars
	if v["was"] != 0 || v["isA"] != 1 || v["now"] != 1 || v["me"] != 1 || v["mine"] != 1 || in.Sprite != "spr_a" {
		t.Errorf("%v, sprite %q", v, in.Sprite)
	}
}

type playedSounds []string

func (p *playedSounds) Play(name string, loop bool) { *p = append(*p, name) }
func (p *playedSounds) Stop(string)                 {}
func (p *playedSounds) StopAll()                    {}
func (p *playedSounds) Playing(string) bool         { return false }

// Sounds, backgrounds and timelines as ids: names evaluate to ids, and what
// takes one takes an id (or a name).
func TestSoundBackgroundTimelineIDs(t *testing.T) {
	ids := &IDs{Sprites: map[string]int{}, Of: map[string]map[string]int{
		"sound": {"snd_coin": 0, "snd_win": 1}, "background": {"bg_sky": 0, "bg_sea": 1}, "timeline": {"tl_intro": 0}}}
	o := &Object{Name: "o"}
	var played playedSounds
	g := &Game{Objects: []*Object{o}, IDs: ids, Sound: &played,
		Timelines: map[string]*Timeline{"tl_intro": {Name: "tl_intro", Moments: map[int]Handler{}}}}
	h, err := g.Code(`sound_play(snd_win); s = snd_win
was = background_index[0]; background_index[0] = bg_sea; now = background_index[0]
action_set_timeline(tl_intro, 0); tl = timeline_index`)
	if err != nil {
		t.Fatal(err)
	}
	o.Events.Create = h
	r := &Room{W: 64, H: 64, G: &Globals{Vars: map[string]float64{}}, Backgrounds: []Background{{Name: "bg_sky", Visible: true}}}
	in := r.Create(o, 0, 0)
	if len(g.Errors) > 0 {
		t.Fatal(g.Errors)
	}
	v := in.Vars
	if len(played) != 1 || played[0] != "snd_win" || v["s"] != 1 || v["was"] != 0 || v["now"] != 1 || r.Backgrounds[0].Name != "bg_sea" || v["tl"] != 0 || in.Timeline == nil {
		t.Errorf("played %v, vars %v, background %q", played, v, r.Backgrounds[0].Name)
	}
}
