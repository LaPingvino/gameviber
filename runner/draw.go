package runner

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/LaPingvino/gameviber/gml"
)

// Drawer is what draw events draw with (the renderer's): colours are
// Windows 0xBBGGRR; font is GM 5's font string (name, size, colour, bold…).
type Drawer interface {
	Rect(x1, y1, x2, y2 float64, fill, outline int32, filled bool)
	// Ellipse fills (brush) and outlines (pen) the ellipse in the box, as Rect.
	Ellipse(x1, y1, x2, y2 float64, fill, outline int32, filled bool)
	// Line draws a one-pixel line in the pen colour; Point sets one pixel.
	Line(x1, y1, x2, y2 float64, colour int32)
	Point(x, y float64, colour int32)
	Sprite(name string, frame, x, y float64)
	Background(name string, x, y float64)
	// SpriteStretched fills the box (x, y, w, h) with the frame (its origin
	// ignored); SpriteTiled repeats it over the whole drawing area with one
	// copy at (x, y) (origin applied). The background ones likewise.
	SpriteStretched(name string, frame, x, y, w, h float64)
	SpriteTiled(name string, frame, x, y float64)
	BackgroundStretched(name string, x, y, w, h float64)
	BackgroundTiled(name string, x, y float64)
	Text(x, y float64, s string, font Font)
}

