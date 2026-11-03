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
