package runner

import "testing"

// string_width_ext / string_height_ext measure as draw_text_ext wraps;
// url_open hands the address to the backend; window_get_width / height are
// the backend's window, or the room's size without one.
func TestTextExtURLAndWindow(t *testing.T) {
	old := MeasureText
	defer func() { MeasureText = old }()
	MeasureText = func(s string, f Font) (float64, float64) { return float64(len(s)) * 10, 12 }
	o := &Object{Name: "o"}
	var opened []string
	g := &Game{Objects: []*Object{o}, OpenURL: func(u, target string) { opened = append(opened, u+"|"+target) }}
	h, err := g.Code(`hx = string_height_ext("one two three four", -1, 90); wx = string_width_ext("one two three four", 20, 90)
url_open("../f/card.html"); url_open(""); url_open_ext("/", "_self")
ww = window_get_width(); wh = window_get_height()`)
	if err != nil {
		t.Fatal(err)
	}
	o.Events.Step = h
	r := &Room{W: 64, H: 48, G: &Globals{Vars: map[string]float64{}}}
	in := r.Create(o, 0, 0)
	r.Step(keyKeys{})
	if len(g.Errors) > 0 {
		t.Fatal(g.Errors)
	}
	// "one two" / "three" / "four": three lines of 12, the widest 70.
	if in.Vars["hx"] != 36 || in.Vars["wx"] != 70 {
		t.Errorf("height_ext %v width_ext %v", in.Vars["hx"], in.Vars["wx"])
	}
	if len(opened) != 2 || opened[0] != "../f/card.html|" || opened[1] != "/|_self" {
		t.Errorf("opened %q", opened)
	}
	if in.Vars["ww"] != 64 || in.Vars["wh"] != 48 {
		t.Errorf("window without a backend: %v × %v, want the room's 64 × 48", in.Vars["ww"], in.Vars["wh"])
	}
	g.WindowSize = func() (int, int) { return 390, 844 }
	r.Step(keyKeys{})
	if in.Vars["ww"] != 390 || in.Vars["wh"] != 844 {
		t.Errorf("window %v × %v, want 390 × 844", in.Vars["ww"], in.Vars["wh"])
	}
}