// MeasureText is how big a text is in a font (string_width /
// string_height, draw_text_ext's wrapping): the renderer sets it (render's
// init); nil leaves the sizes 0.
var MeasureText func(s string, f Font) (w, h float64)

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
		"string_width": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (string): in the current font, the widest line
			w, _ := g.measure(str(a, 0))
			return gml.RealV(w), nil
		},
		"string_height": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (string): in the current font, all lines
			_, h := g.measure(str(a, 0))
			return gml.RealV(h), nil
		},
		"draw_text_ext": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (x, y, string, sep, w): wrapped at w (−1: no wrap), lines sep apart (−1: the font's)
			if d() == nil {
				return ok, nil
			}
			x, y, sep, w := num(a, 0), num(a, 1), num(a, 3), num(a, 4)
			f := g.font()
			if sep < 0 {
				_, sep = g.measure("M")
			}
			for i, line := range g.wrap(str(a, 2), w) {
				d().Text(x, y+float64(i)*sep, line, f)
			}
			return ok, nil
		},
		"draw_rectangle": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (x1, y1, x2, y2): GM 5 fills with the brush, outlines with the pen
			if d() != nil {
				d().Rect(num(a, 0), num(a, 1), num(a, 2), num(a, 3), g.colour("brush_color"), g.colour("pen_color"), true)
			}
			return ok, nil
		},
		// GM 5's other shapes: the brush fills, the pen outlines (lines and
		// points: the pen).
		"draw_ellipse": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (x1, y1, x2, y2)
			if d() != nil {
				d().Ellipse(num(a, 0), num(a, 1), num(a, 2), num(a, 3), g.colour("brush_color"), g.colour("pen_color"), true)
			}
			return ok, nil
		},
		"draw_circle": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (x, y, r)
			if d() != nil {
				x, y, r := num(a, 0), num(a, 1), num(a, 2)
				d().Ellipse(x-r, y-r, x+r, y+r, g.colour("brush_color"), g.colour("pen_color"), true)
			}
			return ok, nil
		},
		"draw_line": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (x1, y1, x2, y2)
			if d() != nil {
				d().Line(num(a, 0), num(a, 1), num(a, 2), num(a, 3), g.colour("pen_color"))
			}
			return ok, nil
		},
		"draw_triangle": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (x1, y1, x2, y2, x3, y3): brush inside, pen edges
			if d() != nil {
				pts := [][2]float64{{num(a, 0), num(a, 1)}, {num(a, 2), num(a, 3)}, {num(a, 4), num(a, 5)}}
				fillPolygon(d(), pts, g.colour("brush_color"))
				for i := range pts {
					p, q := pts[i], pts[(i+1)%3]
					d().Line(p[0], p[1], q[0], q[1], g.colour("pen_color"))
				}
			}
			return ok, nil
		},
		"draw_roundrect": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (x1, y1, x2, y2): a rectangle with rounded corners
			if d() != nil {
				drawRoundRect(d(), num(a, 0), num(a, 1), num(a, 2), num(a, 3), g.colour("brush_color"), g.colour("pen_color"))
			}
			return ok, nil
		},
		"draw_arrow": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (x1, y1, x2, y2, size): a line with a head at (x2, y2)
			if d() != nil {
				x1, y1, x2, y2, size := num(a, 0), num(a, 1), num(a, 2), num(a, 3), num(a, 4)
				pen := g.colour("pen_color")
				d().Line(x1, y1, x2, y2, pen)
				if l := math.Hypot(x2-x1, y2-y1); l > 0 {
					ux, uy := (x2-x1)/l, (y2-y1)/l
					size = min(size, l)
					for _, sgn := range []float64{1, -1} { // the head's two sides, 30° off the line
						c, s := math.Cos(math.Pi/6), sgn*math.Sin(math.Pi/6)
						hx, hy := ux*c-uy*s, ux*s+uy*c
						d().Line(x2, y2, x2-hx*size, y2-hy*size, pen)
					}
				}
			}
			return ok, nil
		},
		"draw_point": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (x, y)
			if d() != nil {
				d().Point(num(a, 0), num(a, 1), g.colour("pen_color"))
			}
			return ok, nil
		},
		"draw_sprite": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (sprite, subimage, x, y)
			if d() != nil && len(a) > 3 {
				d().Sprite(g.IDs.SpriteName(a[0]), num(a, 1), num(a, 2), num(a, 3))
			}
			return ok, nil
		},
		"draw_sprite_stretched": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (sprite, subimg, x, y, w, h): filling the box
			if d() != nil && len(a) > 0 {
				d().SpriteStretched(g.IDs.SpriteName(a[0]), g.frameArg(a, 1), num(a, 2), num(a, 3), num(a, 4), num(a, 5))
			}
			return ok, nil
		},
		"draw_sprite_tiled": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (sprite, subimg, x, y): over the whole room, one copy at (x, y)
			if d() != nil && len(a) > 0 {
				d().SpriteTiled(g.IDs.SpriteName(a[0]), g.frameArg(a, 1), num(a, 2), num(a, 3))
			}
			return ok, nil
		},
		"draw_background_stretched": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (back, x, y, w, h)
			if d() != nil && len(a) > 0 {
				d().BackgroundStretched(g.IDs.Name("background", a[0]), num(a, 1), num(a, 2), num(a, 3), num(a, 4))
			}
			return ok, nil
		},
		"draw_background_tiled": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (back, x, y): over the whole room
			if d() != nil && len(a) > 0 {
				d().BackgroundTiled(g.IDs.Name("background", a[0]), num(a, 1), num(a, 2))
			}
			return ok, nil
		},
		"draw_background": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (background, x, y)
			if d() != nil && len(a) > 2 {
				d().Background(g.IDs.Name("background", a[0]), num(a, 1), num(a, 2))
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

// fillPolygon fills a convex polygon row by row with horizontal lines (each
// pixel row whose centre lies inside).
func fillPolygon(d Drawer, pts [][2]float64, colour int32) {
	top, bottom := math.Inf(1), math.Inf(-1)
	for _, p := range pts {
		top, bottom = min(top, p[1]), max(bottom, p[1])
	}
	for y := math.Ceil(top - 0.5); y+0.5 <= bottom; y++ {
		cy := y + 0.5
		l, r := math.Inf(1), math.Inf(-1)
		for i := range pts {
			p, q := pts[i], pts[(i+1)%len(pts)]
			if (p[1] <= cy) == (q[1] <= cy) {
				continue
			}
			x := p[0] + (cy-p[1])*(q[0]-p[0])/(q[1]-p[1])
			l, r = min(l, x), max(r, x)
		}
		if l <= r {
			d.Line(math.Round(l), y, math.Round(r)-1, y, colour)
		}
	}
}

// drawRoundRect is GM 5's draw_roundrect: the brush inside, the pen around,
// corners rounded with a radius of a quarter of the shorter side, at most 8.
// ASSUMPTION [engine]: GM 5 used Windows' RoundRect; its corner size is not
// documented, this one looks alike.
func drawRoundRect(d Drawer, x1, y1, x2, y2 float64, fill, pen int32) {
	l, r, t, b := math.Round(min(x1, x2)), math.Round(max(x1, x2)), math.Round(min(y1, y2)), math.Round(max(y1, y2))
	rad := math.Min(8, math.Floor(math.Min(r-l, b-t)/4))
	inset := func(y float64) float64 { // how far the row's ends are pulled in by the corners
		dy := 0.0
		if y < t+rad {
			dy = t + rad - y
		} else if y > b-rad {
			dy = y - (b - rad)
		}
		if dy <= 0 || rad <= 0 {
			return 0
		}
		return math.Round(rad - math.Sqrt(max(0, rad*rad-dy*dy)))
	}
	for y := t; y <= b; y++ {
		k := inset(y)
		if y == t || y == b {
			d.Line(l+k, y, r-k, y, pen)
			continue
		}
		d.Line(l+k+1, y, r-k-1, y, fill)
		d.Point(l+k, y, pen)
		d.Point(r-k, y, pen)
		if k2 := inset(y - 1); y > t && k2 > k+1 { // close the corner's steps
			d.Line(l+k, y, l+k2-1, y, pen)
			d.Line(r-k2+1, y, r-k, y, pen)
		}
		if k2 := inset(y + 1); y < b && k2 > k+1 {
			d.Line(l+k, y, l+k2-1, y, pen)
			d.Line(r-k2+1, y, r-k, y, pen)
		}
	}
}

// str is argument i as text ("" when missing).
func str(a []gml.Value, i int) string {
	if i < len(a) {
		return a[i].String()
	}
	return ""
}

// measure is MeasureText in the current draw font.
func (g *Game) measure(s string) (float64, float64) {
	if MeasureText == nil {
		return 0, 0
	}
	return MeasureText(s, g.font())
}

// wrap splits a text into lines: at '#' (GM 5's newline; \# is a real #)
// and newlines, then words onto a line while it stays within w (w < 0: no
// wrapping; a word longer than w gets a line of its own).
func (g *Game) wrap(s string, w float64) []string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\\#", "\x00"), "#", "\n")
	var out []string
	for _, para := range strings.Split(s, "\n") {
		para = strings.ReplaceAll(para, "\x00", "\\#")
		if w < 0 {
			out = append(out, para)
			continue
		}
		line := ""
		for _, word := range strings.Fields(para) {
			try := word
			if line != "" {
				try = line + " " + word
			}
			if tw, _ := g.measure(try); tw > w && line != "" {
				out = append(out, line)
				line = word
				continue
			}
			line = try
		}
		out = append(out, line)
	}
	return out
}

// frameArg is a subimage argument: −1 means the instance's image_index.
func (g *Game) frameArg(a []gml.Value, i int) float64 {
	f := 0.0
	if i < len(a) {
		f = a[i].N
	}
	if f == -1 {
		f = g.self().ImageIndex
	}
	return f
}
