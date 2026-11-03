package runner

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/LaPingvino/gameviber/gml"
)

// Drawer is what draw events draw with (the renderer's): colours are
// Windows 0xBBGGRR; font is GM 5's font string (name, size, colour, bold…).
type Drawer interface {
	Rect(x1, y1, x2, y2 float64, fill, outline int32, filled bool)
	Sprite(name string, frame, x, y float64)
	Background(name string, x, y float64)
	Text(x, y float64, s string, font Font)
}

// Font is the draw font: name, size (points), colour, bold.
type Font struct {
	Name   string
	Size   float64
	Colour int32
	Bold   bool
}

// drawState is GM 5's drawing state, kept across events like GM's: the
// font's name / size / bold here; brush_color, pen_color and font_color are
// game-wide variables code sets too (they live in the globals).
type drawState struct {
	font Font
}

func (g *Game) colour(name string) int32 { return int32(g.room.G.Vars[name]) }

func (g *Game) font() Font {
	f := g.draw.font
	if f.Name == "" {
		f = Font{Name: "Arial", Size: 10}
	}
	if c, ok := g.room.G.Vars["font_color"]; ok {
		f.Colour = int32(c)
	}
	return f
}

// parseFont reads GM 5's font string: "name",size,colour,bold,italic,…
func parseFont(s string) Font {
	f := Font{Name: "Arial", Size: 10}
	parts := strings.Split(s, ",")
	if len(parts) > 0 {
		f.Name = strings.Trim(parts[0], "\"'")
	}
	num := func(i int) float64 {
		if i < len(parts) {
			v, _ := strconv.ParseFloat(strings.TrimSpace(parts[i]), 64)
			return v
		}
		return 0
	}
	if v := num(1); v > 0 {
		f.Size = v
	}
	f.Colour = int32(num(2))
	f.Bold = num(3) != 0
	return f
}

func (g *Game) drawFuncs() map[string]gml.Func {
	num := func(a []gml.Value, n int) float64 {
		if n < len(a) {
			return a[n].N
		}
		return 0
	}
	ok := gml.RealV(0)
	at := func(a []gml.Value, i, relArg int) (float64, float64) { // x, y (relative to self when the box is on)
		x, y := num(a, i), num(a, i+1)
		if rel(a, relArg) {
			x, y = x+g.self().X, y+g.self().Y
		}
		return x, y
	}
	d := func() Drawer { return g.Drawer }
	return map[string]gml.Func{
		"action_color": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (brush[, pen])
			g.room.G.Vars["brush_color"] = num(a, 0)
			g.room.G.Vars["pen_color"] = num(a, 0)
			if len(a) > 1 {
				g.room.G.Vars["pen_color"] = num(a, 1)
			}
			return ok, nil
		},
		"action_draw_font": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			if len(a) > 0 {
				g.draw.font = parseFont(a[0].String())
				g.room.G.Vars["font_color"] = float64(g.draw.font.Colour)
			}
			return ok, nil
		},
		"action_draw_rectangle": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (x1, y1, x2, y2, rel)
			if d() == nil {
				return ok, nil
			}
			x1, y1 := at(a, 0, 4)
			x2, y2 := at(a, 2, 4)
			d().Rect(x1, y1, x2, y2, g.colour("brush_color"), g.colour("pen_color"), true)
			return ok, nil
		},
		"action_draw_text": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (text, x, y, rel)
			if d() == nil || len(a) == 0 {
				return ok, nil
			}
			x, y := at(a, 1, 3)
			d().Text(x, y, a[0].String(), g.font())
			return ok, nil
		},
		"action_draw_score": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (x, y, caption, rel)
			if d() == nil {
				return ok, nil
			}
			x, y := at(a, 0, 3)
			caption := ""
			if len(a) > 2 {
				caption = a[2].String()
			}
			d().Text(x, y, caption+strconv.Itoa(g.room.G.Score), g.font())
			return ok, nil
		},
		"action_draw_life": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (x, y, caption, rel)
			if d() == nil {
				return ok, nil
			}
			x, y := at(a, 0, 3)
			caption := ""
			if len(a) > 2 {
				caption = a[2].String()
			}
			d().Text(x, y, caption+strconv.Itoa(g.room.G.Lives), g.font())
			return ok, nil
		},
		"action_draw_life_images": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (x, y, sprite, rel)
			if d() == nil || len(a) < 3 {
				return ok, nil
			}
			x, y := at(a, 0, 3)
			w := float64(g.SpriteWidth(a[2].String()))
			for i := range max(g.room.G.Lives, 0) {
				d().Sprite(g.IDs.SpriteName(a[2]), 0, x+float64(i)*w, y)
			}
			return ok, nil
		},
		"action_draw_sprite": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (sprite, x, y, subimage −1 = the instance's, rel)
			if d() == nil || len(a) < 3 {
				return ok, nil
			}
			x, y := at(a, 1, 4)
			frame := num(a, 3)
			if frame == -1 {
				frame = g.self().ImageIndex
			}
			d().Sprite(g.IDs.SpriteName(a[0]), frame, x, y)
			return ok, nil
		},
		"draw_text": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (x, y, string)
			if d() != nil && len(a) > 2 {
				d().Text(num(a, 0), num(a, 1), a[2].String(), g.font())
			}
			return ok, nil
		},
		"draw_rectangle": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (x1, y1, x2, y2): GM 5 fills with the brush, outlines with the pen
			if d() != nil {
				d().Rect(num(a, 0), num(a, 1), num(a, 2), num(a, 3), g.colour("brush_color"), g.colour("pen_color"), true)
			}
			return ok, nil
		},
		"draw_sprite": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (sprite, subimage, x, y)
			if d() != nil && len(a) > 3 {
				d().Sprite(g.IDs.SpriteName(a[0]), num(a, 1), num(a, 2), num(a, 3))
			}
			return ok, nil
		},
		"draw_background": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (background, x, y)
			if d() != nil && len(a) > 2 {
				d().Background(a[0].String(), num(a, 1), num(a, 2))
			}
			return ok, nil
		},
	}
}

// RunDraw runs an instance's draw event with the drawer (the renderer calls
// it for instances whose object has one).
func (g *Game) RunDraw(r *Room, in *Instance, d Drawer) {
	h := in.Obj.handler(func(e *Events) Handler { return e.Draw })
	if h == nil {
		return
	}
	g.Drawer = d
	defer func() { g.Drawer = nil }()
	h(r, in)
}

// HasDraw reports whether the instance's object draws itself.
func HasDraw(in *Instance) bool {
	return in.Obj.handler(func(e *Events) Handler { return e.Draw }) != nil
}

var _ = fmt.Sprint
