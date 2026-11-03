// Package render draws a running room into an image: headless (plain Go
// images), the reference the screen backend is checked against.
package render

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"sort"

	"github.com/LaPingvino/gameviber/import/gmfile"
	"github.com/LaPingvino/gameviber/runner"
)

// Assets are the decoded images of a project: sprite frames and backgrounds
// by name, with the sprites' origins.
type Assets struct {
	Frames  map[string][]*image.NRGBA
	Origins map[string]image.Point
	BGs     map[string]*image.NRGBA
}

// LoadAssets decodes a project's sprites and backgrounds.
func LoadAssets(p *gmfile.Project) *Assets {
	a := &Assets{Frames: map[string][]*image.NRGBA{}, Origins: map[string]image.Point{}, BGs: map[string]*image.NRGBA{}}
	for _, s := range p.Sprites {
		if s == nil {
			continue
		}
		for _, f := range s.Frames {
			if img, err := gmfile.DecodeBMP(f, s.Transparent); err == nil {
				a.Frames[s.Name] = append(a.Frames[s.Name], img)
			}
		}
		a.Origins[s.Name] = image.Pt(int(s.OriginX), int(s.OriginY))
	}
	for _, b := range p.Backgrounds {
		if b != nil && b.Image != nil {
			if img, err := gmfile.DecodeBMP(b.Image, false); err == nil {
				a.BGs[b.Name] = img
			}
		}
	}
	return a
}

// Frame draws the room: the background colour (background_color when the
// game set it, else the room's), then the instances, deepest first, each its
// sprite's frame at floor(image_index).
// Background layers (behind, then foreground in front) tile and scroll.
// PARTIAL: views, stretched layers and draw events come next.
func Frame(dst *image.RGBA, g *runner.Game, r *runner.Room, a *Assets, roomColour int32, drawColour bool) []TextCall {
	if drawColour {
		c := roomColour
		if v, ok := r.G.Vars["background_color"]; ok {
			c = int32(v)
		}
		u := uint32(c)
		draw.Draw(dst, dst.Bounds(), image.NewUniform(color.RGBA{uint8(u), uint8(u >> 8), uint8(u >> 16), 255}), image.Point{}, draw.Src)
	}
	// Through view 0 when the room has views (PARTIAL: one view, at the
	// port's top-left of dst).
	ox, oy := 0.0, 0.0
	if r.ViewsEnabled && len(r.Views) > 0 && r.Views[0].Visible {
		ox, oy = float64(r.Views[0].X), float64(r.Views[0].Y)
	}
	layers := func(fore bool) {
		for _, b := range r.Backgrounds {
			img := a.BGs[b.Name]
			if !b.Visible || b.Foreground != fore || img == nil {
				continue
			}
			b.X, b.Y = b.X-ox, b.Y-oy
			drawLayer(dst, img, b)
		}
	}
	layers(false)
	defer layers(true)
	insts := make([]*runner.Instance, 0, len(r.Instances))
	for _, in := range r.Instances {
		if !in.Destroyed && in.Obj.Visible {
			insts = append(insts, in)
		}
	}
	sort.SliceStable(insts, func(i, j int) bool { return insts[i].Obj.Depth > insts[j].Obj.Depth })
	fd := &frameDrawer{dst: dst, a: a, ox: ox, oy: oy}
	for _, in := range insts {
		if g != nil && runner.HasDraw(in) {
			g.RunDraw(r, in, fd)
			continue
		}
		frames := a.Frames[in.Sprite]
		if len(frames) == 0 {
			continue
		}
		k := int(math.Floor(in.ImageIndex)) % len(frames)
		if k < 0 {
			k += len(frames)
		}
		f := frames[k]
		o := a.Origins[in.Sprite]
		at := image.Pt(int(math.Round(in.X-ox))-o.X, int(math.Round(in.Y-oy))-o.Y)
		draw.Draw(dst, f.Bounds().Add(at), f, image.Point{}, draw.Over)
	}
	return fd.texts
}

// TextCall is a text the draw events drew: where, what, with which font.
// PARTIAL: text is recorded, not rasterised — a font rasteriser needs a new
// module (golang.org/x/image), which waits for Joop's OK.
type TextCall struct {
	X, Y float64
	S    string
	Font runner.Font
}

// frameDrawer draws draw events into the frame.
type frameDrawer struct {
	dst    *image.RGBA
	a      *Assets
	ox, oy float64 // the view's position (room coordinates → the frame)
	texts  []TextCall
}

func bgr(c int32) color.RGBA {
	u := uint32(c)
	return color.RGBA{uint8(u), uint8(u >> 8), uint8(u >> 16), 255}
}

// Rect fills (brush) and outlines (pen) a rectangle, corners inclusive as
// Windows' Rectangle with GM's +1 (ASSUMPTION, as the hand ports have it).
func (d *frameDrawer) Rect(x1, y1, x2, y2 float64, fill, outline int32, filled bool) {
	x1, y1, x2, y2 = x1-d.ox, y1-d.oy, x2-d.ox, y2-d.oy
	r := image.Rect(int(math.Round(min(x1, x2))), int(math.Round(min(y1, y2))), int(math.Round(max(x1, x2)))+1, int(math.Round(max(y1, y2)))+1)
	if filled {
		draw.Draw(d.dst, r, image.NewUniform(bgr(fill)), image.Point{}, draw.Src)
	}
	o := image.NewUniform(bgr(outline))
	for _, e := range []image.Rectangle{{r.Min, image.Pt(r.Max.X, r.Min.Y+1)}, {image.Pt(r.Min.X, r.Max.Y-1), r.Max},
		{r.Min, image.Pt(r.Min.X+1, r.Max.Y)}, {image.Pt(r.Max.X-1, r.Min.Y), r.Max}} {
		draw.Draw(d.dst, e, o, image.Point{}, draw.Src)
	}
}

func (d *frameDrawer) Sprite(name string, frame, x, y float64) {
	x, y = x-d.ox, y-d.oy
	frames := d.a.Frames[name]
	if len(frames) == 0 {
		return
	}
	k := int(math.Floor(frame)) % len(frames)
	if k < 0 {
		k += len(frames)
	}
	o := d.a.Origins[name]
	at := image.Pt(int(math.Round(x))-o.X, int(math.Round(y))-o.Y)
	draw.Draw(d.dst, frames[k].Bounds().Add(at), frames[k], image.Point{}, draw.Over)
}

func (d *frameDrawer) Background(name string, x, y float64) {
	x, y = x-d.ox, y-d.oy
	if img := d.a.BGs[name]; img != nil {
		draw.Draw(d.dst, img.Bounds().Add(image.Pt(int(x), int(y))), img, image.Point{}, draw.Over)
	}
}

func (d *frameDrawer) Text(x, y float64, s string, f runner.Font) {
	d.texts = append(d.texts, TextCall{x - d.ox, y - d.oy, s, f})
}

// drawLayer draws a background layer at its position, tiled as set.
func drawLayer(dst *image.RGBA, img *image.NRGBA, b runner.Background) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	x0, y0 := int(math.Floor(b.X)), int(math.Floor(b.Y))
	xs, ys := []int{x0}, []int{y0}
	if b.TileH && w > 0 {
		xs = xs[:0]
		for x := ((x0%w)+w)%w - w; x < dst.Bounds().Dx(); x += w {
			xs = append(xs, x)
		}
	}
	if b.TileV && h > 0 {
		ys = ys[:0]
		for y := ((y0%h)+h)%h - h; y < dst.Bounds().Dy(); y += h {
			ys = append(ys, y)
		}
	}
	for _, y := range ys {
		for _, x := range xs {
			draw.Draw(dst, img.Bounds().Add(image.Pt(x, y)), img, image.Point{}, draw.Over)
		}
	}
}
